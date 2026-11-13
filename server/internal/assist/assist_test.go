package assist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/uug-ai/inspect/server/internal/issue"
)

func azureReply(text string) string {
	b, _ := json.Marshal(map[string]any{
		"model":   "gpt-5-mini-2025-08-07",
		"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": text}}},
	})
	return string(b)
}

func TestDraftRequestAndResponse(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/v1/chat/completions" {
			t.Errorf("path %q", r.URL.Path)
		}
		if r.Header.Get("api-key") != "k" {
			t.Errorf("api-key %q", r.Header.Get("api-key"))
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, azureReply(`{"title":"Dashboard: counter stuck","description":"## Summary\nIt is stuck."}`))
	}))
	defer srv.Close()

	c, err := New(Config{Endpoint: srv.URL, APIKey: "k", Deployment: "gpt-x", ReasoningEffort: "low"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.Draft(context.Background(), Request{
		Title:       "",
		Description: "counter stays 0",
		Context: issue.Context{
			URL:     "https://app.example/dashboard",
			Console: []issue.ConsoleEntry{{Level: "error", Message: "HTTP 500 /api/cases"}},
		},
		Images: []Image{{Data: []byte("RIFFxxxxWEBP"), ContentType: "image/webp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Dashboard: counter stuck" || !strings.HasPrefix(d.Description, "## Summary") {
		t.Fatalf("draft %+v", d)
	}

	if got["model"] != "gpt-x" || got["store"] != false || got["reasoning_effort"] != "low" {
		t.Errorf("model/store/reasoning: %v %v %v", got["model"], got["store"], got["reasoning_effort"])
	}
	format := got["response_format"].(map[string]any)
	if format["type"] != "json_schema" || format["json_schema"].(map[string]any)["strict"] != true {
		t.Errorf("response_format %v", format)
	}
	msgs := got["messages"].([]any)
	system := msgs[0].(map[string]any)
	if system["role"] != "system" || !strings.Contains(system["content"].(string), "## Acceptance criteria") {
		t.Error("system message should hold the instructions, incl. acceptance criteria")
	}
	content := msgs[1].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	for _, want := range []string{"counter stays 0", "Title: (none)", "https://app.example/dashboard", "HTTP 500 /api/cases", "1 screenshot(s)"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt text missing %q:\n%s", want, text)
		}
	}
	img := content[1].(map[string]any)
	if img["type"] != "image_url" || !strings.HasPrefix(img["image_url"].(map[string]any)["url"].(string), "data:image/webp;base64,") {
		t.Errorf("image part %v", img)
	}
}

func TestDraftRetriesOnceOnThrottling(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"slow down"}}`)
			return
		}
		_, _ = io.WriteString(w, azureReply(`{"title":"t","description":"d"}`))
	}))
	defer srv.Close()
	c, _ := New(Config{Endpoint: srv.URL + "/openai/v1", APIKey: "k", Deployment: "m"})
	if _, err := c.Draft(context.Background(), Request{Description: "x"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

func TestDraftErrors(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
	}{
		"bad request":  {400, `{"error":{"message":"bad image"}}`},
		"refusal":      {200, `{"choices":[{"finish_reason":"stop","message":{"content":null,"refusal":"no"}}]}`},
		"incomplete":   {200, `{"choices":[{"finish_reason":"length","message":{"content":"{\"title\":"}}]}`},
		"no choices":   {200, `{"choices":[]}`},
		"not json":     {200, azureReply(`not json`)},
		"empty fields": {200, azureReply(`{"title":"","description":""}`)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			c, _ := New(Config{Endpoint: srv.URL, APIKey: "k", Deployment: "m"})
			_, err := c.Draft(context.Background(), Request{Description: "x"})
			if err == nil {
				t.Fatal("expected an error")
			}
			if tc.status == 200 && !errors.Is(err, ErrUnavailable) {
				t.Errorf("want ErrUnavailable, got %v", err)
			}
		})
	}
}

func TestNewValidates(t *testing.T) {
	if _, err := New(Config{Endpoint: "https://x", APIKey: "k"}); err == nil {
		t.Error("missing deployment should fail")
	}
	if _, err := New(Config{Endpoint: "ftp://x", APIKey: "k", Deployment: "m"}); err == nil {
		t.Error("non-http endpoint should fail")
	}
}
