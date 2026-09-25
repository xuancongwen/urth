package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"urth/internal/effect"
	"urth/internal/item"
)

// Test rules: 10 + 2*level health, 1 damage unarmed, weapon damage with a
// weapon, +1 health per round out of combat, 60 xp per kill, 100 xp per
// level.

func TestKillMobAwardsXPAndLevels(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "n") // dog is here (12 health at level 1)
	bob.take()
	dog := w.contents(w.players[1].Room).mobs[0]
	send(w, 1, "kill dog")
	out := bob.take()
	if !strings.Contains(out, "Your punch hits a stray dog. [1]") {
		t.Fatalf("first swing: %q", out)
	}
	for i := 0; i < 10; i++ { // one round: the dog swings back
		w.Tick()
	}
	if out := bob.take(); !strings.Contains(out, "A stray dog's punch hits you. [1]") {
		t.Fatalf("mob did not retaliate: %q", out)
	}
	send(w, 1, "n")
	if out := bob.take(); !strings.Contains(out, "You are fighting") {
		t.Fatalf("moving while fighting allowed: %q", out)
	}
	// Bob has 12 hp too; both deal 1 per round. Bob struck first, so the
	// dog dies on the 12th swing, before Bob does.
	dead := false
	for i := 0; i < 200 && !dead; i++ {
		w.Tick()
		if o := bob.take(); strings.Contains(o, "A stray dog is DEAD!!") {
			dead = true
			out = o
		}
	}
	if !dead {
		t.Fatal("dog never died")
	}
	if !strings.Contains(out, "You receive 60 experience points.") {
		t.Fatalf("xp not awarded: %q", out)
	}
	if w.players[1].Fighting != nil {
		t.Fatal("still fighting after kill")
	}
	for _, m := range w.contents(w.players[1].Room).mobs {
		if m == dog {
			t.Fatal("dead dog still in room")
		}
	}
	if dog.Room != nil {
		t.Fatal("dead dog still placed")
	}
	// A second dog kill crosses 100 xp: level 2 with the stat delta.
	for _, a := range w.areas {
		w.resetArea(a)
	}
	w.players[1].Health = w.players[1].HealthMax
	send(w, 1, "kill dog")
	leveled := false
	for i := 0; i < 200 && !leveled; i++ {
		w.Tick()
		if o := bob.take(); strings.Contains(o, "Level up.") {
			leveled = true
			out = o
		}
	}
	if !leveled {
		t.Fatal("never leveled")
	}
	p := w.players[1]
	if p.Level != 2 || p.Stats["might"] != 1 || p.HealthMax != 14 || p.Health != 14 || p.StatPoints != 1 {
		t.Fatalf("level-up state wrong: level=%d stats=%v hp=%d/%d points=%d", p.Level, p.Stats, p.Health, p.HealthMax, p.StatPoints)
	}
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "Level 2") || !strings.Contains(o, "Might        1") || !strings.Contains(o, "Stat points  1") {
		t.Fatalf("score: %q", o)
	}
}

func TestWeaponDamageAndDeathDrops(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "kill guard") // guard wields the 4-damage sword and carries bread
	bob.take()
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	out := bob.take()
	if !strings.Contains(out, "A city guard's slash hits you. [4]") {
		t.Fatalf("weapon damage not used: %q", out)
	}
	// Bob: 12 hp, takes 4/round after his own 1. Guard kills Bob first.
	died := false
	for i := 0; i < 200 && !died; i++ {
		w.Tick()
		if o := bob.take(); strings.Contains(o, "You have been KILLED!!") {
			died = true
			out = o
		}
	}
	if !died {
		t.Fatal("bob never died")
	}
	p := w.players[1]
	if p.Health != p.HealthMax || p.Room.Vnum != 1 || p.Fighting != nil {
		t.Fatalf("respawn state wrong: hp=%d room=%d fighting=%v", p.Health, p.Room.Vnum, p.Fighting)
	}
	guard := w.contents(p.Room).mobs[0]
	if guard.Fighting != nil {
		t.Fatal("guard still fighting a dead player")
	}
	// Now make the guard die: give bob the sword equivalent by editing
	// health directly, then confirm drops.
	guard.Health = 1
	send(w, 1, "kill guard")
	out = bob.take()
	if !strings.Contains(out, "A city guard is DEAD!!") {
		t.Fatalf("guard death: %q", out)
	}
	corpse := findCorpse(w.contents(p.Room).items, "city guard")
	if corpse == nil || !hasVnum(corpse.Contents, 10) || !hasVnum(corpse.Contents, 13) {
		t.Fatalf("guard's sword and bread not in a corpse: %v", w.contents(p.Room).items)
	}
	send(w, 1, "look")
	if o := bob.take(); !strings.Contains(o, "The corpse of a city guard lies here.") {
		t.Fatalf("corpse not listed: %q", o)
	}
	send(w, 1, "get all guard") // the corpse carries the guard's keywords; Bob's own corpse is here too
	if o := bob.take(); !strings.Contains(o, "You get a rusty sword from the corpse of a city guard.") {
		t.Fatalf("looting: %q", o)
	}
	if !hasVnum(p.Inventory, 10) || !hasVnum(p.Inventory, 13) {
		t.Fatalf("loot not in inventory: %v", p.Inventory)
	}
	// The corpse decays after corpseRounds (3 in the test rules).
	for i := 0; i < 30; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "The corpse of a city guard crumbles into dust.") {
		t.Fatalf("corpse did not decay: %q", o)
	}
	if findCorpse(w.contents(p.Room).items, "city guard") != nil {
		t.Fatal("corpse still present after decay")
	}
}

func findCorpse(list []*item.Item, of string) *item.Item {
	for _, it := range list {
		if it.Proto.HasFlag("corpse") && strings.Contains(it.Name(), of) {
			return it
		}
	}
	return nil
}

func TestFleeEndsFight(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "flee")
	if o := bob.take(); !strings.Contains(o, "aren't fighting") {
		t.Fatalf("flee when idle: %q", o)
	}
	send(w, 1, "kill guard")
	bob.take()
	send(w, 1, "flee")
	o := bob.take()
	if !strings.Contains(o, "You flee north!") {
		t.Fatalf("flee: %q", o)
	}
	p := w.players[1]
	if p.Fighting != nil || p.Room.Vnum != 2 {
		t.Fatalf("flee state: fighting=%v room=%d", p.Fighting, p.Room.Vnum)
	}
	guard := w.contents(w.content.Rooms.Rooms[1]).mobs[0]
	if guard.Fighting != nil {
		t.Fatal("guard still fighting after target fled")
	}
}

func TestRegenOutOfCombat(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	p.Health = 5
	for i := 0; i < 30; i++ { // 3 rounds
		w.Tick()
	}
	if p.Health != 8 {
		t.Fatalf("expected 8 health after 3 rounds of regen, got %d", p.Health)
	}
}

func TestEditingRulesChangesOutcomeWithoutRestart(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "[1]") {
		t.Fatalf("baseline damage: %q", o)
	}
	send(w, 1, "flee")
	bob.take()

	edited := strings.Replace(testRules, "damage: w && w.weapon ? w.weapon.damage : 1", "damage: 7", 1)
	path := filepath.Join(w.scriptDir, "rules.js")
	future := time.Now().Add(3 * time.Second)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, future, future)
	// The reload check runs on the round; wait one round.
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	bob.take()
	send(w, 1, "s")
	bob.take()
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "Your punch hits a city guard. [7]") {
		t.Fatalf("edited damage not live: %q", o)
	}
	send(w, 1, "flee")
	bob.take()

	// A broken edit keeps the old rules and warns the admin once.
	later := future.Add(3 * time.Second)
	os.WriteFile(path, []byte("function resolveAttack( {"), 0o644)
	os.Chtimes(path, later, later)
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "Script reload failed") {
		t.Fatalf("admin not warned: %q", o)
	}
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); strings.Contains(o, "Script reload failed") {
		t.Fatalf("warning repeated every round: %q", o)
	}
	for _, a := range w.areas {
		w.resetArea(a) // make sure a guard is there to hit
	}
	send(w, 1, "s")
	bob.take()
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "[7]") {
		t.Fatalf("old rules not kept: %q", o)
	}
}

func TestSimulateCommand(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "simulate 21 20 50 7") // dog vs guard with sword
	o := bob.take()
	if !strings.Contains(o, "Simulated 50 fights") || !strings.Contains(o, "B wins 100.0%") {
		t.Fatalf("simulate: %q", o)
	}
	send(w, 1, "simulate me 21 20 1")
	o = bob.take()
	if !strings.Contains(o, "you vs a stray dog") || !strings.Contains(o, "A wins 100.0%") {
		t.Fatalf("simulate me: %q", o)
	}
	// Seeded runs are identical.
	send(w, 1, "simulate 21 21 20 42")
	a := bob.take()
	send(w, 1, "simulate 21 21 20 42")
	b := bob.take()
	if a != b {
		t.Fatalf("seeded simulations differ:\n%s\n%s", a, b)
	}
	// The live world was not touched.
	if w.countMobs(w.content.Mobs[21]) != 1 || w.countMobs(w.content.Mobs[20]) != 1 {
		t.Fatal("simulation leaked mobs into the world")
	}
	if w.players[1].Health != w.players[1].HealthMax {
		t.Fatal("simulation damaged the real player")
	}
}

func TestSheetPersists(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w1, _ := testWorldWithStore(t, playerDir)
	bob := login(t, w1, 1, "Bob")
	p := w1.players[1]
	p.Level, p.Experience, p.Health = 3, 250, 7
	p.Stats["might"] = 2
	p.StatPoints = 4
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "feat", Params: map[string]any{"which": "tough"}}})
	send(w1, 1, "quit")
	bob.take()
	w2, _ := testWorldWithStore(t, playerDir)
	signin(t, w2, 1, "Bob", "secret5")
	q := w2.players[1]
	if q.Level != 3 || q.Experience != 250 || q.Health != 7 || q.HealthMax != 21 || q.Stats["might"] != 2 || q.StatPoints != 4 {
		t.Fatalf("sheet not restored: level=%d xp=%d hp=%d/%d stats=%v points=%d", q.Level, q.Experience, q.Health, q.HealthMax, q.Stats, q.StatPoints)
	}
	if len(q.Effects) != 1 || q.Effects[0].Kind != "feat" || q.Effects[0].Params["which"] != "tough" {
		t.Fatalf("effects not restored: %+v", q.Effects)
	}
}

// setRules replaces the rule script and reloads it immediately.
func setRules(t *testing.T, w *World, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(w.scriptDir, "rules.js"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.scripts.Load(); err != nil {
		t.Fatal(err)
	}
	w.hookErrors = map[string]time.Time{}
	w.resolveBaselines()
}

func countOccurrences(s, sub string) int { return strings.Count(s, sub) }

func TestSpeedMeter(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	// Bob swings twice a round, the guard once every other round.
	setRules(t, w, strings.Replace(testRules, "speed: 1", "speed: c.isPlayer ? 2 : 0.5", 1))
	w.players[1].HealthMax, w.players[1].Health = 1000, 1000
	guard := w.contents(w.players[1].Room).mobs[0]
	guard.HealthMax, guard.Health = 1000, 1000
	send(w, 1, "kill guard")
	bob.take()
	for i := 0; i < 40; i++ { // four rounds
		w.Tick()
	}
	o := bob.take()
	if n := countOccurrences(o, "Your punch hits a city guard. [1]"); n != 8 {
		t.Fatalf("expected 8 swings over 4 rounds at speed 2, got %d: %q", n, o)
	}
	if n := countOccurrences(o, "A city guard's slash hits you. [4]"); n != 2 {
		t.Fatalf("expected 2 swings over 4 rounds at speed 0.5, got %d: %q", n, o)
	}
}

func TestDodgeAndBlockMessages(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, strings.Replace(testRules, `return { hit: true, damage: w && w.weapon ? w.weapon.damage : 1, crit: false, verb: w && w.weapon ? w.weapon.verb : "punch", stage: "" };`,
		`if (a.isPlayer) return { hit: false, stage: "dodge", verb: "punch" }; return { hit: false, stage: "block", verb: "slash" };`, 1))
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "A city guard dodges your punch.") {
		t.Fatalf("dodge message: %q", o)
	}
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "You block a city guard's slash.") {
		t.Fatalf("block message: %q", o)
	}
}

const baselineRules = testRules + `
function resolveAttack(a, d, w, round) {
  var atk = w && w.weapon ? w.weapon : (a.mob ? a.mob.attack : { damage: 1, verb: "punch" });
  return { hit: true, damage: atk.damage, crit: false, verb: atk.verb, stage: "" };
}
function itemBaseline(p) {
  if (p.type !== "weapon" && p.type !== "armor") return {};
  var w = p.weapon || {}, a = p.armor || {};
  return {
    weapon: { damage: w.damage || 4 + 2 * p.level, spread: w.spread || 0.2, speed: w.speed || 1, hands: w.hands, verb: w.verb || "hit" },
    armor: { defense: a.defense || p.level + 2, spread: a.spread || 0.2 }
  };
}
function mobBaseline(p) {
  var a = p.attack || {};
  return { stats: { might: p.stats.might || p.level }, health: p.health, xp: p.xp,
    attack: { damage: a.damage || 2, speed: 1, verb: a.verb || "bite" }, armor: { defense: p.level + 2 } };
}
`

func TestBaselinesResolveAndReload(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	// Without the hooks, stated values stand and unstated ones are zero.
	spear := w.content.Items[15]
	if spear.Resolved.Weapon.Damage != 0 || spear.Resolved.Weapon.Speed != 1 || spear.Resolved.Source != "stated" {
		t.Fatalf("stated resolution wrong: %+v", spear.Resolved)
	}
	setRules(t, w, baselineRules)
	if spear.Resolved.Weapon.Damage != 10 || spear.Resolved.Weapon.Verb != "hit" || spear.Resolved.Weapon.Hands != 2 || spear.Resolved.Source != "baseline" {
		t.Fatalf("baseline not applied to spear: %+v", spear.Resolved)
	}
	sword := w.content.Items[10]
	if sword.Resolved.Weapon.Damage != 4 || sword.Resolved.Weapon.Verb != "slash" {
		t.Fatalf("stated values did not win: %+v", sword.Resolved)
	}
	if cap := w.content.Items[11]; cap.Resolved.Armor.Defense != 1 || cap.Resolved.Armor.Spread != 0.2 {
		t.Fatalf("armor mix of stated and baseline wrong: %+v", cap.Resolved)
	}
	dog := w.content.Mobs[21]
	if dog.Resolved.Attack.Verb != "bite" || dog.Resolved.Stats["might"] != 1 || dog.Resolved.Armor.Defense != 3 {
		t.Fatalf("mob baseline wrong: %+v", dog.Resolved)
	}
	// Live mobs picked up the new stats, and the natural verb reaches messages.
	send(w, 1, "n")
	bob.take()
	if m := w.contents(w.players[1].Room).mobs[0]; m.Stats["might"] != 0 {
		// Stats are copied at spawn; a reload does not rewrite a live mob's
		// base stats, only its derived values. Respawned mobs get the new ones.
		t.Fatalf("live mob stats rewritten unexpectedly: %v", m.Stats)
	}
	send(w, 1, "kill dog")
	bob.take()
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "A stray dog's bite hits you. [2]") {
		t.Fatalf("natural attack verb and damage: %q", o)
	}
	send(w, 1, "flee")
	bob.take()
	// The builder sees where numbers came from.
	send(w, 1, "load obj 15")
	bob.take()
	send(w, 1, "stat spear")
	if o := bob.take(); !strings.Contains(o, "Numbers from: baseline") || !strings.Contains(o, "damage 10") || !strings.Contains(o, "Baseline: standard") {
		t.Fatalf("stat item: %q", o)
	}
	// Editing the curve re-derives every prototype on reload.
	setRules(t, w, strings.Replace(baselineRules, "4 + 2 * p.level", "100 * p.level", 1))
	if spear.Resolved.Weapon.Damage != 300 {
		t.Fatalf("reload did not re-derive: %+v", spear.Resolved)
	}
}

func TestDeathXPLossAndRespawnHealth(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	rules := DeathRules{XpFraction: 0.2, XpLevelCap: 0.5}
	// Level 2 spans 100 to 200. 20% of 150 is 30, under the 50-point cap.
	p.Level, p.Experience = 2, 150
	if got := w.deathXPLoss(p.Character, rules); got != 30 {
		t.Fatalf("plain loss: got %d want 30", got)
	}
	// 20% of 400 is 80, capped at half the level's cost (50).
	p.Level, p.Experience = 4, 400
	if got := w.deathXPLoss(p.Character, rules); got != 50 {
		t.Fatalf("capped loss: got %d want 50", got)
	}
	// Never below the level's threshold.
	p.Level, p.Experience = 2, 110
	if got := w.deathXPLoss(p.Character, rules); got != 10 {
		t.Fatalf("floored loss: got %d want 10", got)
	}
	// Live: the rules in play cost 20% and respawn at a quarter health.
	setRules(t, w, strings.Replace(testRules, "xpFraction: 0, xpLevelCap: 0, corpseRounds: 3, respawnHealth: 1", "xpFraction: 0.2, xpLevelCap: 0.5, corpseRounds: 3, respawnHealth: 0.25", 1))
	p.Level, p.Experience = 2, 150
	w.recalc(p.Character)
	p.Health = 1
	send(w, 1, "kill guard")
	bob := w.players[1].conn.(*fakeConn)
	bob.take()
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	o := bob.take()
	if !strings.Contains(o, "You lose 30 experience points.") || p.Experience != 120 {
		t.Fatalf("death xp: %q xp=%d", o, p.Experience)
	}
	// Regen may have run once in the same round as the respawn.
	if p.Health < p.HealthMax/4 || p.Health > p.HealthMax/4+1 {
		t.Fatalf("respawn health: %d/%d", p.Health, p.HealthMax)
	}
}

func TestTrainSpendsStatPoints(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "train mi")
	if o := bob.take(); !strings.Contains(o, "no stat points") {
		t.Fatalf("train with no points: %q", o)
	}
	w.players[1].StatPoints = 2
	send(w, 1, "train")
	if o := bob.take(); !strings.Contains(o, "2 stat points") || !strings.Contains(o, "might 0") {
		t.Fatalf("train listing: %q", o)
	}
	send(w, 1, "train mi")
	if o := bob.take(); !strings.Contains(o, "You train might to 1. 1 points left.") {
		t.Fatalf("train: %q", o)
	}
	send(w, 1, "train luck")
	if o := bob.take(); !strings.Contains(o, "can't train that") {
		t.Fatalf("train unknown: %q", o)
	}
	if p := w.players[1]; p.Stats["might"] != 1 || p.StatPoints != 1 {
		t.Fatalf("state: %v %d", p.Stats, p.StatPoints)
	}
}

func TestTimedEffectsExpire(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "tough"}, Rounds: 2})
	w.recalc(p.Character)
	if p.HealthMax != 17 { // 10 + 2 + 5 for one effect
		t.Fatalf("effect not counted: %d", p.HealthMax)
	}
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if len(p.Effects) != 1 || p.Effects[0].Rounds != 1 {
		t.Fatalf("after one round: %+v", p.Effects)
	}
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if len(p.Effects) != 0 || p.HealthMax != 12 {
		t.Fatalf("effect did not expire: %+v hp=%d", p.Effects, p.HealthMax)
	}
}

const featRules = testRules + `
function onLevel(c, l) { return { statPoints: 1, featPicks: 1, statDeltas: {}, message: "Level up." }; }
function featList() {
  return [
    { id: "tough", name: "Toughness", level: 1, description: "More health.", effect: { kind: "tough", params: { amount: 5 } } },
    { id: "keen", name: "Keen Edge", level: 3, description: "Crits.", effect: { kind: "crit", params: { chance: 0.5 } } },
    { id: "deadly", name: "Deadly Edge", level: 1, requires: ["keen"], description: "More crits.", effect: { kind: "crit", params: { chance: 1 } } }
  ];
}
function standardKit(level) {
  return [ { name: "a standard sword", type: "weapon", slot: "wield", baseline: "standard", weapon: { hands: 1, verb: "slash" } },
           { name: "a standard helm", type: "armor", slot: "head", baseline: "medium" } ];
}
function itemBaseline(p) {
  if (p.type === "weapon") return { weapon: { damage: 4 + 2 * p.level, speed: 1, verb: p.weapon.verb } };
  if (p.type === "armor") return { armor: { defense: p.level + 2 } };
  return {};
}
`

func TestFeatsPickAndGate(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, featRules)
	send(w, 1, "feat")
	if o := bob.take(); !strings.Contains(o, "You may choose 0 feats.") || !strings.Contains(o, "Toughness") || !strings.Contains(o, "(level 3)") || !strings.Contains(o, "needs Keen Edge") {
		t.Fatalf("feat list: %q", o)
	}
	send(w, 1, "feat tough")
	if o := bob.take(); !strings.Contains(o, "no feat picks") {
		t.Fatalf("pick without points: %q", o)
	}
	p := w.players[1]
	p.FeatPoints = 2
	send(w, 1, "feat keen")
	if o := bob.take(); !strings.Contains(o, "Keen Edge needs level 3.") {
		t.Fatalf("level gate: %q", o)
	}
	send(w, 1, "feat deadly")
	if o := bob.take(); !strings.Contains(o, "Deadly Edge needs Keen Edge.") {
		t.Fatalf("requirement gate: %q", o)
	}
	send(w, 1, "feat tough")
	if o := bob.take(); !strings.Contains(o, "You learn Toughness.") {
		t.Fatalf("pick: %q", o)
	}
	// The feat is a permanent effect tagged with its id, and derived stats
	// see it (the test rules add 5 health per effect).
	if len(p.Effects) != 1 || p.Effects[0].Params["feat"] != "tough" || p.Effects[0].Rounds != 0 || p.HealthMax != 17 || p.FeatPoints != 1 {
		t.Fatalf("feat state: effects=%+v hp=%d points=%d", p.Effects, p.HealthMax, p.FeatPoints)
	}
	send(w, 1, "feat tough")
	if o := bob.take(); !strings.Contains(o, "already have Toughness") {
		t.Fatalf("double pick: %q", o)
	}
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "Feats     Toughness") || !strings.Contains(o, "Feat picks   1") {
		t.Fatalf("score: %q", o)
	}
	// Levelling grants a pick.
	w.grantXP(p.Character, 100)
	w.flush()
	if o := bob.take(); !strings.Contains(o, "You may choose 2 feats.") || p.FeatPoints != 2 {
		t.Fatalf("level pick: %q points=%d", o, p.FeatPoints)
	}
}

func TestSimulateFighterUsesStandardKit(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, featRules)
	f := w.simFighter(5)
	if f.Level != 5 || f.Stats["might"] != 0 || f.Experience != 400 {
		t.Fatalf("fighter sheet: level=%d stats=%v xp=%d", f.Level, f.Stats, f.Experience)
	}
	sword, helm := f.Equipment["wield"], f.Equipment["head"]
	if sword == nil || helm == nil || sword.Proto.Resolved.Weapon.Damage != 14 || helm.Proto.Resolved.Armor.Defense != 7 || sword.Proto.Level != 5 {
		t.Fatalf("kit wrong: %+v %+v", sword, helm)
	}
	send(w, 1, "simulate fighter:5 21 20 3")
	if o := bob.take(); !strings.Contains(o, "a level 5 fighter vs a stray dog") || !strings.Contains(o, "A wins 100.0%") {
		t.Fatalf("simulate fighter: %q", o)
	}
	send(w, 1, "simulate fighter:x 21")
	if o := bob.take(); !strings.Contains(o, "Fighter level must be") {
		t.Fatalf("bad level: %q", o)
	}
}

const magicRules = testRules + `
function spellList() {
  return [
    { id: "bolt", name: "Bolt", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: true, cooldown: 3,
      materials: [{ material: "ash", count: 1 }], target: "single", save: "reflex", saveEffect: "half" },
    { id: "ward", name: "Ward", branch: "abjuration" === "x" ? "" : "arcane", school: "abjuration", castRounds: 0, interruptOnDamage: false, cooldown: 0,
      materials: [], target: "self", save: "none" },
    { id: "nova", name: "Nova", branch: "arcane", school: "evocation", castRounds: 2, interruptOnDamage: false, cooldown: 0,
      materials: [], target: "area", save: "none" },
    { id: "mend", name: "Mend", branch: "divine", deity: "good", castRounds: 0, interruptOnDamage: false, cooldown: 0,
      materials: [], target: "ally", save: "none" }
  ];
}
function resolveCast(caster, targets, spell) {
  var out = { ok: true, message: "", consume: [], targets: [], casterEffects: [] };
  for (var i = 0; i < targets.length; i++) {
    var t = targets[i];
    if (spell.id === "bolt") out.targets.push({ index: i, damage: 5, saved: t.name === "Dodger", effects: [] });
    if (spell.id === "nova") out.targets.push({ index: i, damage: 3, effects: [] });
    if (spell.id === "ward") out.targets.push({ index: i, effects: [{ kind: "tough", params: {}, rounds: 4 }] });
    if (spell.id === "mend") out.targets.push({ index: i, heal: 6 });
  }
  return out;
}
`

func giveItem(w *World, c *Character, vnum int) *item.Item {
	it := item.New(w.content.Items[vnum])
	c.Inventory = append(c.Inventory, it)
	return it
}

func TestTotemSacrificeAndSpellAccess(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, magicRules)
	send(w, 1, "spells")
	if o := bob.take(); !strings.Contains(o, "You know no spells.") {
		t.Fatalf("no spells: %q", o)
	}
	send(w, 1, "cast bolt guard")
	if o := bob.take(); !strings.Contains(o, "don't know that spell") {
		t.Fatalf("cast unknown: %q", o)
	}
	giveItem(w, w.players[1].Character, 16) // the ember totem: evocation
	send(w, 1, "consume totem")
	if o := bob.take(); !strings.Contains(o, "The ways of evocation open to you.") {
		t.Fatalf("consume totem: %q", o)
	}
	send(w, 1, "spells")
	if o := bob.take(); !strings.Contains(o, "Bolt") || strings.Contains(o, "Ward") || !strings.Contains(o, "needs 1 ash") {
		t.Fatalf("spells after totem: %q", o)
	}
	// Sacrifice only at the temple, only the right item.
	giveItem(w, w.players[1].Character, 18) // the sun cup: sacrifice good
	send(w, 1, "sacrifice cup")
	if o := bob.take(); !strings.Contains(o, "Drop it first, or find a temple") {
		t.Fatalf("sacrifice outside temple: %q", o)
	}
	send(w, 1, "n")
	bob.take()
	send(w, 1, "e") // room 3 is the temple in the test world
	bob.take()
	send(w, 1, "sacrifice sack")
	if o := bob.take(); !strings.Contains(o, "don't see that here") {
		t.Fatalf("sacrifice missing: %q", o)
	}
	send(w, 1, "sacrifice cup")
	if o := bob.take(); !strings.Contains(o, "apostle of the good god") {
		t.Fatalf("sacrifice: %q", o)
	}
	send(w, 1, "spells")
	if o := bob.take(); !strings.Contains(o, "Mend") || !strings.Contains(o, "Apostle of the good god") {
		t.Fatalf("spells after sacrifice: %q", o)
	}
	// Access persists.
	p := w.players[1]
	w.save(p)
	if len(p.rec.Schools) != 1 || p.rec.Schools[0] != "evocation" || p.rec.Deity != "good" {
		t.Fatalf("access not saved: %+v %q", p.rec.Schools, p.rec.Deity)
	}
}

func TestCastTimingMaterialsAndInterruption(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, magicRules)
	p := w.players[1]
	p.Schools = []string{"evocation", "abjuration"}
	p.HealthMax, p.Health = 100, 100
	send(w, 1, "cast bolt guard")
	if o := bob.take(); !strings.Contains(o, "You need 1 ash to cast Bolt.") {
		t.Fatalf("materials check: %q", o)
	}
	giveItem(w, p.Character, 17) // ash
	giveItem(w, p.Character, 17)
	send(w, 1, "cast bolt guard")
	if o := bob.take(); !strings.Contains(o, "You begin casting Bolt.") {
		t.Fatalf("begin cast: %q", o)
	}
	if n := len(materialsHeld(p.Character, "ash")); n != 1 {
		t.Fatalf("material not committed at start: %d ash held", n)
	}
	if p.casting == nil || p.casting.rounds != 1 {
		t.Fatalf("casting state: %+v", p.casting)
	}
	for i := 0; i < 10; i++ { // one round: the cast completes
		w.Tick()
	}
	o := bob.take()
	if !strings.Contains(o, "You cast Bolt.") || !strings.Contains(o, "Your bolt hits a city guard. [5]") {
		t.Fatalf("cast resolved: %q", o)
	}
	guard := w.contents(p.Room).mobs[0]
	if guard.Health != 12-5 || guard.Fighting != p.Character || p.Fighting != guard.Character {
		t.Fatalf("hostile cast did not start the fight: hp=%d", guard.Health)
	}
	send(w, 1, "cast bolt")
	if o := bob.take(); !strings.Contains(o, "Bolt is not ready for another 3 rounds.") {
		t.Fatalf("cooldown: %q", o)
	}
	// A free cast resolves at once and attaches a timed effect.
	send(w, 1, "cast ward")
	if o := bob.take(); !strings.Contains(o, "You cast Ward.") || !strings.Contains(o, "Your ward takes hold.") {
		t.Fatalf("instant cast: %q", o)
	}
	if len(p.Effects) != 1 || p.Effects[0].Kind != "tough" || p.Effects[0].Rounds < 3 || p.Effects[0].Rounds > 4 {
		t.Fatalf("effect not attached: %+v", p.Effects)
	}
	// Damage interrupts an interruptible cast; materials are gone.
	send(w, 1, "flee")
	bob.take()
	send(w, 1, "s")
	bob.take()
	for i := 0; i < 40; i++ { // wait out the cooldown
		w.Tick()
	}
	bob.take()
	send(w, 1, "kill guard")
	bob.take()
	send(w, 1, "cast bolt")
	if o := bob.take(); !strings.Contains(o, "You begin casting Bolt.") {
		t.Fatalf("second cast: %q", o)
	}
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "Your Bolt is interrupted!") {
		t.Fatalf("damage interruption: %q", o)
	}
	if len(materialsHeld(p.Character, "ash")) != 0 || p.casting != nil {
		t.Fatal("interrupted cast should keep the materials spent and clear the cast")
	}
	// Movement interrupts too.
	send(w, 1, "flee")
	bob.take()
	send(w, 1, "cast nova")
	bob.take()
	send(w, 1, "s")
	if o := bob.take(); !strings.Contains(o, "You stop casting as you move.") {
		t.Fatalf("movement interruption: %q", o)
	}
}

func TestAreaSpellSafeRoomAndGroupXP(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	setRules(t, w, magicRules)
	w.players[1].Schools = []string{"evocation"}
	// Alice follows Bob and joins his group.
	send(w, 2, "follow bob")
	if o := alice.take(); !strings.Contains(o, "You now follow Bob.") {
		t.Fatalf("follow: %q", o)
	}
	send(w, 1, "group alice")
	if o := bob.take(); !strings.Contains(o, "Alice joins your group.") {
		t.Fatalf("group: %q", o)
	}
	send(w, 1, "gtell hello")
	if o := alice.take(); !strings.Contains(o, "Bob tells the group 'hello'") {
		t.Fatalf("gtell: %q", o)
	}
	// Nova hits everyone here who is not in the group: the guard, not Alice.
	send(w, 1, "cast nova")
	bob.take()
	for i := 0; i < 20; i++ {
		w.Tick()
	}
	o := bob.take()
	if !strings.Contains(o, "Your nova hits a city guard. [3]") || strings.Contains(o, "hits Alice") {
		t.Fatalf("area targeting: %q", o)
	}
	if w.players[2].Health != w.players[2].HealthMax {
		t.Fatal("area spell hit a group member")
	}
	// The guard fights Bob; Alice auto-assists because she is grouped and here.
	if w.players[2].Fighting == nil {
		t.Fatal("grouped player did not assist")
	}
	// A kill pays every grouped player here.
	guard := w.contents(w.players[1].Room).mobs[0]
	guard.Health = 1
	w.attackRound(w.players[1].Character, guard.Character)
	w.Tick()
	bob.take()
	alice.take()
	if w.players[1].Experience != 60 || w.players[2].Experience != 60 {
		t.Fatalf("group xp: bob=%d alice=%d", w.players[1].Experience, w.players[2].Experience)
	}
	// Followers move with the leader; the safe room forbids fighting.
	send(w, 1, "n")
	bob.take()
	if o := alice.take(); !strings.Contains(o, "You follow Bob.") || w.players[2].Room.Vnum != 2 {
		t.Fatalf("follow move: %q room=%d", o, w.players[2].Room.Vnum)
	}
	send(w, 1, "kill alice")
	if o := bob.take(); !strings.Contains(o, "can't attack a member of your group") {
		t.Fatalf("group protection: %q", o)
	}
	send(w, 1, "e") // room 3: safe temple
	bob.take()
	alice.take()
	send(w, 2, "follow self")
	alice.take()
	bob.take()
	send(w, 1, "kill alice")
	if o := bob.take(); !strings.Contains(o, "You cannot fight here.") {
		t.Fatalf("safe room: %q", o)
	}
}

func TestKillSwitchesTargetAndManyOnOne(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.HealthMax, p.Health = 1000, 1000
	send(w, 1, "n") // two dogs
	bob.take()
	dogs := w.contents(p.Room).mobs
	if len(dogs) < 2 {
		t.Skip("need two dogs")
	}
	send(w, 1, "kill dog")
	bob.take()
	send(w, 1, "kill 2.dog")
	if o := bob.take(); !strings.Contains(o, "You turn on a stray dog.") {
		t.Fatalf("switch: %q", o)
	}
	if p.Fighting != dogs[1].Character {
		t.Fatal("target did not switch")
	}
	// The first dog keeps fighting Bob: two attackers, one target.
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if n := countOccurrences(bob.take(), "A stray dog's punch hits you. [1]"); n != 2 {
		t.Fatalf("expected both dogs to swing, got %d", n)
	}
	if en := w.enemiesOf(p.Character); len(en) != 2 {
		t.Fatalf("enemies: %d", len(en))
	}
	// When the current target dies, Bob turns on the other attacker.
	dogs[1].Health = 1
	w.attackRound(p.Character, dogs[1].Character)
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if p.Fighting != dogs[0].Character {
		t.Fatal("did not retarget the remaining attacker")
	}
}

func TestConsiderAndConditionLines(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	send(w, 1, "consider guard")
	if o := bob.take(); !strings.Contains(o, "A fair fight.") || !strings.Contains(o, "A city guard looks perfectly healthy.") {
		t.Fatalf("consider even: %q", o)
	}
	guard := w.contents(p.Room).mobs[0]
	guard.Level = 6
	send(w, 1, "consider guard")
	if o := bob.take(); !strings.Contains(o, "You ARE mad!") {
		t.Fatalf("consider +5: %q", o)
	}
	guard.Level = 1
	// A rules hook overrides the wording.
	setRules(t, w, testRules+`function consider(me, t) { return "Verdict from the rules."; }`)
	send(w, 1, "consider guard")
	if o := bob.take(); !strings.Contains(o, "Verdict from the rules.") {
		t.Fatalf("consider hook: %q", o)
	}
	// Bands.
	guard.HealthMax = 100
	for hp, want := range map[int]string{100: "looks perfectly healthy", 85: "looks slightly wounded", 70: "looks wounded", 45: "looks badly wounded", 25: "looks gravely wounded", 5: "is near death"} {
		guard.Health = hp
		if got := condition(guard.Character); got != want {
			t.Fatalf("condition at %d: got %q want %q", hp, got, want)
		}
	}
	// Each round, a fighting player sees the target's condition.
	guard.Health = 100
	p.HealthMax, p.Health = 1000, 1000
	send(w, 1, "kill guard")
	bob.take()
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "A city guard looks perfectly healthy.") && !strings.Contains(o, "A city guard looks slightly wounded.") {
		t.Fatalf("round condition: %q", o)
	}
	send(w, 1, "look guard")
	if o := bob.take(); !strings.Contains(o, "A city guard looks") {
		t.Fatalf("look condition: %q", o)
	}
}

func TestLookShowsItemNumbersUnlessUnidentified(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, baselineRules+`function describeEffect(e) { return e.kind === "crit" ? "Crits sometimes." : ""; }`)
	sword := w.content.Items[10]
	sword.Effects = append(sword.Effects, effect.Spec{Kind: "crit", Params: map[string]any{"chance": 0.1}})
	sword.Effects = append(sword.Effects, effect.Spec{Kind: "odd", Params: map[string]any{"x": 2, "why": "test"}})
	giveItem(w, w.players[1].Character, 10)
	send(w, 1, "look sword")
	o := bob.take()
	for _, want := range []string{"Pitted and dull.", "Type weapon, level 1", "Damage 4 per swing, speed 1, varies by 20 percent", "Your swings are slashes.", "Crits sometimes.", "odd: why test, x 2"} {
		if !strings.Contains(o, want) {
			t.Fatalf("look sword missing %q: %q", want, o)
		}
	}
	send(w, 1, "look cap")
	if o := bob.take(); !strings.Contains(o, "worn on head") || !strings.Contains(o, "Defense 1") {
		t.Fatalf("look armor: %q", o)
	}
	giveItem(w, w.players[1].Character, 16)
	send(w, 1, "look totem")
	if o := bob.take(); !strings.Contains(o, "A totem of evocation.") {
		t.Fatalf("look totem: %q", o)
	}
	sword.Flags = append(sword.Flags, "unidentified")
	send(w, 1, "look sword")
	if o := bob.take(); !strings.Contains(o, "can't tell anything more") || strings.Contains(o, "Damage") {
		t.Fatalf("unidentified: %q", o)
	}
}

const skillRules = testRules + `
function skillList() {
  return [
    { id: "kick", name: "Kick", level: 1, passive: false, target: "single", cooldown: 2, start: 30, innate: true, description: "A kick." },
    { id: "bash", name: "Bash", level: 3, passive: false, target: "single", cooldown: 4, start: 25, innate: false, price: 250, description: "A bash." },
    { id: "twin", name: "Twin Strike", level: 1, passive: true, cooldown: 0, start: 50, innate: true, description: "An extra swing." }
  ];
}
function useSkill(user, target, skill, e) {
  var eff = e.state.effectiveness;
  var out = { ok: true, hit: true, stage: "", damage: Math.round(10 * eff / 100), verb: skill.id, effects: [], skills: {} };
  out.skills[skill.id] = eff + 10;
  if (skill.id === "bash") out.effects.push({ on: "target", kind: "speedMult", params: { mult: 0 }, rounds: 1 });
  return out;
}
function derivedStats(c) {
  var extra = 0;
  for (var i = 0; i < c.effects.length; i++) if (c.effects[i].kind === "skill" && c.effects[i].params.skill === "twin") extra = c.effects[i].state.effectiveness / 100;
  return { healthMax: 10 + c.level * 2, manaMax: 5, speed: 1 + extra };
}
function onTick(c) {
  var out = { healthDelta: c.fighting ? 0 : 1, manaDelta: 0, skills: {} };
  if (c.fighting) out.skills.twin = 60;
  return out;
}
`

func TestSkillsUseImproveAndPassives(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, skillRules)
	p := w.players[1]
	w.ensurePassives(p.Character)
	// Passive Twin Strike exists at once and feeds derived stats.
	if e := skillEffect(p.Character, "twin"); e == nil || effectiveness(e) != 50 || p.Speed != 1.5 {
		t.Fatalf("passive: %+v speed=%v", e, p.Speed)
	}
	send(w, 1, "skills")
	if o := bob.take(); !strings.Contains(o, "Kick") || !strings.Contains(o, "new") || strings.Contains(o, "Bash") || !strings.Contains(o, "Twin Strike") || !strings.Contains(o, "50%") {
		t.Fatalf("skills list: %q", o)
	}
	send(w, 1, "kick")
	if o := bob.take(); !strings.Contains(o, "Kick whom?") {
		t.Fatalf("kick no target: %q", o)
	}
	send(w, 1, "kick guard")
	o := bob.take()
	if !strings.Contains(o, "Your kick hits a city guard. [3]") {
		t.Fatalf("kick: %q", o)
	}
	if e := skillEffect(p.Character, "kick"); e == nil || effectiveness(e) != 40 {
		t.Fatalf("kick rating after use: %+v", e)
	}
	if p.Fighting == nil {
		t.Fatal("kick did not start the fight")
	}
	// One skill per round, then the cooldown.
	send(w, 1, "kick guard")
	if o := bob.take(); !strings.Contains(o, "still recovering") {
		t.Fatalf("second use same round: %q", o)
	}
	p.cooldowns["skill:kick"] = 2 // pin it: a round may already have fired during the refused use
	nextRound(w)
	bob.take()
	send(w, 1, "kick guard")
	if o := bob.take(); !strings.Contains(o, "Kick is not ready for another 1 round.") {
		t.Fatalf("cooldown: %q", o)
	}
	// Passives improve while fighting.
	if e := skillEffect(p.Character, "twin"); effectiveness(e) != 60 || p.Speed != 1.6 {
		t.Fatalf("passive improvement: %v speed=%v", effectiveness(e), p.Speed)
	}
	// Bash needs level 3; help knows skills.
	send(w, 1, "bash guard")
	if o := bob.take(); !strings.Contains(o, "Huh?") {
		t.Fatalf("bash below level: %q", o)
	}
	send(w, 1, "help kick")
	if o := bob.take(); !strings.Contains(o, "Kick (skill, level 1)") {
		t.Fatalf("help skill: %q", o)
	}
	p.Level = 3
	for i := 0; i < 20; i++ {
		w.Tick()
	}
	bob.take()
	send(w, 1, "bash guard")
	if o := bob.take(); !strings.Contains(o, "Huh?") {
		t.Fatalf("bash without a trainer: %q", o)
	}
	ensureSkill(p.Character, Skill{ID: "bash", Start: 25})
	send(w, 1, "bash guard")
	if o := bob.take(); !strings.Contains(o, "Your bash hits a city guard.") {
		t.Fatalf("bash: %q", o)
	}
	guard := w.contents(p.Room).mobs[0]
	if len(guard.Effects) != 1 || guard.Effects[0].Kind != "speedMult" {
		t.Fatalf("bash effect: %+v", guard.Effects)
	}
	// Ratings persist with the character.
	w.save(p)
	found := false
	for _, e := range p.rec.Effects {
		if e.Kind == "skill" && e.Params["skill"] == "kick" && e.State["effectiveness"] == 40.0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("skill rating not saved: %+v", p.rec.Effects)
	}
}

// nextRound ticks until exactly one round has run.
func nextRound(w *World) {
	r := w.roundCount
	for w.roundCount == r {
		w.Tick()
	}
}

func TestMoneyDropsGiveAndCorpse(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	setRules(t, w, testRules+`function moneyFor(v) { return 37; }`)
	p := w.players[1]
	if moneyString(0) != "no coins" || moneyString(5) != "5 silver" || moneyString(100) != "1 gold" || moneyString(250) != "2 gold and 50 silver" {
		t.Fatalf("money strings: %q %q %q %q", moneyString(0), moneyString(5), moneyString(100), moneyString(250))
	}
	// A mob's corpse holds its coins; get takes them into the wallet.
	guard := w.contents(p.Room).mobs[0]
	guard.Health = 1
	send(w, 1, "kill guard")
	bob.take()
	send(w, 1, "get all guard")
	if o := bob.take(); !strings.Contains(o, "You get 37 silver from the corpse of a city guard.") || p.Silver != 37 {
		t.Fatalf("loot coins: %q silver=%d", o, p.Silver)
	}
	// Give and drop coins.
	send(w, 1, "give 1 gold alice")
	if o := bob.take(); !strings.Contains(o, "don't have that much") {
		t.Fatalf("give too much: %q", o)
	}
	send(w, 1, "give 12 silver alice")
	if o := alice.take(); !strings.Contains(o, "Bob gives you 12 silver.") || w.players[2].Silver != 12 || p.Silver != 25 {
		t.Fatalf("give: %q", o)
	}
	send(w, 1, "drop 5 silver")
	bob.take()
	send(w, 1, "get coins")
	if o := bob.take(); !strings.Contains(o, "You get 5 silver.") || p.Silver != 25 {
		t.Fatalf("drop and get: %q silver=%d", o, p.Silver)
	}
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "25 silver") {
		t.Fatalf("score coins: %q", o)
	}
	// Death puts the wallet in the corpse.
	p.Health = 1
	w.attackRound(guard.Character, p.Character)
	w.Tick()
	bob.take()
	if p.Silver != 0 {
		t.Fatalf("wallet survived death: %d", p.Silver)
	}
	// The wallet persists.
	p.Silver = 300
	w.save(p)
	if p.rec.Silver != 300 {
		t.Fatal("silver not saved")
	}
}

func TestTrainerTeachesForCoin(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, skillRules)
	p := w.players[1]
	send(w, 1, "practice")
	if o := bob.take(); !strings.Contains(o, "nobody here to teach") {
		t.Fatalf("no trainer: %q", o)
	}
	w.content.Mobs[20].Teaches = []string{"bash"}
	send(w, 1, "practice")
	if o := bob.take(); !strings.Contains(o, "A city guard can teach you:") || !strings.Contains(o, "Bash") || !strings.Contains(o, "2 gold and 50 silver") || !strings.Contains(o, "(level 3)") {
		t.Fatalf("practice list: %q", o)
	}
	send(w, 1, "practice bash")
	if o := bob.take(); !strings.Contains(o, "Come back at level 3.") {
		t.Fatalf("level gate: %q", o)
	}
	p.Level = 3
	send(w, 1, "practice bash")
	if o := bob.take(); !strings.Contains(o, "Bash costs 2 gold and 50 silver. You have no coins.") {
		t.Fatalf("price gate: %q", o)
	}
	p.Silver = 300
	send(w, 1, "practice bash")
	if o := bob.take(); !strings.Contains(o, "You pay a city guard 2 gold and 50 silver and learn Bash.") || p.Silver != 50 {
		t.Fatalf("buy: %q silver=%d", o, p.Silver)
	}
	if e := skillEffect(p.Character, "bash"); e == nil || effectiveness(e) != 25 {
		t.Fatalf("skill not learned: %+v", e)
	}
	send(w, 1, "practice bash")
	if o := bob.take(); !strings.Contains(o, "already know Bash") {
		t.Fatalf("double buy: %q", o)
	}
	send(w, 1, "practice kick")
	if o := bob.take(); !strings.Contains(o, "can't teach you that") {
		t.Fatalf("not taught: %q", o)
	}
	send(w, 1, "skills")
	if o := bob.take(); !strings.Contains(o, "25%") || !strings.Contains(o, "Kick          new") {
		t.Fatalf("skills after buying: %q", o)
	}
}

func TestSacrificeCorpse(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	guard := w.contents(p.Room).mobs[0]
	guard.Health = 1
	send(w, 1, "kill guard")
	bob.take()
	send(w, 1, "sacrifice guard")
	o := bob.take()
	if !strings.Contains(o, "The contents of the corpse of a city guard spill out.") || !strings.Contains(o, "You sacrifice the corpse of a city guard to the gods.") || !strings.Contains(o, "The gods give you 1 silver for your sacrifice.") {
		t.Fatalf("sacrifice corpse: %q", o)
	}
	items := w.contents(p.Room).items
	if findCorpse(items, "city guard") != nil || !hasVnum(items, 10) || !hasVnum(items, 13) || p.Silver != 1 {
		t.Fatalf("after sacrifice: items=%v silver=%d", items, p.Silver)
	}
	send(w, 1, "sacrifice altar")
	if o := bob.take(); !strings.Contains(o, "The gods do not want a stone altar.") {
		t.Fatalf("fixed item: %q", o)
	}
	send(w, 1, "get sword")
	bob.take()
	send(w, 1, "sacrifice sword")
	if o := bob.take(); !strings.Contains(o, "Drop it first") {
		t.Fatalf("inventory item: %q", o)
	}
	send(w, 1, "sacrifice nothing")
	if o := bob.take(); !strings.Contains(o, "don't see that here") {
		t.Fatalf("missing: %q", o)
	}
}
