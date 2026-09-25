package world

import (
	"errors"
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/script"
)

// Rule hooks. Each wrapper builds read-only views, calls the script, and
// falls back to a harmless default if the hook is missing or broken, so a
// bad script edit never crashes the game. Errors are logged once per hook
// per script load so a broken formula is visible but not spammy. The
// contract is documented in docs/RULES.md section 1.

// AttackResult is what resolveAttack returns. Stage says why a swing did
// not land ("dodge", "block", or "miss") so the transport can say so.
type AttackResult struct {
	Hit    bool   `json:"hit"`
	Damage int    `json:"damage"`
	Crit   bool   `json:"crit"`
	Verb   string `json:"verb"`
	Stage  string `json:"stage"`
}

// TickResult is what onTick returns.
type TickResult struct {
	HealthDelta int `json:"healthDelta"`
	ManaDelta   int `json:"manaDelta"`
}

// LevelResult is what onLevel returns.
type LevelResult struct {
	StatPoints int            `json:"statPoints"`
	FeatPicks  int            `json:"featPicks"`
	StatDeltas map[string]int `json:"statDeltas"`
	Message    string         `json:"message"`
}

// CreateResult is what onCreate returns for a brand-new character.
type CreateResult struct {
	Stats      map[string]int `json:"stats"`
	StatPoints int            `json:"statPoints"`
	FeatPicks  int            `json:"featPicks"`
	Message    string         `json:"message"`
}

// Feat is one entry of featList (docs/RULES.md 7.2): a permanent effect
// with a minimum level and optional prerequisites.
type Feat struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Level       int         `json:"level"`
	Description string      `json:"description"`
	Requires    []string    `json:"requires"`
	Effect      effect.Spec `json:"effect"`
}

// kitEntry is one item of standardKit: enough to build a prototype.
type kitEntry struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Slot     string           `json:"slot"`
	Baseline string           `json:"baseline"`
	Weapon   *item.WeaponSpec `json:"weapon"`
	Armor    *item.ArmorSpec  `json:"armor"`
}

// Derived is what derivedStats returns. Speed is swings per round and may
// be fractional (docs/RULES.md 4.4).
type Derived struct {
	HealthMax int     `json:"healthMax"`
	ManaMax   int     `json:"manaMax"`
	Speed     float64 `json:"speed"`
}

// DeathRules is what deathRules returns (docs/RULES.md 4.6).
type DeathRules struct {
	XpFraction    float64 `json:"xpFraction"`
	XpLevelCap    float64 `json:"xpLevelCap"`
	CorpseRounds  int     `json:"corpseRounds"`
	RespawnHealth float64 `json:"respawnHealth"`
}

// itemBaselineResult is what itemBaseline returns.
type itemBaselineResult struct {
	Weapon item.WeaponSpec `json:"weapon"`
	Armor  item.ArmorSpec  `json:"armor"`
}

// mobBaselineResult is what mobBaseline returns.
type mobBaselineResult struct {
	Stats  map[string]int  `json:"stats"`
	Health int             `json:"health"`
	Attack item.WeaponSpec `json:"attack"`
	Armor  item.ArmorSpec  `json:"armor"`
	XP     int             `json:"xp"`
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

// call runs a required hook into out, reporting failures. It returns false
// when the default should be used.
func (w *World) call(name string, out any, args ...any) bool {
	if w.scripts == nil {
		return false
	}
	if err := w.scripts.Call(name, out, args...); err != nil {
		w.hookErr(name, err)
		return false
	}
	return true
}

// callOptional is call for hooks a rule set may leave out; a missing hook
// is silent, a broken one is reported.
func (w *World) callOptional(name string, out any, args ...any) bool {
	if w.scripts == nil || !w.scripts.Has(name) {
		return false
	}
	return w.call(name, out, args...)
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
		"name":       c.Name,
		"level":      c.Level,
		"xp":         c.Experience,
		"stats":      stats,
		"statPoints": c.StatPoints,
		"featPoints": c.FeatPoints,
		"health":     c.Health,
		"healthMax":  c.HealthMax,
		"mana":       c.Mana,
		"manaMax":    c.ManaMax,
		"speed":      c.Speed,
		"equipment":  eq,
		"effects":    effect.Views(nil, c.Effects),
		"isPlayer":   c.player != nil,
		"fighting":   c.Fighting != nil,
	}
	if c.Room != nil {
		v["room"] = map[string]any{"vnum": c.Room.Vnum, "name": c.Room.Name, "area": c.Room.Area}
	}
	if c.mob != nil {
		p := c.mob.Proto
		v["vnum"] = p.Vnum
		v["flags"] = p.Flags
		v["mob"] = map[string]any{
			"vnum": p.Vnum, "flags": p.Flags, "health": p.Resolved.Health, "xp": p.Resolved.XP,
			"attack": weaponView(p.Resolved.Attack), "armor": armorView(p.Resolved.Armor),
			"effects": effect.Views(p.Effects, nil),
		}
	}
	return v
}

func weaponView(s item.WeaponSpec) map[string]any {
	return map[string]any{"damage": s.Damage, "spread": s.Spread, "speed": s.Speed, "hands": s.Hands, "kind": s.Kind, "verb": s.Verb}
}

func armorView(s item.ArmorSpec) map[string]any {
	return map[string]any{"defense": s.Defense, "spread": s.Spread}
}

func itemView(it *item.Item) map[string]any {
	if it == nil {
		return nil
	}
	p := it.Proto
	v := map[string]any{
		"vnum": p.Vnum, "name": p.Name, "type": string(p.Type), "slot": string(p.Slot),
		"level": p.Level, "baseline": p.Baseline,
		"weight": p.Weight, "value": p.Value, "flags": p.Flags,
		"effects": it.AllEffects(),
	}
	if p.Type == item.Weapon {
		v["weapon"] = weaponView(p.Resolved.Weapon)
	}
	if p.Type == item.Armor {
		v["armor"] = armorView(p.Resolved.Armor)
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

// protoView is the script-facing snapshot of an item prototype as stated,
// for itemBaseline.
func protoView(p *item.Proto) map[string]any {
	v := map[string]any{
		"vnum": p.Vnum, "name": p.Name, "type": string(p.Type), "slot": string(p.Slot),
		"level": p.Level, "baseline": p.Baseline, "flags": p.Flags,
	}
	if p.Weapon != nil {
		v["weapon"] = weaponView(*p.Weapon)
	}
	if p.Armor != nil {
		v["armor"] = armorView(*p.Armor)
	}
	return v
}

// mobProtoView is the stated snapshot of a mob prototype, for mobBaseline.
func mobProtoView(p *mob.Proto) map[string]any {
	stats := map[string]any{}
	for k, val := range p.Stats {
		stats[k] = val
	}
	v := map[string]any{"vnum": p.Vnum, "name": p.Name, "level": p.Level, "flags": p.Flags, "stats": stats, "health": p.Health, "xp": p.XP}
	if p.Attack != nil {
		v["attack"] = weaponView(*p.Attack)
	}
	if p.Armor != nil {
		v["armor"] = armorView(*p.Armor)
	}
	return v
}

// resolveBaselines fills every prototype's Resolved numbers through the
// itemBaseline and mobBaseline hooks. Without the hooks, stated values are
// used as they are. Runs after content load and after every script reload
// so a curve edit re-derives every prototype without a restart.
func (w *World) resolveBaselines() {
	for _, p := range w.content.Items {
		w.resolveItem(p)
	}
	for _, p := range w.content.Mobs {
		p.ResolveStated()
		var r mobBaselineResult
		if w.callOptional("mobBaseline", &r, mobProtoView(p)) {
			p.Resolved = mob.Resolved{Stats: r.Stats, Health: r.Health, Attack: r.Attack, Armor: r.Armor, XP: r.XP, Source: "baseline"}
			if p.Resolved.Stats == nil {
				p.Resolved.Stats = map[string]int{}
			}
			if p.Resolved.Attack.Speed <= 0 {
				p.Resolved.Attack.Speed = 1
			}
		}
	}
	// Live characters keep pointers to prototypes, so their derived values
	// may have moved.
	for _, c := range w.allCharacters() {
		w.recalc(c)
	}
}

// resolveItem fills one prototype's Resolved numbers.
func (w *World) resolveItem(p *item.Proto) {
	p.ResolveStated()
	var r itemBaselineResult
	if w.callOptional("itemBaseline", &r, protoView(p)) {
		p.Resolved = item.Resolved{Weapon: r.Weapon, Armor: r.Armor, Source: "baseline"}
		if p.Resolved.Weapon.Speed <= 0 {
			p.Resolved.Weapon.Speed = 1
		}
	}
}

// featList asks the rules which feats exist. Optional; default none.
func (w *World) featList() []Feat {
	var list []Feat
	w.callOptional("featList", &list)
	return list
}

// standardKit asks the rules what a level-N character wears for balance
// runs, and builds resolved prototypes for it. Optional; default nothing.
func (w *World) standardKit(level int) []*item.Proto {
	var entries []kitEntry
	if !w.callOptional("standardKit", &entries, level) {
		return nil
	}
	var out []*item.Proto
	for _, e := range entries {
		p := &item.Proto{Name: e.Name, Keywords: keywordsOf(e.Name), Type: item.Type(e.Type), Slot: item.Slot(e.Slot),
			Level: level, Baseline: e.Baseline, Weapon: e.Weapon, Armor: e.Armor}
		if p.Type == item.Weapon && p.Slot == "" {
			p.Slot = "wield"
		}
		w.resolveItem(p)
		out = append(out, p)
	}
	return out
}

// keywordsOf makes keywords from a short description, dropping articles.
func keywordsOf(name string) []string {
	var out []string
	for _, word := range strings.Fields(strings.ToLower(name)) {
		switch word {
		case "a", "an", "the", "of":
			continue
		}
		out = append(out, word)
	}
	return out
}

// resolveAttack asks the script how one swing goes. Default: a miss.
func (w *World) resolveAttack(att, def *Character, weapon *item.Item, round int) AttackResult {
	var r AttackResult
	if !w.call("resolveAttack", &r, w.view(att), w.view(def), itemView(weapon), round) {
		return AttackResult{Stage: "miss"}
	}
	if r.Damage < 0 {
		r.Damage = 0
	}
	if r.Verb == "" {
		r.Verb = attackVerb(att, weapon)
	}
	if !r.Hit && r.Stage == "" {
		r.Stage = "miss"
	}
	return r
}

// attackVerb is the word for a swing when the script gives none: the
// weapon's, the mob's natural attack's, or "punch".
func attackVerb(att *Character, weapon *item.Item) string {
	if weapon != nil && weapon.Proto.Resolved.Weapon.Verb != "" {
		return weapon.Proto.Resolved.Weapon.Verb
	}
	if att.mob != nil && att.mob.Proto.Resolved.Attack.Verb != "" {
		return att.mob.Proto.Resolved.Attack.Verb
	}
	return "punch"
}

// onTick asks for per-round regeneration. Default: nothing.
func (w *World) onTick(c *Character) TickResult {
	var r TickResult
	if !w.call("onTick", &r, w.view(c)) {
		return TickResult{}
	}
	return r
}

// derived asks for maxima and speed. Default keeps a character alive with
// one swing a round.
func (w *World) derived(c *Character) Derived {
	d := Derived{HealthMax: 1, ManaMax: 0, Speed: 1}
	if !w.call("derivedStats", &d, w.view(c)) {
		return Derived{HealthMax: 1, Speed: 1}
	}
	if d.HealthMax < 1 {
		d.HealthMax = 1
	}
	if d.ManaMax < 0 {
		d.ManaMax = 0
	}
	if d.Speed <= 0 {
		d.Speed = 1
	}
	return d
}

// xpForKill asks how much experience a kill is worth. Default: none.
func (w *World) xpForKill(killer, victim *Character) int {
	var n int
	if !w.call("xpForKill", &n, w.view(killer), w.view(victim)) {
		return 0
	}
	return max(n, 0)
}

// xpToLevel asks how much total experience level needs. Default:
// unreachable.
func (w *World) xpToLevel(level int) int {
	if level <= 1 {
		return 0
	}
	var n int
	if !w.call("xpToLevel", &n, level) {
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
	if !w.call("onLevel", &r, w.view(c), newLevel) {
		return LevelResult{}
	}
	return r
}

// onCreate asks how a new character's sheet starts. Optional; default is
// an empty sheet.
func (w *World) onCreate(c *Character) CreateResult {
	var r CreateResult
	w.callOptional("onCreate", &r, w.view(c))
	return r
}

// deathRules asks what death costs. Optional; the default costs nothing
// and leaves a corpse for a while.
func (w *World) deathRules() DeathRules {
	r := DeathRules{CorpseRounds: 300, RespawnHealth: 1}
	var got DeathRules
	if w.callOptional("deathRules", &got) {
		r = got
	}
	if r.RespawnHealth <= 0 {
		r.RespawnHealth = 1
	}
	return r
}

// recalc refreshes derived values and clamps vitals. Call after anything
// that can change them: login, spawn, equipment, level, effects.
func (w *World) recalc(c *Character) {
	d := w.derived(c)
	c.HealthMax = d.HealthMax
	c.ManaMax = d.ManaMax
	c.Speed = d.Speed
	if c.Health > c.HealthMax {
		c.Health = c.HealthMax
	}
	if c.Mana > c.ManaMax {
		c.Mana = c.ManaMax
	}
}
