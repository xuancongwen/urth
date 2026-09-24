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
