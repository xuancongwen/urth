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
	{"Movement", []string{"north", "east", "south", "west", "up", "down", "look", "exits", "scan", "open", "close", "lock", "unlock", "pick", "recall"}},
	{"Objects", []string{"get", "drop", "put", "give", "wear", "wield", "second", "hold", "remove", "inventory", "equipment"}},
	{"Combat", []string{"kill", "flee", "consider", "assist", "skills", "autoloot", "autogold", "autosac"}},
	{"Magic", []string{"cast", "spells", "swear", "brandish", "study", "consume", "sacrifice"}},
	{"Trade", []string{"list", "buy", "sell"}},
	{"Groups", []string{"follow", "group", "gtell"}},
	{"Talking", []string{"say", "tell", "reply", "chat", "yell"}},
	{"Resting", []string{"rest", "sleep", "wake", "stand", "affects"}},
	{"Character", []string{"score", "stats", "settings", "scroll", "train", "feat", "practice", "quest", "who", "color", "save", "password", "expedition", "client", "quit"}},
	{"Builder", []string{"goto", "at", "stat", "load", "purge", "force", "restore", "transfer", "peace", "reload", "simulate", "loot", "copyover", "shutdown"}},
	{"Admin", []string{"outfit", "promote", "demote", "passwd", "deny", "allow", "users"}},
}

var helpText = map[string]string{
	"north":    "north, east, south, west, up, down: walk through an exit. One letter is enough.",
	"look":     "look | look <thing> | look in <container> | look <direction>: the room, a character, an item and its numbers, what a container holds, or the way out in a direction and whether its door is shut. In the room listing, fixtures that cannot be taken read like the description, items you can pick up are green, and creatures are yellow. Only an unidentified item hides its numbers.",
	"exits":    "exits: list the obvious ways out of this room and where they lead. A closed door is not obvious.",
	"scan":     "scan: who stands here with you and in each adjacent room. Nothing shows beyond a closed door or in the dark.",
	"open":     "open <direction|door>: open a door.",
	"close":    "close <direction|door>: shut a door. A closed door blocks the way and hides the exit from both sides.",
	"lock":     "lock <direction|door>: lock a closed door. You need its key, carried or held.",
	"unlock":   "unlock <direction|door>: unlock a locked door with its key. Then 'open' it.",
	"pick":     "pick <direction|door>: try to pick a lock without the key. Nimble hands do better; some locks cannot be picked. One try a round.",
	"tell":     "tell <player> <message>: say something to one player, wherever they are. The first letters of a name are enough.",
	"reply":    "reply <message>: answer whoever last sent you a tell.",
	"rest":     "rest: sit down. Health comes back twice as fast, but you can't walk, fight, or cast until you 'stand'.",
	"sleep":    "sleep: lie down and sleep. Health comes back three times as fast, but you can do little until you 'wake'. A fight wakes you.",
	"wake":     "wake | wake <player>: wake up and stand, or shake a sleeping player awake.",
	"stand":    "stand: get up from resting or sleeping.",
	"affects":  "affects: what is acting on you: spells, poisons, and burning with the time they have left, then your permanent effects.",
	"autoloot": "autoloot: toggle taking everything from the corpses of what you kill.",
	"autogold": "autogold: toggle taking just the coins from the corpses of what you kill.",
	"scroll":   "scroll | scroll <lines> | scroll on | scroll off: how long a reply runs before it pauses. A long list (inventory, skills, help, a shop's wares) shows one page of that many lines; press Enter for the next, or type any command to drop the rest. 'scroll off' shows everything at once.",
	"autosac":  "autosac: toggle sacrificing the corpses of what you kill, once they are empty, for a coin.",
	"recall":   "recall | recall set | recall clear | recall show: recall takes you straight to your recall point, anywhere in the world, but not out of a fight. 'recall set' makes the room you stand in your recall point; 'recall clear' puts it back to where everyone starts; 'recall show' names it. 'recall tutorial' goes back to the start of the tutorial whenever you want a refresher.",

	"get":       "get <item> | get all | get <item> <container> | get all <container>: pick things up, here or from a container (a corpse is a container).",
	"drop":      "drop <item> | drop all | drop <amount> silver|gold: put things or coins down.",
	"put":       "put <item> <container>: put an item into a container you can see.",
	"give":      "give <item> <character> | give <amount> silver|gold <character>: hand over an item or coins.",
	"wear":      "wear <item> | wear all: put on armor, or wield a weapon. wear all wields a weapon only if your hand is empty.",
	"wield":     "wield <weapon>: take up a weapon. Its verb is what your swings are called. A two-handed weapon leaves no hand for a shield or a second weapon.",
	"second":    "second <weapon>: wield a one-handed weapon in your off hand beside your main one, for extra swings each round. It takes the hand a shield would, and without the Dual Wield feat both hands hit much softer.",
	"hold":      "hold <item>: hold an item, such as an orb or a tome. A torch or lantern is lit by wearing or holding it; dark rooms need one.",
	"remove":    "remove <item>: take off something worn, wielded, or held.",
	"inventory": "inventory: what you carry. Weapons and armor are marked against the weakest worn in the same slot: (+n) better, (-n) worse, (=) about the same, (empty slot) nothing worn there. Weapons weigh damage per round, armor its defense; effects are not counted.",
	"equipment": "equipment: what you wear, wield, and hold.",

	"list": "list: what the merchant here sells and its price. Some wares cost silver, some cost other goods, some both.",
	"buy":  "buy <item>: trade a merchant its price for an item: silver, the goods it asks for from your inventory, or both.",
	"sell": "sell <item>: sell an item from your inventory to the merchant here for a tenth of its value. A weapon or armor with no stated value is worth ten silver a level.",

	"kill":     "kill <target>: attack. While fighting, kill <other> switches your target. Anyone who attacks you is fought back, one at a time. Nobody can fight in a safe room.",
	"flee":     "flee: run through a random exit to end a fight.",
	"consider": "consider <target>: how the fight would go, and how hurt they look.",
	"assist":   "assist [member]: attack whatever a group member here is fighting.",
	"skills":   "skills | skills all: the skills you know or can learn now and how good you are at each; 'all' adds those your level or stats do not allow yet. Use one by name: kick, bash. One skill per round, each costing stamina, which comes back quickly, even mid-fight (slower in heavy armor); a few also have a cooldown. Skills improve with use. Most are taught by trainers for coin; a few everyone knows.",
	"practice": "practice | practice <skill or spell>: at a trainer, see what they teach and for how much, or pay to learn one. A spell is sold only to someone with its level and stat, and, unless it is common, its specialty or god. 100 silver is a gold.",
	"quest":    "quest request | info | complete | quit | points | list | buy <item>: a questmaster gives you a task, to slay a mob or recover a lost item somewhere in the world, with a time limit. Finish, come back, and 'quest complete' pays quest points. 'quest quit' abandons it and costs a longer wait. 'quest list' and 'quest buy' trade points for items with whoever sells them.",

	"cast":      "cast <spell> [target] | cast '<spell name>' [target]: cast a spell you know. Casting takes rounds; moving always interrupts, and some spells break when you are hit. Mana, and the materials the few item-bound spells need, are spent when you begin. Mana comes back out of a fight, faster resting or asleep.",
	"spells":    "spells | spells all: the spells you can cast, what they cost, and what is on cooldown; then those you can learn now, from a trainer or from a scroll. Common spells are open to anyone (divine ones to any god's apostle); school spells need your specialty, and rare ones are found on scrolls. 'all' adds the spells your level, stats, specialty, or god do not allow yet.",
	"brandish":  "brandish <totem>: raise a totem to swear to its school of magic, as a Tower's Test does. The totem crumbles to dust. Totems are found only in the builders' lab.",
	"swear":     "swear | swear <school>: at the end of a Tower of High Sorcery's Test, swear to one of its Order's two schools. It is for life: the school's spells open to you and grow stronger, and the opposite school closes. 'swear' alone says what the Order keeps and what it asks.",
	"study":     "study <scroll>: learn the spell written on a scroll. It takes the spell's school or god, its level, and its stat, and the scroll is used up.",
	"consume":   "consume <item>: eat or drink something. Food and drink do nothing yet; a totem is brandished, not consumed.",
	"sacrifice": "sacrifice <thing>: offer a corpse or other item lying here to the gods; it vanishes and they leave you a coin. At a god's temple, giving up what the god wants makes you its apostle.",

	"follow": "follow <player> | follow self: follow someone, moving when they move, or go your own way.",
	"group":  "group | group <follower>: as leader, add someone following you to your group, or list the group. Grouped players share kills and assist each other.",
	"gtell":  "gtell <message>: talk to your group wherever they are.",

	"say":  "say <message> or '<message>: talk to the room.",
	"chat": "chat <message>: talk to everyone in the world.",
	"yell": "yell <message>: shout. Heard up to four rooms away along the exits.",

	"score":    "score (or stats): your character sheet.",
	"stats":    "stats: your character sheet; the same as score.",
	"train":    "train | train <stat>: see your stats, or spend a stat point on one.",
	"feat":     "feat | feat all | feat <name>: see the feats you have and can take now, or with 'all' every feat and what each needs (a level, trained stats, another feat, and for caster feats a school or a god), or spend a pick on one.",
	"who":      "who: who is playing. Admins are marked [Admin]. Invisible and hidden players are listed only if you can see them.",
	"color":    "color: toggle color.",
	"settings": "settings | settings <name>: see your toggles (autoloot, autogold, autosac, color) and whether each is on; with a name, flip that one.",
	"save":     "save: write your character to disk (it also saves on its own).",
	"password": "password <old> <new>: change your password.",
	"quit":     "quit: leave the game.",
	"help":     "help | help <command>: this list, or one command in detail.",

	"goto":       "goto <room|player|mob>: go there.",
	"at":         "at <room|player|mob> <command>: run a command as if you were there.",
	"stat":       "stat | stat <room|character|item>: everything the engine knows about it, including where its numbers came from.",
	"load":       "load mob <vnum> | load obj <vnum>: create one here.",
	"purge":      "purge [target]: destroy a mob or item here, or everything here.",
	"force":      "force <character> <command>: make them do it.",
	"restore":    "restore <player>: full health.",
	"transfer":   "transfer <player> [room]: bring them here, or send them there.",
	"peace":      "peace: stop every fight in the room.",
	"reload":     "reload [scripts | area <name> | world]: re-read the rules, one area, or everything from disk.",
	"client":     "client state [on|off]: ask for a snapshot of your character, room, gear, and skills whenever it changes. Used by graphical clients such as /play.html; text clients have no use for it.",
	"expedition": "expedition | expedition <area> [fights] [seed]: from a safe room, send your character on a run of fights in an area near its level. It sleeps when hurt, turns back when badly hurt, and returns with experience, coin, and whatever dropped, or falls and returns with nothing. Fights use auto-attack only. The seed replays a run exactly; afterwards you rest for half the time it took.",
	"loot":       "loot <mob> [kills] [seed]: kill the mob that many times on paper and report what its deaths drop: each line of its loot table and how often it came up, then the extra drops by rarity, with an example of each tier (RULES 5.3).",
	"simulate":   "simulate <mob | me | fighter[:level] | plain[:level] | outfit[:level]> <mob> [fights] [seed]: run fights through the rules and report the numbers. fighter:N wears the standard kit with the effects expected at its level, plain:N the same kit without them, outfit:N the outfit set.",
	"outfit":     "outfit: conjure the admin regalia and put on every piece that fits a free slot; the rest go in your pack. The set grows with your level, so it stays strong without being godlike.",
	"copyover":   "copyover: restart the server without disconnecting anyone.",
	"shutdown":   "shutdown: stop the server.",

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
	// Anything the sections miss still shows, so help never hides a command.
	listed := map[string]bool{"help": true}
	for _, sec := range helpSections {
		for _, n := range sec.names {
			listed[n] = true
		}
	}
	var other []string
	for _, c := range commands {
		if !listed[c.name] && (p.Admin || !c.admin) {
			other = append(other, c.name)
		}
	}
	if len(other) > 0 {
		b.WriteString("{c}" + padRight("Other", 11) + "{x}" + strings.Join(other, "  ") + "\n")
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
			o.SendChat("{M}You chat '" + msg + "'{x}\n")
		} else {
			o.SendChat("{M}" + p.DisplayName() + " chats '" + msg + "'{x}\n")
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
	p.SendChat("{Y}You yell '" + msg + "'{x}\n")
	for _, r := range w.roomsWithin(p.Room, yellRange) {
		for _, o := range w.playersIn(r) {
			if o != p {
				o.SendChat("{Y}" + p.DisplayName() + " yells '" + msg + "'{x}\n")
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
