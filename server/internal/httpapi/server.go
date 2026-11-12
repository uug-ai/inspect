// Package httpapi exposes the inspect HTTP API.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/uug-ai/inspect/server/internal/config"
	"github.com/uug-ai/inspect/server/internal/issue"
	"github.com/uug-ai/inspect/server/internal/store"
)

const (
	// MaxAttachments is the maximum number of "attachment" parts.
	MaxAttachments = 5
	maxPayloadSize = 256 << 10
	// multipartOverhead is allowed on top of the attachment budget for the
	// payload field and multipart framing.
	multipartOverhead = 1 << 20
)

// Submitter is implemented by *issue.Service.
type Submitter interface {
	Submit(ctx context.Context, p *issue.Payload, files []issue.File) (*issue.Result, error)
}

// Server holds the HTTP handlers.
type Server struct {
	Cfg      *config.Config
	Issues   Submitter
	Local    *store.Local    // non-nil: serve /assets/
	Previews *issue.Previews // non-nil: serve /preview/{id}
	Limiter  *RateLimiter
	Log      *slog.Logger
	// Site registers the landing page routes (GET / and GET /static/); optional.
	Site func(*http.ServeMux)
}

// Handler returns the routed handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("OPTIONS /v1/issues", s.preflight)
	mux.HandleFunc("POST /v1/issues", s.createIssue)
	if s.Local != nil {
		mux.Handle("GET "+store.AssetsPrefix, s.Local.Handler())
	}
	if s.Previews != nil {
		mux.HandleFunc("GET /preview/{id}", s.preview)
	}
	if s.Site != nil {
		s.Site(mux)
	}
	return mux
}

// cors sets CORS response headers and reports whether the request may
// proceed. Requests without an Origin header (curl, server-to-server) are
// not browser cross-origin requests and pass. A browser request from an
// origin that is not allowed is rejected server-side too: multipart POSTs
// are "simple" requests that browsers send without a preflight, so relying
// on the browser alone would let any website file issues.
func (s *Server) cors(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	w.Header().Add("Vary", "Origin")
	if origin == "" {
		return true
	}
	if !s.Cfg.OriginAllowed(origin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	return true
}

func (s *Server) preflight(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") == "" || !s.cors(w, r) {
		writeError(w, http.StatusForbidden, "origin not allowed")
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	h.Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authorized(r *http.Request) bool {
	if len(s.Cfg.APIKeys) == 0 {
		return true
	}
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return false
	}
	tok = strings.TrimSpace(tok)
	valid := 0
	for _, k := range s.Cfg.APIKeys {
		valid |= subtle.ConstantTimeCompare([]byte(tok), []byte(k))
	}
	return valid == 1
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	if !s.cors(w, r) {
		writeError(w, http.StatusForbidden, "origin not allowed")
		return
	}
	if ok, wait := s.Limiter.Allow(clientIP(r, s.Cfg.TrustProxy)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if !s.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="inspect"`)
		writeError(w, http.StatusUnauthorized, "missing or invalid API key")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes+multipartOverhead)
	payload, files, status, err := s.readForm(r)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}

	res, err := s.Issues.Submit(r.Context(), payload, files)
	if err != nil {
		var ve *issue.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, ve.Msg)
			return
		}
		s.Log.Error("submit failed", "err", err)
		writeError(w, http.StatusBadGateway, "failed to create issue")
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// readForm streams the multipart body, enforcing count and size limits and
// sniffing every attachment's real type.
func (s *Server) readForm(r *http.Request) (*issue.Payload, []issue.File, int, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, nil, http.StatusBadRequest, errors.New("expected multipart/form-data body")
	}

	var (
		payload *issue.Payload
		files   []issue.File
		total   int64
	)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, bodyErrStatus(err), bodyErr(err)
		}
		switch part.FormName() {
		case "payload":
			if payload != nil {
				return nil, nil, http.StatusBadRequest, errors.New("payload given more than once")
			}
			b, err := readLimited(part, maxPayloadSize)
			if err != nil {
				return nil, nil, bodyErrStatus(err), fmt.Errorf("payload: %w", bodyErr(err))
			}
			if payload, err = issue.ParsePayload(b); err != nil {
				return nil, nil, http.StatusBadRequest, err
			}
		case "attachment":
			if len(files) >= MaxAttachments {
				return nil, nil, http.StatusBadRequest, fmt.Errorf("too many attachments (max %d)", MaxAttachments)
			}
			b, err := readLimited(part, s.Cfg.MaxFileBytes)
			if err != nil {
				return nil, nil, bodyErrStatus(err), fmt.Errorf("attachment %d: %w", len(files)+1, bodyErr(err))
			}
			if len(b) == 0 {
				return nil, nil, http.StatusBadRequest, fmt.Errorf("attachment %d is empty", len(files)+1)
			}
			if total += int64(len(b)); total > s.Cfg.MaxUploadBytes {
				return nil, nil, http.StatusRequestEntityTooLarge,
					fmt.Errorf("attachments exceed %d MB in total", s.Cfg.MaxUploadBytes>>20)
			}
			ct, ext, ok := issue.SniffMedia(b)
			if !ok {
				return nil, nil, http.StatusUnsupportedMediaType,
					fmt.Errorf("attachment %d: unsupported type (allowed: png, jpeg, webp, webm, mp4)", len(files)+1)
			}
			files = append(files, issue.File{Data: b, ContentType: ct, Ext: ext})
		default:
			_, _ = io.Copy(io.Discard, part) // ignore unknown fields
		}
		part.Close()
	}
	if payload == nil {
		return nil, nil, http.StatusBadRequest, errors.New("missing payload field")
	}
	return payload, files, 0, nil
}

var errTooLarge = errors.New("too large")

func readLimited(p *multipart.Part, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(p, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("%w (max %d bytes)", errTooLarge, max)
	}
	return b, nil
}

func bodyErrStatus(err error) int {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) || errors.Is(err, errTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func bodyErr(err error) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return errors.New("request body too large")
	}
	if errors.Is(err, errTooLarge) {
		return err
	}
	return errors.New("malformed multipart body")
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	doc, ok := s.Previews.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "preview not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, doc)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
