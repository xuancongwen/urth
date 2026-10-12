package world

import (
	"strconv"
	"strings"

	"urth/internal/output"
)

// The pager holds a long reply to a page, as ROM's scroll does. A command
// whose text runs past the player's page length (inventory, skills, help,
// a merchant's list) shows the first page; Enter shows the next, and any
// other command drops the rest and runs. Only what a typed command says is
// paged: fight rounds, speech, and room descriptions always arrive whole.
// The tap client, which draws lists itself, is never paged.

const (
	// defaultPageLines is the page length until a player sets their own.
	defaultPageLines = 24
	minPageLines     = 10
	maxPageLines     = 200
)

// pageLines is how many lines a page holds for p, 0 when paging is off.
// The saved value is 0 for the default and negative for off.
func (p *Player) pageLines() int {
	if p.rec == nil || p.rec.Scroll == 0 {
		return defaultPageLines
	}
	return max(p.rec.Scroll, 0)
}

// command runs one typed line for a player in the game, paging what it
// says. A blank line (or "c") while a reply is held shows its next page.
func (w *World) command(p *Player, line string) {
	if len(p.more) > 0 {
		if t := strings.TrimSpace(line); t == "" || strings.EqualFold(t, "c") {
			p.showPage()
			return
		}
		p.more = nil
	}
	from := len(p.pending)
	w.dispatch(p, line)
	p.page(from)
}

// page holds back what the command queued from pending[from] on, past the
// first page, when it is long. Output that interleaves text with a room or
// speech is left alone, since holding part of it would reorder it.
func (p *Player) page(from int) {
	limit := p.pageLines()
	if limit == 0 || p.wantState || from > len(p.pending) {
		return
	}
	var text strings.Builder
	var data []output.Message
	for _, m := range p.pending[from:] {
		switch m.Type {
		case output.Text:
			text.WriteString(m.Text)
		case output.Expeditions, output.Offers, output.Reward, output.Death, output.State, output.Session, output.Commands:
			data = append(data, m) // not shown by text clients
		default:
			return
		}
	}
	lines := strings.SplitAfter(text.String(), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= limit {
		return
	}
	p.pending = append(p.pending[:from], data...)
	p.more = lines
	p.showPage()
}

// showPage sends the next page of a held reply and says how much is left.
func (p *Player) showPage() {
	limit := p.pageLines()
	if limit == 0 {
		limit = len(p.more) // paging was turned off mid-reply: show the rest
	}
	n := min(limit, len(p.more))
	p.Send(strings.Join(p.more[:n], ""))
	p.more = p.more[n:]
	if len(p.more) > 0 {
		p.Send("{x}{c}[ " + itoa(len(p.more)) + " more lines: Enter for the next page, or type a command ]{x}\n")
	} else {
		p.more = nil
	}
}

// cmdScroll: scroll | scroll <lines> | scroll off. Shows or sets how many
// lines a long reply shows before it waits for Enter.
func cmdScroll(w *World, p *Player, args string) {
	if p.rec == nil {
		return
	}
	arg := strings.ToLower(strings.TrimSpace(args))
	switch arg {
	case "":
		if n := p.pageLines(); n > 0 {
			p.Send("Long replies pause every " + itoa(n) + " lines. scroll <lines> changes it; scroll off stops it.\n")
		} else {
			p.Send("Long replies come all at once. scroll <lines> pauses them every so many lines.\n")
		}
		return
	case "off", "0":
		p.rec.Scroll = -1
		p.more = nil
		p.Send("Long replies will come all at once.\n")
	case "on":
		p.rec.Scroll = 0
		p.Send("Long replies will pause every " + itoa(defaultPageLines) + " lines.\n")
	default:
		n, err := strconv.Atoi(arg)
		if err != nil || n < minPageLines || n > maxPageLines {
			p.Send("Syntax: scroll <" + itoa(minPageLines) + " to " + itoa(maxPageLines) + "> | scroll on | scroll off\n")
			return
		}
		p.rec.Scroll = n
		if n == defaultPageLines {
			p.rec.Scroll = 0
		}
		p.Send("Long replies will pause every " + itoa(n) + " lines.\n")
	}
	w.save(p)
}

// toggleScroll is scroll's entry in settings: off if on, else the default.
func toggleScroll(w *World, p *Player, _ string) {
	if p.pageLines() > 0 {
		cmdScroll(w, p, "off")
	} else {
		cmdScroll(w, p, "on")
	}
}
