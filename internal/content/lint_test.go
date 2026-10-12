package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWorld(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func kinds(problems []Problem) map[string]int {
	out := map[string]int{}
	for _, p := range problems {
		out[p.Kind]++
	}
	return out
}

func TestLintFindsTheUsualMistakes(t *testing.T) {
	dir := writeWorld(t, map[string]string{
		"a/rooms/1.yaml": "vnum: 1\nname: Hub\ndescription: The hub.\nexits:\n  north: 2\n  east: 3\n",
		"a/rooms/2.yaml": "vnum: 2\nname: North\ndescription: x\nexits:\n  south: 1\n",
		// 3 is reached from 1 but its way back goes to 2: a mismatch.
		"a/rooms/3.yaml": "vnum: 3\nname: East\nexits:\n  west: 2\n  up: 4\n",
		// 4 has no way back: one-way.
		"a/rooms/4.yaml": "vnum: 4\nname: Loft\ndescription: x\nexits: {}\n",
		// 9 is in area b and connects to nothing.
		"b/rooms/9.yaml": "vnum: 9\nname: Lost\ndescription: x\nexits: {}\n",
		// b's item vnum sits inside a's range.
		"b/items/5.yaml":  "vnum: 5\nname: a stray coin\nkeywords: [coin]\n",
		"a/items/10.yaml": "vnum: 10\nname: a sword\nkeywords: [sword]\ntype: weapon\n",
		// A cap with a material's rarity; the material's own is fine.
		"a/items/11.yaml": "vnum: 11\nname: a cap\nkeywords: [cap]\ntype: armor\nslot: head\nrarity: rare\n",
		"a/items/12.yaml": "vnum: 12\nname: a fang\nkeywords: [fang]\ntype: material\nmaterial: fang\nrarity: rare\n",
		"a/mobs/20.yaml":  "vnum: 20\nname: a guard\nkeywords: [guard]\n",
		"a/mobs/21.yaml":  "vnum: 21\nname: a ghost\nkeywords: [ghost]\n",
		"a/resets.yaml":   "resets:\n  - mob: 20\n    room: 9\n    equip:\n      - item: 10\n        slot: wield\n",
	})
	w, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	problems := Lint(w, 1)
	got := kinds(problems)
	want := map[string]int{
		"unreachable":   1, // room 9
		"description":   1, // room 3
		"one-way":       2, // 3 up to 4, 3 west to 2
		"mismatch":      1, // 1 east to 3, 3 west to 2
		"unplaced":      4, // item 5, item 11, item 12, mob 21
		"foreign-reset": 1, // a's reset into b's room 9
		"vnum-overlap":  1,
		"stray-rarity":  1, // item 11
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%s: got %d, want %d\n%s", k, got[k], n, dump(problems))
		}
	}
	if Errors(problems) != 1 {
		t.Errorf("errors: got %d, want 1", Errors(problems))
	}
	for _, p := range problems {
		if p.Kind == "unreachable" && (p.Area != "b" || p.Vnum != 9 || !strings.HasSuffix(p.File, "9.yaml")) {
			t.Errorf("unreachable finding is wrong: %+v", p)
		}
	}
	// Sorted by area, errors first within an area.
	if problems[0].Area != "" && problems[0].Area != "a" {
		t.Errorf("not sorted by area: %s", dump(problems))
	}
}

func TestLintDetachedAreaIsLeftAlone(t *testing.T) {
	dir := writeWorld(t, map[string]string{
		"a/rooms/1.yaml":  "vnum: 1\nname: Hub\ndescription: x\nexits: {}\n",
		"z/area.yaml":     "name: Range\ndetached: true\n",
		"z/rooms/90.yaml": "vnum: 90\nname: Range\ndescription: x\nexits: {}\n",
		"z/mobs/91.yaml":  "vnum: 91\nname: a dummy\nkeywords: [dummy]\n",
	})
	w, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if problems := Lint(w, 1); len(problems) != 0 {
		t.Fatalf("detached area reported: %s", dump(problems))
	}
	if problems := Lint(w, 77); Errors(problems) != 1 || problems[0].Kind != "start" {
		t.Fatalf("missing start room not an error: %s", dump(problems))
	}
}

func TestLintShippedWorldHasNoErrors(t *testing.T) {
	w, err := Load(filepath.Join("..", "..", "data", "world"))
	if err != nil {
		t.Fatal(err)
	}
	problems := Lint(w, 1)
	if n := Errors(problems); n != 0 {
		t.Fatalf("%d errors in data/world:\n%s", n, dump(problems))
	}
}

func dump(problems []Problem) string {
	var b strings.Builder
	for _, p := range problems {
		b.WriteString("  " + p.String() + "\n")
	}
	return b.String()
}

// TestLintKinds: a weapon, a natural attack, or an effect naming a damage
// kind the rules do not know is a warning; families count for resists.
func TestLintKinds(t *testing.T) {
	dir := writeWorld(t, map[string]string{
		"a/rooms/1.yaml":  "vnum: 1\nname: Hub\ndescription: x\nexits: {}\n",
		"a/items/10.yaml": "vnum: 10\nname: a sword\nkeywords: [sword]\ntype: weapon\nweapon: { kind: slash }\n",
		"a/items/11.yaml": "vnum: 11\nname: a wand\nkeywords: [wand]\ntype: weapon\nweapon: { kind: blast }\n",
		"a/items/12.yaml": "vnum: 12\nname: a ring\nkeywords: [ring]\ntype: armor\nslot: finger\neffects:\n  - { kind: resist, params: { kind: elemental, amount: 0.2 } }\n  - { kind: proc, params: { kind: plasma, power: 0.5 } }\n",
		"a/mobs/20.yaml":  "vnum: 20\nname: a ghost\nkeywords: [ghost]\nattack: { kind: wail }\neffects:\n  - { kind: resist, params: { kind: physical, amount: 1 } }\n  - { kind: resist, params: { kind: shadow, amount: 1 } }\n",
	})
	w, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, p := range LintKinds(w, []string{"slash", "fire"}) {
		texts = append(texts, p.Text)
	}
	got := strings.Join(texts, "\n")
	for _, want := range []string{`item 11 (a wand) names damage kind "blast"`, `item 12 (a ring)'s proc effect names damage kind "plasma"`,
		`mob 20 (a ghost) names damage kind "wail"`, `mob 20 (a ghost)'s resist effect names damage kind "shadow"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if len(texts) != 4 {
		t.Errorf("want 4 warnings, got %d:\n%s", len(texts), got)
	}
}
