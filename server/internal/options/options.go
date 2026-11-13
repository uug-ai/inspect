// Package options serves a repository's labels and assignable users for the
// report dialog's pickers, and checks the reporter's choices against them, so a
// browser can only use labels that exist and assign people who may be assigned.
package options

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/uug-ai/inspect/server/internal/github"
)

// TTL is how long a repository's options are reused before they're fetched again.
const TTL = 5 * time.Minute

// Lister is implemented by *github.Client.
type Lister interface {
	ListLabels(ctx context.Context, repo string) ([]github.Label, error)
	ListAssignees(ctx context.Context, repo string) ([]github.User, error)
}

// Label is a label as shown in the picker.
type Label struct {
	Name        string `json:"name"`
	Color       string `json:"color"` // hex without '#'
	Description string `json:"description,omitempty"`
}

// Assignee is a user who can be assigned.
type Assignee struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// Options are what the pickers offer for one repository.
type Options struct {
	Labels    []Label    `json:"labels"`
	Assignees []Assignee `json:"assignees"`
}

type entry struct {
	opts    *Options
	fetched time.Time
}

// Cache fetches and caches options per repository.
type Cache struct {
	List Lister
	Now  func() time.Time // for tests

	mu      sync.Mutex
	entries map[string]entry
	locks   map[string]*sync.Mutex
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Get returns the repository's options, fetching them when missing or older than
// TTL. If a refresh fails but older options exist, those are returned.
func (c *Cache) Get(ctx context.Context, repo string) (*Options, error) {
	key := strings.ToLower(repo)
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]entry{}
		c.locks = map[string]*sync.Mutex{}
	}
	if e, ok := c.entries[key]; ok && c.now().Sub(e.fetched) < TTL {
		c.mu.Unlock()
		return e.opts, nil
	}
	lock := c.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		c.locks[key] = lock
	}
	c.mu.Unlock()

	// One fetch per repository at a time; others wait and reuse its result.
	lock.Lock()
	defer lock.Unlock()
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && c.now().Sub(e.fetched) < TTL {
		c.mu.Unlock()
		return e.opts, nil
	}
	stale, hasStale := c.entries[key]
	c.mu.Unlock()

	opts, err := c.fetch(ctx, repo)
	if err != nil {
		if hasStale {
			return stale.opts, nil
		}
		return nil, err
	}
	c.mu.Lock()
	c.entries[key] = entry{opts: opts, fetched: c.now()}
	c.mu.Unlock()
	return opts, nil
}

func (c *Cache) fetch(ctx context.Context, repo string) (*Options, error) {
	labels, err := c.List.ListLabels(ctx, repo)
	if err != nil {
		return nil, err
	}
	users, err := c.List.ListAssignees(ctx, repo)
	if err != nil {
		return nil, err
	}
	o := &Options{Labels: make([]Label, 0, len(labels)), Assignees: make([]Assignee, 0, len(users))}
	for _, l := range labels {
		o.Labels = append(o.Labels, Label{Name: l.Name, Color: l.Color, Description: l.Description})
	}
	for _, u := range users {
		o.Assignees = append(o.Assignees, Assignee{Login: u.Login, AvatarURL: u.AvatarURL})
	}
	return o, nil
}

// FilterLabels keeps the requested labels that exist in the repository, using the
// repository's spelling, without duplicates.
func (o *Options) FilterLabels(requested []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range requested {
		for _, l := range o.Labels {
			if strings.EqualFold(l.Name, strings.TrimSpace(r)) && !seen[strings.ToLower(l.Name)] {
				seen[strings.ToLower(l.Name)] = true
				out = append(out, l.Name)
				break
			}
		}
	}
	return out
}

// FilterAssignees keeps the requested logins that may be assigned in the repository.
func (o *Options) FilterAssignees(requested []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range requested {
		for _, a := range o.Assignees {
			if strings.EqualFold(a.Login, strings.TrimSpace(r)) && !seen[strings.ToLower(a.Login)] {
				seen[strings.ToLower(a.Login)] = true
				out = append(out, a.Login)
				break
			}
		}
	}
	return out
}
