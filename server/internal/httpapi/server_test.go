package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uug-ai/inspect/server/internal/config"
	"github.com/uug-ai/inspect/server/internal/issue"
)

var pngBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")

type fakeSubmitter struct {
	calls int
	files []issue.File
	err   error
}

func (f *fakeSubmitter) Submit(_ context.Context, _ *issue.Payload, files []issue.File) (*issue.Result, error) {
	f.calls++
	f.files = files
	if f.err != nil {
		return nil, f.err
	}
	return &issue.Result{Number: 7, URL: "https://github.com/o/r/issues/7"}, nil
}

func newServer(t *testing.T, mod func(c *config.Config)) (*Server, *fakeSubmitter) {
	t.Helper()
	cfg := &config.Config{
		DefaultRepo:    "o/r",
		AllowedRepos:   []string{"o/r"},
		AllowedOrigins: []string{"https://app.example.com"},
		MaxUploadBytes: 1 << 20,
		MaxFileBytes:   1 << 20,
		RateLimit:      100,
	}
	if mod != nil {
		mod(cfg)
	}
	sub := &fakeSubmitter{}
	return &Server{
		Cfg:     cfg,
		Issues:  sub,
		Limiter: NewRateLimiter(cfg.RateLimit),
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, sub
}

type part struct {
	field, filename, ctype string
	data                   []byte
}

func form(t *testing.T, parts ...part) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		var w io.Writer
		var err error
		if p.filename != "" {
			h := make(map[string][]string)
			h["Content-Disposition"] = []string{`form-data; name="` + p.field + `"; filename="` + p.filename + `"`}
			h["Content-Type"] = []string{p.ctype}
			w, err = mw.CreatePart(h)
		} else {
			w, err = mw.CreateFormField(p.field)
		}
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(p.data)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

var okPayload = part{field: "payload", data: []byte(`{"title":"Broken","kind":"screenshot","context":{"timestamp":"2026-11-12T10:00:00Z"}}`)}

func post(t *testing.T, s *Server, headers map[string]string, parts ...part) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := form(t, parts...)
	req := httptest.NewRequest(http.MethodPost, "/v1/issues", body)
	req.Header.Set("Content-Type", ct)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func errMsg(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct{ Error string }
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("error body not JSON: %q", rec.Body.String())
	}
	return e.Error
}

func TestCreateIssue(t *testing.T) {
	s, sub := newServer(t, nil)
	rec := post(t, s, nil, okPayload, part{"attachment", "a.png", "image/png", pngBytes})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res issue.Result
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Number != 7 || res.URL != "https://github.com/o/r/issues/7" {
		t.Errorf("result %+v", res)
	}
	if sub.calls != 1 || len(sub.files) != 1 || sub.files[0].ContentType != "image/png" {
		t.Errorf("submit not called correctly: %+v", sub)
	}
}

func TestCreateIssueErrors(t *testing.T) {
	png := part{"attachment", "a.png", "image/png", pngBytes}
	tests := []struct {
		name    string
		mod     func(*config.Config)
		parts   []part
		subErr  error
		status  int
		wantErr string
	}{
		{name: "missing payload", parts: []part{png}, status: 400, wantErr: "missing payload"},
		{name: "invalid payload", parts: []part{{field: "payload", data: []byte(`{"kind":"note"}`)}}, status: 400, wantErr: "title is required"},
		{name: "too many attachments", parts: []part{okPayload, png, png, png, png, png, png}, status: 400, wantErr: "too many attachments"},
		{name: "spoofed content type", parts: []part{okPayload, {"attachment", "x.png", "image/png", []byte("<html><script>alert(1)</script>")}}, status: 415, wantErr: "unsupported type"},
		{name: "empty attachment", parts: []part{okPayload, {"attachment", "x.png", "image/png", nil}}, status: 400, wantErr: "empty"},
		{
			name:   "file too large",
			mod:    func(c *config.Config) { c.MaxFileBytes = 50 },
			parts:  []part{okPayload, png},
			status: 413, wantErr: "too large",
		},
		{
			name:   "total too large",
			mod:    func(c *config.Config) { c.MaxUploadBytes = int64(len(pngBytes)) + 10 },
			parts:  []part{okPayload, png, png},
			status: 413, wantErr: "in total",
		},
		{name: "validation from service", parts: []part{okPayload}, subErr: &issue.ValidationError{Msg: `repo "x/y" is not allowed`}, status: 400, wantErr: "not allowed"},
		{name: "upstream failure hidden", parts: []part{okPayload}, subErr: errors.New("github: 500 secret detail"), status: 502, wantErr: "failed to create issue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, sub := newServer(t, tt.mod)
			sub.err = tt.subErr
			rec := post(t, s, nil, tt.parts...)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
			}
			if msg := errMsg(t, rec); !strings.Contains(msg, tt.wantErr) {
				t.Errorf("error %q, want %q", msg, tt.wantErr)
			}
		})
	}

	t.Run("not multipart", func(t *testing.T) {
		s, _ := newServer(t, nil)
		req := httptest.NewRequest(http.MethodPost, "/v1/issues", strings.NewReader(`{"title":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Errorf("status %d", rec.Code)
		}
	})
}

func TestCORS(t *testing.T) {
	tests := []struct {
		name      string
		origins   []string
		method    string
		origin    string
		status    int
		allowOrig string
	}{
		{"preflight allowed", []string{"https://app.example.com"}, http.MethodOptions, "https://app.example.com", 204, "https://app.example.com"},
		{"preflight denied", []string{"https://app.example.com"}, http.MethodOptions, "https://evil.example.com", 403, ""},
		{"preflight wildcard", []string{"*"}, http.MethodOptions, "https://any.example.com", 204, "https://any.example.com"},
		{"preflight no list", nil, http.MethodOptions, "https://app.example.com", 403, ""},
		{"post allowed", []string{"https://app.example.com"}, http.MethodPost, "https://app.example.com", 201, "https://app.example.com"},
		{"post denied (simple request without preflight)", []string{"https://app.example.com"}, http.MethodPost, "https://evil.example.com", 403, ""},
		{"post without origin (curl)", []string{"https://app.example.com"}, http.MethodPost, "", 201, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newServer(t, func(c *config.Config) { c.AllowedOrigins = tt.origins })
			var rec *httptest.ResponseRecorder
			if tt.method == http.MethodOptions {
				req := httptest.NewRequest(http.MethodOptions, "/v1/issues", nil)
				req.Header.Set("Origin", tt.origin)
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "authorization")
				rec = httptest.NewRecorder()
				s.Handler().ServeHTTP(rec, req)
			} else {
				h := map[string]string{}
				if tt.origin != "" {
					h["Origin"] = tt.origin
				}
				rec = post(t, s, h, okPayload)
			}
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d", rec.Code, tt.status)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.allowOrig {
				t.Errorf("Allow-Origin %q, want %q", got, tt.allowOrig)
			}
			if tt.method == http.MethodOptions && tt.status == 204 {
				if h := rec.Header().Get("Access-Control-Allow-Headers"); h != "Content-Type, Authorization" {
					t.Errorf("Allow-Headers %q", h)
				}
				if m := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(m, "POST") {
					t.Errorf("Allow-Methods %q", m)
				}
			}
			if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Origin") {
				t.Error("missing Vary: Origin")
			}
		})
	}
}

func TestAuth(t *testing.T) {
	tests := []struct {
		name   string
		keys   []string
		header string
		status int
	}{
		{"no keys configured", nil, "", 201},
		{"no keys configured, header ignored", nil, "Bearer whatever", 201},
		{"missing header", []string{"k1"}, "", 401},
		{"wrong key", []string{"k1"}, "Bearer nope", 401},
		{"wrong scheme", []string{"k1"}, "Basic k1", 401},
		{"valid key", []string{"k1", "k2"}, "Bearer k2", 201},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, sub := newServer(t, func(c *config.Config) { c.APIKeys = tt.keys })
			h := map[string]string{}
			if tt.header != "" {
				h["Authorization"] = tt.header
			}
			rec := post(t, s, h, okPayload)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d", rec.Code, tt.status)
			}
			if tt.status == 401 && sub.calls != 0 {
				t.Error("submit called despite auth failure")
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewRateLimiter(3)
	l.now = func() time.Time { return now }
	for i := range 3 {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d denied", i)
		}
	}
	ok, wait := l.Allow("a")
	if ok || wait != 20*time.Second {
		t.Fatalf("4th: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other key should have its own bucket")
	}
	now = now.Add(20 * time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("token should have refilled")
	}
	now = now.Add(2 * time.Minute)
	l.Allow("c") // triggers sweep
	if _, ok := l.buckets["b"]; ok {
		t.Error("full bucket should have been swept")
	}
	if NewRateLimiter(0) != nil {
		t.Error("0 should disable")
	}
	var disabled *RateLimiter
	if ok, _ := disabled.Allow("x"); !ok {
		t.Error("nil limiter allows")
	}
}

func TestRateLimitHTTP(t *testing.T) {
	tests := []struct {
		name       string
		trustProxy bool
		xff        []string // per request
		lastStatus int
	}{
		{"same ip limited", false, []string{"", "", ""}, 429},
		{"xff ignored when not trusted", false, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, 429},
		{"xff distinguishes clients when trusted", true, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, 201},
		{"spoofed left-most xff ignored", true, []string{"6.6.6.6, 1.1.1.1", "7.7.7.7, 1.1.1.1", "8.8.8.8, 1.1.1.1"}, 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newServer(t, func(c *config.Config) { c.RateLimit = 2; c.TrustProxy = tt.trustProxy })
			var rec *httptest.ResponseRecorder
			for _, x := range tt.xff {
				h := map[string]string{}
				if x != "" {
					h["X-Forwarded-For"] = x
				}
				rec = post(t, s, h, okPayload)
			}
			if rec.Code != tt.lastStatus {
				t.Fatalf("last status %d, want %d", rec.Code, tt.lastStatus)
			}
			if rec.Code == 429 && rec.Header().Get("Retry-After") == "" {
				t.Error("missing Retry-After")
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	s, _ := newServer(t, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Errorf("healthz %d %q", rec.Code, rec.Body)
	}
}
