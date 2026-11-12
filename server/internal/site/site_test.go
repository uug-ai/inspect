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

// The page loads the library from unpkg; keep the CSP and the pinned, integrity-checked tag in sync.
func TestIndexPinsLibrary(t *testing.T) {
	index, err := files.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(index)
	if !strings.Contains(s, `src="https://unpkg.com/@uug-ai/inspect@0.`) || !strings.Contains(s, `integrity="sha384-`) {
		t.Fatal("index.html must load a pinned @uug-ai/inspect version from unpkg with an integrity hash")
	}
	if !strings.Contains(contentSecurityPolicy, "https://unpkg.com") {
		t.Fatal("CSP must allow unpkg for the library")
	}
}
