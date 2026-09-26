package world

import (
	"strings"

	"urth/internal/item"
	"urth/internal/output"
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
var commands []command

// The table is built in init because at and force dispatch commands, which
// would otherwise be an initialization cycle.
func init() {
	commands = []command{
		{"north", 1, cmdMove("north"), false},
		{"east", 1, cmdMove("east"), false},
		{"south", 1, cmdMove("south"), false},
		{"west", 1, cmdMove("west"), false},
		{"up", 1, cmdMove("up"), false},
		{"down", 1, cmdMove("down"), false},
		{"look", 1, cmdLook, false},
		{"get", 1, cmdGet, false},
		{"inventory", 1, cmdInventory, false},
		{"exits", 2, cmdExits, false},
		{"scan", 3, cmdScan, false},
		{"open", 2, cmdOpen, false},
		{"close", 3, cmdClose, false},
		{"say", 1, cmdSay, false},
		{"chat", 2, cmdChat, false},
		{"yell", 1, cmdYell, false},
		{"drop", 2, cmdDrop, false},
		{"put", 1, cmdPut, false},
		{"give", 2, cmdGive, false},
		{"wear", 3, cmdWear, false},
		{"wield", 2, cmdWield, false},
		{"hold", 2, cmdHold, false},
		{"remove", 3, cmdRemove, false},
		{"equipment", 2, cmdEquipment, false},
		{"kill", 1, cmdKill, false},
		{"consider", 3, cmdConsider, false},
		{"flee", 2, cmdFlee, false},
		{"score", 2, cmdScore, false},
		{"train", 2, cmdTrain, false},
		{"feat", 3, cmdFeat, false},
		{"follow", 3, cmdFollow, false},
		{"group", 2, cmdGroup, false},
		{"gtell", 2, cmdGtell, false},
		{"assist", 2, cmdAssist, false},
		{"cast", 1, cmdCast, false},
		{"spells", 3, cmdSpells, false},
		{"skills", 3, cmdSkills, false},
		{"practice", 2, cmdPractice, false},
		{"quest", 2, cmdQuest, false},
		{"consume", 4, cmdConsume, false},
		{"sacrifice", 3, cmdSacrifice, false},
		{"who", 2, cmdWho, false},
		{"color", 3, cmdColor, false},
		{"save", 2, cmdSave, false},
		{"password", 4, cmdPassword, false},
		{"quit", 3, cmdQuit, false},
		{"help", 1, cmdHelp, false},
		{"goto", 2, cmdGoto, true},
		{"at", 2, cmdAt, true},
		{"stat", 3, cmdStat, true},
		{"load", 3, cmdLoad, true},
		{"purge", 3, cmdPurge, true},
		{"force", 3, cmdForce, true},
		{"restore", 4, cmdRestore, true},
		{"transfer", 3, cmdTransfer, true},
		{"peace", 3, cmdPeace, true},
		{"reload", 3, cmdReload, true},
		{"promote", 4, cmdPromote, true},
		{"demote", 4, cmdDemote, true},
		{"passwd", 6, cmdPasswd, true},
		{"deny", 4, cmdDeny, true},
		{"allow", 5, cmdAllow, true},
		{"users", 4, cmdUsers, true},
		{"simulate", 3, cmdSimulate, true},
		{"copyover", 8, cmdCopyover, true},
		{"shutdown", 8, cmdShutdown, true},
	}
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
		if w.trySkill(p, word, args) {
			return
		}
		p.Send("Huh?\n")
		return
	}
	c.fn(w, p, args)
}

func cmdMove(dir string) func(w *World, p *Player, args string) {
	return func(w *World, p *Player, _ string) {
		if p.Fighting != nil {
			p.Send("No way! You are fighting.\n")
			return
		}
		if _, ok := p.Room.Exits[dir]; !ok {
			p.Send("Alas, you cannot go that way.\n")
			return
		}
		if w.doorClosed(p.Room, dir) {
			p.Send(capitalize(output.Escape(p.Room.Door(dir).Name)) + " is closed.\n")
			return
		}
		dest := w.passable(p.Room, dir)
		if dest == nil {
			p.Send("That way leads nowhere.\n")
			return
		}
		w.interruptCast(p.Character, "You stop casting as you move.")
		w.act("$n leaves $t.", p.Character, nil, dir, toRoom)
		from := p.Room
		p.Room = dest
		w.act("$n arrives $t.", p.Character, nil, arrivesFrom(dir), toRoom)
		w.look(p)
		w.followLeader(p.Character, from, dest, dir)
	}
}

func cmdLook(w *World, p *Player, args string) {
	if args == "" {
		w.look(p)
		return
	}
	w.lookAt(p, args)
}

// look renders the player's room in ROM's layout, with structured data for
// clients that want it.
func (w *World) look(p *Player) {
	r := p.Room
	if p.visited == nil {
		p.visited = map[int]bool{}
	}
	p.visited[r.Vnum] = true
	if !w.canSee(p.Character) {
		p.SendMsg(output.Message{Type: output.Room, Text: "It is pitch black. You can't see a thing.\n",
			Data: output.RoomData{Vnum: r.Vnum, Name: "Darkness", Area: r.Area, Exits: w.openExits(r), Players: []string{}, Mobs: []output.Entity{}, Items: []output.Entity{}}})
		return
	}
	data := output.RoomData{Vnum: r.Vnum, Name: r.Name, Area: r.Area, Exits: w.openExits(r), Players: []string{}, Map: w.mapAround(r, p)}
	data.Mobs, data.Items = w.roomEntities(r, p)
	if data.Mobs == nil {
		data.Mobs = []output.Entity{}
	}
	if data.Items == nil {
		data.Items = []output.Entity{}
	}
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
	// Fixed objects are part of the room, so they follow the description
	// in its color; what can be picked up is green; creatures are yellow.
	groups := item.Group(w.contents(r).items)
	writeItems := func(fixed bool, color string) {
		for _, g := range groups {
			if g.Item.Proto.HasFlag("nopickup") != fixed {
				continue
			}
			b.WriteString(color)
			if g.Count > 1 {
				b.WriteString("(" + itoa(g.Count) + ") ")
			}
			b.WriteString(output.Escape(g.Item.Proto.Description))
			if color != "" {
				b.WriteString("{x}")
			}
			b.WriteString("\n")
		}
	}
	writeItems(true, "")
	writeItems(false, "{G}")
	for _, m := range w.contents(r).mobs {
		b.WriteString("{Y}" + output.Escape(m.Proto.Description) + "{x}\n")
	}
	for _, other := range w.playersIn(r) {
		if other != p {
			data.Players = append(data.Players, other.Name)
			b.WriteString(output.Escape(other.Name) + " is here.\n")
		}
	}
	p.SendMsg(output.Message{Type: output.Room, Text: b.String(), Data: data})
}

// cmdExits: exits. The ways out and where they lead. A closed door is not
// an obvious exit, so it is left out; look <direction> finds it.
func cmdExits(w *World, p *Player, _ string) {
	if !w.canSee(p.Character) {
		p.Send("You can't see a thing.\n")
		return
	}
	exits := w.openExits(p.Room)
	if len(exits) == 0 {
		p.Send("Obvious exits:\nNone.\n")
		return
	}
	var b strings.Builder
	b.WriteString("Obvious exits:\n")
	for _, d := range exits {
		dest := w.passable(p.Room, d)
		if dest == nil {
			continue
		}
		b.WriteString(padRight(capitalize(d), 6) + "- " + output.Escape(dest.Name) + "\n")
	}
	p.Send(b.String())
}

func cmdSay(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Say what?\n")
		return
	}
	said := output.Escape(args)
	w.act("You say '{G}$t{x}'", p.Character, nil, said, toChar)
	w.act("$n says '{G}$t{x}'", p.Character, nil, said, toRoom)
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
