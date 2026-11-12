package store

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/uug-ai/inspect/server/internal/github"
)

const branchReadme = `# inspect assets

This orphan branch is managed by the inspect server. It stores screenshots and
recordings referenced from issues created through inspect. It intentionally
shares no history with the repository's main code.
`

// GitHub commits files to a dedicated (orphan) branch of a repository via
// the Contents API and returns blob URLs that GitHub renders in issues.
type GitHub struct {
	Client *github.Client
	Repo   string // owner/name
	Branch string

	mu       sync.Mutex // serialises commits to the branch from this process
	ensured  bool
	backoffs []time.Duration
}

// NewGitHub returns a store committing to branch of repo.
func NewGitHub(c *github.Client, repo, branch string) *GitHub {
	return &GitHub{
		Client:   c,
		Repo:     repo,
		Branch:   branch,
		backoffs: []time.Duration{250 * time.Millisecond, time.Second, 2 * time.Second},
	}
}

// Put implements Store.
func (g *GitHub) Put(ctx context.Context, name, contentType string, data []byte) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for attempt := 0; ; attempt++ {
		if err := g.ensureBranch(ctx); err != nil {
			return "", fmt.Errorf("ensure branch %s:%s: %w", g.Repo, g.Branch, err)
		}
		res, err := g.Client.PutFile(ctx, g.Repo, g.Branch, name, "Add "+name, data)
		if err == nil {
			return g.rawURL(res, name), nil
		}
		status := github.StatusOf(err)
		retryable := status == http.StatusConflict || // branch head moved (another replica committed)
			status == http.StatusNotFound || // branch deleted since we cached it
			status >= 500
		if !retryable || attempt >= len(g.backoffs) {
			return "", err
		}
		if status == http.StatusNotFound {
			g.ensured = false
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(g.backoffs[attempt]):
		}
	}
}

func (g *GitHub) ensureBranch(ctx context.Context) error {
	if g.ensured {
		return nil
	}
	ok, err := g.Client.BranchExists(ctx, g.Repo, g.Branch)
	if err != nil {
		return err
	}
	if !ok {
		if err := g.Client.CreateOrphanBranch(ctx, g.Repo, g.Branch, branchReadme); err != nil {
			return err
		}
	}
	g.ensured = true
	return nil
}

// rawURL returns https://github.com/{o}/{r}/blob/{branch}/{path}?raw=true.
// GitHub's html_url is preferred (correct host on GitHub Enterprise).
func (g *GitHub) rawURL(res *github.PutFileResult, name string) string {
	u := res.Content.HTMLURL
	if u == "" {
		u = "https://github.com/" + g.Repo + "/blob/" + g.Branch + "/" + name
	}
	if strings.Contains(u, "?") {
		return u + "&raw=true"
	}
	return u + "?raw=true"
}
