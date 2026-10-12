package world

import "urth/internal/effect"

// Visibility: whether one character can see another. Two ways to be
// unseen, each an effect kind the engine recognizes on the character or
// on worn gear (as it recognizes "skill" and feats):
//
//	invisible      the character cannot be seen without detectInvisible
//	hidden         the character cannot be seen without detectHidden
//	sneak          the character is seen, but its comings and goings, and
//	               its presence to an aggressive mob, are noticed only
//	               with detectHidden
//	detectInvisible, detectHidden  the viewer sees through the matching one
//
// Whoever is fighting you can always be seen. Hiding on the character
// itself (the Hide skill) ends when it moves or a fight starts, and
// invisibility on it when a fight starts (docs/RULES.md 7.8).
//
// The rules scripts decide how a character comes by these (a spell, a
// skill, a cloak); the engine only asks the questions below. Admins see
// everyone, and everyone sees themselves.

const (
	effectInvisible       = "invisible"
	effectHidden          = "hidden"
	effectDetectInvisible = "detectInvisible"
	effectDetectHidden    = "detectHidden"
	effectSneak           = "sneak"
)

// hasEffect reports whether c carries an effect of kind, on itself, its
// mob prototype, or anything it wears.
func hasEffect(c *Character, kind string) bool {
	has := func(list []effect.Active) bool {
		for _, e := range list {
			if e.Kind == kind {
				return true
			}
		}
		return false
	}
	if has(c.Effects) {
		return true
	}
	if c.mob != nil {
		for _, e := range c.mob.Proto.Effects {
			if e.Kind == kind {
				return true
			}
		}
	}
	for _, it := range c.Equipment {
		if it != nil && has(it.Effects) {
			return true
		}
	}
	return false
}

// Invisible reports whether the character is invisible.
func (c *Character) Invisible() bool { return hasEffect(c, effectInvisible) }

// Hidden reports whether the character is hiding.
func (c *Character) Hidden() bool { return hasEffect(c, effectHidden) }

// isAdmin reports whether the character is an admin player.
func (c *Character) isAdmin() bool { return c.player != nil && c.player.Admin }

// canSeeChar reports whether viewer can see target, ignoring light and
// distance: only target's invisibility or hiding and viewer's ability
// to see through it.
func (w *World) canSeeChar(viewer, target *Character) bool {
	if viewer == target || viewer.isAdmin() || target.Fighting == viewer {
		return true
	}
	if target.Invisible() && !hasEffect(viewer, effectDetectInvisible) {
		return false
	}
	if target.Hidden() && !hasEffect(viewer, effectDetectHidden) {
		return false
	}
	return true
}

// noticesChar reports whether viewer notices target coming, going, or
// standing by: it can see target, and target is not sneaking past it.
func (w *World) noticesChar(viewer, target *Character) bool {
	if !w.canSeeChar(viewer, target) {
		return false
	}
	if viewer == target || viewer.isAdmin() || target.Fighting == viewer {
		return true
	}
	return !hasEffect(target, effectSneak) || hasEffect(viewer, effectDetectHidden)
}

// actMove tells the players in c's room who notice c that it arrives or
// leaves (format as for act, with $t the direction words).
func (w *World) actMove(format string, c *Character, dir string) {
	if c.Room == nil {
		return
	}
	msg := capitalize(expandAct(format, c, nil, "", dir)) + "\n"
	for _, p := range w.playersIn(c.Room) {
		if p.Character != c && w.noticesChar(p.Character, c) {
			p.Send(msg)
		}
	}
}

// reveal ends c's own hiding (the Hide skill and the like; not gear or a
// mob's nature), and its own invisibility too when fighting is true.
func (w *World) reveal(c *Character, fighting bool) {
	kept := c.Effects[:0]
	var hid, invis bool
	for _, e := range c.Effects {
		switch {
		case e.Kind == effectHidden:
			hid = true
			continue
		case e.Kind == effectInvisible && fighting:
			invis = true
			continue
		}
		kept = append(kept, e)
	}
	c.Effects = kept
	if hid {
		c.Send("You step out of hiding.\n")
	}
	if invis {
		c.Send("You fade into view.\n")
	}
}
