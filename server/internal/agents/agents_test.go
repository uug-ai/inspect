package agents

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePresets(t *testing.T) {
	s, err := Parse(" claude, Copilot ,codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 3 || !reflect.DeepEqual(s[0], Presets["claude"]) || !reflect.DeepEqual(s[1], Presets["copilot"]) || s[2].Assignee != "openai-code-agent[bot]" {
		t.Fatalf("got %+v", s)
	}
	if s, err := Parse(""); s != nil || err != nil {
		t.Fatalf("empty: %v %v", s, err)
	}
}

func TestParseJSON(t *testing.T) {
	s, err := Parse(`[{"id":"claude","label":"ai-fix","repos":["uug-ai/hub-frontend"]},
		{"id":"gemini","name":"Gemini","comment":"@gemini-cli /fix #{number}"}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := Set{
		{ID: "claude", Name: "Claude", Assignee: "anthropic-code-agent[bot]", Label: "ai-fix", Repos: []string{"uug-ai/hub-frontend"}},
		{ID: "gemini", Name: "Gemini", Comment: "@gemini-cli /fix #{number}"},
	}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("got %+v", s)
	}
	if c := s[1].RenderComment(12, "u"); c != "@gemini-cli /fix #12" {
		t.Errorf("comment %q", c)
	}
}

func TestParseErrors(t *testing.T) {
	for in, want := range map[string]string{
		"devin":                         "not a preset",
		"claude,claude":                 "twice",
		`[{"id":"x","name":"X"}]`:       "needs a label",
		`[{"id":"Bad Id","label":"l"}]`: "use 1-32",
		`[{"id":`:                       "invalid JSON",
	} {
		if _, err := Parse(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err %v, want %q", in, err, want)
		}
	}
}

func TestForAndPick(t *testing.T) {
	s := Set{
		{ID: "claude", Name: "Claude Code", Label: "claude", Repos: []string{"o/front"}},
		{ID: "copilot", Name: "Copilot", Assignee: "copilot-swe-agent[bot]"},
	}
	if got := s.For("O/Front"); len(got) != 2 {
		t.Errorf("front: %+v", got)
	}
	if got := s.For("o/back"); len(got) != 1 || got[0].ID != "copilot" {
		t.Errorf("back: %+v", got)
	}
	got := s.Pick("o/back", []string{"Claude", "copilot", "copilot", "nope"})
	if len(got) != 1 || got[0].ID != "copilot" {
		t.Errorf("pick: %+v", got)
	}
}

func TestParseReadmeExample(t *testing.T) {
	s, err := Parse(`[{"id":"copilot"},{"id":"claude","name":"Claude Code","assignee":"","label":"claude","repos":["uug-ai/hub-frontend"]}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := Agent{ID: "claude", Name: "Claude Code", Label: "claude", Repos: []string{"uug-ai/hub-frontend"}}
	if len(s) != 2 || !reflect.DeepEqual(s[0], Presets["copilot"]) || !reflect.DeepEqual(s[1], want) {
		t.Fatalf("got %+v", s)
	}
}
