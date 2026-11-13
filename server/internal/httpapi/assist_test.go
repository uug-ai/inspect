package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uug-ai/inspect/server/internal/assist"
)

type fakeAssistant struct {
	got assist.Request
	err error
}

func (f *fakeAssistant) Draft(_ context.Context, req assist.Request) (*assist.Draft, error) {
	f.got = req
	if f.err != nil {
		return nil, f.err
	}
	return &assist.Draft{Title: "Drafted title", Description: "## Summary\nDrafted."}, nil
}

func postAssist(t *testing.T, s *Server, headers map[string]string, parts ...part) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := form(t, parts...)
	req := httptest.NewRequest(http.MethodPost, "/v1/assist", body)
	req.Header.Set("Content-Type", ct)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestCapabilities(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s, _ := newServer(t, nil)
		if enabled {
			s.Assist = &fakeAssistant{}
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		var got map[string]bool
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != 200 || got["assist"] != enabled {
			t.Errorf("enabled=%v: %d %s", enabled, rec.Code, rec.Body)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
			t.Error("missing CORS header")
		}
	}
}

func TestAssist(t *testing.T) {
	notes := part{field: "payload", data: []byte(`{"title":"","description":"counter stays at 0","kind":"screenshot","context":{"timestamp":"2026-11-12T10:00:00Z","url":"https://app.example.com/dashboard"}}`)}
	webm := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0xF7, 0x81, 0x01, 0x42, 0xF2, 0x81, 0x04, 0x42, 0xF3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'}

	t.Run("drafts title and description", func(t *testing.T) {
		s, _ := newServer(t, nil)
		fa := &fakeAssistant{}
		s.Assist = fa
		rec := postAssist(t, s, map[string]string{"Origin": "https://app.example.com"}, notes, part{"attachment", "s.png", "image/png", pngBytes})
		if rec.Code != 200 {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var d assist.Draft
		_ = json.Unmarshal(rec.Body.Bytes(), &d)
		if d.Title != "Drafted title" {
			t.Errorf("draft %+v", d)
		}
		if fa.got.Title != "" || fa.got.Description != "counter stays at 0" || len(fa.got.Images) != 1 || fa.got.Context.URL == "" {
			t.Errorf("assistant got %+v", fa.got)
		}
	})

	cases := []struct {
		name    string
		setup   func(*Server)
		headers map[string]string
		parts   []part
		status  int
		msg     string
	}{
		{"disabled", func(s *Server) { s.Assist = nil }, nil, []part{notes}, 404, "not enabled"},
		{"nothing to work with", nil, nil, []part{{field: "payload", data: []byte(`{"kind":"screenshot","context":{"timestamp":"t"}}`)}}, 400, "write a few words"},
		{"video", nil, nil, []part{notes, {"attachment", "r.webm", "video/webm", webm}}, 415, "screenshots only"},
		{"bad origin", nil, map[string]string{"Origin": "https://evil.example"}, []part{notes}, 403, "origin"},
		{"model fails", func(s *Server) { s.Assist = &fakeAssistant{err: errors.New("boom")} }, nil, []part{notes}, 502, "couldn't help"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newServer(t, nil)
			s.Assist = &fakeAssistant{}
			if tc.setup != nil {
				tc.setup(s)
			}
			rec := postAssist(t, s, tc.headers, tc.parts...)
			if rec.Code != tc.status || !strings.Contains(errMsg(t, rec), tc.msg) {
				t.Errorf("got %d %q, want %d containing %q", rec.Code, errMsg(t, rec), tc.status, tc.msg)
			}
		})
	}
}
