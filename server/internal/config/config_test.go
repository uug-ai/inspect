package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, c *Config)
	}{
		{
			name:    "default repo required",
			env:     map[string]string{"GITHUB_TOKEN": "x"},
			wantErr: "INSPECT_DEFAULT_REPO is required",
		},
		{
			name:    "token required for github store",
			env:     map[string]string{"INSPECT_DEFAULT_REPO": "o/r"},
			wantErr: "GITHUB_TOKEN is required",
		},
		{
			name:    "token required for local store without dry run",
			env:     map[string]string{"INSPECT_DEFAULT_REPO": "o/r", "INSPECT_STORE": "local"},
			wantErr: "GITHUB_TOKEN is required",
		},
		{
			name:    "bad repo",
			env:     map[string]string{"INSPECT_DEFAULT_REPO": "nope", "GITHUB_TOKEN": "x"},
			wantErr: "not owner/name",
		},
		{
			name:    "bad store",
			env:     map[string]string{"INSPECT_DEFAULT_REPO": "o/r", "GITHUB_TOKEN": "x", "INSPECT_STORE": "s3"},
			wantErr: "INSPECT_STORE",
		},
		{
			name: "local dry run needs no token; defaults",
			env:  map[string]string{"INSPECT_DEFAULT_REPO": "o/r", "INSPECT_STORE": "local", "INSPECT_DRY_RUN": "true"},
			check: func(t *testing.T, c *Config) {
				if c.Port != "8080" || c.AssetsBranch != "inspect-assets" || c.RateLimit != 10 {
					t.Errorf("defaults: %+v", c)
				}
				if c.MaxUploadBytes != 50<<20 || c.MaxFileBytes != 50<<20 {
					t.Errorf("limits: %d %d", c.MaxUploadBytes, c.MaxFileBytes)
				}
				if len(c.Labels) != 1 || c.Labels[0] != "inspect" {
					t.Errorf("labels: %v", c.Labels)
				}
				if !c.RepoAllowed("O/R") || c.RepoAllowed("o/other") {
					t.Errorf("allowed repos: %v", c.AllowedRepos)
				}
				if c.PublicURL != "http://localhost:8080" {
					t.Errorf("public url: %s", c.PublicURL)
				}
			},
		},
		{
			name: "lists and origins",
			env: map[string]string{
				"INSPECT_DEFAULT_REPO": "o/r", "GITHUB_TOKEN": "x",
				"INSPECT_ALLOWED_REPOS":   "o/a, o/b",
				"INSPECT_ALLOWED_ORIGINS": "https://app.example.com/, http://localhost:4200",
				"INSPECT_LABELS":          "bug,from-inspect",
				"INSPECT_MAX_UPLOAD_MB":   "10",
				"INSPECT_MAX_FILE_MB":     "20",
			},
			check: func(t *testing.T, c *Config) {
				if !c.RepoAllowed("o/a") || !c.RepoAllowed("o/r") {
					t.Errorf("allowed repos: %v", c.AllowedRepos)
				}
				if !c.OriginAllowed("https://app.example.com") || c.OriginAllowed("https://evil.example.com") {
					t.Errorf("origins: %v", c.AllowedOrigins)
				}
				if len(c.Labels) != 2 {
					t.Errorf("labels: %v", c.Labels)
				}
				if c.MaxFileBytes != 10<<20 {
					t.Errorf("file limit should be capped at total: %d", c.MaxFileBytes)
				}
			},
		},
		{
			name: "wildcard origin",
			env:  map[string]string{"INSPECT_DEFAULT_REPO": "o/r", "GITHUB_TOKEN": "x", "INSPECT_ALLOWED_ORIGINS": "*"},
			check: func(t *testing.T, c *Config) {
				if !c.OriginAllowed("https://anything.example") {
					t.Error("wildcard should allow any origin")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(env(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, c)
		})
	}
}
