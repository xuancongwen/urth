package output

import "testing"

func TestANSI(t *testing.T) {
	cases := map[string]string{
		"plain":             "plain",
		"{R}hot{x} cold":    "\033[1;31mhot\033[0m cold",
		"{g}green":          "\033[0;32mgreen\033[0m",
		"a {{ brace":        "a { brace",
		"{q}unknown":        "{q}unknown",
		"{r":                "{r",
		"{{r}":              "{r}",
		"end{x}":            "end\033[0m",
		"{R}{G}multi{x}{x}": "\033[1;31m\033[1;32mmulti\033[0m\033[0m",
	}
	for in, want := range cases {
		if got := ANSI(in); got != want {
			t.Errorf("ANSI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStrip(t *testing.T) {
	cases := map[string]string{
		"{R}hot{x} cold": "hot cold",
		"a {{ brace":     "a { brace",
		"{q}unknown":     "{q}unknown",
		"":               "",
	}
	for in, want := range cases {
		if got := Strip(in); got != want {
			t.Errorf("Strip(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHTML(t *testing.T) {
	cases := map[string]string{
		"{R}hot{x} cold": `<span class="c-R">hot</span> cold`,
		"{r}a{g}b":       `<span class="c-r">a</span><span class="c-g">b</span>`,
		"<b> & {{":       "&lt;b&gt; &amp; {",
		"say '{r}x{x}'":  `say &#39;<span class="c-r">x</span>&#39;`,
	}
	for in, want := range cases {
		if got := HTML(in); got != want {
			t.Errorf("HTML(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeRoundTrip(t *testing.T) {
	user := "I typed {R}this{x}"
	if got := Strip(Escape(user)); got != user {
		t.Fatalf("escaped user text altered: %q", got)
	}
	if got := ANSI(Escape(user)); got != user {
		t.Fatalf("escaped user text colored: %q", got)
	}
}

func TestRenderText(t *testing.T) {
	b := Batch{Messages: []Message{
		{Type: Text, Text: "{C}Hub{x}\n"},
		{Type: Text, Text: "Bob is here.\n"},
		{Type: Prompt, Text: "> "},
	}}
	if got := RenderText(b); got != "Hub\nBob is here.\n\n> " {
		t.Fatalf("plain: %q", got)
	}
	b.Color = true
	if got := RenderText(b); got != "\033[1;36mHub\033[0m\nBob is here.\n\n> " {
		t.Fatalf("color: %q", got)
	}
}
