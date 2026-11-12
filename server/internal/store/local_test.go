package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalPut(t *testing.T) {
	dir := t.TempDir()
	l := &Local{Dir: dir, BaseURL: "http://localhost:8080/"}
	tests := []struct {
		name, want string
		wantErr    bool
	}{
		{name: "inspect/2026/11/12/abc/screenshot-1.png", want: "http://localhost:8080/assets/inspect/2026/11/12/abc/screenshot-1.png"},
		{name: "../escape.png", wantErr: true},
		{name: "a/../../escape.png", wantErr: true},
		{name: "/abs.png", wantErr: true},
		{name: "", wantErr: true},
	}
	for _, tt := range tests {
		url, err := l.Put(context.Background(), tt.name, "image/png", []byte("x"))
		if (err != nil) != tt.wantErr || url != tt.want {
			t.Errorf("Put(%q) = %q, %v", tt.name, url, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "inspect/2026/11/12/abc/screenshot-1.png")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.png")); err == nil {
		t.Error("escaped the store directory")
	}
}
