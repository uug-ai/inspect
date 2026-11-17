// Package agents lets a reporter hand an issue to an AI coding agent (Claude
// Code, GitHub Copilot, Codex, …) so it is picked up and resolved without a
// person triaging it first.
//
// Inspect doesn't run agents itself. Each agent already listens to GitHub in
// its own way, and inspect only does what that agent waits for, right after
// the issue is created:
//
//   - assignee: assign a bot account (GitHub's coding agents: Copilot, Claude, Codex)
//   - label:    add a label (e.g. Claude Code's GitHub Action with label_trigger)
//   - comment:  post a comment that mentions the agent (e.g. a trigger phrase)
//
// The agents on offer come from INSPECT_AGENTS; nothing is offered by default.
package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Agent is one AI agent the reporter may pick, and how it is triggered.
// At least one of Label, Assignee and Comment is set.
type Agent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Label is added to the issue once it exists.
	Label string `json:"label,omitempty"`
	// Assignee is a (bot) login assigned to the issue once it exists.
	Assignee string `json:"assignee,omitempty"`
	// Comment is posted on the issue once it exists. "{number}" and "{url}"
	// are replaced with the issue's number and URL.
	Comment string `json:"comment,omitempty"`
	// Repos limits the agent to these repositories (owner/name); empty means
	// every allowed repository.
	Repos []string `json:"repos,omitempty"`
}

// Presets are the agents that can be enabled by id alone. All three are
// GitHub coding agents (Agent HQ): assigning the bot starts a session that
// opens a pull request. The token's account needs a Copilot seat that
// includes the agent, and the agent must be enabled for the repository.
// To trigger an agent another way, define it in the JSON form, for example
// {"id":"claude","label":"claude"} for anthropics/claude-code-action with
// label_trigger: claude.
var Presets = map[string]Agent{
	"copilot": {ID: "copilot", Name: "Copilot", Assignee: "copilot-swe-agent[bot]"},
	"claude":  {ID: "claude", Name: "Claude", Assignee: "anthropic-code-agent[bot]"},
	"codex":   {ID: "codex", Name: "Codex", Assignee: "openai-code-agent[bot]"},
}

// Set is the configured agents, in the order they're offered.
type Set []Agent

// Parse reads INSPECT_AGENTS: either a comma-separated list of preset ids
// ("claude,copilot") or a JSON array of agents. In the JSON form an entry
// whose id is a preset starts from that preset, so {"id":"claude","label":"ai"}
// only changes the label.
func Parse(s string) (Set, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var raw []json.RawMessage
	if strings.HasPrefix(s, "[") {
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	} else {
		for _, id := range strings.Split(s, ",") {
			if id = strings.TrimSpace(id); id != "" {
				b, _ := json.Marshal(map[string]string{"id": id})
				raw = append(raw, b)
			}
		}
	}

	var out Set
	seen := map[string]bool{}
	var errs []error
	for _, r := range raw {
		var head struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(r, &head); err != nil {
			errs = append(errs, fmt.Errorf("invalid entry: %w", err))
			continue
		}
		id := strings.ToLower(strings.TrimSpace(head.ID))
		a, preset := Presets[id] // a custom agent starts empty
		if err := json.Unmarshal(r, &a); err != nil {
			errs = append(errs, fmt.Errorf("agent %q: %w", head.ID, err))
			continue
		}
		a.ID = id
		a.Name = strings.TrimSpace(a.Name)
		a.Label = strings.TrimSpace(a.Label)
		a.Assignee = strings.TrimPrefix(strings.TrimSpace(a.Assignee), "@")
		a.Comment = strings.TrimSpace(a.Comment)
		switch {
		case !validID(id):
			errs = append(errs, fmt.Errorf("agent id %q: use 1-32 of a-z, 0-9, - and _", head.ID))
			continue
		case seen[id]:
			errs = append(errs, fmt.Errorf("agent %q is listed twice", id))
			continue
		case !preset && a.Name == "" && a.Label == "" && a.Assignee == "" && a.Comment == "":
			errs = append(errs, fmt.Errorf("agent %q is not a preset (claude, copilot, codex); define it in the JSON form with a label, assignee or comment", id))
			continue
		case a.Label == "" && a.Assignee == "" && a.Comment == "":
			errs = append(errs, fmt.Errorf("agent %q needs a label, assignee or comment to trigger it", id))
			continue
		}
		if a.Name == "" {
			a.Name = id
		}
		seen[id] = true
		out = append(out, a)
	}
	return out, errors.Join(errs...)
}

// For returns the agents offered for repo.
func (s Set) For(repo string) Set {
	var out Set
	for _, a := range s {
		if a.allows(repo) {
			out = append(out, a)
		}
	}
	return out
}

// Pick returns the requested agents (by id, case-insensitive) that are offered
// for repo, without duplicates. Unknown ids are ignored.
func (s Set) Pick(repo string, ids []string) Set {
	var out Set
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		for _, a := range s {
			if a.ID == id && !seen[id] && a.allows(repo) {
				seen[id] = true
				out = append(out, a)
			}
		}
	}
	return out
}

// Repos returns every repository named in a Repos restriction.
func (s Set) Repos() []string {
	var out []string
	for _, a := range s {
		out = append(out, a.Repos...)
	}
	return out
}

func (a Agent) allows(repo string) bool {
	if len(a.Repos) == 0 {
		return true
	}
	for _, r := range a.Repos {
		if strings.EqualFold(r, repo) {
			return true
		}
	}
	return false
}

// RenderComment fills in the comment template.
func (a Agent) RenderComment(number int, url string) string {
	return strings.NewReplacer("{number}", fmt.Sprint(number), "{url}", url).Replace(a.Comment)
}

func validID(id string) bool {
	if id == "" || len(id) > 32 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
