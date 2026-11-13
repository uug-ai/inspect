package issue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/uug-ai/inspect/server/internal/config"
	"github.com/uug-ai/inspect/server/internal/github"
	"github.com/uug-ai/inspect/server/internal/options"
	"github.com/uug-ai/inspect/server/internal/store"
)

// IssueCreator creates GitHub issues (implemented by *github.Client).
type IssueCreator interface {
	CreateIssue(ctx context.Context, repo string, in github.IssueRequest) (*github.Issue, error)
}

// File is a validated attachment.
type File struct {
	Data        []byte
	ContentType string // sniffed
	Ext         string
}

// Result is what the API returns.
type Result struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// Service turns a submission into stored media plus a GitHub issue.
type Service struct {
	Cfg      *config.Config
	Issues   IssueCreator                  // unused in dry-run mode
	StoreFor func(repo string) store.Store // store for media of an issue in repo
	Previews *Previews                     // dry-run previews
	Log      *slog.Logger
	Now      func() time.Time
	// Options checks the reporter's labels and assignees; nil turns the pickers off.
	Options RepoOptions
}

// Submit stores attachments and creates the issue (or a dry-run preview).
func (s *Service) Submit(ctx context.Context, p *Payload, files []File) (*Result, error) {
	repo := s.Cfg.DefaultRepo
	if p.Repo != "" {
		if !s.Cfg.RepoAllowed(p.Repo) {
			return nil, invalid("repo %q is not allowed", p.Repo)
		}
		repo = p.Repo
	}

	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	id := randomID()
	prefix := fmt.Sprintf("inspect/%s/%s/", now().UTC().Format("2006/01/02"), id)

	st := s.StoreFor(repo)
	atts := make([]Attachment, 0, len(files))
	var nImg, nVid int
	for _, f := range files {
		var name string
		if IsVideo(f.ContentType) {
			nVid++
			name = fmt.Sprintf("recording-%d.%s", nVid, f.Ext)
		} else {
			nImg++
			name = fmt.Sprintf("screenshot-%d.%s", nImg, f.Ext)
		}
		url, err := st.Put(ctx, prefix+name, f.ContentType, f.Data)
		if err != nil {
			return nil, fmt.Errorf("store %s: %w", name, err)
		}
		atts = append(atts, Attachment{Name: name, URL: url, IsVideo: IsVideo(f.ContentType)})
	}

	labels, assignees := s.choices(ctx, repo, p)
	req := github.IssueRequest{
		Title:     p.Title,
		Body:      RenderBody(p, atts),
		Labels:    labels,
		Assignees: assignees,
	}

	if s.Cfg.DryRun {
		s.Log.Info("dry run: would create issue",
			"repo", repo, "title", req.Title, "labels", req.Labels, "assignees", req.Assignees, "attachments", len(atts), "body", req.Body)
		s.Previews.Add(id, repo, req)
		return &Result{Number: 0, URL: s.Cfg.PublicURL + "/preview/" + id}, nil
	}

	iss, err := s.Issues.CreateIssue(ctx, repo, req)
	if err != nil {
		return nil, fmt.Errorf("create issue in %s: %w", repo, err)
	}
	s.Log.Info("issue created", "repo", repo, "number", iss.Number, "url", iss.HTMLURL, "attachments", len(atts))
	return &Result{Number: iss.Number, URL: iss.HTMLURL}, nil
}

// RepoOptions looks up a repository's labels and assignable users; implemented
// by *options.Cache. Nil means the pickers are off.
type RepoOptions interface {
	Get(ctx context.Context, repo string) (*options.Options, error)
}

// choices returns the labels and assignees for the issue: the configured labels,
// plus the reporter's picks that exist in the repository (labels) or may be
// assigned there (assignees). If the repository can't be checked, the picks are
// dropped and the report is filed anyway.
func (s *Service) choices(ctx context.Context, repo string, p *Payload) ([]string, []string) {
	client := p.Labels
	var assignees []string
	if s.Options != nil && (len(p.Labels) > 0 || len(p.Assignees) > 0) {
		if o, err := s.Options.Get(ctx, repo); err != nil {
			s.Log.Warn("could not check labels and assignees; filing without them", "repo", repo, "err", err)
			client = nil
		} else {
			client = o.FilterLabels(p.Labels)
			assignees = o.FilterAssignees(p.Assignees)
		}
	}
	return s.labels(client, s.Options != nil), assignees
}

// labels returns the configured labels plus client labels, de-duplicated.
// Client labels must be on INSPECT_ALLOWED_LABELS when that's set; without an
// allowlist they're only kept when they were checked against the repository
// (picker on), otherwise dropped.
func (s *Service) labels(client []string, checked bool) []string {
	out := make([]string, 0, len(s.Cfg.Labels)+len(client))
	seen := map[string]bool{}
	add := func(l string) {
		k := strings.ToLower(l)
		if !seen[k] {
			seen[k] = true
			out = append(out, l)
		}
	}
	for _, l := range s.Cfg.Labels {
		add(l)
	}
	for _, l := range client {
		if len(s.Cfg.AllowedLabels) == 0 {
			if checked {
				add(l)
			}
			continue
		}
		for _, a := range s.Cfg.AllowedLabels {
			if strings.EqualFold(a, l) {
				add(a)
				break
			}
		}
	}
	return out
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Previews keeps the most recent dry-run issues in memory.
type Previews struct {
	mu    sync.Mutex
	max   int
	order []string
	items map[string]string
}

// NewPreviews keeps at most max previews.
func NewPreviews(max int) *Previews {
	return &Previews{max: max, items: map[string]string{}}
}

// Add records a preview.
func (p *Previews) Add(id, repo string, req github.IssueRequest) {
	doc := fmt.Sprintf("<!-- dry run: repo=%s labels=%s -->\n# %s\n\n%s", repo, strings.Join(req.Labels, ","), req.Title, req.Body)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items[id] = doc
	p.order = append(p.order, id)
	for len(p.order) > p.max {
		delete(p.items, p.order[0])
		p.order = p.order[1:]
	}
}

// Get returns a preview's markdown.
func (p *Previews) Get(id string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	doc, ok := p.items[id]
	return doc, ok
}
