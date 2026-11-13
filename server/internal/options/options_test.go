package options

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/uug-ai/inspect/server/internal/github"
)

type fakeLister struct {
	calls int
	err   error
}

func (f *fakeLister) ListLabels(context.Context, string) ([]github.Label, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return []github.Label{{Name: "bug", Color: "d73a4a"}, {Name: "Frontend", Color: "1d76db", Description: "UI"}}, nil
}

func (f *fakeLister) ListAssignees(context.Context, string) ([]github.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []github.User{{Login: "cedricve", AvatarURL: "https://avatars/1"}, {Login: "kilian"}}, nil
}

func TestCacheReusesThenRefreshes(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	f := &fakeLister{}
	c := &Cache{List: f, Now: func() time.Time { return now }}

	o, err := c.Get(context.Background(), "uug-ai/hub-frontend")
	if err != nil || len(o.Labels) != 2 || len(o.Assignees) != 2 || o.Labels[1].Description != "UI" {
		t.Fatalf("first get: %+v %v", o, err)
	}
	_, _ = c.Get(context.Background(), "UUG-AI/hub-frontend") // same repo, any case
	if f.calls != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", f.calls)
	}

	now = now.Add(TTL + time.Second)
	_, _ = c.Get(context.Background(), "uug-ai/hub-frontend")
	if f.calls != 2 {
		t.Fatalf("calls = %d, want 2 (refreshed after TTL)", f.calls)
	}

	// GitHub down: keep serving the last good options.
	now = now.Add(TTL + time.Second)
	f.err = errors.New("github down")
	if o, err := c.Get(context.Background(), "uug-ai/hub-frontend"); err != nil || len(o.Labels) != 2 {
		t.Fatalf("stale fallback: %+v %v", o, err)
	}
	// Never fetched before and GitHub down: error.
	if _, err := c.Get(context.Background(), "uug-ai/other"); err == nil {
		t.Fatal("want an error for a repo that was never fetched")
	}
}

func TestFilter(t *testing.T) {
	o := &Options{
		Labels:    []Label{{Name: "bug"}, {Name: "Frontend"}},
		Assignees: []Assignee{{Login: "cedricve"}, {Login: "kilian"}},
	}
	if got := o.FilterLabels([]string{"frontend", "BUG", "made-up", "bug"}); !reflect.DeepEqual(got, []string{"Frontend", "bug"}) {
		t.Errorf("labels %v", got)
	}
	if got := o.FilterAssignees([]string{"Kilian", "someone-else", "kilian"}); !reflect.DeepEqual(got, []string{"kilian"}) {
		t.Errorf("assignees %v", got)
	}
}
