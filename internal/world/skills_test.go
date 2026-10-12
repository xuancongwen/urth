package world

import (
	"strings"
	"testing"

	"urth/internal/config"
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
  return [{ id: "drain", name: "Drain", innate: true, branch: "arcane", school: "necromancy", castRounds: 0, cooldown: 0,
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

// staminaRules gives a pool of 10 that refills 2 a round, and a kick that
// costs 4 with no cooldown.
const staminaRules = testRules + `
function skillList() {
  return [{ id: "kick", name: "Kick", level: 1, target: "single", cooldown: 0, stamina: 4, start: 50, innate: true, description: "Kick." }];
}
function useSkill(user, target, skill, e) { return { ok: true, hit: true, stage: "", damage: 1, verb: "kick", effects: [], skills: {} }; }
`

func TestSkillsSpendStamina(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	src := strings.Replace(staminaRules, "manaMax: 5,", "manaMax: 5, staminaMax: 10,", 1)
	src = strings.Replace(src, "manaDelta: 0 }", "manaDelta: 0, staminaDelta: 2 }", 1)
	setRules(t, w, src)
	p := w.players[1]
	p.Stamina = p.StaminaMax
	if p.StaminaMax != 10 {
		t.Fatalf("stamina max %d", p.StaminaMax)
	}
	send(w, 1, "kill guard")
	send(w, 1, "kick")
	if p.Stamina != 6 {
		t.Fatalf("after one kick: %d", p.Stamina)
	}
	for i := 0; i < 10; i++ { // a round, so the next skill is allowed
		w.Tick()
	}
	p.Stamina = 3
	bob.take()
	send(w, 1, "kick")
	if o := bob.take(); !strings.Contains(o, "too winded for Kick: it takes 4 stamina and you have 3") {
		t.Fatalf("winded: %q", o)
	}
	for i := 0; i < 10; i++ { // one round
		w.Tick()
	}
	if p.Stamina != 5 {
		t.Fatalf("after a round: %d", p.Stamina)
	}
	send(w, 1, "skills")
	if o := bob.take(); !strings.Contains(o, "4 stamina") {
		t.Fatalf("skills: %q", o)
	}
}

// TestShippedCostsReachTheEngine guards the rules' list functions, which
// copy fields by name: every shipped active skill costs stamina or keeps
// a cooldown, every shipped spell costs mana, and pools are non-empty.
func TestShippedCostsReachTheEngine(t *testing.T) {
	w := shippedWorld(t)
	for _, sk := range w.skillList() {
		if !sk.Passive && sk.Stamina <= 0 && sk.Cooldown <= 0 {
			t.Errorf("skill %s costs nothing", sk.ID)
		}
	}
	for _, sk := range w.skillList() {
		if !sk.Innate && len(sk.Stats) == 0 {
			t.Errorf("taught skill %s has no stat requirement", sk.ID)
		}
	}
	for _, sp := range w.spellList() {
		if sp.Mana <= 0 {
			t.Errorf("spell %s costs no mana", sp.ID)
		}
		if sp.Level < 1 || len(sp.Stats) == 0 {
			t.Errorf("spell %s has no level or stat requirement: %d %v", sp.ID, sp.Level, sp.Stats)
		}
	}
	c := w.simFighter(5)
	if c.ManaMax <= 0 || c.StaminaMax <= 0 {
		t.Errorf("pools: mana %d stamina %d", c.ManaMax, c.StaminaMax)
	}
}

// TestShippedRoundLengthAgrees: the rules turn wall-clock timers into
// rounds of their own round length, which must be the config's.
func TestShippedRoundLengthAgrees(t *testing.T) {
	w := shippedWorld(t)
	var secs float64
	if !w.callOptional("roundSeconds", &secs) {
		t.Fatal("the shipped rules have no roundSeconds")
	}
	if want := float64(config.Default().Timing.RoundMs) / 1000; secs != want {
		t.Fatalf("rules assume %v s rounds, the default config %v s", secs, want)
	}
}

// TestRequirements: a trainer will not teach a skill whose stats you lack,
// and a spell you have access to will not cast below its level or stat.
func TestRequirements(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	guard := w.content.Mobs[20]
	guard.Flags = append(guard.Flags, "peaceful")
	guard.Teaches = []string{"stab"}
	setRules(t, w, testRules+`
function skillList() {
  return [{ id: "stab", name: "Stab", level: 1, target: "single", cooldown: 0, stamina: 0, stats: { might: 3 }, start: 50, price: 0, description: "Stab." }];
}
function spellList() {
  return [{ id: "nova", name: "Nova", innate: true, branch: "arcane", school: "evocation", level: 4, stats: { might: 2 }, castRounds: 0, target: "self", save: "none", materials: [] }];
}
function resolveCast(c, ts, sp) { return { ok: true, message: "", consume: [], targets: [], casterEffects: [] }; }
`)
	p.Stats["might"] = 1
	send(w, 1, "practice")
	if o := bob.take(); !strings.Contains(o, "(needs 3 might)") {
		t.Fatalf("listing: %q", o)
	}
	send(w, 1, "practice stab")
	if o := bob.take(); !strings.Contains(o, "You'll want 3 might before I teach you Stab") || skillEffect(p.Character, "stab") != nil {
		t.Fatalf("taught without the stat: %q", o)
	}
	p.Stats["might"] = 3
	send(w, 1, "practice stab")
	if skillEffect(p.Character, "stab") == nil {
		t.Fatalf("not taught with the stat: %q", bob.take())
	}

	p.Schools = []string{"evocation"}
	send(w, 1, "cast nova")
	if o := bob.take(); !strings.Contains(o, "You cannot cast Nova yet: it takes level 4 and 2 might, and you lack level 4.") {
		t.Fatalf("cast below level: %q", o)
	}
	p.Level = 4
	send(w, 1, "cast nova")
	if o := bob.take(); strings.Contains(o, "cannot cast") {
		t.Fatalf("cast refused at level with the stat: %q", o)
	}
}

// TestTrainerTeachesSpells walks a spell from a trainer: listed with what
// it needs, refused without the school, the level, the stat, or the coin,
// then learned and castable; a found spell is never sold.
func TestTrainerTeachesSpells(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	setRules(t, w, testRules+`
function spellList() {
  return [
    { id: "spark", name: "Spark", innate: true, branch: "arcane", school: "evocation", castRounds: 0, target: "self", save: "none", materials: [] },
    { id: "nova", name: "Nova", price: 120, branch: "arcane", school: "evocation", level: 4, stats: { might: 2 }, castRounds: 0, target: "self", save: "none", materials: [] },
    { id: "star", name: "Starfall", found: true, branch: "arcane", school: "evocation", castRounds: 0, target: "self", save: "none", materials: [] }
  ];
}
function resolveCast(c, ts, sp) { return { ok: true, message: "", consume: [], targets: [], casterEffects: [] }; }
`)
	w.content.Mobs[20].Teaches = []string{"nova", "star"}
	send(w, 1, "practice")
	if o := bob.take(); !strings.Contains(o, "Nova") || !strings.Contains(o, "(needs the evocation school)") || strings.Contains(o, "Starfall") {
		t.Fatalf("listing without the school: %q", o)
	}
	send(w, 1, "practice nova")
	if o := bob.take(); !strings.Contains(o, "Nova is not for you") || learnedSpell(p.Character, "nova") {
		t.Fatalf("taught without the school: %q", o)
	}
	send(w, 1, "spells all")
	if o := bob.take(); !strings.Contains(o, "Spark         with the evocation school\n") || !strings.Contains(o, "Nova          from a trainer  (the evocation school, level 4 and 2 might)") {
		t.Fatalf("spells all without the school: %q", o)
	}
	p.Schools = []string{"evocation"}
	send(w, 1, "spells")
	if o := bob.take(); !strings.Contains(o, "Spark") || !strings.Contains(o, "Still to learn:") || strings.Contains(o, "Nova") || !strings.Contains(o, "Starfall      from a scroll") || !strings.Contains(o, "'spells all' lists 1 more") {
		t.Fatalf("spells with the school: %q", o)
	}
	send(w, 1, "spells all")
	if o := bob.take(); !strings.Contains(o, "Nova          from a trainer  (level 4 and 2 might)") {
		t.Fatalf("spells all: %q", o)
	}
	send(w, 1, "cast nova")
	if o := bob.take(); !strings.Contains(o, "don't know that spell") {
		t.Fatalf("cast unlearned: %q", o)
	}
	send(w, 1, "practice nova")
	if o := bob.take(); !strings.Contains(o, "Come back at level 4.") {
		t.Fatalf("level gate: %q", o)
	}
	p.Level = 4
	send(w, 1, "practice nova")
	if o := bob.take(); !strings.Contains(o, "You'll want 2 might") {
		t.Fatalf("stat gate: %q", o)
	}
	p.Stats["might"] = 2
	send(w, 1, "practice nova")
	if o := bob.take(); !strings.Contains(o, "Nova costs 1 gold and 20 silver") {
		t.Fatalf("price gate: %q", o)
	}
	p.Silver = 200
	send(w, 1, "practice nova")
	if o := bob.take(); !strings.Contains(o, "learn Nova") || p.Silver != 80 || !learnedSpell(p.Character, "nova") {
		t.Fatalf("learn: %q silver=%d", o, p.Silver)
	}
	send(w, 1, "practice nova")
	if o := bob.take(); !strings.Contains(o, "already know Nova") {
		t.Fatalf("double buy: %q", o)
	}
	send(w, 1, "practice starfall")
	if o := bob.take(); !strings.Contains(o, "can't teach you that") {
		t.Fatalf("found spell sold: %q", o)
	}
	send(w, 1, "cast nova")
	if o := bob.take(); strings.Contains(o, "don't know") || strings.Contains(o, "cannot cast") {
		t.Fatalf("cast learned: %q", o)
	}
	send(w, 1, "affects")
	if o := bob.take(); strings.Contains(o, "nova") {
		t.Fatalf("learned spell shows as an affect: %q", o)
	}
}

// TestShippedTeachesKnownIDs holds every trainer in the shipped world to
// ids the rules define, and to spells a trainer can sell.
func TestShippedTeachesKnownIDs(t *testing.T) {
	w := shippedWorld(t)
	skills, spells := map[string]bool{}, map[string]Spell{}
	for _, sk := range w.skillList() {
		skills[sk.ID] = true
	}
	for _, sp := range w.spellList() {
		spells[sp.ID] = sp
	}
	for _, m := range w.content.Mobs {
		for _, id := range m.Teaches {
			sp, isSpell := spells[id]
			switch {
			case skills[id] && isSpell:
				t.Errorf("%s (%d) teaches %q, which is both a skill and a spell", m.Name, m.Vnum, id)
			case isSpell && (sp.Innate || sp.Found):
				t.Errorf("%s (%d) teaches %q, an innate or found spell no trainer sells", m.Name, m.Vnum, id)
			case !skills[id] && !isSpell:
				t.Errorf("%s (%d) teaches %q, which is neither a skill nor a spell", m.Name, m.Vnum, id)
			}
		}
	}
}

// TestStudyScroll: a scroll teaches its spell to someone with the school,
// level, and stat, and is used up; otherwise it says what is missing.
func TestStudyScroll(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	setRules(t, w, testRules+`
function spellList() {
  return [{ id: "star", name: "Starfall", found: true, branch: "arcane", school: "evocation", level: 3, stats: { might: 2 }, castRounds: 0, target: "self", save: "none", materials: [] }];
}
function resolveCast(c, ts, sp) { return { ok: true, message: "", consume: [], targets: [], casterEffects: [] }; }
`)
	scroll := &item.Proto{Vnum: 9100, Name: "a scroll of starfall", Keywords: []string{"scroll", "starfall"}, Type: item.Other, Spell: "star"}
	plain := &item.Proto{Vnum: 9101, Name: "a blank scroll", Keywords: []string{"blank"}, Type: item.Other}
	p.Inventory = append(p.Inventory, item.New(scroll), item.New(plain))
	send(w, 1, "study blank")
	if o := bob.take(); !strings.Contains(o, "nothing in a blank scroll to learn") {
		t.Fatalf("blank: %q", o)
	}
	send(w, 1, "study starfall")
	if o := bob.take(); !strings.Contains(o, "It takes the evocation school.") {
		t.Fatalf("no school: %q", o)
	}
	p.Schools = []string{"evocation"}
	send(w, 1, "study starfall")
	if o := bob.take(); !strings.Contains(o, "it takes level 3 and 2 might, and you lack level 3") || len(p.Inventory) != 2 {
		t.Fatalf("level: %q", o)
	}
	p.Level, p.Stats["might"] = 3, 2
	send(w, 1, "study starfall")
	if o := bob.take(); !strings.Contains(o, "Starfall is yours") || !learnedSpell(p.Character, "star") || len(p.Inventory) != 1 {
		t.Fatalf("study: %q inv=%d", o, len(p.Inventory))
	}
	send(w, 1, "spells")
	if o := bob.take(); !strings.Contains(o, "Starfall") || strings.Contains(o, "Still to learn") {
		t.Fatalf("spells after study: %q", o)
	}
}

// TestShippedScrolls: every scroll names a spell that exists and is not
// innate, and every spell that only a scroll teaches has a scroll.
func TestShippedScrolls(t *testing.T) {
	w := shippedWorld(t)
	spells := map[string]Spell{}
	for _, sp := range w.spellList() {
		spells[sp.ID] = sp
	}
	scrolled := map[string]bool{}
	for _, it := range w.content.Items {
		if it.Spell == "" {
			continue
		}
		sp, ok := spells[it.Spell]
		switch {
		case !ok:
			t.Errorf("%s (%d) teaches %q, which is not a spell", it.Name, it.Vnum, it.Spell)
		case sp.Innate:
			t.Errorf("%s (%d) teaches %q, which is innate", it.Name, it.Vnum, it.Spell)
		}
		scrolled[it.Spell] = true
	}
	for _, sp := range spells {
		if sp.Found && !scrolled[sp.ID] {
			t.Errorf("%s is found only on a scroll, and no item teaches it", sp.Name)
		}
	}
}
