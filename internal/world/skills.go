package world

import (
	"strings"

	"urth/internal/effect"
	"urth/internal/output"
)

// Skills (docs/RULES.md 7.4): the one action a character may take each
// round beyond the auto-attack, plus passives that improve with use. The
// rules define them (skillList) and resolve them (useSkill); a skill's
// effectiveness lives in the state of a "skill" effect on the character,
// and any hook may return updated ratings for the engine to store.

// Skill is one entry of skillList.
type Skill struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Level    int     `json:"level"`
	Passive  bool    `json:"passive"`
	Target   string  `json:"target"` // single, area, ally, or none, for active skills
	Cooldown int     `json:"cooldown"`
	Start    float64 `json:"start"`  // effectiveness on first use
	Innate   bool    `json:"innate"` // known by everyone at level; others need a trainer
	Price    int     `json:"price"`  // in silver, at a trainer
	// Requires are items handed over as well as coin, by material name.
	Requires    []SpellMaterial `json:"requires"`
	Description string          `json:"description"`
}

// SkillResult is what useSkill returns.
type SkillResult struct {
	OK      bool               `json:"ok"`
	Message string             `json:"message"`
	Hit     bool               `json:"hit"`
	Stage   string             `json:"stage"`
	Damage  int                `json:"damage"`
	Verb    string             `json:"verb"`
	Effects []skillEffectSpec  `json:"effects"`
	Skills  map[string]float64 `json:"skills"`
	// Heal is health the user regains at once (Second Wind).
	Heal int `json:"heal"`
	// Taunt, on an ally skill that hit, turns the ally's attackers on the
	// user (Rescue).
	Taunt bool `json:"taunt"`
	// Cooldown, when set, replaces the skill's own for this use (feats
	// that shorten cooldowns).
	Cooldown *int `json:"cooldown"`
}

type skillEffectSpec struct {
	On     string         `json:"on"` // target or self
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params"`
	Rounds int            `json:"rounds"`
}

// skillList asks the rules which skills exist. Optional; default none.
func (w *World) skillList() []Skill {
	var list []Skill
	w.callOptional("skillList", &list)
	return list
}

// skillEffect finds the effect holding a skill's rating, or nil.
func skillEffect(c *Character, id string) *effect.Active {
	for i := range c.Effects {
		e := &c.Effects[i]
		if e.Kind == "skill" && e.Params["skill"] == id {
			return e
		}
	}
	return nil
}

// ensureSkill creates the rating effect for a skill at its starting value.
func ensureSkill(c *Character, sk Skill) *effect.Active {
	if e := skillEffect(c, sk.ID); e != nil {
		return e
	}
	c.Effects = append(c.Effects, effect.Active{
		Spec:  effect.Spec{Kind: "skill", Params: map[string]any{"skill": sk.ID}},
		State: map[string]any{"effectiveness": sk.Start},
	})
	return &c.Effects[len(c.Effects)-1]
}

// ensurePassives gives a character every passive skill its level allows,
// so derivedStats can see them. Called on login, level, and reload.
func (w *World) ensurePassives(c *Character) {
	changed := false
	for _, sk := range w.skillList() {
		if sk.Passive && sk.Innate && c.Level >= sk.Level && skillEffect(c, sk.ID) == nil {
			ensureSkill(c, sk)
			changed = true
		}
	}
	if changed {
		w.recalc(c)
	}
}

// applySkillRatings stores ratings a hook returned.
func applySkillRatings(c *Character, ratings map[string]float64) {
	for id, v := range ratings {
		if e := skillEffect(c, id); e != nil {
			if e.State == nil {
				e.State = map[string]any{}
			}
			e.State["effectiveness"] = min(max(v, 0), 100)
		}
	}
}

func effectiveness(e *effect.Active) float64 {
	if e == nil || e.State == nil {
		return 0
	}
	switch v := e.State["effectiveness"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

// findSkill matches a typed word against the skills c's level allows.
func (w *World) findSkill(c *Character, word string) (Skill, bool) {
	word = strings.ToLower(word)
	var match *Skill
	list := w.skillList()
	for i := range list {
		sk := &list[i]
		if !knowsSkill(c, *sk) {
			continue
		}
		if strings.ToLower(sk.Name) == word || strings.ToLower(sk.ID) == word {
			return *sk, true
		}
		if strings.HasPrefix(strings.ToLower(sk.Name), word) && match == nil {
			match = sk
		}
	}
	if match != nil {
		return *match, true
	}
	return Skill{}, false
}

// cmdSkills lists the skills the character's level allows, with ratings.
func cmdSkills(w *World, p *Player, _ string) {
	var b strings.Builder
	n := 0
	for _, sk := range w.skillList() {
		if p.Level < sk.Level {
			continue
		}
		n++
		line := padRight(output.Escape(sk.Name), 14)
		if e := skillEffect(p.Character, sk.ID); e != nil {
			line += padRight(itoa(int(effectiveness(e)+0.5))+"%", 9)
		} else if sk.Innate {
			line += padRight("new", 9)
		} else {
			line += padRight("trainer", 9)
		}
		if sk.Passive {
			line += "passive  "
		} else {
			line += "cooldown " + itoa(sk.Cooldown) + "  "
		}
		if cd := p.cooldowns["skill:"+sk.ID]; cd > 0 {
			line += "{R}(" + itoa(cd) + " rounds){x} "
		}
		line += output.Escape(sk.Description)
		b.WriteString(line + "\n")
	}
	if n == 0 {
		b.WriteString("You have no skills yet.\n")
	}
	p.Send(b.String())
}

// trySkill runs a typed word as a skill if it names one. Used by dispatch
// after the command table, so skills never shadow commands.
func (w *World) trySkill(p *Player, word, args string) bool {
	sk, ok := w.findSkill(p.Character, word)
	if !ok || sk.Passive {
		return false
	}
	w.useSkill(p, sk, args)
	return true
}

// useSkill resolves one use of an active skill. A single skill strikes
// one foe, an area skill every foe fighting the user, an ally skill a
// friend (Rescue), and a skill with no target the user alone.
func (w *World) useSkill(p *Player, sk Skill, args string) {
	c := p.Character
	if c.lastSkillRound == w.roundCount+1 { // stored as round+1 so 0 means never
		p.Send("You are still recovering from your last move.\n")
		return
	}
	key := "skill:" + sk.ID
	if cd := c.cooldowns[key]; cd > 0 {
		p.Send(output.Escape(sk.Name) + " is not ready for another " + plural(cd, "round") + ".\n")
		return
	}
	targets, ok := w.skillTargets(p, sk, args)
	if !ok {
		return
	}
	e := ensureSkill(c, sk)
	used := false
	for _, target := range targets {
		if target != nil && (target.Health <= 0 || target.Room != c.Room) {
			continue
		}
		var r SkillResult
		args2 := []any{w.view(c), nil, skillView(sk), e.View()}
		if target != nil {
			args2[1] = w.view(target)
		}
		if !w.call("useSkill", &r, args2...) || !r.OK {
			if used {
				continue // an area skill that cannot touch one foe still struck the rest
			}
			msg := r.Message
			if msg == "" {
				msg = "You can't do that right now."
			}
			p.Send(output.Escape(msg) + "\n")
			return
		}
		if !used {
			// The first result sets the pace: one use, one improvement roll,
			// one cooldown, however many foes an area skill reaches.
			used = true
			c.lastSkillRound = w.roundCount + 1
			if c.cooldowns == nil {
				c.cooldowns = map[string]int{}
			}
			cd := sk.Cooldown
			if r.Cooldown != nil {
				cd = max(*r.Cooldown, 0)
			}
			if cd > 0 {
				c.cooldowns[key] = cd
			}
			applySkillRatings(c, r.Skills)
		}
		w.skillOutcome(c, sk, target, r)
		if c.Health <= 0 || c.Room == nil {
			return
		}
	}
}

// skillTargets finds whom a use of sk touches: one foe, every foe, one
// ally, or nobody (a nil entry). It reports and returns false when the use
// cannot go ahead.
func (w *World) skillTargets(p *Player, sk Skill, args string) ([]*Character, bool) {
	c := p.Character
	switch sk.Target {
	case "single":
		var target *Character
		if args != "" {
			target = w.findCharacter(p.Room, c, args)
			if target == nil {
				p.Send("They aren't here.\n")
				return nil, false
			}
		} else {
			target = c.Fighting
			if target == nil {
				p.Send(output.Escape(sk.Name) + " whom?\n")
				return nil, false
			}
		}
		if target.mob != nil && target.mob.Proto.HasFlag("peaceful") {
			p.Send("You can't bring yourself to attack " + output.Escape(target.Name) + ".\n")
			return nil, false
		}
		if p.Room.Safe() {
			p.Send("You cannot fight here.\n")
			return nil, false
		}
		if sameGroup(c, target) {
			p.Send("You can't attack a member of your group.\n")
			return nil, false
		}
		return []*Character{target}, true
	case "area":
		if p.Room.Safe() {
			p.Send("You cannot fight here.\n")
			return nil, false
		}
		var foes []*Character
		for _, o := range w.enemiesOf(c) {
			if !sameGroup(c, o) && (o.mob == nil || !o.mob.Proto.HasFlag("peaceful")) {
				foes = append(foes, o)
			}
		}
		if len(foes) == 0 {
			p.Send("You aren't fighting anyone.\n")
			return nil, false
		}
		return foes, true
	case "ally":
		if args == "" {
			p.Send(output.Escape(sk.Name) + " whom?\n")
			return nil, false
		}
		ally := w.findCharacter(p.Room, c, args)
		switch {
		case ally == nil:
			p.Send("They aren't here.\n")
			return nil, false
		case ally == c:
			p.Send("You can't do that to yourself.\n")
			return nil, false
		case ally.mob != nil && !sameGroup(c, ally):
			p.Send("Only a player or a member of your group.\n")
			return nil, false
		case ally.Fighting == c:
			p.Send(ally.DisplayName() + " is fighting you!\n")
			return nil, false
		case len(w.attackersOf(ally)) == 0:
			p.Send("Nobody is fighting " + output.Escape(ally.Name) + ".\n")
			return nil, false
		}
		return []*Character{ally}, true
	}
	return []*Character{nil}, true
}

// attackersOf is everyone in c's room fighting c.
func (w *World) attackersOf(c *Character) []*Character {
	if c.Room == nil {
		return nil
	}
	var out []*Character
	for _, o := range w.charactersIn(c.Room) {
		if o != c && o.Fighting == c {
			out = append(out, o)
		}
	}
	return out
}

// skillOutcome applies and narrates one result of a skill on one target
// (nil for a skill on the user alone).
func (w *World) skillOutcome(c *Character, sk Skill, target *Character, r SkillResult) {
	verb := output.Escape(r.Verb)
	if verb == "" {
		verb = output.Escape(strings.ToLower(sk.Name))
	}
	switch {
	case target != nil && sk.Target == "ally":
		if !r.Hit {
			w.act("You fail to "+verb+" $N.", c, target, "", toChar)
			break
		}
		w.act("You "+verb+" $N!", c, target, "", toChar)
		w.act("$n "+verb+"s you!", c, target, "", toVict)
		w.act("$n "+verb+"s $N!", c, target, "", toNotVict)
		if r.Taunt {
			// Everyone fighting the ally turns on the rescuer.
			for _, o := range w.attackersOf(target) {
				o.Fighting = c
				o.swing = 0
				if c.Fighting == nil {
					c.Fighting = o
					c.swing = 0
				}
			}
		}
	case target != nil:
		w.startFightIfIdle(c, target)
		if !r.Hit {
			switch r.Stage {
			case "dodge":
				w.act("$N dodges your "+verb+".", c, target, "", toChar)
				w.act("You dodge $n's "+verb+".", c, target, "", toVict)
				w.act("$N dodges $n's "+verb+".", c, target, "", toNotVict)
			case "block":
				w.act("$N blocks your "+verb+".", c, target, "", toChar)
				w.act("You block $n's "+verb+".", c, target, "", toVict)
				w.act("$N blocks $n's "+verb+".", c, target, "", toNotVict)
			default:
				w.act("Your "+verb+" misses $N.", c, target, "", toChar)
				w.act("$n's "+verb+" misses you.", c, target, "", toVict)
				w.act("$n's "+verb+" misses $N.", c, target, "", toNotVict)
			}
		} else if r.Damage <= 0 {
			// A landed use that does no harm by itself (Feint, Disarm).
			w.act("Your "+verb+" catches $N.", c, target, "", toChar)
			w.act("$n's "+verb+" catches you.", c, target, "", toVict)
			w.act("$n's "+verb+" catches $N.", c, target, "", toNotVict)
		} else {
			dmg := itoa(r.Damage)
			_, word, punct := w.hitWords(r.Damage, target)
			w.act("Your "+verb+" "+word+" $N"+punct+" {W}["+dmg+"]{x}", c, target, "", toChar)
			w.act("$n's "+verb+" "+word+" you"+punct+" {R}["+dmg+"]{x}", c, target, "", toVict)
			w.act("$n's "+verb+" "+word+" $N"+punct, c, target, "", toNotVict)
			target.Health -= r.Damage
		}
	case r.Message == "" && r.Heal <= 0:
		w.act("You use $t.", c, nil, output.Escape(sk.Name), toChar)
	}
	if r.Heal > 0 && c.Health > 0 {
		healed := max(0, min(r.Heal, c.HealthMax-c.Health))
		c.Health += healed
		w.act("Your "+verb+" heals you. {G}["+itoa(healed)+"]{x}", c, nil, "", toChar)
		w.act("$n looks steadier.", c, nil, "", toRoom)
	}
	for _, se := range r.Effects {
		on := c
		if se.On == "target" && target != nil {
			on = target
		}
		if se.Kind == "" {
			continue
		}
		on.Effects = append(on.Effects, effect.Active{Spec: effect.Spec{Kind: se.Kind, Params: se.Params}, Rounds: se.Rounds})
		w.recalc(on)
	}
	if r.Message != "" {
		c.Send(output.Escape(r.Message) + "\n")
	}
	if target != nil && target.Health <= 0 {
		w.die(target, c)
	}
}

// startFightIfIdle makes a hostile act start a fight without resetting one
// already under way.
func (w *World) startFightIfIdle(att, def *Character) {
	if att.Fighting == nil {
		w.startFight(att, def)
		return
	}
	if def.Fighting == nil {
		def.Fighting = att
		def.swing = 0
	}
}

func skillView(sk Skill) map[string]any {
	return map[string]any{"id": sk.ID, "name": sk.Name, "level": sk.Level, "passive": sk.Passive,
		"target": sk.Target, "cooldown": sk.Cooldown, "start": sk.Start, "innate": sk.Innate, "price": sk.Price, "requires": sk.Requires}
}
