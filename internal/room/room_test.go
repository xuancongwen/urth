package room

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

func TestLoadTwoAreas(t *testing.T) {
	dir := writeWorld(t, map[string]string{
		"a/area.yaml":      "name: Area A\n",
		"a/rooms/1.yaml":   "vnum: 1\nname: One\ndescription: |\n  First.\nexits:\n  north: 2\n",
		"a/rooms/2.yaml":   "vnum: 2\nname: Two\ndescription: Second.\nexits:\n  south: 1\n  east: 10\n",
		"b/rooms/10.yaml":  "vnum: 10\nname: Ten\ndescription: Tenth.\n",
		"b/rooms/junk.txt": "ignored",
	})
	w, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Rooms) != 3 {
		t.Fatalf("expected 3 rooms, got %d", len(w.Rooms))
	}
	if w.Areas["a"].Name != "Area A" || w.Areas["b"].Name != "b" {
		t.Fatalf("area names wrong: %+v", w.Areas)
	}
	r, _ := w.Get(2)
	if r.Area != "a" || strings.Join(r.ExitList(), ",") != "east,south" {
		t.Fatalf("room 2 wrong: %+v exits %v", r, r.ExitList())
	}
	one, _ := w.Get(1)
	if one.Description != "First." {
		t.Fatalf("description not trimmed: %q", one.Description)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]map[string]string{
		"duplicate vnum": {
			"a/rooms/1.yaml": "vnum: 1\nname: A\n",
			"b/rooms/1.yaml": "vnum: 1\nname: B\n",
		},
		"dangling exit": {
			"a/rooms/1.yaml": "vnum: 1\nname: A\nexits:\n  north: 99\n",
		},
		"bad direction": {
			"a/rooms/1.yaml": "vnum: 1\nname: A\nexits:\n  sideways: 1\n",
		},
		"missing name": {
			"a/rooms/1.yaml": "vnum: 1\n",
		},
		"no rooms": {
			"a/area.yaml": "name: Empty\n",
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeWorld(t, files)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDoorsMirrorAndValidate(t *testing.T) {
	dir := writeWorld(t, map[string]string{
		"a/rooms/1.yaml": "vnum: 1\nname: One\nexits:\n  north: 2\n  east: 3\ndoors:\n  north: {name: the oak door, closed: true}\n  east: {name: the one-way hatch}\n",
		"a/rooms/2.yaml": "vnum: 2\nname: Two\nexits:\n  south: 1\n",
		"a/rooms/3.yaml": "vnum: 3\nname: Three\nexits:\n  north: 2\n",
	})
	w, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := w.Rooms[2].Door("south")
	if d == nil || d.Name != "the oak door" || !d.Closed {
		t.Fatalf("door not mirrored: %+v", d)
	}
	if d != w.Rooms[1].Door("north") {
		t.Fatal("mirrored door is not shared")
	}
	if w.Rooms[3].Door("west") != nil {
		t.Fatal("door mirrored onto an exit that does not lead back")
	}

	for name, body := range map[string]string{
		"no exit": "vnum: 1\nname: One\nexits: {}\ndoors:\n  north: {name: the door}\n",
		"no name": "vnum: 1\nname: One\nexits:\n  north: 1\ndoors:\n  north: {closed: true}\n",
	} {
		dir := writeWorld(t, map[string]string{"a/rooms/1.yaml": body})
		if _, err := Load(dir); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
