package output

import (
	"html"
	"strings"
)

// Color tokens are three characters: an open brace, a code, a close brace.
// "{r}" is red, "{R}" bright red, "{x}" reset. A literal brace is "{{".
// Player-supplied text must pass through Escape before it is embedded.
//
//	d  black     D  dark grey
//	r  red       R  bright red
//	g  green     G  bright green
//	y  yellow    Y  bright yellow
//	b  blue      B  bright blue
//	m  magenta   M  bright magenta
//	c  cyan      C  bright cyan
//	w  white     W  bright white
//	x  reset
var ansiCodes = map[byte]string{
	'x': "\033[0m",
	'd': "\033[0;30m", 'D': "\033[1;30m",
	'r': "\033[0;31m", 'R': "\033[1;31m",
	'g': "\033[0;32m", 'G': "\033[1;32m",
	'y': "\033[0;33m", 'Y': "\033[1;33m",
	'b': "\033[0;34m", 'B': "\033[1;34m",
	'm': "\033[0;35m", 'M': "\033[1;35m",
	'c': "\033[0;36m", 'C': "\033[1;36m",
	'w': "\033[0;37m", 'W': "\033[1;37m",
}

// Escape makes player-typed text safe to embed: braces become literal.
func Escape(s string) string { return strings.ReplaceAll(s, "{", "{{") }

// walk scans s, calling text for literal runs and code for each color token.
func walk(s string, text func(string), code func(byte)) {
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '{' {
			text(s[start : i+1]) // one literal brace
			i++
			start = i + 1
			continue
		}
		if i+2 < len(s) && s[i+2] == '}' {
			if _, ok := ansiCodes[s[i+1]]; ok {
				text(s[start:i])
				code(s[i+1])
				i += 2
				start = i + 1
			}
		}
	}
	text(s[start:])
}

// ANSI renders tokens as terminal escape sequences and appends a reset if
// any color was used, so a client never inherits stray color.
func ANSI(s string) string {
	var b strings.Builder
	var last byte
	walk(s,
		func(t string) { b.WriteString(t) },
		func(c byte) { last = c; b.WriteString(ansiCodes[c]) },
	)
	if last != 0 && last != 'x' {
		b.WriteString(ansiCodes['x'])
	}
	return b.String()
}

// Strip removes tokens, leaving plain text.
func Strip(s string) string {
	var b strings.Builder
	walk(s, func(t string) { b.WriteString(t) }, func(byte) {})
	return b.String()
}

// HTML escapes the text and wraps colored runs in <span class="c-CODE">.
func HTML(s string) string {
	var b strings.Builder
	open := false
	closeSpan := func() {
		if open {
			b.WriteString("</span>")
			open = false
		}
	}
	walk(s,
		func(t string) { b.WriteString(html.EscapeString(t)) },
		func(c byte) {
			closeSpan()
			if c != 'x' {
				b.WriteString(`<span class="c-` + string(c) + `">`)
				open = true
			}
		},
	)
	closeSpan()
	return b.String()
}
