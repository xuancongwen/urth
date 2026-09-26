package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/room"
)

// help, chat, and yell. Help is built from the command table so it can
// never list a command that does not exist; descriptions live here.

type helpSection struct {
	title string
	names []string
}

var helpSections = []helpSection{
	{"Movement", []string{"north", "east", "south", "west", "up", "down", "look", "exits", "scan", "open", "close"}},
	{"Objects", []string{"get", "drop", "put", "give", "wear", "wield", "hold", "remove", "inventory", "equipment"}},
	{"Combat", []string{"kill", "flee", "consider", "assist", "skills"}},
	{"Magic", []string{"cast", "spells", "consume", "sacrifice"}},
	{"Groups", []string{"follow", "group", "gtell"}},
	{"Talking", []string{"say", "chat", "yell"}},
	{"Character", []string{"score", "train", "feat", "practice", "quest", "who", "color", "save", "password", "quit"}},
	{"Builder", []string{"goto", "at", "stat", "load", "purge", "force", "restore", "transfer", "peace", "reload", "simulate", "copyover", "shutdown"}},
	{"Admin", []string{"promote", "demote", "passwd", "deny", "allow", "users"}},
}

var helpText = map[string]string{
	"north": "north, east, south, west, up, down: walk through an exit. One letter is enough.",
	"look":  "look | look <thing> | look in <container> | look <direction>: the room, a character, an item and its numbers, what a container holds, or the way out in a direction and whether its door is shut. In the room listing, fixtures that cannot be taken read like the description, items you can pick up are green, and creatures are yellow. Only an unidentified item hides its numbers.",
	"exits": "exits: list the obvious ways out of this room and where they lead. A closed door is not obvious.",
	"scan":  "scan: who stands in each adjacent room. Nothing shows beyond a closed door or in the dark.",
	"open":  "open <direction|door>: open a door.",
	"close": "close <direction|door>: shut a door. A closed door blocks the way and hides the exit from both sides.",

	"get":       "get <item> | get all | get <item> <container> | get all <container>: pick things up, here or from a container (a corpse is a container).",
	"drop":      "drop <item> | drop all | drop <amount> silver|gold: put things or coins down.",
	"put":       "put <item> <container>: put an item into a container you can see.",
	"give":      "give <item> <character> | give <amount> silver|gold <character>: hand over an item or coins.",
	"wear":      "wear <item> | wear all: put on armor.",
	"wield":     "wield <weapon>: take up a weapon. Its verb is what your swings are called.",
	"hold":      "hold <item>: hold an item in your off hand. A torch or lantern is lit by wearing or holding it; dark rooms need one.",
	"remove":    "remove <item>: take off something worn, wielded, or held.",
	"inventory": "inventory: what you carry.",
	"equipment": "equipment: what you wear, wield, and hold.",

	"kill":     "kill <target>: attack. While fighting, kill <other> switches your target. Nobody can fight in a safe room.",
	"flee":     "flee: run through a random exit to end a fight.",
	"consider": "consider <target>: how the fight would go, and how hurt they look.",
	"assist":   "assist [member]: attack whatever a group member here is fighting.",
	"skills":   "skills: the skills your level allows and how good you are at each. Use one by name: kick, bash. One skill per round; they improve with use. Most are taught by trainers for coin; a few everyone knows.",
	"practice": "practice | practice <skill>: at a trainer, see what they teach and for how much, or pay to learn one. 100 silver is a gold.",
	"quest":    "quest request | info | complete | quit | points | list | buy <item>: a questmaster gives you a task, to slay a mob or recover a lost item somewhere in the world, with a time limit. Finish, come back, and 'quest complete' pays quest points. 'quest quit' abandons it and costs a longer wait. 'quest list' and 'quest buy' trade points for items with whoever sells them.",

	"cast":      "cast <spell> [target] | cast '<spell name>' [target]: cast a spell you know. Casting takes rounds; moving always interrupts, and some spells break when you are hit. Materials are spent when you begin.",
	"spells":    "spells: the spells you can cast, what they cost, and what is on cooldown.",
	"consume":   "consume <totem>: consume a totem to learn its school of magic.",
	"sacrifice": "sacrifice <thing>: offer a corpse or other item lying here to the gods; it vanishes and they leave you a coin. At a god's temple, giving up what the god wants makes you its apostle.",

	"follow": "follow <player> | follow self: follow someone, moving when they move, or go your own way.",
	"group":  "group | group <follower>: as leader, add someone following you to your group, or list the group. Grouped players share kills and assist each other.",
	"gtell":  "gtell <message>: talk to your group wherever they are.",

	"say":  "say <message> or '<message>: talk to the room.",
	"chat": "chat <message>: talk to everyone in the world.",
	"yell": "yell <message>: shout. Heard up to four rooms away along the exits.",

	"score":    "score: your character sheet.",
	"train":    "train | train <stat>: see your stats, or spend a stat point on one.",
	"feat":     "feat | feat <name>: see the feats you can learn, or spend a pick on one.",
	"who":      "who: who is playing. Invisible and hidden players are listed only if you can see them.",
	"color":    "color: toggle color.",
	"save":     "save: write your character to disk (it also saves on its own).",
	"password": "password <old> <new>: change your password.",
	"quit":     "quit: leave the game.",
	"help":     "help | help <command>: this list, or one command in detail.",

	"goto":     "goto <room|player|mob>: go there.",
	"at":       "at <room|player|mob> <command>: run a command as if you were there.",
	"stat":     "stat | stat <room|character|item>: everything the engine knows about it, including where its numbers came from.",
	"load":     "load mob <vnum> | load obj <vnum>: create one here.",
	"purge":    "purge [target]: destroy a mob or item here, or everything here.",
	"force":    "force <character> <command>: make them do it.",
	"restore":  "restore <player>: full health.",
	"transfer": "transfer <player> [room]: bring them here, or send them there.",
	"peace":    "peace: stop every fight in the room.",
	"reload":   "reload [scripts | area <name> | world]: re-read the rules, one area, or everything from disk.",
	"simulate": "simulate <mob | me | fighter[:level]> <mob> [fights] [seed]: run fights through the rules and report the numbers.",
	"copyover": "copyover: restart the server without disconnecting anyone.",
	"shutdown": "shutdown: stop the server.",

	"promote": "promote <player>: make a character an admin. Works on someone offline too.",
	"demote":  "demote <player>: take admin away. Not from yourself.",
	"passwd":  "passwd <player> <new password>: set someone else's password.",
	"deny":    "deny <player>: lock the account; they are dropped now and cannot log in until allowed.",
	"allow":   "allow <player>: undo deny.",
	"users":   "users: every connection, including those still logging in, with its address.",
}

// cmdHelp: help | help <command>
func cmdHelp(w *World, p *Player, args string) {
	if args != "" {
		word := strings.ToLower(strings.Fields(args)[0])
		c := lookup(word, p.Admin)
		if c == nil {
			if sk, ok := w.findSkill(p.Character, word); ok {
				use := "Use it by name"
				if sk.Passive {
					use = "It works on its own"
				}
				p.Send("{C}" + output.Escape(sk.Name) + "{x} (skill, level " + itoa(sk.Level) + ")\n  " + output.Escape(sk.Description) + " " + use + "; it improves with use.\n")
				return
			}
			p.Send("There is no command called that. Type 'help' for the list.\n")
			return
		}
		text, ok := helpText[c.name]
		if !ok {
			for _, dir := range []string{"east", "south", "west", "up", "down"} {
				if c.name == dir {
					text = helpText["north"]
					ok = true
				}
			}
		}
		if !ok {
			p.Send(output.Escape(c.name) + ": no help written yet.\n")
			return
		}
		p.Send("{C}" + output.Escape(c.name) + "{x}\n  " + output.Escape(text) + "\n")
		return
	}
	var b strings.Builder
	b.WriteString("Commands. Type 'help <command>' for one in detail; the first letters usually suffice.\n")
	for _, sec := range helpSections {
		var names []string
		for _, n := range sec.names {
			if c := lookup(n, p.Admin); c != nil && c.name == n {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			continue
		}
		b.WriteString("{c}" + padRight(sec.title, 11) + "{x}" + strings.Join(names, "  ") + "\n")
	}
	var skills []string
	for _, sk := range w.skillList() {
		if !sk.Passive && p.Level >= sk.Level {
			skills = append(skills, strings.ToLower(sk.Name))
		}
	}
	if len(skills) > 0 {
		b.WriteString("{c}" + padRight("Skills", 11) + "{x}" + strings.Join(skills, "  ") + "\n")
	}
	b.WriteString("{c}" + padRight("Also", 11) + "{x}help  and  ' as shorthand for say\n")
	p.Send(b.String())
}

// cmdChat: chat <message>. The world channel.
func cmdChat(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Chat what?\n")
		return
	}
	msg := output.Escape(args)
	for _, o := range w.players {
		if o.State != StatePlaying {
			continue
		}
		if o == p {
			o.Send("{M}You chat '" + msg + "'{x}\n")
		} else {
			o.Send("{M}" + p.DisplayName() + " chats '" + msg + "'{x}\n")
		}
	}
}

// yellRange is how many rooms a yell carries along the exits.
const yellRange = 4

// cmdYell: yell <message>. Heard in every room within yellRange exits.
func cmdYell(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Yell what?\n")
		return
	}
	msg := output.Escape(args)
	p.Send("{Y}You yell '" + msg + "'{x}\n")
	for _, r := range w.roomsWithin(p.Room, yellRange) {
		for _, o := range w.playersIn(r) {
			if o != p {
				o.Send("{Y}" + p.DisplayName() + " yells '" + msg + "'{x}\n")
			}
		}
	}
}

// roomsWithin returns every room reachable from start in at most n steps
// along exits, start included.
func (w *World) roomsWithin(start *room.Room, n int) []*room.Room {
	seen := map[int]bool{start.Vnum: true}
	frontier := []*room.Room{start}
	out := []*room.Room{start}
	for step := 0; step < n && len(frontier) > 0; step++ {
		var next []*room.Room
		for _, r := range frontier {
			for _, to := range r.Exits {
				if seen[to] {
					continue
				}
				seen[to] = true
				if dest, ok := w.content.Rooms.Get(to); ok {
					next = append(next, dest)
					out = append(out, dest)
				}
			}
		}
		frontier = next
	}
	return out
}
