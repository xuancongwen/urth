package world

import (
	"strings"

	"urth/internal/item"
	"urth/internal/output"
)

// Trainers (docs/RULES.md 7.4): a mob whose prototype lists the skills it
// teaches. practice at a trainer buys a skill for its price in silver;
// innate skills need no trainer.

// trainersHere returns every trainer in the player's room.
func (w *World) trainersHere(p *Player) []*Character {
	var out []*Character
	for _, m := range w.contents(p.Room).mobs {
		if len(m.Proto.Teaches) > 0 {
			out = append(out, m.Character)
		}
	}
	return out
}

func teaches(trainer *Character, id string) bool {
	if trainer == nil || trainer.mob == nil {
		return false
	}
	for _, t := range trainer.mob.Proto.Teaches {
		if t == id {
			return true
		}
	}
	return false
}

// knowsSkill reports whether c can use sk: innate skills at level, others
// once bought.
func knowsSkill(c *Character, sk Skill) bool {
	if c.Level < sk.Level {
		return false
	}
	return sk.Innate || skillEffect(c, sk.ID) != nil
}

// cmdPractice: practice | practice <skill>
func cmdPractice(w *World, p *Player, args string) {
	trainers := w.trainersHere(p)
	if len(trainers) == 0 {
		p.Send("There is nobody here to teach you.\n")
		return
	}
	skills := w.skillList()
	if args == "" {
		var b strings.Builder
		for _, trainer := range trainers {
			b.WriteString(trainer.DisplayName() + " can teach you:\n")
			n := 0
			for _, sk := range skills {
				if !teaches(trainer, sk.ID) {
					continue
				}
				n++
				cost := escapeMoney(sk.Price)
				for _, r := range sk.Requires {
					cost += " and " + plural(max(r.Count, 1), output.Escape(r.Material))
				}
				line := "  " + padRight(output.Escape(sk.Name), 14) + padRight(cost, 34)
				switch {
				case skillEffect(p.Character, sk.ID) != nil:
					line += "(you know this)"
				case p.Level < sk.Level:
					line += "(level " + itoa(sk.Level) + ")"
				}
				b.WriteString(line + "\n")
			}
			if n == 0 {
				b.WriteString("  nothing, it turns out.\n")
			}
		}
		b.WriteString("You have " + escapeMoney(p.Silver) + ".\n")
		p.Send(b.String())
		return
	}
	want := strings.ToLower(args)
	var match *Skill
	var trainer *Character
	for i := range skills {
		sk := &skills[i]
		var who *Character
		for _, t := range trainers {
			if teaches(t, sk.ID) {
				who = t
				break
			}
		}
		if who == nil {
			continue
		}
		if strings.ToLower(sk.Name) == want || strings.ToLower(sk.ID) == want {
			match, trainer = sk, who
			break
		}
		if strings.HasPrefix(strings.ToLower(sk.Name), want) && match == nil {
			match, trainer = sk, who
		}
	}
	if match == nil {
		w.act("$N can't teach you that.", p.Character, trainers[0], "", toChar)
		return
	}
	if skillEffect(p.Character, match.ID) != nil {
		p.Send("You already know " + output.Escape(match.Name) + ".\n")
		return
	}
	if p.Level < match.Level {
		w.act("$N looks you over. 'Come back at level "+itoa(match.Level)+".'", p.Character, trainer, "", toChar)
		return
	}
	if p.Silver < match.Price {
		p.Send(output.Escape(match.Name) + " costs " + escapeMoney(match.Price) + ". You have " + escapeMoney(p.Silver) + ".\n")
		return
	}
	for _, r := range match.Requires {
		if len(materialsHeld(p.Character, r.Material)) < max(r.Count, 1) {
			w.act("$N wants "+plural(max(r.Count, 1), output.Escape(r.Material))+" for that, and you don't have them.", p.Character, trainer, "", toChar)
			return
		}
	}
	for _, r := range match.Requires {
		held := materialsHeld(p.Character, r.Material)
		for i := 0; i < max(r.Count, 1) && i < len(held); i++ {
			p.Inventory = item.Remove(p.Inventory, held[i])
		}
		w.act("You hand $N "+plural(max(r.Count, 1), output.Escape(r.Material))+".", p.Character, trainer, "", toChar)
	}
	p.Silver -= match.Price
	ensureSkill(p.Character, *match)
	w.recalc(p.Character)
	w.act("You pay $N $t and learn {Y}"+output.Escape(match.Name)+"{x}.", p.Character, trainer, escapeMoney(match.Price), toChar)
	w.act("$n practices with $N.", p.Character, trainer, "", toNotVict)
	w.save(p)
}
