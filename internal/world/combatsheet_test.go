package world

import (
	"strings"
	"testing"
)

// The score sheet's combat rows add up feats, gear, and stats under the
// shipped rules: extra attacks show as swings a round, the best crit with
// Precise Strikes' bonus, and rows such as Soak only when something grants
// them.
func TestCombatSheetShippedRules(t *testing.T) {
	w := shippedWorld(t)
	rows := func(feats ...string) map[string]string {
		c := w.simFighter(20)
		byID := map[string]Feat{}
		for _, f := range w.featList() {
			byID[f.ID] = f
		}
		for _, id := range feats {
			f, ok := byID[id]
			if !ok {
				t.Fatalf("no feat %q", id)
			}
			w.grantFeat(c, f)
		}
		w.recalc(c)
		out := map[string]string{}
		for _, r := range w.combatSheet(c) {
			out[r.Label] = r.Value
		}
		return out
	}

	plain := rows()
	if plain["Critical"] != "none" || plain["Damage"] != "+0%" || plain["Soak"] != "" || plain["Damage taken"] != "" {
		t.Fatalf("plain fighter: %v", plain)
	}
	if !strings.HasSuffix(plain["Attacks"], " a round") || !strings.HasSuffix(plain["Armor"], "% off hits") {
		t.Fatalf("plain fighter core rows: %v", plain)
	}

	fit := rows("second_attack", "third_attack", "keen", "deadly", "precise", "weapon_master", "thick_hide", "juggernaut", "executioner", "sure_footing")
	if fit["Attacks"] == plain["Attacks"] {
		t.Fatalf("two extra attacks change nothing: %v vs %v", fit["Attacks"], plain["Attacks"])
	}
	for label, want := range map[string]string{
		"Critical":     "10% for 2.5x",
		"Damage":       "+10%",
		"Damage taken": "-10%",
		"Execute":      "+25% under 20%",
		"Footing":      "50% keep a round",
	} {
		if fit[label] != want {
			t.Fatalf("%s = %q, want %q (%v)", label, fit[label], want, fit)
		}
	}
	if !strings.HasSuffix(fit["Soak"], " off each hit") {
		t.Fatalf("soak: %v", fit)
	}
}
