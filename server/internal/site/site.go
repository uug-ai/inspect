// Package site serves the inspect.uug.ai landing page. The files are embedded
// in the binary, so the hardened image stays a single static executable.
package site

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web
var files embed.FS

// contentSecurityPolicy allows only what the page uses: its own files, Google
// Fonts, and the pinned @uug-ai/inspect build from unpkg (also checked with
// Subresource Integrity). style-src needs 'unsafe-inline' because the library
// injects its stylesheet into a Shadow DOM.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' https://unpkg.com; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; " +
	"img-src 'self' data: blob:; " +
	"media-src 'self' blob:; " +
	"connect-src 'self'; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'"

// Register adds GET / and GET /static/ to mux.
func Register(mux *http.ServeMux) {
	web, err := fs.Sub(files, "web")
	if err != nil {
		panic(err) // the embed path is fixed at build time
	}
	index, err := fs.ReadFile(web, "index.html")
	if err != nil {
		panic(err)
	}

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		secure(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(index)
	})

	static := http.StripPrefix("/static/", http.FileServerFS(web))
	mux.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/static/")
		if name == "" || name == "index.html" || strings.HasSuffix(name, "/") {
			http.NotFound(w, r) // no directory listings; the page lives at /
			return
		}
		secure(w)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		static.ServeHTTP(w, r)
	})
}

func secure(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", contentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	// The live demo captures this tab, so display-capture stays allowed for the page itself.
	h.Set("Permissions-Policy", "geolocation=(), payment=(), usb=(), display-capture=(self)")
}
