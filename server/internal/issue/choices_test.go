package issue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/uug-ai/inspect/server/internal/config"
	"github.com/uug-ai/inspect/server/internal/github"
	"github.com/uug-ai/inspect/server/internal/options"
	"github.com/uug-ai/inspect/server/internal/store"
)

type captureIssues struct{ got github.IssueRequest }

func (c *captureIssues) CreateIssue(_ context.Context, _ string, in github.IssueRequest) (*github.Issue, error) {
	c.got = in
	return &github.Issue{Number: 7, HTMLURL: "https://github.com/o/r/issues/7"}, nil
}

type fixedOptions struct {
	o   *options.Options
	err error
}

func (f fixedOptions) Get(context.Context, string) (*options.Options, error) { return f.o, f.err }

type nopStore struct{}

func (nopStore) Put(context.Context, string, string, []byte) (string, error) { return "https://x", nil }

func submitWith(t *testing.T, opts RepoOptions, allowed []string, p Payload) github.IssueRequest {
	t.Helper()
	issues := &captureIssues{}
	s := &Service{
		Cfg:      &config.Config{DefaultRepo: "o/r", AllowedRepos: []string{"o/r"}, Labels: []string{"inspect"}, AllowedLabels: allowed},
		Issues:   issues,
		StoreFor: func(string) store.Store { return nopStore{} },
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Options:  opts,
	}
	p.Title, p.Kind = "t", KindNote
	if _, err := s.Submit(context.Background(), &p, nil); err != nil {
		t.Fatal(err)
	}
	return issues.got
}

func TestChoices(t *testing.T) {
	repo := &options.Options{
		Labels:    []options.Label{{Name: "bug"}, {Name: "Frontend"}},
		Assignees: []options.Assignee{{Login: "cedricve"}},
	}
	picks := Payload{Labels: []string{"frontend", "made-up"}, Assignees: []string{"cedricve", "stranger"}}

	t.Run("picker on: existing labels and assignable users only", func(t *testing.T) {
		got := submitWith(t, fixedOptions{o: repo}, nil, picks)
		if !reflect.DeepEqual(got.Labels, []string{"inspect", "Frontend"}) || !reflect.DeepEqual(got.Assignees, []string{"cedricve"}) {
			t.Errorf("labels %v assignees %v", got.Labels, got.Assignees)
		}
	})
	t.Run("allowlist still applies", func(t *testing.T) {
		got := submitWith(t, fixedOptions{o: repo}, []string{"bug"}, Payload{Labels: []string{"frontend", "bug"}})
		if !reflect.DeepEqual(got.Labels, []string{"inspect", "bug"}) {
			t.Errorf("labels %v", got.Labels)
		}
	})
	t.Run("GitHub unavailable: filed without picks", func(t *testing.T) {
		got := submitWith(t, fixedOptions{err: errors.New("down")}, nil, picks)
		if !reflect.DeepEqual(got.Labels, []string{"inspect"}) || got.Assignees != nil {
			t.Errorf("labels %v assignees %v", got.Labels, got.Assignees)
		}
	})
	t.Run("picker off: no client labels without an allowlist, no assignees", func(t *testing.T) {
		got := submitWith(t, nil, nil, picks)
		if !reflect.DeepEqual(got.Labels, []string{"inspect"}) || got.Assignees != nil {
			t.Errorf("labels %v assignees %v", got.Labels, got.Assignees)
		}
	})
}

func TestPayloadAssignees(t *testing.T) {
	p := Payload{Title: "t", Kind: KindNote, Assignees: []string{" @cedricve "}}
	if err := p.Validate(); err != nil || p.Assignees[0] != "cedricve" {
		t.Fatalf("%v %v", err, p.Assignees)
	}
	p.Assignees = make([]string, MaxAssignees+1)
	for i := range p.Assignees {
		p.Assignees[i] = "x"
	}
	if err := p.Validate(); err == nil {
		t.Fatal("too many assignees should fail")
	}
}
