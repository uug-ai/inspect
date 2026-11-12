package store

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Local writes files to a directory and serves them under /assets/. It is
// meant for development and dry runs.
type Local struct {
	Dir     string
	BaseURL string // public base URL of the server, e.g. http://localhost:8080
}

// AssetsPrefix is the URL path the Local store's files are served under.
const AssetsPrefix = "/assets/"

// Put implements Store.
func (l *Local) Put(_ context.Context, name, _ string, data []byte) (string, error) {
	clean := path.Clean("/" + name)[1:]
	if clean == "" || clean != name || strings.HasPrefix(clean, "../") {
		return "", errors.New("local store: invalid name")
	}
	full := filepath.Join(l.Dir, filepath.FromSlash(clean))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", err
	}
	return strings.TrimRight(l.BaseURL, "/") + AssetsPrefix + clean, nil
}

// Handler serves stored files. Mount it at AssetsPrefix.
func (l *Local) Handler() http.Handler {
	fs := http.StripPrefix(strings.TrimSuffix(AssetsPrefix, "/"), http.FileServer(noDirFS{http.Dir(l.Dir)}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fs.ServeHTTP(w, r)
	})
}

// noDirFS hides directory listings.
type noDirFS struct{ fs http.FileSystem }

func (n noDirFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err == nil && st.IsDir() {
		f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}
