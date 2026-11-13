// Package issue validates submissions, renders the issue body and creates
// the issue.
package issue

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kinds of submission.
const (
	KindScreenshot = "screenshot"
	KindRecording  = "recording"
	KindNote       = "note"
)

// Limits applied to the payload. Hard limits reject the request; soft limits
// (context fields) truncate, because context is best-effort diagnostics.
const (
	MaxTitle       = 256
	MaxDescription = 20000
	MaxLabels      = 20
	MaxAssignees   = 10
	MaxLabelLen    = 50

	maxURL        = 2048
	maxShortField = 512
	maxSelector   = 1024
	maxConsole    = 50
	maxConsoleMsg = 2000
	maxAppKeys    = 50
	maxAppKey     = 100
	maxAppValue   = 1000
)

// Payload is the JSON "payload" form field.
type Payload struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Repo        string   `json:"repo"`
	Labels      []string `json:"labels"`
	Assignees   []string `json:"assignees"`
	Context     Context  `json:"context"`
}

// Context is browser/page diagnostics captured by the client.
type Context struct {
	URL       string         `json:"url"`
	Title     string         `json:"title"`
	UserAgent string         `json:"userAgent"`
	Viewport  *Viewport      `json:"viewport"`
	Timestamp string         `json:"timestamp"`
	Selector  string         `json:"selector"`
	Region    *Region        `json:"region"`
	Console   []ConsoleEntry `json:"console"`
	App       map[string]any `json:"app"` // values are expected to be strings; others are rendered as JSON
}

// Viewport is the browser viewport.
type Viewport struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	DPR    float64 `json:"dpr"`
}

// Region is the selected page region in CSS pixels.
type Region struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ConsoleEntry is one captured console message.
type ConsoleEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

// ValidationError is a client error (HTTP 400).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, a ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, a...)}
}

// ParsePayload decodes and validates the payload JSON. On success the
// payload is normalised (trimmed, context truncated).
func ParsePayload(b []byte) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, invalid("payload: invalid JSON: %v", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate checks hard limits and truncates soft ones.
func (p *Payload) Validate() error {
	p.Title = strings.Join(strings.FieldsFunc(p.Title, unicode.IsControl), " ")
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" {
		return invalid("title is required")
	}
	if n := utf8.RuneCountInString(p.Title); n > MaxTitle {
		return invalid("title is too long (%d > %d characters)", n, MaxTitle)
	}

	p.Description = strings.TrimSpace(strings.ToValidUTF8(p.Description, "�"))
	if n := utf8.RuneCountInString(p.Description); n > MaxDescription {
		return invalid("description is too long (%d > %d characters)", n, MaxDescription)
	}

	switch p.Kind {
	case KindScreenshot, KindRecording, KindNote:
	default:
		return invalid("kind must be one of screenshot, recording, note")
	}

	p.Repo = strings.TrimSpace(p.Repo)

	if len(p.Assignees) > MaxAssignees {
		return invalid("too many assignees (max %d)", MaxAssignees)
	}
	for i, a := range p.Assignees {
		a = strings.TrimPrefix(strings.TrimSpace(a), "@")
		if a == "" || len(a) > 39 {
			return invalid("invalid assignee %q", a)
		}
		p.Assignees[i] = a
	}

	if len(p.Labels) > MaxLabels {
		return invalid("too many labels (max %d)", MaxLabels)
	}
	for i, l := range p.Labels {
		l = strings.TrimSpace(l)
		if l == "" || utf8.RuneCountInString(l) > MaxLabelLen {
			return invalid("label %d must be 1..%d characters", i, MaxLabelLen)
		}
		p.Labels[i] = l
	}

	c := &p.Context
	c.URL = truncate(c.URL, maxURL)
	c.Title = truncate(c.Title, maxShortField)
	c.UserAgent = truncate(c.UserAgent, maxShortField)
	c.Timestamp = truncate(c.Timestamp, 64)
	c.Selector = truncate(c.Selector, maxSelector)
	if len(c.Console) > maxConsole {
		c.Console = c.Console[len(c.Console)-maxConsole:] // keep the most recent
	}
	for i := range c.Console {
		e := &c.Console[i]
		e.Level = truncate(e.Level, 16)
		e.Time = truncate(e.Time, 64)
		e.Message = truncate(e.Message, maxConsoleMsg)
	}
	if len(c.App) > maxAppKeys {
		return invalid("context.app has too many keys (max %d)", maxAppKeys)
	}
	return nil
}

// truncate shortens s to max runes (marking the cut) and fixes bad UTF-8.
func truncate(s string, max int) string {
	s = strings.ToValidUTF8(s, "�")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}
