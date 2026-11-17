package issue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/uug-ai/inspect/server/internal/agents"
	"github.com/uug-ai/inspect/server/internal/config"
	"github.com/uug-ai/inspect/server/internal/store"
)

type recordTrigger struct {
	calls []string
	fail  string // call prefix that fails
}

func (r *recordTrigger) record(call string) error {
	r.calls = append(r.calls, call)
	if r.fail != "" && strings.HasPrefix(call, r.fail) {
		return errors.New("boom")
	}
	return nil
}

func (r *recordTrigger) AddLabels(_ context.Context, repo string, n int, l []string) error {
	return r.record("label " + repo + " " + strings.Join(l, ","))
}

func (r *recordTrigger) AddAssignees(_ context.Context, repo string, n int, l []string) error {
	return r.record("assign " + repo + " " + strings.Join(l, ","))
}

func (r *recordTrigger) CreateComment(_ context.Context, repo string, n int, body string) error {
	return r.record("comment " + repo + " " + body)
}

func TestHandOffToAgents(t *testing.T) {
	set, err := agents.Parse("claude,copilot,codex")
	if err != nil {
		t.Fatal(err)
	}
	set[0] = agents.Agent{ID: "claude", Name: "Claude Code", Label: "claude"} // the Actions route
	set[2].Comment = "@codex fix #{number}"
	newService := func(tr *recordTrigger, issues *captureIssues) *Service {
		return &Service{
			Cfg:      &config.Config{DefaultRepo: "o/r", AllowedRepos: []string{"o/r"}, Agents: set},
			Issues:   issues,
			Agents:   tr,
			StoreFor: func(string) store.Store { return nopStore{} },
			Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
	}

	t.Run("each agent triggered its own way", func(t *testing.T) {
		tr, issues := &recordTrigger{}, &captureIssues{}
		res, err := newService(tr, issues).Submit(context.Background(), &Payload{Title: "t", Kind: KindNote, Agents: []string{"codex", "claude", "copilot", "unknown"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"assign o/r openai-code-agent[bot]", "comment o/r @codex fix #7", "label o/r claude", "assign o/r copilot-swe-agent[bot]"}
		if !reflect.DeepEqual(tr.calls, want) {
			t.Errorf("calls %q", tr.calls)
		}
		if len(res.Agents) != 3 || !res.Agents[0].OK || res.Agents[1].Name != "Claude Code" {
			t.Errorf("result %+v", res.Agents)
		}
		if !strings.Contains(issues.got.Body, "handed this issue to Codex and Claude Code and Copilot") {
			t.Errorf("body %q", issues.got.Body)
		}
	})

	t.Run("a failed trigger keeps the issue", func(t *testing.T) {
		tr := &recordTrigger{fail: "assign"}
		res, err := newService(tr, &captureIssues{}).Submit(context.Background(), &Payload{Title: "t", Kind: KindNote, Agents: []string{"copilot", "claude"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.Number != 7 || res.Agents[0].OK || res.Agents[0].Error == "" || !res.Agents[1].OK {
			t.Errorf("result %+v", res)
		}
	})

	t.Run("no agents picked: nothing extra", func(t *testing.T) {
		tr, issues := &recordTrigger{}, &captureIssues{}
		res, _ := newService(tr, issues).Submit(context.Background(), &Payload{Title: "t", Kind: KindNote}, nil)
		if len(tr.calls) != 0 || res.Agents != nil || strings.Contains(issues.got.Body, "handed") {
			t.Errorf("calls %q result %+v", tr.calls, res)
		}
	})
}
