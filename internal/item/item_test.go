package item

import (
	"os"
	"path/filepath"
	"testing"
)

func proto(vnum int, name string, kws ...string) *Proto {
	return &Proto{Vnum: vnum, Name: name, Keywords: kws, Type: Other}
}

func TestParseTarget(t *testing.T) {
	cases := map[string]Target{
		"sword":     {Word: "sword", Index: 1},
		"2.sword":   {Word: "sword", Index: 2},
		"all":       {All: true},
		"all.sword": {Word: "sword", All: true},
		"0.sword":   {Word: "0.sword", Index: 1},
		"Sword":     {Word: "sword", Index: 1},
	}
	for in, want := range cases {
		if got := ParseTarget(in); got != want {
			t.Errorf("ParseTarget(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestFindAndMatch(t *testing.T) {
	swordProto := proto(1, "a rusty sword", "rusty", "sword")
	sword1 := New(swordProto)
	sword2 := New(swordProto)
	cap := New(proto(2, "a leather cap", "leather", "cap"))
	list := []*Item{sword1, cap, sword2}

	if got := Find(list, ParseTarget("sw")); len(got) != 1 || got[0] != sword1 {
		t.Fatalf("prefix match wrong: %v", got)
	}
	if got := Find(list, ParseTarget("2.sword")); len(got) != 1 || got[0] != sword2 {
		t.Fatalf("indexed match wrong: %v", got)
	}
	if got := Find(list, ParseTarget("3.sword")); got != nil {
		t.Fatalf("out of range should be nil: %v", got)
	}
	if got := Find(list, ParseTarget("all.sword")); len(got) != 2 {
		t.Fatalf("all.sword wrong: %v", got)
	}
	if got := Find(list, ParseTarget("all")); len(got) != 3 {
		t.Fatalf("all wrong: %v", got)
	}
	if got := Find(list, ParseTarget("word")); got != nil {
		t.Fatalf("mid-word should not match: %v", got)
	}
	list = Remove(list, cap)
	if len(list) != 2 || list[1] != sword2 {
		t.Fatalf("remove wrong: %v", list)
	}
	g := Group([]*Item{sword1, cap, sword2})
	if len(g) != 2 || g[0].Count != 2 || g[1].Item != cap {
		t.Fatalf("group wrong: %+v", g)
	}
}

func TestLoadArea(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "items"), 0o755)
	os.WriteFile(filepath.Join(dir, "items", "10.yaml"), []byte("vnum: 10\nname: a sword\nkeywords: [Sword]\ntype: weapon\nweapon:\n  damage: 5\n  hands: 1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "items", "11.yaml"), []byte("vnum: 11\nname: a cap\nkeywords: [cap]\ntype: armor\nslot: head\narmor:\n  defense: 1\n"), 0o644)
	protos := map[int]*Proto{}
	if err := LoadArea(dir, "a", protos); err != nil {
		t.Fatal(err)
	}
	if protos[10].WearSlot() != "wield" || protos[10].Keywords[0] != "sword" || protos[10].Description != "A sword is here." {
		t.Fatalf("weapon defaults wrong: %+v", protos[10])
	}
	if protos[11].WearSlot() != "head" || protos[11].Armor.Defense != 1 {
		t.Fatalf("armor wrong: %+v", protos[11])
	}
	os.WriteFile(filepath.Join(dir, "items", "12.yaml"), []byte("vnum: 12\nname: bad\nkeywords: [bad]\ntype: armor\nslot: elbow\n"), 0o644)
	if err := LoadArea(dir, "a", map[int]*Proto{}); err == nil {
		t.Fatal("expected bad slot error")
	}
}
