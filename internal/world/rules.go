package world

import (
	"errors"

	"urth/internal/item"
	"urth/internal/script"
)

// Rule hooks. Each wrapper builds read-only views, calls the script, and
// falls back to a harmless default if the hook is missing or broken, so a
// bad script edit never crashes the game. Errors are logged once per hook
// per script load so a broken formula is visible but not spammy.

// AttackResult is what resolveAttack returns.
type AttackResult struct {
	Hit    bool   `json:"hit"`
	Damage int    `json:"damage"`
	Crit   bool   `json:"crit"`
	Verb   string `json:"verb"`
}

// TickResult is what onTick returns.
type TickResult struct {
	HealthDelta int `json:"healthDelta"`
	ManaDelta   int `json:"manaDelta"`
}

// LevelResult is what onLevel returns.
type LevelResult struct {
	StatDeltas map[string]int `json:"statDeltas"`
	Message    string         `json:"message"`
}

// Derived is what derivedStats returns.
type Derived struct {
	HealthMax       int `json:"healthMax"`
	ManaMax         int `json:"manaMax"`
	AttacksPerRound int `json:"attacksPerRound"`
}

func (w *World) hookErr(name string, err error) {
	if w.scripts == nil {
		return
	}
	loaded := w.scripts.LoadedAt()
	if w.hookErrors[name].Equal(loaded) {
		return
	}
	w.hookErrors[name] = loaded
	if errors.Is(err, script.ErrNoHook) {
		w.log.Warn("rule hook missing, using default", "hook", name)
	} else {
		w.log.Error("rule hook failed, using default", "hook", name, "err", err)
	}
	w.broadcastAdmins("{R}Rule hook " + name + ": " + err.Error() + "{x}\n")
}

// view builds the script-facing snapshot of a character.
func (w *World) view(c *Character) map[string]any {
	eq := map[string]any{}
	for slot, it := range c.Equipment {
		eq[string(slot)] = itemView(it)
	}
	stats := map[string]any{}
	for k, v := range c.Stats {
		stats[k] = v
	}
	v := map[string]any{
		"name":      c.Name,
		"level":     c.Level,
		"xp":        c.Experience,
		"stats":     stats,
		"health":    c.Health,
		"healthMax": c.HealthMax,
		"mana":      c.Mana,
		"manaMax":   c.ManaMax,
		"equipment": eq,
		"isPlayer":  c.player != nil,
		"fighting":  c.Fighting != nil,
	}
	if c.Room != nil {
		v["room"] = map[string]any{"vnum": c.Room.Vnum, "name": c.Room.Name, "area": c.Room.Area}
	}
	if c.mob != nil {
		v["vnum"] = c.mob.Proto.Vnum
		v["flags"] = c.mob.Proto.Flags
	}
	return v
}

func itemView(it *item.Item) map[string]any {
	if it == nil {
		return nil
	}
	p := it.Proto
	v := map[string]any{
		"vnum": p.Vnum, "name": p.Name, "type": string(p.Type), "slot": string(p.Slot),
		"weight": p.Weight, "value": p.Value, "flags": p.Flags,
	}
	if p.Weapon != nil {
		v["weapon"] = map[string]any{"damage": p.Weapon.Damage, "hands": p.Weapon.Hands, "kind": p.Weapon.Kind}
	}
	if p.Armor != nil {
		v["armor"] = map[string]any{"defense": p.Armor.Defense}
	}
	if len(p.Mods) > 0 {
		mods := map[string]any{}
		for k, val := range p.Mods {
			mods[k] = val
		}
		v["mods"] = mods
	}
	return v
}

// resolveAttack asks the script how one swing goes. Default: a miss.
func (w *World) resolveAttack(att, def *Character, weapon *item.Item, round int) AttackResult {
	var r AttackResult
	if w.scripts == nil {
		return r
	}
	if err := w.scripts.Call("resolveAttack", &r, w.view(att), w.view(def), itemView(weapon), round); err != nil {
		w.hookErr("resolveAttack", err)
		return AttackResult{}
	}
	if r.Damage < 0 {
		r.Damage = 0
	}
	if r.Verb == "" {
		r.Verb = "hit"
	}
	return r
}

// onTick asks for per-round regeneration. Default: nothing.
func (w *World) onTick(c *Character) TickResult {
	var r TickResult
	if w.scripts == nil {
		return r
	}
	if err := w.scripts.Call("onTick", &r, w.view(c)); err != nil {
		w.hookErr("onTick", err)
		return TickResult{}
	}
	return r
}

// derived asks for maxima and attack count. Default keeps a character
// alive with one attack.
func (w *World) derived(c *Character) Derived {
	d := Derived{HealthMax: 1, ManaMax: 0, AttacksPerRound: 1}
	if w.scripts == nil {
		return d
	}
	if err := w.scripts.Call("derivedStats", &d, w.view(c)); err != nil {
		w.hookErr("derivedStats", err)
		return Derived{HealthMax: 1, AttacksPerRound: 1}
	}
	if d.HealthMax < 1 {
		d.HealthMax = 1
	}
	if d.ManaMax < 0 {
		d.ManaMax = 0
	}
	if d.AttacksPerRound < 1 {
		d.AttacksPerRound = 1
	}
	return d
}

// xpForKill asks how much experience a kill is worth. Default: none.
func (w *World) xpForKill(killer, victim *Character) int {
	if w.scripts == nil {
		return 0
	}
	var n int
	if err := w.scripts.Call("xpForKill", &n, w.view(killer), w.view(victim)); err != nil {
		w.hookErr("xpForKill", err)
		return 0
	}
	return max(n, 0)
}

// xpToLevel asks how much experience level needs. Default: unreachable.
func (w *World) xpToLevel(level int) int {
	if w.scripts == nil {
		return 1 << 30
	}
	var n int
	if err := w.scripts.Call("xpToLevel", &n, level); err != nil {
		w.hookErr("xpToLevel", err)
		return 1 << 30
	}
	if n <= 0 {
		return 1 << 30
	}
	return n
}

// onLevel asks what a new level grants. Default: nothing.
func (w *World) onLevel(c *Character, newLevel int) LevelResult {
	var r LevelResult
	if w.scripts == nil {
		return r
	}
	if err := w.scripts.Call("onLevel", &r, w.view(c), newLevel); err != nil {
		w.hookErr("onLevel", err)
		return LevelResult{}
	}
	return r
}

// recalc refreshes derived values and clamps vitals. Call after anything
// that can change them: login, spawn, equipment, level.
func (w *World) recalc(c *Character) {
	d := w.derived(c)
	c.HealthMax = d.HealthMax
	c.ManaMax = d.ManaMax
	c.AttacksPerRound = d.AttacksPerRound
	if c.Health > c.HealthMax {
		c.Health = c.HealthMax
	}
	if c.Mana > c.ManaMax {
		c.Mana = c.ManaMax
	}
}
