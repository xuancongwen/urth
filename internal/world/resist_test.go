package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
)

// Resistance by damage kind (docs/RULES.md 4.8), against the shipped rules.

// resistFixture is a level-10 player and a level-10 mob from the shipped
// world, the player fighting bare-handed (blunt).
func resistFixture(t *testing.T) (w *World, att, def *Character) {
	t.Helper()
	w = shippedWorld(t)
	for _, p := range w.content.Mobs {
		if p.Level == 10 && !p.HasFlag("peaceful") && len(p.Effects) == 0 {
			def = w.newMob(p).Character
			break
		}
	}
	if def == nil {
		t.Fatal("no plain level-10 mob")
	}
	def.Effects = nil
	att = newCharacter("Tester", []string{"tester"})
	att.Level = 10
	w.recalc(att)
	return w, att, def
}

func resist(kind string, amount float64) effect.Active {
	return effect.Active{Spec: effect.Spec{Kind: "resist", Params: map[string]any{"kind": kind, "amount": amount}}}
}

// meanSwing is the average damage of n landed swings by att on def.
func meanSwing(t *testing.T, w *World, att, def *Character, n int) (mean float64, resisted int) {
	t.Helper()
	total, hits := 0, 0
	for i := 0; i < n; i++ {
		var r AttackResult
		if err := w.scripts.Call("resolveAttack", &r, w.view(att), w.view(def), nil, 1, "wield"); err != nil {
			t.Fatal(err)
		}
		if !r.Hit {
			continue
		}
		hits++
		total += r.Damage
		if r.Resisted {
			resisted++
		}
	}
	return float64(total) / float64(max(hits, 1)), resisted
}

func TestResistScalesASwingByItsKind(t *testing.T) {
	w, att, def := resistFixture(t)
	base, _ := meanSwing(t, w, att, def, 600)
	if base <= 0 {
		t.Fatalf("no damage to compare: %v", base)
	}
	for _, c := range []struct {
		name   string
		effect effect.Active
		lo, hi float64
	}{
		{"half blunt", resist("blunt", 0.5), 0.4, 0.6},
		{"half physical", resist("physical", 0.5), 0.4, 0.6},
		{"weak to blunt", resist("blunt", -1), 1.7, 2.3},
		{"fire does not touch blunt", resist("fire", 1), 0.85, 1.15},
	} {
		def.Effects = []effect.Active{c.effect}
		got, _ := meanSwing(t, w, att, def, 600)
		if r := got / base; r < c.lo || r > c.hi {
			t.Errorf("%s: %.2f of the unresisted swing, want %.2f to %.2f", c.name, r, c.lo, c.hi)
		}
	}
	// Immune: every landed swing is resisted and does nothing.
	def.Effects = []effect.Active{resist("all", 1)}
	got, resisted := meanSwing(t, w, att, def, 100)
	if got != 0 || resisted == 0 {
		t.Fatalf("immune: mean %v, resisted %d", got, resisted)
	}
}

// A proc is weighed on its own kind: a blade's fire still burns a
// creature immune to the blade, and is shrugged off by one immune to fire.
func TestResistProcsByTheirOwnKind(t *testing.T) {
	w, att, def := resistFixture(t)
	att.Effects = append(att.Effects, effect.Active{Spec: effect.Spec{Kind: "proc",
		Params: map[string]any{"kind": "fire", "chance": 1.0, "power": 1.0, "verb": "searing flames"}}})
	swing := func() AttackResult {
		for i := 0; i < 50; i++ {
			var r AttackResult
			if err := w.scripts.Call("resolveAttack", &r, w.view(att), w.view(def), nil, 1, "wield"); err != nil {
				t.Fatal(err)
			}
			if r.Hit {
				return r
			}
		}
		t.Fatal("never landed")
		return AttackResult{}
	}
	def.Effects = []effect.Active{resist("blunt", 1)}
	if r := swing(); !r.Resisted || len(r.Procs) != 1 || r.Procs[0].Damage <= 0 || r.Procs[0].Kind != "fire" {
		t.Fatalf("immune to the fist, not the fire: %+v", r)
	}
	def.Effects = []effect.Active{resist("fire", 1)}
	if r := swing(); r.Resisted || r.Damage <= 0 || len(r.Procs) != 1 || !r.Procs[0].Resisted || r.Procs[0].Damage != 0 {
		t.Fatalf("immune to the fire, not the fist: %+v", r)
	}
}

// A player's resistance stops short of immunity; a mob's does not.
func TestResistPlayerLimit(t *testing.T) {
	w, att, def := resistFixture(t)
	att.Effects = append(att.Effects, resist("fire", 2))
	def.Effects = []effect.Active{resist("fire", 2)}
	var onPlayer, onMob float64
	if err := w.scripts.Call("resistDamage", &onPlayer, w.view(att), "fire", 100); err != nil {
		t.Fatal(err)
	}
	if err := w.scripts.Call("resistDamage", &onMob, w.view(def), "fire", 100); err != nil {
		t.Fatal(err)
	}
	if onPlayer != 25 || onMob != 0 {
		t.Fatalf("player takes %v, mob %v; want 25 and 0", onPlayer, onMob)
	}
}

// Auras land per target through the engine, so each enemy's own
// resistance counts.
func TestResistAura(t *testing.T) {
	w, _, def := resistFixture(t)
	def.Effects = []effect.Active{resist("fire", 1)}
	if pr := w.auraOn(def, Proc{Verb: "corona", Kind: "fire", Damage: 40}); !pr.Resisted || pr.Damage != 0 {
		t.Fatalf("fire-immune: %+v", pr)
	}
	def.Effects = []effect.Active{resist("fire", -0.5)}
	if pr := w.auraOn(def, Proc{Verb: "corona", Kind: "fire", Damage: 40}); pr.Resisted || pr.Damage != 60 {
		t.Fatalf("weak to fire: %+v", pr)
	}
}

// Damage over time burns with its kind.
func TestResistDamageOverTime(t *testing.T) {
	w, _, def := resistFixture(t)
	def.Health = def.HealthMax / 2
	tick := func() int {
		var r TickResult
		if err := w.scripts.Call("onTick", &r, w.view(def)); err != nil {
			t.Fatal(err)
		}
		return r.HealthDelta
	}
	clean := tick()
	burn := effect.Active{Spec: effect.Spec{Kind: "dot", Params: map[string]any{"damage": 30.0, "kind": "fire"}}, Rounds: 3}
	def.Effects = []effect.Active{burn}
	if got := tick(); got != clean-30 {
		t.Fatalf("burning: delta %d, want %d", got, clean-30)
	}
	def.Effects = []effect.Active{burn, resist("fire", 1)}
	if got := tick(); got != clean {
		t.Fatalf("burning but immune: delta %d, want %d", got, clean)
	}
}

// A damage spell a target is immune to is shrugged off; a spell has a
// kind, and magic is the default.
func TestResistSpells(t *testing.T) {
	w, att, def := resistFixture(t)
	cast := func(id string) CastResult {
		var r CastResult
		if err := w.scripts.Call("resolveCast", &r, w.view(att), []any{w.view(def)}, map[string]any{"id": id}); err != nil {
			t.Fatal(err)
		}
		if len(r.Targets) != 1 {
			t.Fatalf("%s: %+v", id, r)
		}
		return r
	}
	def.Effects = []effect.Active{resist("fire", 1)}
	if r := cast("firebolt"); !r.Targets[0].Negated || r.Targets[0].Damage != 0 {
		t.Fatalf("firebolt on the fire-immune: %+v", r.Targets[0])
	}
	if r := cast("lightning_bolt"); r.Targets[0].Negated || r.Targets[0].Damage <= 0 {
		t.Fatalf("lightning on the fire-immune: %+v", r.Targets[0])
	}
	// Acid Splash also burns over time: immune to acid, the splash does
	// nothing and the burn carries acid, so it will not hurt either.
	def.Effects = []effect.Active{resist("acid", 1)}
	r := cast("acid")
	if r.Targets[0].Damage != 0 || len(r.Targets[0].Effects) != 1 || r.Targets[0].Effects[0].Params["kind"] != "acid" {
		t.Fatalf("acid on the acid-immune: %+v", r.Targets[0])
	}
}

// consider names immunities and vulnerabilities; identify words resist.
func TestResistShownOnConsiderAndItems(t *testing.T) {
	w, att, def := resistFixture(t)
	def.Effects = []effect.Active{resist("fire", 1), resist("cold", -0.5), resist("pierce", 0.3)}
	got := w.consider(att, def)
	if !strings.Contains(got, "It cannot be hurt by fire.") || !strings.Contains(got, "It is weak to cold.") || strings.Contains(got, "pierce") {
		t.Fatalf("consider: %q", got)
	}
	att.Effects = append(att.Effects, resist("fire", 0.3), resist("cold", -0.2))
	var rows []sheetRow
	if err := w.scripts.Call("combatSheet", &rows, w.view(att)); err != nil {
		t.Fatal(err)
	}
	sheet := ""
	for _, r := range rows {
		if r.Label == "Resist" {
			sheet = r.Value
		}
	}
	if sheet != "fire 30%, cold weak 20%" {
		t.Errorf("score sheet resist row: %q", sheet)
	}
	for _, c := range []struct {
		params map[string]any
		want   string
	}{
		{map[string]any{"kind": "fire", "amount": 0.3}, "Resists fire damage by 30 percent"},
		{map[string]any{"kind": "fire", "amount": 1.0}, "Immune to fire damage"},
		{map[string]any{"kind": "cold", "amount": -0.5}, "Weak to cold damage: takes 50 percent more"},
	} {
		var words string
		if err := w.scripts.Call("describeEffect", &words, map[string]any{"kind": "resist", "params": c.params, "rounds": 0}); err != nil || words != c.want {
			t.Errorf("describe %v: %q (%v), want %q", c.params, words, err, c.want)
		}
	}
}

// The shipped weapon baselines give every weapon a kind.
func TestEveryShippedWeaponHasAKind(t *testing.T) {
	w := shippedWorld(t)
	for _, p := range w.content.Items {
		if p.Type == item.Weapon && p.Resolved.Weapon.Kind == "" {
			t.Errorf("item %d (%s) has no damage kind", p.Vnum, p.Name)
		}
	}
	for _, p := range w.content.Mobs {
		if p.Resolved.Attack.Kind == "" {
			t.Errorf("mob %d (%s) has no attack kind", p.Vnum, p.Name)
		}
	}
}

// The engine says so when a swing, a proc, or a skill does nothing for
// the target's immunity.
func TestResistedNarration(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, testRules+`
function resolveAttack(a, d, w, round) {
  if (a.mob) return { hit: false, damage: 0, verb: "bite", stage: "miss" };
  return { hit: true, damage: 0, resisted: true, crit: false, verb: "punch", stage: "",
    procs: [{ verb: "searing flames", damage: 0, kind: "fire", resisted: true }, { verb: "frost", damage: 3, kind: "cold" }] };
}
`)
	send(w, 1, "n")
	bob.take()
	send(w, 1, "kill dog")
	out := bob.take()
	for _, want := range []string{"A stray dog shrugs off your punch.", "A stray dog shrugs off your searing flames.", "Your frost hits a stray dog. [3]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

// A skill that lands on a target immune to its kind is shrugged off.
func TestResistedSkillNarration(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, skillRules+`
function useSkill(user, target, skill, e) {
  return { ok: true, hit: true, stage: "", damage: 0, resisted: true, verb: "kick", effects: [], skills: {} };
}
`)
	send(w, 1, "n")
	bob.take()
	send(w, 1, "kick dog")
	if out := bob.take(); !strings.Contains(out, "A stray dog shrugs off your kick.") {
		t.Fatalf("kick on the immune: %q", out)
	}
}

// Immunity is rare (4.8, 8): only a boss or a named creature is immune to
// anything, and nothing is immune to every weapon.
func TestShippedImmunitiesAreRare(t *testing.T) {
	w := shippedWorld(t)
	immune := 0
	for _, p := range w.content.Mobs {
		total := map[string]float64{}
		for _, e := range p.Effects {
			if e.Kind == "resist" {
				k, _ := e.Params["kind"].(string)
				total[k] += toFloat(e.Params["amount"])
			}
		}
		named := p.HasFlag("boss") || !hasArticle(p.Name)
		for k, a := range total {
			if a < 1 {
				continue
			}
			immune++
			if k == "physical" || k == "all" {
				t.Errorf("mob %d (%s) is immune to %s", p.Vnum, p.Name, k)
			}
			if !named {
				t.Errorf("mob %d (%s) is immune to %s but is neither a boss nor named", p.Vnum, p.Name, k)
			}
		}
	}
	if immune == 0 {
		t.Error("no shipped creature is immune to anything")
	}
}

// hasArticle reports whether a mob's name is a common noun ("a skeleton",
// "the crypt ghoul") rather than a name ("Strahd", "the Butcher").
func hasArticle(name string) bool {
	for _, a := range []string{"a ", "an ", "some "} {
		if strings.HasPrefix(name, a) {
			return true
		}
	}
	rest, ok := strings.CutPrefix(name, "the ")
	return ok && rest != "" && strings.ToLower(rest[:1]) == rest[:1]
}

// A mob's prototype effects count once (Trackstar 1040): the Bloodthirster
// is written with half resistance to fire and full to unholy, and that is
// what the rules see.
func TestMobPrototypeEffectsCountOnce(t *testing.T) {
	w := shippedWorld(t)
	m := w.newMob(w.content.Mobs[1814])
	w.recalc(m.Character)
	for kind, want := range map[string]float64{"fire": 0.5, "unholy": 1, "holy": -0.5} {
		var got float64
		if err := w.scripts.Call("resistance", &got, w.view(m.Character), kind); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("resistance to %s: %v, want %v", kind, got, want)
		}
	}
}
