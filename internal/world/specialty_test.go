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
