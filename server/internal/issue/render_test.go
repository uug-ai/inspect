package issue

import (
	"strings"
	"testing"
)

func TestNeutraliseMentions(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"no mentions", "no mentions"},
		{"@alice", "@‍alice"},
		{"hi @alice and @org/team!", "hi @‍alice and @‍org/team!"},
		{"(@bob)", "(@‍bob)"},
		{"line\n@carol", "line\n@‍carol"},
		{"mail me at dev@example.com", "mail me at dev@example.com"},
		{"@ alone", "@ alone"},
		{"@@dave", "@@‍dave"},
		{"@-dash", "@-dash"},
		{"`@code`", "`@code`"},
		{"x @9lives", "x @‍9lives"},
		{"émoji @ünicode", "émoji @ünicode"}, // not a valid GitHub login start
	}
	for _, tt := range tests {
		if got := NeutraliseMentions(tt.in); got != tt.want {
			t.Errorf("NeutraliseMentions(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCell(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "—"},
		{"plain", "plain"},
		{"a|b", `a\|b`},
		{"a\nb\r\nc", "a b c"},
		{"`code`", "\\`code\\`"},
		{"</details><script>", "&lt;/details&gt;&lt;script&gt;"},
		{`back\slash`, `back\\slash`},
		{"ping @eve", "ping @‍eve"},
		{"https://app.example.com/a_b?x=1", "https://app.example.com/a_b?x=1"},
	}
	for _, tt := range tests {
		if got := cell(tt.in); got != tt.want {
			t.Errorf("cell(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFence(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "```"},
		{"a ``` b", "````"},
		{"`````", "``````"},
	}
	for _, tt := range tests {
		if got := fence(tt.in); got != tt.want {
			t.Errorf("fence(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderBody(t *testing.T) {
	p := &Payload{
		Title:       "Button broken",
		Description: "The **save** button does nothing. cc @boss",
		Kind:        KindScreenshot,
		Context: Context{
			URL:       "https://app.example.com/settings",
			Title:     "Settings | App",
			UserAgent: "Mozilla/5.0",
			Viewport:  &Viewport{Width: 1440, Height: 900, DPR: 2},
			Timestamp: "2026-11-12T10:00:00Z",
			Selector:  "button.save",
			Region:    &Region{X: 10, Y: 20.5, Width: 100, Height: 50},
			Console: []ConsoleEntry{
				{Level: "error", Message: "TypeError: x is undefined\n    at foo", Time: "2026-11-12T09:59:59Z"},
				{Level: "warn", Message: "sneaky ``` fence"},
			},
			App: map[string]any{"version": "1.2.3", "user": "u|1", "build": 42.0},
		},
	}
	atts := []Attachment{
		{Name: "screenshot-1.png", URL: "https://github.com/o/r/blob/inspect-assets/inspect/x/screenshot-1.png?raw=true"},
		{Name: "recording-1.webm", URL: "https://github.com/o/r/blob/inspect-assets/inspect/x/recording-1.webm?raw=true", IsVideo: true},
	}
	body := RenderBody(p, atts)

	wantInOrder := []string{
		"The **save** button does nothing. cc @‍boss\n\n",
		"![screenshot-1.png](https://github.com/o/r/blob/inspect-assets/inspect/x/screenshot-1.png?raw=true)\n\n",
		"https://github.com/o/r/blob/inspect-assets/inspect/x/recording-1.webm?raw=true\n\n" +
			"[▶ recording-1.webm](https://github.com/o/r/blob/inspect-assets/inspect/x/recording-1.webm?raw=true)\n\n",
		"<details><summary>Context</summary>\n\n| | |\n|---|---|\n",
		"| URL | https://app.example.com/settings |\n",
		"| Page title | Settings \\| App |\n",
		"| Viewport | 1440 × 900 (DPR 2) |\n",
		"| Browser | Mozilla/5.0 |\n",
		"| Selector | button.save |\n",
		"| Region | x=10 y=20.5 w=100 h=50 |\n",
		"| Time | 2026-11-12T10:00:00Z |\n",
		"**App**\n\n| Key | Value |\n|---|---|\n| build | 42 |\n| user | u\\|1 |\n| version | 1.2.3 |\n",
		"**Console** (2)\n\n````text\n[2026-11-12T09:59:59Z] ERROR TypeError: x is undefined\n    at foo\nWARN sneaky ``` fence\n````\n",
		"</details>",
	}
	rest := body
	for _, w := range wantInOrder {
		i := strings.Index(rest, w)
		if i < 0 {
			t.Fatalf("body missing (or out of order) %q\n--- body ---\n%s", w, body)
		}
		rest = rest[i+len(w):]
	}
}

func TestRenderBodyContextOmission(t *testing.T) {
	tests := []struct {
		name        string
		ctx         Context
		want, avoid []string
	}{
		{
			name:  "no context at all",
			want:  []string{"_No description provided._", "<sub>Reported with inspect</sub>"},
			avoid: []string{"<details>", "| URL"},
		},
		{
			name:  "timestamp only (technical context unticked)",
			ctx:   Context{Timestamp: "2026-11-12T10:00:00Z"},
			avoid: []string{"<details>", "| Time"},
		},
		{
			name:  "partial context omits empty rows",
			ctx:   Context{URL: "https://a.example/", Timestamp: "2026-11-12T10:00:00Z", Viewport: &Viewport{}},
			want:  []string{"<details><summary>Context</summary>", "| URL | https://a.example/ |", "| Time | 2026-11-12T10:00:00Z |", "</details>"},
			avoid: []string{"Page title", "Viewport", "Browser", "Selector", "Region", "**App**", "**Console**", "—"},
		},
		{
			name:  "console only still renders block",
			ctx:   Context{Console: []ConsoleEntry{{Level: "error", Message: "boom"}}},
			want:  []string{"<details>", "ERROR boom"},
			avoid: []string{"| URL"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := RenderBody(&Payload{Title: "t", Kind: KindNote, Context: tt.ctx}, nil)
			for _, w := range tt.want {
				if !strings.Contains(body, w) {
					t.Errorf("missing %q in\n%s", w, body)
				}
			}
			for _, a := range tt.avoid {
				if strings.Contains(body, a) {
					t.Errorf("unexpected %q in\n%s", a, body)
				}
			}
		})
	}
}
