package world

import (
	"strings"
	"testing"
)

const schoolRules = `
function magicSchools() {
  return { schools: [
      { id: "evocation", order: "white", opposite: "necromancy" },
      { id: "abjuration", order: "white", opposite: "conjuration" },
      { id: "necromancy", order: "black", opposite: "evocation" } ],
    orders: { white: "the White Robes", black: "the Black Robes" }, level: 3, stats: { might: 2 } };
}
`

// TestSwearAtTheTest: an oath is taken in a Test room of the school's
// Order, once, with the level and stats the rules ask (docs/RULES.md 6.1).
func TestSwearAtTheTest(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, testRules+schoolRules)
	p := w.players[1]
	send(w, 1, "swear evocation")
	if o := bob.take(); !strings.Contains(o, "nothing here to swear to") {
		t.Fatalf("swear outside a Test: %q", o)
	}
	p.Room.Order = "white"
	defer func() { p.Room.Order = "" }()
	send(w, 1, "swear")
	if o := bob.take(); !strings.Contains(o, "swear to the White Robes, in abjuration or evocation") || !strings.Contains(o, "level 3 and 2 might") {
		t.Fatalf("swear alone: %q", o)
	}
	send(w, 1, "swear necromancy")
	if o := bob.take(); !strings.Contains(o, "keep abjuration and evocation, not that") {
		t.Fatalf("another Order's school: %q", o)
	}
	send(w, 1, "swear evocation")
	if o := bob.take(); !strings.Contains(o, "will not take your oath yet") || p.Specialty != "" {
		t.Fatalf("unready: %q", o)
	}
	p.Level, p.Stats["might"] = 3, 2
	send(w, 1, "swear evo")
	if o := bob.take(); !strings.Contains(o, "You swear yourself to evocation") || !strings.Contains(o, "Necromancy is closed to you now.") || p.Specialty != "evocation" {
		t.Fatalf("oath: %q specialty %q", o, p.Specialty)
	}
	send(w, 1, "swear abjuration")
	if o := bob.take(); !strings.Contains(o, "already sworn to evocation, of the White Robes") || p.Specialty != "evocation" {
		t.Fatalf("second oath: %q", o)
	}
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "evocation, of the White Robes") {
		t.Fatalf("score: %q", o)
	}
}

// TestOldSchoolsAreReset: a saved list of schools from totems is cleared
// on load, with a word, and never written back (docs/RULES.md 6.1).
func TestOldSchoolsAreReset(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.rec.Schools = []string{"evocation", "illusion"}
	w.loadSheet(p)
	w.Tick()
	if o := bob.take(); !strings.Contains(o, "reclaimed the schools") || p.Specialty != "" {
		t.Fatalf("reset: %q specialty %q", o, p.Specialty)
	}
	w.save(p)
	if len(p.rec.Schools) != 0 {
		t.Fatalf("old schools written back: %v", p.rec.Schools)
	}
}

// TestSpecialtyBarsTheOppositeAndStrengthensItsOwn, under the shipped
// rules: a necromancer cannot learn or cast evocation, common or not, and
// its own school hits 30 percent harder.
func TestSpecialtyBarsTheOppositeAndStrengthensItsOwn(t *testing.T) {
	w := shippedWorld(t)
	c := w.simFighter(10)
	c.Stats["intelligence"] = 16
	firebolt, chill := shippedSpell(t, w, "firebolt"), shippedSpell(t, w, "chill_touch")
	if firebolt.Opposite != "necromancy" || chill.Opposite != "evocation" {
		t.Fatalf("opposites: firebolt %q, chill touch %q", firebolt.Opposite, chill.Opposite)
	}
	target := w.simFighter(10)
	plain, _, _ := meanCast(w, c, target, chill, 400)
	c.Specialty = "necromancy"
	if hasAccess(c, firebolt) || !barred(c, firebolt) || spellBlocker(c, firebolt) != "closed to you" {
		t.Fatalf("a necromancer reached Firebolt")
	}
	if !hasAccess(c, chill) || !hasAccess(c, shippedSpell(t, w, "vampiric_touch")) || hasAccess(c, shippedSpell(t, w, "fireball")) {
		t.Fatalf("necromancer access is wrong")
	}
	sworn, _, _ := meanCast(w, c, target, chill, 400)
	if r := sworn / plain; r < 1.25 || r > 1.35 {
		t.Fatalf("specialty bonus: ratio %.2f, want about 1.3", r)
	}
	s := w.magicSchools()
	if got := strings.Join(s.offeredSchools("red"), ","); got != "conjuration,illusion" {
		t.Fatalf("the Red Robes keep %q", got)
	}
}

// TestNewPlayersFindMagicNearTheStart: the hedge-wizard above the Long
// Hall teaches only simple common spells, and the Good god's beginner
// offering is on a level-3 hobgoblin in Solace (story 1029).
func TestNewPlayersFindMagicNearTheStart(t *testing.T) {
	w := shippedWorld(t)
	byID := map[string]Spell{}
	for _, sp := range w.spellList() {
		byID[sp.ID] = sp
	}
	wizard := w.content.Mobs[215]
	if wizard == nil || len(wizard.Teaches) == 0 {
		t.Fatal("no hedge-wizard")
	}
	for _, id := range wizard.Teaches {
		sp, ok := byID[id]
		if !ok || !sp.common() || sp.Level > 3 || sp.Price <= 0 {
			t.Errorf("the hedge-wizard teaches %q, which is not a simple common spell for sale", id)
		}
	}
	medallion := w.content.Items[338]
	if medallion == nil || medallion.Sacrifice != "good" {
		t.Fatal("no sun medallion for the Good god")
	}
	carried := false
	for _, a := range w.content.Resets {
		for _, r := range a.Resets {
			if r.Mob != 307 {
				continue
			}
			for _, eq := range r.Equip {
				carried = carried || eq.Item == 338
			}
		}
	}
	if !carried || w.content.Mobs[307].Level > 4 {
		t.Fatal("Fewmaster Toede should carry the medallion, and be an early fight")
	}
}

// TestMagicPrizesAreGuardedFromTheWeak: the aggressive guards on the way
// to the Evil altar and at the weirwood seed attack from level 1, so a
// character far under them cannot walk past for a god's favour (story
// 1027).
func TestMagicPrizesAreGuardedFromTheWeak(t *testing.T) {
	w := shippedWorld(t)
	for _, vnum := range []int{705, 706, 712, 1510} {
		p := w.content.Mobs[vnum]
		if p == nil || !p.HasFlag("aggressive") || p.Resolved.Aggro.Min > 1 {
			t.Errorf("mob %d should attack anyone from level 1: %+v", vnum, p.Resolved.Aggro)
		}
	}
}

// TestWayrethTakesTheRedOath: the Tower at Wayreth ends in a Chamber of
// the Test for the Red Order, behind a gate whose key a forest renegade
// carries, and its librarian teaches only the Red Order's school spells
// (story 1036).
func TestWayrethTakesTheRedOath(t *testing.T) {
	w := shippedWorld(t)
	chamber := w.content.Rooms.Rooms[6217]
	if chamber == nil || chamber.Order != "red" {
		t.Fatal("no Chamber of the Test for the Red Order")
	}
	gate := w.content.Rooms.Rooms[6207].Door("north")
	if gate == nil || !gate.Locked || !gate.Pickproof || gate.Key != 6200 {
		t.Fatalf("the Tower's gate: %+v", gate)
	}
	keyHeld := false
	for _, a := range w.content.Resets {
		for _, r := range a.Resets {
			for _, eq := range r.Equip {
				if eq.Item == 6200 && w.content.Mobs[r.Mob].HasFlag("aggressive") {
					keyHeld = true
				}
			}
		}
	}
	if !keyHeld {
		t.Fatal("the Conclave summons should be carried by a hostile in the forest")
	}
	s := w.magicSchools()
	byID := map[string]Spell{}
	for _, sp := range w.spellList() {
		byID[sp.ID] = sp
	}
	for _, id := range w.content.Mobs[6208].Teaches {
		sp := byID[id]
		sc, _ := s.school(sp.School)
		if sp.common() || sc.Order != "red" {
			t.Errorf("the Red librarian teaches %s (%s, %s)", id, sp.School, sp.tierWord())
		}
	}
}

// TestNoTotemsInTheWorld: since 2026-10-12 a specialty is earned at a
// Tower; totems are only the balance lab's (story 1026).
func TestNoTotemsInTheWorld(t *testing.T) {
	w := shippedWorld(t)
	for vnum, p := range w.content.Items {
		if p.Type == "totem" && p.Area != "balance" {
			t.Errorf("item %d (%s, %s) is a totem outside the lab", vnum, p.Name, p.Area)
		}
	}
}

// TestPalanthasTakesTheBlackOath: the Tower at Palanthas ends in a
// Chamber of the Test for the Black Order, its keeper teaches only the
// Black Order's school spells, and the Shoikan Grove guards it from
// anyone (story 1037).
func TestPalanthasTakesTheBlackOath(t *testing.T) {
	w := shippedWorld(t)
	if r := w.content.Rooms.Rooms[6317]; r == nil || r.Order != "black" {
		t.Fatal("no Chamber of the Test for the Black Order")
	}
	s := w.magicSchools()
	byID := map[string]Spell{}
	for _, sp := range w.spellList() {
		byID[sp.ID] = sp
	}
	for _, id := range w.content.Mobs[6307].Teaches {
		sp := byID[id]
		sc, _ := s.school(sp.School)
		if sp.common() || sc.Order != "black" {
			t.Errorf("the keeper of the black books teaches %s (%s, %s)", id, sp.School, sp.tierWord())
		}
	}
	for _, vnum := range []int{6300, 6301, 6302} {
		if p := w.content.Mobs[vnum]; p.Resolved.Aggro.Min > 1 {
			t.Errorf("grove guard %d lets the weak through: %+v", vnum, p.Resolved.Aggro)
		}
	}
}
