package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/room"
	"urth/internal/store"
)

// command is one entry in the dispatch table.
type command struct {
	name string
	// minAbbrev is the fewest leading characters that select this command.
	// ROM makes dangerous commands like quit require more than one letter.
	minAbbrev int
	fn        func(w *World, p *Player, args string)
	// admin restricts the command to admin characters; non-admins get Huh?
	admin bool
}

// commands is searched in order. Earlier entries win ties on prefix, which is
// how "n" means north and not anything else starting with n. Keep movement
// first, as ROM does.
var commands = []command{
	{"north", 1, cmdMove("north"), false},
	{"east", 1, cmdMove("east"), false},
	{"south", 1, cmdMove("south"), false},
	{"west", 1, cmdMove("west"), false},
	{"up", 1, cmdMove("up"), false},
	{"down", 1, cmdMove("down"), false},
	{"look", 1, cmdLook, false},
	{"exits", 2, cmdExits, false},
	{"say", 1, cmdSay, false},
	{"who", 2, cmdWho, false},
	{"color", 3, cmdColor, false},
	{"save", 2, cmdSave, false},
	{"password", 4, cmdPassword, false},
	{"quit", 3, cmdQuit, false},
	{"copyover", 8, cmdCopyover, true},
	{"shutdown", 8, cmdShutdown, true},
}

// lookup finds the command a typed word selects for a player, or nil.
// Admin commands are invisible to non-admins.
func lookup(word string, admin bool) *command {
	word = strings.ToLower(word)
	for i := range commands {
		c := &commands[i]
		if c.name == word && (admin || !c.admin) {
			return c
		}
	}
	for i := range commands {
		c := &commands[i]
		if len(word) >= c.minAbbrev && strings.HasPrefix(c.name, word) && (admin || !c.admin) {
			return c
		}
	}
	return nil
}

// dispatch parses one input line and runs it.
func (w *World) dispatch(p *Player, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		p.SendMsg(output.Message{Type: output.Text, Text: ""})
		return
	}
	var word, args string
	if line[0] == '\'' {
		// ROM shorthand: 'hello  ==  say hello
		word, args = "say", strings.TrimSpace(line[1:])
	} else {
		word, args, _ = strings.Cut(line, " ")
		args = strings.TrimSpace(args)
	}
	c := lookup(word, p.Admin)
	if c == nil {
		p.Send("Huh?\n")
		return
	}
	c.fn(w, p, args)
}

func cmdMove(dir string) func(w *World, p *Player, args string) {
	return func(w *World, p *Player, _ string) {
		to, ok := p.Room.Exits[dir]
		if !ok {
			p.Send("Alas, you cannot go that way.\n")
			return
		}
		dest, ok := w.rooms.Get(to)
		if !ok {
			p.Send("That way leads nowhere.\n")
			return
		}
		w.act("$n leaves $t.", p, nil, dir, toRoom)
		p.Room = dest
		w.act("$n arrives from the $t.", p, nil, room.Opposite[dir], toRoom)
		w.look(p)
	}
}

func cmdLook(w *World, p *Player, _ string) { w.look(p) }

// look renders the player's room in ROM's layout, with structured data for
// clients that want it.
func (w *World) look(p *Player) {
	r := p.Room
	data := output.RoomData{Vnum: r.Vnum, Name: r.Name, Exits: r.ExitList(), Players: []string{}}
	var b strings.Builder
	b.WriteString("{C}" + output.Escape(r.Name) + "{x}\n")
	if r.Description != "" {
		b.WriteString(output.Escape(r.Description))
		b.WriteString("\n")
	}
	if len(data.Exits) == 0 {
		b.WriteString("{c}[Exits: none]{x}\n")
	} else {
		b.WriteString("{c}[Exits: " + strings.Join(data.Exits, " ") + "]{x}\n")
	}
	for _, other := range w.playersIn(r) {
		if other != p {
			data.Players = append(data.Players, other.Name)
			b.WriteString(output.Escape(other.Name) + " is here.\n")
		}
	}
	p.SendMsg(output.Message{Type: output.Room, Text: b.String(), Data: data})
}

func cmdExits(w *World, p *Player, _ string) {
	exits := p.Room.ExitList()
	if len(exits) == 0 {
		p.Send("Obvious exits:\nNone.\n")
		return
	}
	var b strings.Builder
	b.WriteString("Obvious exits:\n")
	for _, d := range exits {
		dest, _ := w.rooms.Get(p.Room.Exits[d])
		b.WriteString(padRight(strings.ToUpper(d[:1])+d[1:], 6) + "- " + output.Escape(dest.Name) + "\n")
	}
	p.Send(b.String())
}

func cmdSay(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Say what?\n")
		return
	}
	said := output.Escape(args)
	w.act("You say '{G}$t{x}'", p, nil, said, toChar)
	w.act("$n says '{G}$t{x}'", p, nil, said, toRoom)
}

func cmdWho(w *World, p *Player, _ string) {
	var names []string
	for _, other := range w.players {
		if other.State == StatePlaying {
			names = append(names, other.Name)
		}
	}
	sortStrings(names)
	var b strings.Builder
	b.WriteString("Players online:\n")
	for _, n := range names {
		b.WriteString("  " + output.Escape(n) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(plural(len(names), "player") + " online.\n")
	p.Send(b.String())
}

func cmdColor(_ *World, p *Player, _ string) {
	p.Color = !p.Color
	if p.Color {
		p.Send("{G}Color is now on.{x}\n")
	} else {
		p.Send("Color is now off.\n")
	}
}

func cmdSave(w *World, p *Player, _ string) {
	w.save(p)
	p.Send("Saved.\n")
}

// cmdPassword changes the password: password <old> <new>. Both checks run
// off the world goroutine.
func cmdPassword(w *World, p *Player, args string) {
	oldPw, newPw, _ := strings.Cut(args, " ")
	newPw = strings.TrimSpace(newPw)
	if oldPw == "" || newPw == "" {
		p.Send("Syntax: password <old> <new>\n")
		return
	}
	if len(newPw) < minPasswordLen || len(newPw) > maxPasswordLen {
		p.Send("New password must be 5 to 64 characters.\n")
		return
	}
	rec := p.rec
	go func() {
		if !store.CheckPassword(rec.PasswordHash, oldPw) {
			w.post(func() {
				if w.stillConnected(p) {
					p.Send("Wrong password. Nothing changed.\n")
				}
			})
			return
		}
		hash, err := w.store.HashPassword(newPw)
		w.post(func() {
			if !w.stillConnected(p) {
				return
			}
			if err != nil {
				p.Send("Something went wrong. Nothing changed.\n")
				return
			}
			rec.PasswordHash = hash
			w.save(p)
			p.Send("Password changed.\n")
		})
	}()
}

func cmdQuit(_ *World, p *Player, _ string) {
	p.Send("Alas, all good things must come to an end.\n")
	p.disconnect()
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return itoa(n) + " " + noun + "s"
}
