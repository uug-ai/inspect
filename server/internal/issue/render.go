package issue

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// zwj is inserted after "@" to stop GitHub from treating user text as a
// mention (which would notify that user or team). A zero-width joiner is
// invisible when rendered, so "@alice" still reads as "@alice", but GitHub's
// mention parser no longer matches it. The cost: copying such text yields an
// extra invisible character, including inside code blocks in the
// description. We accept that over letting anonymous reporters ping people.
const zwj = "‍"

// NeutraliseMentions inserts a zero-width joiner after every "@" that
// GitHub could parse as the start of a mention: an "@" at the start of the
// text or after a non-word character, followed by a letter or digit.
// "name@example.com" is left alone since GitHub does not treat it as a mention.
func NeutraliseMentions(s string) string {
	if !strings.Contains(s, "@") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	prev := rune(-1)
	for i, r := range s {
		b.WriteRune(r)
		if r == '@' && (prev == -1 || !isWordRune(prev)) {
			next, _ := utf8.DecodeRuneInString(s[i+1:])
			if next < utf8.RuneSelf && (unicode.IsLetter(next) || unicode.IsDigit(next)) {
				b.WriteString(zwj)
			}
		}
		prev = r
	}
	return b.String()
}

func isWordRune(r rune) bool {
	return r == '_' || r == '`' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// cell makes user text safe for a single markdown table cell: one line, no
// pipes, no backticks/HTML that could break out of the table or the
// surrounding <details>, and no mentions.
func cell(s string) string {
	if s == "" {
		return "—"
	}
	r := strings.NewReplacer(
		"\r\n", " ", "\n", " ", "\r", " ", "\t", " ",
		"\\", "\\\\",
		"|", "\\|",
		"`", "\\`",
		"<", "&lt;",
		">", "&gt;",
	)
	return NeutraliseMentions(r.Replace(s))
}

// fence returns a backtick fence longer than any backtick run in s, so the
// content cannot terminate the code block early.
func fence(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

// Attachment is a stored attachment as rendered in the body.
type Attachment struct {
	Name    string // e.g. screenshot-1.png
	URL     string
	IsVideo bool
}

// RenderBody builds the issue markdown.
func RenderBody(p *Payload, atts []Attachment) string {
	var b strings.Builder

	if p.Description != "" {
		b.WriteString(NeutraliseMentions(p.Description))
		b.WriteString("\n\n")
	} else {
		b.WriteString("_No description provided._\n\n")
	}

	for _, a := range atts {
		if a.IsVideo {
			// GitHub only inlines videos uploaded through its own UI, so link
			// it: a bare URL on its own line, plus an explicit link.
			fmt.Fprintf(&b, "%s\n\n[▶ %s](%s)\n\n", a.URL, a.Name, a.URL)
		} else {
			fmt.Fprintf(&b, "![%s](%s)\n\n", a.Name, a.URL)
		}
	}

	renderContext(&b, &p.Context)
	b.WriteString("<sub>Reported with inspect</sub>\n")
	return b.String()
}

// renderContext writes the collapsed Context section. Empty fields are
// omitted, and the whole section is omitted when nothing beyond the
// timestamp was supplied (the user opted out of technical context).
func renderContext(b *strings.Builder, c *Context) {
	type kv struct{ k, v string }
	var rows []kv
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			rows = append(rows, kv{k, cell(v)})
		}
	}
	add("URL", c.URL)
	add("Page title", c.Title)
	if c.Viewport != nil && (c.Viewport.Width != 0 || c.Viewport.Height != 0) {
		v := fmt.Sprintf("%s × %s", num(c.Viewport.Width), num(c.Viewport.Height))
		if c.Viewport.DPR != 0 {
			v += " (DPR " + num(c.Viewport.DPR) + ")"
		}
		rows = append(rows, kv{"Viewport", v}) // built from numbers only, no escaping needed
	}
	add("Browser", c.UserAgent)
	add("Selector", c.Selector)
	if c.Region != nil {
		rows = append(rows, kv{"Region", fmt.Sprintf("x=%s y=%s w=%s h=%s",
			num(c.Region.X), num(c.Region.Y), num(c.Region.Width), num(c.Region.Height))})
	}
	if len(rows) == 0 && len(c.App) == 0 && len(c.Console) == 0 {
		return
	}
	add("Time", c.Timestamp)

	b.WriteString("<details><summary>Context</summary>\n\n")
	b.WriteString("| | |\n|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(b, "| %s | %s |\n", r.k, r.v)
	}

	if len(c.App) > 0 {
		b.WriteString("\n**App**\n\n| Key | Value |\n|---|---|\n")
		keys := make([]string, 0, len(c.App))
		for k := range c.App {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(b, "| %s | %s |\n", cell(truncate(k, maxAppKey)), cell(truncate(appValue(c.App[k]), maxAppValue)))
		}
	}

	if len(c.Console) > 0 {
		var log strings.Builder
		for _, e := range c.Console {
			line := strings.ReplaceAll(e.Message, "\r\n", "\n")
			if e.Time != "" {
				fmt.Fprintf(&log, "[%s] ", oneLine(e.Time))
			}
			fmt.Fprintf(&log, "%s %s\n", strings.ToUpper(oneLine(e.Level)), line)
		}
		f := fence(log.String())
		fmt.Fprintf(b, "\n**Console** (%d)\n\n%stext\n%s%s\n", len(c.Console), f, log.String(), f)
	}

	b.WriteString("\n</details>\n\n")
}

func appValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		bs, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(bs)
	}
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
