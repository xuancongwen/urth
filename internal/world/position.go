package world

// Positions: a character stands, rests, or sleeps. Resting and sleeping
// speed up regeneration (the rules read "position" from the view), and
// cost the character what it can do: nobody walks, fights, or casts
// sitting down, and a sleeper does little but wake. A fight, or being
// moved by magic, brings anyone to their feet.

const (
	posStanding = ""
	posResting  = "resting"
	posSleeping = "sleeping"
)

// positionName is the position as the rules and players see it.
func positionName(c *Character) string {
	if c.Position == posStanding {
		return "standing"
	}
	return c.Position
}

// restingBlocks are the commands that need a character on its feet.
// Skills used by name need it too.
var restingBlocks = map[string]bool{
	"north": true, "east": true, "south": true, "west": true, "up": true, "down": true,
	"kill": true, "flee": true, "assist": true, "cast": true, "recall": true,
	"pick": true, "brandish": true,
}

// sleepingAllows are the only commands a sleeper can use.
var sleepingAllows = map[string]bool{
	"wake": true, "stand": true, "rest": true, "sleep": true,
	"score": true, "affects": true, "inventory": true, "equipment": true,
	"who": true, "help": true, "save": true, "quit": true, "password": true, "color": true,
	"autoloot": true, "autogold": true, "autosac": true,
}

// positionAllows reports whether p may use the command in its position,
// and tells it why not when it may not.
func positionAllows(p *Player, name string) bool {
	switch p.Position {
	case posSleeping:
		if !sleepingAllows[name] {
			p.Send("In your dreams, or what? 'wake' first.\n")
			return false
		}
	case posResting:
		if restingBlocks[name] {
			p.Send("Nah... you feel too relaxed. 'stand' first.\n")
			return false
		}
	}
	return true
}

// standUp puts c on its feet without a word, as a fight or a teleport does.
// It reports whether c was down.
func standUp(c *Character) bool {
	was := c.Position
	c.Position = posStanding
	return was != posStanding
}

// cmdRest: rest. Sit down; health comes back twice as fast.
func cmdRest(w *World, p *Player, _ string) {
	switch {
	case p.Fighting != nil:
		p.Send("Maybe you should finish this fight first?\n")
	case p.Position == posResting:
		p.Send("You are already resting.\n")
	case p.Position == posSleeping:
		p.Position = posResting
		p.Send("You wake up and sit up to rest.\n")
		w.act("$n wakes up and sits up.", p.Character, nil, "", toRoom)
	default:
		p.Position = posResting
		w.interruptCast(p.Character, "You stop casting as you sit down.")
		p.Send("You sit down and rest.\n")
		w.act("$n sits down and rests.", p.Character, nil, "", toRoom)
	}
}

// cmdSleep: sleep. Lie down; health comes back three times as fast.
func cmdSleep(w *World, p *Player, _ string) {
	switch {
	case p.Fighting != nil:
		p.Send("Maybe you should finish this fight first?\n")
	case p.Position == posSleeping:
		p.Send("You are already sound asleep.\n")
	default:
		p.Position = posSleeping
		w.interruptCast(p.Character, "You stop casting as you lie down.")
		p.Send("You lie down and go to sleep.\n")
		w.act("$n lies down and goes to sleep.", p.Character, nil, "", toRoom)
	}
}

// cmdWake: wake | wake <player>. Get up, or shake a sleeper awake.
func cmdWake(w *World, p *Player, args string) {
	if args == "" {
		cmdStand(w, p, "")
		return
	}
	if p.Position == posSleeping {
		p.Send("You can't wake anyone else while you are asleep yourself.\n")
		return
	}
	target := w.findCharacter(p.Room, p.Character, args)
	switch {
	case target == nil:
		p.Send("They aren't here.\n")
	case target == p.Character:
		cmdStand(w, p, "")
	case target.Position != posSleeping:
		w.act("$N is already awake.", p.Character, target, "", toChar)
	default:
		target.Position = posStanding
		w.act("You shake $N awake.", p.Character, target, "", toChar)
		w.act("$n shakes you awake.", p.Character, target, "", toVict)
		w.act("$n shakes $N awake.", p.Character, target, "", toNotVict)
	}
}

// cmdStand: stand. Get to your feet from resting or sleeping.
func cmdStand(w *World, p *Player, _ string) {
	switch p.Position {
	case posSleeping:
		p.Send("You wake and stand up.\n")
		w.act("$n wakes and stands up.", p.Character, nil, "", toRoom)
	case posResting:
		p.Send("You stand up.\n")
		w.act("$n stands up.", p.Character, nil, "", toRoom)
	default:
		p.Send("You are already standing.\n")
		return
	}
	p.Position = posStanding
}

// positionSuffix is how the room listing shows a character: "is here",
// "is resting here", "is sleeping here".
func positionSuffix(c *Character) string {
	switch c.Position {
	case posResting:
		return " is resting here."
	case posSleeping:
		return " is sleeping here."
	}
	return " is here."
}

// wakeForFight stands a character up when a fight finds it, and tells a
// sleeper why.
func (w *World) wakeForFight(c *Character) {
	if c.Position == posSleeping {
		c.Send("{R}You wake up, and find yourself in a fight!{x}\n")
	}
	if standUp(c) {
		w.act("$n scrambles up.", c, nil, "", toRoom)
	}
}
