package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/uug-ai/inspect/server/internal/config"
)

var pngBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
var webmBytes = []byte("\x1A\x45\xDF\xA3\x9F\x42\x86\x81\x01\x42\xF7\x81\x01\x42\x82\x84webm")

// fakeGitHub is a minimal in-memory stand-in for the GitHub REST API.
type fakeGitHub struct {
	t  *testing.T
	mu sync.Mutex

	branchExists  bool
	conflictsLeft int // PUT contents returns 409 this many times first
	calls         []string
	commitParents []string
	files         map[string][]byte // path -> content on the assets branch
	issues        []map[string]any
	issueRepo     string
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)

	if r.Header.Get("Authorization") != "Bearer test-token" {
		w.WriteHeader(401)
		return
	}
	var in map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	p := r.URL.Path
	switch {
	case r.Method == "GET" && p == "/repos/acme/assets/git/ref/heads/inspect-assets":
		if !f.branchExists {
			reply(404, map[string]string{"message": "Not Found"})
			return
		}
		reply(200, map[string]any{"ref": "refs/heads/inspect-assets"})
	case r.Method == "POST" && p == "/repos/acme/assets/git/blobs":
		if !strings.Contains(in["content"].(string), "inspect") {
			f.t.Errorf("README blob content: %v", in["content"])
		}
		reply(201, map[string]string{"sha": "blobsha"})
	case r.Method == "POST" && p == "/repos/acme/assets/git/trees":
		if _, hasBase := in["base_tree"]; hasBase {
			f.t.Error("orphan tree must not have base_tree")
		}
		reply(201, map[string]string{"sha": "treesha"})
	case r.Method == "POST" && p == "/repos/acme/assets/git/commits":
		if in["tree"] != "treesha" {
			f.t.Errorf("commit tree %v", in["tree"])
		}
		parents, _ := in["parents"].([]any)
		for _, x := range parents {
			f.commitParents = append(f.commitParents, x.(string))
		}
		if _, ok := in["parents"]; !ok {
			f.t.Error("commit must send explicit empty parents")
		}
		reply(201, map[string]string{"sha": "commitsha"})
	case r.Method == "POST" && p == "/repos/acme/assets/git/refs":
		if in["ref"] != "refs/heads/inspect-assets" || in["sha"] != "commitsha" {
			f.t.Errorf("create ref %v", in)
		}
		f.branchExists = true
		reply(201, map[string]string{"ref": "refs/heads/inspect-assets"})
	case r.Method == "PUT" && strings.HasPrefix(p, "/repos/acme/assets/contents/"):
		path := strings.TrimPrefix(p, "/repos/acme/assets/contents/")
		if in["branch"] != "inspect-assets" {
			f.t.Errorf("PUT branch %v", in["branch"])
		}
		if !f.branchExists {
			reply(404, map[string]string{"message": "Branch not found"})
			return
		}
		if f.conflictsLeft > 0 {
			f.conflictsLeft--
			reply(409, map[string]string{"message": "is at abc but expected def"})
			return
		}
		data, err := base64.StdEncoding.DecodeString(in["content"].(string))
		if err != nil {
			f.t.Errorf("content not base64: %v", err)
		}
		f.files[path] = data
		reply(201, map[string]any{"content": map[string]string{
			"path":     path,
			"html_url": "https://github.com/acme/assets/blob/inspect-assets/" + path,
		}})
	case r.Method == "POST" && strings.HasSuffix(p, "/issues"):
		f.issueRepo = strings.TrimSuffix(strings.TrimPrefix(p, "/repos/"), "/issues")
		f.issues = append(f.issues, in)
		reply(201, map[string]any{"number": 42, "html_url": "https://github.com/" + f.issueRepo + "/issues/42"})
	default:
		f.t.Errorf("unexpected GitHub call %s %s", r.Method, p)
		reply(404, map[string]string{"message": "Not Found"})
	}
}

func multipartBody(t *testing.T, payload string, files ...[]byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("payload", payload)
	for i, f := range files {
		w, _ := mw.CreateFormFile("attachment", "file"+string(rune('0'+i)))
		_, _ = w.Write(f)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func testConfig(t *testing.T, env map[string]string) *config.Config {
	t.Helper()
	cfg, err := config.Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func submit(t *testing.T, h http.Handler, payload string, files ...[]byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, payload, files...)
	req := httptest.NewRequest(http.MethodPost, "/v1/issues", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const fullPayload = `{
  "title": "Save button does nothing",
  "description": "Clicked save, nothing happened. @ops please look",
  "kind": "recording",
  "repo": "acme/web",
  "labels": ["frontend", "not-allowed"],
  "context": {
    "url": "https://app.example.com/settings", "title": "Settings", "userAgent": "Mozilla/5.0",
    "viewport": {"width": 1440, "height": 900, "dpr": 2}, "timestamp": "2026-11-12T10:00:00Z",
    "console": [{"level": "error", "message": "TypeError", "time": "2026-11-12T09:59:59Z"}],
    "app": {"version": "1.2.3"}
  }
}`

func TestEndToEndGitHub(t *testing.T) {
	fake := &fakeGitHub{t: t, files: map[string][]byte{}}
	gh := httptest.NewServer(fake)
	defer gh.Close()

	cfg := testConfig(t, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_API_URL":          gh.URL,
		"INSPECT_DEFAULT_REPO":    "acme/app",
		"INSPECT_ALLOWED_REPOS":   "acme/app,acme/web",
		"INSPECT_ASSETS_REPO":     "acme/assets",
		"INSPECT_ALLOWED_ORIGINS": "http://localhost:4200",
		"INSPECT_ALLOWED_LABELS":  "frontend,backend",
	})
	h := build(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := submit(t, h, fullPayload, pngBytes, webmBytes)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Number != 42 || res.URL != "https://github.com/acme/web/issues/42" {
		t.Errorf("response %+v", res)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4200" {
		t.Errorf("CORS origin %q", got)
	}

	// Orphan branch was created: blob -> tree -> parentless commit -> ref.
	wantSeq := []string{
		"GET /repos/acme/assets/git/ref/heads/inspect-assets",
		"POST /repos/acme/assets/git/blobs",
		"POST /repos/acme/assets/git/trees",
		"POST /repos/acme/assets/git/commits",
		"POST /repos/acme/assets/git/refs",
	}
	for i, w := range wantSeq {
		if fake.calls[i] != w {
			t.Fatalf("call %d = %q, want %q (all: %v)", i, fake.calls[i], w, fake.calls)
		}
	}
	if len(fake.commitParents) != 0 {
		t.Errorf("orphan commit has parents %v", fake.commitParents)
	}

	// Both files uploaded under inspect/YYYY/MM/DD/<id>/ with sniffed extensions.
	pathRe := regexp.MustCompile(`^inspect/\d{4}/\d{2}/\d{2}/[0-9a-f]{16}/(screenshot-1\.png|recording-1\.webm)$`)
	if len(fake.files) != 2 {
		t.Fatalf("uploaded %d files", len(fake.files))
	}
	var pngPath, webmPath string
	for p, data := range fake.files {
		if !pathRe.MatchString(p) {
			t.Errorf("bad asset path %q", p)
		}
		if strings.HasSuffix(p, ".png") {
			pngPath = p
			if !bytes.Equal(data, pngBytes) {
				t.Error("png content mismatch")
			}
		} else {
			webmPath = p
		}
	}

	// Issue created in the requested (allowed) repo with rendered body.
	if fake.issueRepo != "acme/web" || len(fake.issues) != 1 {
		t.Fatalf("issue repo %q, issues %d", fake.issueRepo, len(fake.issues))
	}
	iss := fake.issues[0]
	if iss["title"] != "Save button does nothing" {
		t.Errorf("title %v", iss["title"])
	}
	labels, _ := json.Marshal(iss["labels"])
	if string(labels) != `["inspect","frontend"]` {
		t.Errorf("labels %s", labels)
	}
	body := iss["body"].(string)
	for _, w := range []string{
		"Clicked save, nothing happened. @‍ops please look",
		"![screenshot-1.png](https://github.com/acme/assets/blob/inspect-assets/" + pngPath + "?raw=true)",
		"https://github.com/acme/assets/blob/inspect-assets/" + webmPath + "?raw=true\n",
		"[▶ recording-1.webm](https://github.com/acme/assets/blob/inspect-assets/" + webmPath + "?raw=true)",
		"<details><summary>Context</summary>",
		"| URL | https://app.example.com/settings |",
		"| version | 1.2.3 |",
		"ERROR TypeError",
	} {
		if !strings.Contains(body, w) {
			t.Errorf("body missing %q\n%s", w, body)
		}
	}

	// A second submission reuses the branch without re-checking or recreating it.
	fake.calls = nil
	rec = submit(t, h, `{"title":"Again","kind":"screenshot"}`, pngBytes)
	if rec.Code != http.StatusCreated {
		t.Fatalf("second status %d: %s", rec.Code, rec.Body)
	}
	for _, c := range fake.calls {
		if strings.Contains(c, "/git/") {
			t.Errorf("unexpected git data call on second submit: %s", c)
		}
	}
	if fake.issueRepo != "acme/app" {
		t.Errorf("default repo not used: %s", fake.issueRepo)
	}

	// Disallowed repo is rejected before anything is uploaded.
	fake.calls = nil
	rec = submit(t, h, `{"title":"x","kind":"note","repo":"evil/repo"}`, pngBytes)
	if rec.Code != http.StatusBadRequest || len(fake.calls) != 0 {
		t.Errorf("disallowed repo: status %d, calls %v", rec.Code, fake.calls)
	}
}

func TestEndToEndExistingBranchAndConflictRetry(t *testing.T) {
	fake := &fakeGitHub{t: t, files: map[string][]byte{}, branchExists: true, conflictsLeft: 1}
	gh := httptest.NewServer(fake)
	defer gh.Close()

	cfg := testConfig(t, map[string]string{
		"GITHUB_TOKEN": "test-token", "GITHUB_API_URL": gh.URL,
		"INSPECT_DEFAULT_REPO": "acme/assets", "INSPECT_ALLOWED_ORIGINS": "*",
	})
	h := build(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := submit(t, h, `{"title":"t","kind":"screenshot","context":{"timestamp":"2026-11-12T10:00:00Z"}}`, pngBytes)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for _, c := range fake.calls {
		if strings.Contains(c, "/git/blobs") || strings.Contains(c, "/git/refs") {
			t.Errorf("branch exists; should not create: %s", c)
		}
	}
	if len(fake.files) != 1 {
		t.Errorf("expected 1 file after retry, got %d", len(fake.files))
	}
	body := fake.issues[0]["body"].(string)
	if strings.Contains(body, "<details>") {
		t.Errorf("timestamp-only context should omit the Context block:\n%s", body)
	}
}

func TestEndToEndDryRunLocal(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(t, map[string]string{
		"INSPECT_DEFAULT_REPO": "acme/app", "INSPECT_ALLOWED_REPOS": "acme/web", "INSPECT_STORE": "local", "INSPECT_DRY_RUN": "true",
		"INSPECT_LOCAL_DIR": dir, "INSPECT_ALLOWED_ORIGINS": "http://localhost:4200",
		"INSPECT_PUBLIC_URL": "http://inspect.test",
	})
	h := build(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := submit(t, h, fullPayload, pngBytes)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Number != 0 || !strings.HasPrefix(res.URL, "http://inspect.test/preview/") {
		t.Fatalf("dry-run result %+v", res)
	}

	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		return r
	}
	prev := get(strings.TrimPrefix(res.URL, "http://inspect.test"))
	if prev.Code != 200 || !strings.Contains(prev.Body.String(), "# Save button does nothing") {
		t.Fatalf("preview %d: %s", prev.Code, prev.Body)
	}
	m := regexp.MustCompile(`http://inspect\.test(/assets/inspect/[^)]+\.png)`).FindStringSubmatch(prev.Body.String())
	if m == nil {
		t.Fatalf("no local asset URL in preview:\n%s", prev.Body)
	}
	asset := get(m[1])
	if asset.Code != 200 || !bytes.Equal(asset.Body.Bytes(), pngBytes) {
		t.Errorf("asset %d", asset.Code)
	}
	if d := get("/assets/inspect/"); d.Code != 404 {
		t.Errorf("directory listing should be hidden, got %d", d.Code)
	}
}
