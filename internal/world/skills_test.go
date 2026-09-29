package world

import (
	"strings"
	"testing"

	"urth/internal/item"
)

// targetRules has one skill of each target kind the engine resolves:
// area strikes every foe, ally rescues, none heals, and single does no
// damage (a feint) with its cooldown shortened by the rules.
const targetRules = testRules + `
function skillList() {
  return [
    { id: "whirl", name: "Whirlwind", level: 1, target: "area", cooldown: 5, start: 50, innate: true, description: "Spin." },
    { id: "rescue", name: "Rescue", level: 1, target: "ally", cooldown: 3, start: 50, innate: true, description: "Step in." },
    { id: "wind", name: "Second Wind", level: 1, target: "none", cooldown: 9, start: 50, innate: true, description: "Breathe." },
    { id: "feint", name: "Feint", level: 1, target: "single", cooldown: 6, start: 50, innate: true, description: "Fool them." }
  ];
}
function useSkill(user, target, skill, e) {
  var out = { ok: true, hit: true, stage: "", damage: 0, verb: skill.id, effects: [], skills: {} };
  if (skill.id === "whirl") { out.damage = 2; out.skills.whirl = e.state.effectiveness + 1; }
  if (skill.id === "rescue") out.taunt = true;
  if (skill.id === "wind") { out.heal = 4; out.verb = "second wind"; }
  if (skill.id === "feint") { out.cooldown = 2; out.effects.push({ on: "target", kind: "dodge", params: { amount: -0.1 }, rounds: 3 }); }
  return out;
}
`

func TestSkillTargetsAreaAllyNone(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	setRules(t, w, targetRules)
	p, a := w.players[1], w.players[2]
	north := w.content.Rooms.Rooms[2]
	for w.countMobs(w.content.Mobs[21]) < 2 {
		w.Tick() // the dogs reset one at a time
	}
	p.Room, a.Room = north, north
	var dogs []*Character
	for _, m := range w.contents(north).mobs {
		dogs = append(dogs, m.Character)
	}
	if len(dogs) < 2 {
		t.Fatalf("want two dogs, got %d", len(dogs))
	}
	for _, d := range dogs {
		d.Health, d.HealthMax = 100, 100
	}
	p.Health, p.HealthMax = 100, 100
	a.Health, a.HealthMax = 100, 100

	// Ally: nobody fights Alice yet, so there is nothing to rescue her from.
	send(w, 1, "rescue alice")
	if o := bob.take(); !strings.Contains(o, "Nobody is fighting Alice") {
		t.Fatalf("rescue with no attackers: %q", o)
	}
	// Both dogs on Alice; Bob steps in and both turn on him.
	for _, d := range dogs {
		d.Fighting = a.Character
	}
	a.Fighting = dogs[0]
	send(w, 1, "rescue alice")
	if o := bob.take(); !strings.Contains(o, "You rescue Alice!") {
		t.Fatalf("rescue: %q", o)
	}
	if o := alice.take(); !strings.Contains(o, "Bob rescues you!") {
		t.Fatalf("rescue, ally's view: %q", o)
	}
	for _, d := range dogs {
		if d.Fighting != p.Character {
			t.Fatalf("dog still on %v", d.Fighting)
		}
	}
	if p.Fighting == nil {
		t.Fatal("rescuer is not fighting")
	}

	// Area: one use, every foe struck, one rating roll, one cooldown.
	p.lastSkillRound = 0
	before := []int{dogs[0].Health, dogs[1].Health}
	send(w, 1, "whirlwind")
	o := bob.take()
	if countOccurrences(o, "Your whirl") != 2 {
		t.Fatalf("whirlwind should strike both dogs: %q", o)
	}
	if dogs[0].Health >= before[0] || dogs[1].Health >= before[1] {
		t.Fatalf("whirlwind damage: %v -> %d %d", before, dogs[0].Health, dogs[1].Health)
	}
	if e := skillEffect(p.Character, "whirl"); effectiveness(e) != 51 {
		t.Fatalf("area skill improved more than once: %v", effectiveness(e))
	}
	if p.cooldowns["skill:whirl"] != 5 {
		t.Fatalf("area cooldown: %v", p.cooldowns)
	}

	// None: a heal, capped at the maximum.
	p.lastSkillRound = 0
	p.Health = p.HealthMax - 10
	send(w, 1, "second")
	if o := bob.take(); !strings.Contains(o, "Your second wind heals you. [4]") {
		t.Fatalf("heal: %q", o)
	}

	// Single, no damage, and a cooldown the rules shortened.
	p.lastSkillRound = 0
	send(w, 1, "feint dog")
	if o := bob.take(); !strings.Contains(o, "Your feint catches a stray dog.") {
		t.Fatalf("feint: %q", o)
	}
	if p.cooldowns["skill:feint"] != 2 {
		t.Fatalf("cooldown override: %v", p.cooldowns)
	}
}

// Every skill and feat in the shipped rules resolves without a hook error,
// armed three ways, and every feat's effect has words.
func TestShippedSkillsAndFeatsResolve(t *testing.T) {
	w := shippedWorld(t)
	byBaseline := func(b string) *item.Item {
		for _, p := range w.content.Items {
			if p.Type == item.Weapon && p.Baseline == b {
				return item.New(p)
			}
		}
		t.Fatalf("no %s weapon in the shipped world", b)
		return nil
	}
	arms := map[string]*item.Item{"unarmed": nil, "dagger": byBaseline("dagger"), "heavy": byBaseline("heavy")}
	var target *Character
	for _, p := range w.content.Mobs {
		if p.Level == 10 && !p.HasFlag("peaceful") {
			target = w.newMob(p).Character
			break
		}
	}
	if target == nil {
		t.Fatal("no level-10 mob")
	}
	target.Health = target.HealthMax / 5

	user := newCharacter("Tester", []string{"tester"})
	user.Level = 20
	for _, f := range w.featList() {
		w.grantFeat(user, f)
		if f.Effect.Kind != "stat" {
			var words string
			if err := w.scripts.Call("describeEffect", &words, map[string]any{"kind": f.Effect.Kind, "params": f.Effect.Params, "rounds": 0}); err != nil || words == "" {
				t.Errorf("feat %s: no description (%v)", f.ID, err)
			}
		}
	}
	for _, sk := range w.skillList() {
		ensureSkill(user, sk)
	}
	w.recalc(user)
	user.Health = user.HealthMax / 2

	for name, weapon := range arms {
		delete(user.Equipment, "wield")
		if weapon != nil {
			user.equip(weapon, "wield")
		}
		for _, fighting := range []bool{false, true} {
			user.Fighting = nil
			if fighting {
				user.Fighting = target
			}
			for _, sk := range w.skillList() {
				if sk.Passive {
					continue
				}
				var r SkillResult
				var tv any
				if sk.Target != "none" {
					tv = w.view(target)
				}
				e := skillEffect(user, sk.ID)
				if err := w.scripts.Call("useSkill", &r, w.view(user), tv, skillView(sk), e.View()); err != nil {
					t.Fatalf("%s (%s, fighting=%v): %v", sk.ID, name, fighting, err)
				}
				if !r.OK && r.Message == "" {
					t.Errorf("%s (%s): refused without a reason", sk.ID, name)
				}
				if r.OK && sk.Cooldown > 0 && (r.Cooldown == nil || *r.Cooldown != max(1, sk.Cooldown-1)) {
					t.Errorf("%s: Relentless cooldown %v, want %d", sk.ID, r.Cooldown, max(1, sk.Cooldown-1))
				}
			}
		}
		// A swing and a tick with every feat and skill in play.
		var ar AttackResult
		if err := w.scripts.Call("resolveAttack", &ar, w.view(user), w.view(target), itemView(weapon), 1); err != nil {
			t.Fatalf("resolveAttack (%s): %v", name, err)
		}
		var tr TickResult
		if err := w.scripts.Call("onTick", &tr, w.view(user)); err != nil {
			t.Fatalf("onTick: %v", err)
		}
	}
}

// Drain: a cast that returns heal gives it to the caster after the damage,
// capped at the maximum.
func TestCastHealsCaster(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, testRules+`
function spellList() {
  return [{ id: "drain", name: "Drain", branch: "arcane", school: "necromancy", castRounds: 0, cooldown: 0,
    materials: [], target: "single", save: "none" }];
}
function resolveCast(caster, targets, spell) {
  return { ok: true, message: "", consume: [], casterEffects: [], heal: 4,
    targets: [{ index: 0, damage: 2, effects: [] }] };
}
`)
	p := w.players[1]
	p.Schools = []string{"necromancy"}
	p.Health = p.HealthMax - 3
	send(w, 1, "cast drain guard")
	if o := bob.take(); !strings.Contains(o, "Your drain heals you. [3]") || p.Health != p.HealthMax {
		t.Fatalf("drain heal: %q health %d/%d", o, p.Health, p.HealthMax)
	}
}
