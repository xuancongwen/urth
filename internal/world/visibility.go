package world

import "urth/internal/effect"

// Visibility: whether one character can see another. Two ways to be
// unseen, each an effect kind the engine recognizes on the character or
// on worn gear (as it recognizes "skill" and feats):
//
//	invisible      the character cannot be seen without detectInvisible
//	hidden         the character cannot be seen without detectHidden
//	detectInvisible, detectHidden  the viewer sees through the matching one
//
// The rules scripts decide how a character comes by these (a spell, a
// skill, a cloak); the engine only asks the questions below. Admins see
// everyone, and everyone sees themselves.

const (
	effectInvisible       = "invisible"
	effectHidden          = "hidden"
	effectDetectInvisible = "detectInvisible"
	effectDetectHidden    = "detectHidden"
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
	if viewer == target || viewer.isAdmin() {
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
