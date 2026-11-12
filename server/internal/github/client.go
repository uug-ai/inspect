// Package github is a deliberately small GitHub REST client covering only
// what the inspect server needs: issues, contents, and the Git Data API.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenProvider supplies the credential used for each API call. It is the
// seam for GitHub App authentication (installation tokens that rotate);
// today only StaticToken is implemented.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken is a personal access token (classic or fine-grained).
type StaticToken string

// Token implements TokenProvider.
func (t StaticToken) Token(context.Context) (string, error) {
	if t == "" {
		return "", errors.New("github: no token configured")
	}
	return string(t), nil
}

// Client talks to the GitHub REST API.
type Client struct {
	BaseURL string // e.g. https://api.github.com
	HTTP    *http.Client
	Tokens  TokenProvider
}

// NewClient returns a client with sensible outbound timeouts.
func NewClient(baseURL string, tokens TokenProvider) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 2 * time.Minute},
		Tokens:  tokens,
	}
}

// Error is a non-2xx response from GitHub.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("github: %d %s", e.Status, e.Message) }

// StatusOf returns the HTTP status of a GitHub error, or 0.
func StatusOf(err error) int {
	var ge *Error
	if errors.As(err, &ge) {
		return ge.Status
	}
	return 0
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	tok, err := c.Tokens.Token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "inspect-server")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		var e struct {
			Message string `json:"message"`
		}
		msg := strings.TrimSpace(string(b))
		if json.Unmarshal(b, &e) == nil && e.Message != "" {
			msg = e.Message
		}
		// GitHub answers 404 (not 403) when a token can't see a private repo; this
		// header names the permission the call needed, which makes that debuggable.
		if need := resp.Header.Get("X-Accepted-GitHub-Permissions"); need != "" && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden) {
			msg += " (token needs " + need + " on this repo; check repository access, permissions, and org token policy)"
		}
		return &Error{Status: resp.StatusCode, Message: msg}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func repoPath(repo string) string {
	owner, name, _ := strings.Cut(repo, "/")
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// IssueRequest is the body of POST /repos/{o}/{r}/issues.
type IssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
}

// Issue is the subset of the issue response we use.
type Issue struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// CreateIssue opens an issue in repo ("owner/name").
func (c *Client) CreateIssue(ctx context.Context, repo string, in IssueRequest) (*Issue, error) {
	var out Issue
	if err := c.do(ctx, http.MethodPost, repoPath(repo)+"/issues", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BranchExists reports whether refs/heads/{branch} exists.
func (c *Client) BranchExists(ctx context.Context, repo, branch string) (bool, error) {
	err := c.do(ctx, http.MethodGet, repoPath(repo)+"/git/ref/heads/"+escapePath(branch), nil, nil)
	switch StatusOf(err) {
	case 0:
		return err == nil, err
	case http.StatusNotFound:
		return false, nil
	default:
		return false, err
	}
}

type shaResp struct {
	SHA string `json:"sha"`
}

// CreateOrphanBranch creates refs/heads/{branch} pointing at a parentless
// commit containing only README.md, so the branch carries none of the
// repository's code. A concurrent creation (422 "Reference already exists")
// is treated as success.
func (c *Client) CreateOrphanBranch(ctx context.Context, repo, branch, readme string) error {
	rp := repoPath(repo)

	var blob shaResp
	if err := c.do(ctx, http.MethodPost, rp+"/git/blobs",
		map[string]string{"content": readme, "encoding": "utf-8"}, &blob); err != nil {
		return fmt.Errorf("create blob: %w", err)
	}

	var tree shaResp
	if err := c.do(ctx, http.MethodPost, rp+"/git/trees", map[string]any{
		"tree": []map[string]string{{"path": "README.md", "mode": "100644", "type": "blob", "sha": blob.SHA}},
	}, &tree); err != nil {
		return fmt.Errorf("create tree: %w", err)
	}

	var commit shaResp
	if err := c.do(ctx, http.MethodPost, rp+"/git/commits", map[string]any{
		"message": "Initialise " + branch + " (inspect asset storage)",
		"tree":    tree.SHA,
		"parents": []string{},
	}, &commit); err != nil {
		return fmt.Errorf("create commit: %w", err)
	}

	err := c.do(ctx, http.MethodPost, rp+"/git/refs",
		map[string]string{"ref": "refs/heads/" + branch, "sha": commit.SHA}, nil)
	if StatusOf(err) == http.StatusUnprocessableEntity {
		if ok, _ := c.BranchExists(ctx, repo, branch); ok {
			return nil
		}
	}
	if err != nil {
		return fmt.Errorf("create ref: %w", err)
	}
	return nil
}

// PutFileResult is the subset of the contents response we use.
type PutFileResult struct {
	Content struct {
		Path    string `json:"path"`
		HTMLURL string `json:"html_url"`
	} `json:"content"`
}

// PutFile creates a file on branch via the Contents API.
func (c *Client) PutFile(ctx context.Context, repo, branch, path, message string, data []byte) (*PutFileResult, error) {
	var out PutFileResult
	err := c.do(ctx, http.MethodPut, repoPath(repo)+"/contents/"+escapePath(path), map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(data),
		"branch":  branch,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
