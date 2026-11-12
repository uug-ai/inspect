package issue

import (
	"strings"
	"testing"
)

func TestParsePayload(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name    string
		json    string
		wantErr string
		check   func(t *testing.T, p *Payload)
	}{
		{name: "invalid json", json: `{`, wantErr: "invalid JSON"},
		{name: "missing title", json: `{"kind":"note"}`, wantErr: "title is required"},
		{name: "blank title", json: `{"title":"  \n ","kind":"note"}`, wantErr: "title is required"},
		{name: "title too long", json: `{"title":"` + long(257) + `","kind":"note"}`, wantErr: "title is too long"},
		{name: "title max", json: `{"title":"` + long(256) + `","kind":"note"}`},
		{name: "bad kind", json: `{"title":"t","kind":"video"}`, wantErr: "kind must be"},
		{name: "missing kind", json: `{"title":"t"}`, wantErr: "kind must be"},
		{name: "description too long", json: `{"title":"t","kind":"note","description":"` + long(20001) + `"}`, wantErr: "description is too long"},
		{name: "too many labels", json: `{"title":"t","kind":"note","labels":[` + strings.Repeat(`"a",`, 20) + `"a"]}`, wantErr: "too many labels"},
		{name: "empty label", json: `{"title":"t","kind":"note","labels":[" "]}`, wantErr: "label 0"},
		{name: "context missing", json: `{"title":"t","kind":"note"}`},
		{name: "context timestamp only", json: `{"title":"t","kind":"recording","context":{"timestamp":"2026-11-12T10:00:00Z"}}`},
		{name: "context null", json: `{"title":"t","kind":"note","context":null}`},
		{name: "wrong type", json: `{"title":5,"kind":"note"}`, wantErr: "invalid JSON"},
		{
			name: "normalises",
			json: `{"title":"  a\nb\tc  ","kind":"screenshot","labels":[" ui "],"unknown":1,
				"context":{"url":"` + "https://x/" + long(3000) + `","viewport":{"width":1440,"height":900,"dpr":2},
				"app":{"version":"1.2.3","n":5}}}`,
			check: func(t *testing.T, p *Payload) {
				if p.Title != "a b c" {
					t.Errorf("title = %q", p.Title)
				}
				if p.Labels[0] != "ui" {
					t.Errorf("labels = %q", p.Labels)
				}
				if n := len([]rune(p.Context.URL)); n != maxURL || !strings.HasSuffix(p.Context.URL, "…") {
					t.Errorf("url not truncated: %d", n)
				}
				if p.Context.Viewport.Width != 1440 {
					t.Errorf("viewport = %+v", p.Context.Viewport)
				}
			},
		},
		{
			name: "console capped to most recent",
			json: `{"title":"t","kind":"note","context":{"console":[` +
				strings.TrimSuffix(strings.Repeat(`{"level":"log","message":"m"},`, 60), ",") +
				`,{"level":"error","message":"last"}]}}`,
			check: func(t *testing.T, p *Payload) {
				if len(p.Context.Console) != maxConsole || p.Context.Console[maxConsole-1].Message != "last" {
					t.Errorf("console len %d", len(p.Context.Console))
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePayload([]byte(tt.json))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if _, ok := err.(*ValidationError); !ok {
					t.Errorf("err type %T, want *ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, p)
			}
		})
	}
}
