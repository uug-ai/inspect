package site

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		path, wantType, wantBody string
		wantStatus               int
	}{
		{"/", "text/html", "Point at the bug.", 200},
		{"/static/site.css", "text/css", "--brand-gradient", 200},
		{"/static/site.js", "javascript", "createInspect", 200},
		{"/static/favicon.svg", "image/svg+xml", "<svg", 200},
		{"/static/demo.mp4", "video/mp4", "", 200},
		{"/static/", "", "", 404},
		{"/static/index.html", "", "", 404},
		{"/static/missing.css", "", "", 404},
		{"/nope", "", "", 404},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			res, err := http.Get(srv.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.wantStatus {
				t.Fatalf("status %d, want %d", res.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus != 200 {
				return
			}
			if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, tc.wantType) {
				t.Errorf("Content-Type %q, want %q", ct, tc.wantType)
			}
			if tc.wantBody != "" && !strings.Contains(string(body), tc.wantBody) {
				t.Errorf("body does not contain %q", tc.wantBody)
			}
			if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
				t.Errorf("missing CSP, got %q", csp)
			}
			if res.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing nosniff")
			}
		})
	}
}

// The "Try it" demo runs the library embedded from client/ (server/Dockerfile).
func TestIndexUsesEmbeddedLibrary(t *testing.T) {
	index, err := files.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `src="/static/lib/inspect.js"`) {
		t.Fatal("index.html must load the embedded library from /static/lib/inspect.js")
	}
	if strings.Contains(contentSecurityPolicy, "https://") && strings.Contains(contentSecurityPolicy, "script-src 'self' https") {
		t.Fatal("scripts must only come from this server")
	}
	// The image build embeds lib/inspect.js; local builds may not have it (scripts/sync-lib.sh).
	if _, err := files.ReadFile("web/lib/inspect.js"); err != nil {
		t.Skip("web/lib/inspect.js not built here; the Dockerfile builds it from client/")
	}
	mux := http.NewServeMux()
	Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/static/lib/inspect.js")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("lib: status %d, type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}
