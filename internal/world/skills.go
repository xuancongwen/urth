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
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Level       int     `json:"level"`
	Passive     bool    `json:"passive"`
	Target      string  `json:"target"` // single or none, for active skills
	Cooldown    int     `json:"cooldown"`
	Start       float64 `json:"start"` // effectiveness on first use
	Description string  `json:"description"`
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
		if sk.Passive && c.Level >= sk.Level && skillEffect(c, sk.ID) == nil {
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
		if c.Level < sk.Level {
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
			line += padRight(itoa(int(effectiveness(e)+0.5))+"%", 6)
		} else {
			line += padRight("new", 6)
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

// useSkill resolves one use of an active skill.
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
	var target *Character
	if sk.Target == "single" {
		if args != "" {
			target = w.findCharacter(p.Room, c, args)
			if target == nil {
				p.Send("They aren't here.\n")
				return
			}
		} else {
			target = c.Fighting
			if target == nil {
				p.Send(output.Escape(sk.Name) + " whom?\n")
				return
			}
		}
		if target.mob != nil && target.mob.Proto.HasFlag("peaceful") {
			p.Send("You can't bring yourself to attack " + output.Escape(target.Name) + ".\n")
			return
		}
		if p.Room.Safe() {
			p.Send("You cannot fight here.\n")
			return
		}
		if sameGroup(c, target) {
			p.Send("You can't attack a member of your group.\n")
			return
		}
	}
	e := ensureSkill(c, sk)
	var r SkillResult
	args2 := []any{w.view(c), nil, skillView(sk), e.View()}
	if target != nil {
		args2[1] = w.view(target)
	}
	if !w.call("useSkill", &r, args2...) || !r.OK {
		msg := r.Message
		if msg == "" {
			msg = "You can't do that right now."
		}
		p.Send(output.Escape(msg) + "\n")
		return
	}
	c.lastSkillRound = w.roundCount + 1
	if c.cooldowns == nil {
		c.cooldowns = map[string]int{}
	}
	if sk.Cooldown > 0 {
		c.cooldowns[key] = sk.Cooldown
	}
	applySkillRatings(c, r.Skills)
	verb := output.Escape(r.Verb)
	if verb == "" {
		verb = output.Escape(strings.ToLower(sk.Name))
	}
	if target != nil {
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
		} else {
			dmg := itoa(r.Damage)
			w.act("Your "+verb+" hits $N. {W}["+dmg+"]{x}", c, target, "", toChar)
			w.act("$n's "+verb+" hits you. {R}["+dmg+"]{x}", c, target, "", toVict)
			w.act("$n's "+verb+" hits $N.", c, target, "", toNotVict)
			target.Health -= r.Damage
		}
	} else if r.Message == "" {
		w.act("You use $t.", c, nil, output.Escape(sk.Name), toChar)
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
		p.Send(output.Escape(r.Message) + "\n")
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
		"target": sk.Target, "cooldown": sk.Cooldown, "start": sk.Start}
}
