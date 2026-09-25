package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/room"
)

// Groups and many-to-many combat (docs/RULES.md 4.7). A Group is a party of
// players for the session; it is not saved. Anyone may fight anyone, any
// number may fight one, and kill switches targets.

// Group is a party. The leader is also a member.
type Group struct {
	leader  *Character
	members []*Character
}

func (g *Group) has(c *Character) bool {
	for _, m := range g.members {
		if m == c {
			return true
		}
	}
	return false
}

// groupOf returns the characters in c's group, or just c.
func groupOf(c *Character) []*Character {
	if c.group == nil {
		return []*Character{c}
	}
	return c.group.members
}

// sameGroup reports whether a and b are the same character or grouped.
func sameGroup(a, b *Character) bool {
	return a == b || (a.group != nil && a.group == b.group)
}

// groupIn returns the members of c's group who are in r, c included.
func (w *World) groupIn(c *Character, r *room.Room) []*Character {
	var out []*Character
	for _, m := range groupOf(c) {
		if m.Room == r {
			out = append(out, m)
		}
	}
	return out
}

// enemiesOf returns c's target and everyone fighting c, in c's room.
func (w *World) enemiesOf(c *Character) []*Character {
	if c.Room == nil {
		return nil
	}
	var out []*Character
	seen := map[*Character]bool{}
	add := func(o *Character) {
		if o != nil && o != c && o.Room == c.Room && !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	add(c.Fighting)
	for _, o := range w.charactersIn(c.Room) {
		if o.Fighting == c {
			add(o)
		}
	}
	return out
}

// hostilesIn returns everyone in c's room who is not c and not grouped
// with c: the set a hostile area spell affects (docs/RULES.md 6.4).
func (w *World) hostilesIn(c *Character) []*Character {
	if c.Room == nil {
		return nil
	}
	var out []*Character
	for _, o := range w.charactersIn(c.Room) {
		if sameGroup(c, o) {
			continue
		}
		if o.mob != nil && o.mob.Proto.HasFlag("peaceful") {
			continue // nothing hostile touches the peaceful
		}
		out = append(out, o)
	}
	return out
}

// leaveGroup removes c from its group and stops anyone following c.
func (w *World) leaveGroup(c *Character) {
	if g := c.group; g != nil {
		var kept []*Character
		for _, m := range g.members {
			if m != c {
				kept = append(kept, m)
			}
		}
		g.members = kept
		if g.leader == c && len(kept) > 0 {
			g.leader = kept[0]
			g.leader.Send("You now lead the group.\n")
		}
		if len(kept) <= 1 {
			for _, m := range kept {
				m.group = nil
				m.Send("Your group has disbanded.\n")
			}
		}
		c.group = nil
	}
	c.following = nil
	for _, o := range w.allCharacters() {
		if o.following == c {
			o.following = nil
			o.Send("You stop following " + output.Escape(c.Name) + ".\n")
		}
	}
}

// cmdFollow: follow <player> | follow self
func cmdFollow(w *World, p *Player, args string) {
	if args == "" {
		if p.following == nil {
			p.Send("You are not following anyone.\n")
		} else {
			p.Send("You are following " + output.Escape(p.following.Name) + ".\n")
		}
		return
	}
	if strings.EqualFold(args, "self") || strings.EqualFold(args, p.Name) {
		if p.following == nil && p.group == nil {
			p.Send("You are already on your own.\n")
			return
		}
		if p.following != nil {
			w.act("$n stops following you.", p.Character, p.following, "", toVict)
		}
		w.leaveGroup(p.Character)
		p.Send("You are on your own.\n")
		return
	}
	target := w.findCharacter(p.Room, p.Character, args)
	if target == nil {
		p.Send("They aren't here.\n")
		return
	}
	if target.player == nil {
		p.Send("You can only follow players.\n")
		return
	}
	if p.group != nil {
		w.leaveGroup(p.Character)
	}
	p.following = target
	w.act("You now follow $N.", p.Character, target, "", toChar)
	w.act("$n now follows you.", p.Character, target, "", toVict)
}

// cmdGroup: group | group <follower>. The leader adds someone who is
// following them; group alone lists the party.
func cmdGroup(w *World, p *Player, args string) {
	if args == "" {
		var b strings.Builder
		if p.group == nil {
			b.WriteString("You are not in a group.\n")
		} else {
			b.WriteString(output.Escape(p.group.leader.Name) + "'s group:\n")
			for _, m := range p.group.members {
				b.WriteString("  " + padRight(output.Escape(m.Name), 14) + " level " + itoa(m.Level) + "  " + itoa(m.Health) + "/" + itoa(m.HealthMax) + " hp\n")
			}
		}
		p.Send(b.String())
		return
	}
	target := w.findCharacter(p.Room, p.Character, args)
	if target == nil {
		p.Send("They aren't here.\n")
		return
	}
	if p.group != nil && p.group.leader != p.Character {
		p.Send("Only the group leader can do that.\n")
		return
	}
	if p.group != nil && p.group.has(target) {
		// Removing a member.
		w.act("$N is no longer in your group.", p.Character, target, "", toChar)
		w.act("You are no longer in $n's group.", p.Character, target, "", toVict)
		w.leaveGroup(target)
		return
	}
	if target.following != p.Character {
		w.act("$N is not following you.", p.Character, target, "", toChar)
		return
	}
	if p.group == nil {
		p.group = &Group{leader: p.Character, members: []*Character{p.Character}}
	}
	if target.group != nil {
		w.leaveGroup(target)
		target.following = p.Character
	}
	target.group = p.group
	p.group.members = append(p.group.members, target)
	w.act("$N joins your group.", p.Character, target, "", toChar)
	w.act("You join $n's group.", p.Character, target, "", toVict)
}

// cmdGtell: gtell <message>
func cmdGtell(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Tell your group what?\n")
		return
	}
	if p.group == nil {
		p.Send("You are not in a group.\n")
		return
	}
	msg := output.Escape(args)
	for _, m := range p.group.members {
		if m == p.Character {
			m.Send("You tell the group '" + msg + "'\n")
		} else {
			m.Send(p.DisplayName() + " tells the group '" + msg + "'\n")
		}
	}
}

// cmdAssist: assist [member]. Attack whatever a group member here is
// fighting.
func cmdAssist(w *World, p *Player, args string) {
	if p.Fighting != nil {
		p.Send("You are already fighting!\n")
		return
	}
	var whom *Character
	if args != "" {
		whom = w.findCharacter(p.Room, p.Character, args)
		if whom == nil {
			p.Send("They aren't here.\n")
			return
		}
	} else {
		for _, m := range w.groupIn(p.Character, p.Room) {
			if m != p.Character && m.Fighting != nil {
				whom = m
				break
			}
		}
		if whom == nil {
			p.Send("Nobody in your group here needs help.\n")
			return
		}
	}
	if whom.Fighting == nil || whom.Fighting.Room != p.Room {
		w.act("$N is not fighting anyone.", p.Character, whom, "", toChar)
		return
	}
	if p.Room.Safe() {
		p.Send("You cannot fight here.\n")
		return
	}
	w.act("You assist $N.", p.Character, whom, "", toChar)
	w.act("$n assists you.", p.Character, whom, "", toVict)
	w.startFight(p.Character, whom.Fighting)
	w.attackRound(p.Character, whom.Fighting)
}

// autoAssist brings grouped players and assisting mobs into a fight that
// just started: att's group joins against def, def's group joins against
// att, and mobs flagged "assist" join a mob of their own kind.
func (w *World) autoAssist(att, def *Character) {
	join := func(helper, target *Character) {
		if helper.Fighting != nil || helper.Room != target.Room || helper.Health <= 0 {
			return
		}
		helper.Fighting = target
		helper.swing = 0
		w.act("You assist $N.", helper, helper.Fighting, "", toChar)
		w.act("$n joins the fight against $N.", helper, target, "", toRoom)
	}
	for _, m := range w.groupIn(att, att.Room) {
		if m != att {
			join(m, def)
		}
	}
	for _, m := range w.groupIn(def, def.Room) {
		if m != def {
			join(m, att)
		}
	}
	if def.mob != nil && def.mob.Proto.HasFlag("assist") {
		for _, m := range w.contents(def.Room).mobs {
			if m.Character != def && m.Proto == def.mob.Proto {
				join(m.Character, att)
			}
		}
	}
}

// followLeader moves everyone in from who is following leader and not
// fighting along with them.
func (w *World) followLeader(leader *Character, from, dest *room.Room, dir string) {
	for _, p := range w.playersIn(from) {
		if p.following != leader || p.Fighting != nil || p.Character == leader {
			continue
		}
		p.Send("You follow " + output.Escape(leader.Name) + ".\n")
		w.act("$n leaves $t.", p.Character, nil, dir, toRoom)
		w.interruptCast(p.Character, "You stop casting as you move.")
		p.Room = dest
		w.act("$n arrives $t.", p.Character, nil, arrivesFrom(dir), toRoom)
		w.look(p)
	}
}
