package world

import (
	"urth/internal/mob"
	"urth/internal/room"
)

// A mob left alone settles: once onTick changes nothing for it, it would
// change nothing again until something it reads changes, so regen skips
// it until then. That keeps an idle world's round cost with what is
// happening in it, not with how many mobs it holds. The engine does not
// decide what the rules care about; it only notices that the inputs are
// the same as for a call that did nothing. That relies on onTick being
// a function of its view, random only where it changes something.
//
// settleKey is the part of a mob's view that can change while it stands
// still. Mobs with anything that counts down each round (their own
// effects, cooldowns, a cast) or a fight on never settle, nor do mobs in
// a group, whose view carries the others.
type settleKey struct {
	gen        uint64 // rules and prototypes: bumped by resolveBaselines
	proto      *mob.Proto
	room       *room.Room
	level      int
	health     int
	healthMax  int
	mana       int
	manaMax    int
	stamina    int
	staminaMax int
	position   string
	equipment  uint64
}

// settleKeyOf returns m's key, and false when m cannot settle at all.
// targeted is everyone someone is fighting this round.
func (w *World) settleKeyOf(m *Mob, targeted map[*Character]bool) (settleKey, bool) {
	c := m.Character
	if c.Fighting != nil || targeted[c] || len(c.Effects) > 0 || c.casting != nil || len(c.cooldowns) > 0 || c.group != nil {
		return settleKey{}, false
	}
	k := settleKey{
		gen: w.rulesGen, proto: m.Proto, room: c.Room, level: c.Level,
		health: c.Health, healthMax: c.HealthMax, mana: c.Mana, manaMax: c.ManaMax,
		stamina: c.Stamina, staminaMax: c.StaminaMax, position: c.Position,
	}
	// Order-free over slots; an item's view moves with its burn, decay
	// and enchantments.
	for slot, it := range c.Equipment {
		h := uint64(14695981039346656037)
		for _, b := range []byte(slot) {
			h = (h ^ uint64(b)) * 1099511628211
		}
		for _, v := range []uint64{it.ID, uint64(len(it.Effects)), uint64(it.Burn), uint64(it.Decay)} {
			h = (h ^ v) * 1099511628211
		}
		k.equipment += h
	}
	return k, true
}
