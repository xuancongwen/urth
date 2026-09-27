package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aligned is a resets file the way the areas write them: hand-aligned,
// with trailing comments and a section header.
const aligned = `interval_seconds: 300
resets:
  # --- the hall ---
  - { mob: 834, room: 800, equip: [{ item: 850, slot: wield }] }   # a tightener
  - { mob: 837, room: 802, max: 3 }                            # pipe-rats
  # --- the vats ---
  - { item: 863, room: 804 }                                   # grease
`

func TestAddResetKeepsTheFile(t *testing.T) {
	out, err := addReset([]byte(aligned), "{ item: 1, room: 802 }   # a key", 802)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(aligned, "# pipe-rats\n", "# pipe-rats\n  - { item: 1, room: 802 }   # a key\n", 1)
	if string(out) != want {
		t.Fatalf("after the same room:\n%s", out)
	}
	out, _ = addReset([]byte(aligned), "{ item: 1, room: 999 }", 999)
	if !strings.HasSuffix(string(out), "# grease\n  - { item: 1, room: 999 }\n") {
		t.Fatalf("at the end:\n%s", out)
	}
	for src, want := range map[string]string{
		"":                                   "resets:\n  - { item: 1, room: 5 }\n",
		"interval_seconds: 60\nresets: []\n": "interval_seconds: 60\nresets:\n  - { item: 1, room: 5 }\n",
		"interval_seconds: 60\n":             "interval_seconds: 60\nresets:\n  - { item: 1, room: 5 }\n",
	} {
		if out, err := addReset([]byte(src), "{ item: 1, room: 5 }", 5); err != nil || string(out) != want {
			t.Errorf("from %q: %v\n%s", src, err, out)
		}
	}
	if _, err := addReset([]byte("resets: [{ item: 1, room: 5 }]\n"), "{ item: 2, room: 5 }", 5); err == nil {
		t.Error("edited a one-line flow list")
	}
}

func TestRemoveResets(t *testing.T) {
	out, err := removeResets([]byte(aligned), []int{1})
	if err != nil || string(out) != strings.Replace(aligned, "  - { mob: 837, room: 802, max: 3 }                            # pipe-rats\n", "", 1) {
		t.Fatalf("remove one: %v\n%s", err, out)
	}
	out, err = removeResets([]byte(aligned), []int{0, 2, 1})
	if err != nil || !strings.Contains(string(out), "resets: []") || strings.Contains(string(out), "room:") {
		t.Fatalf("remove all: %v\n%s", err, out)
	}
	block := "resets:\n  - mob: 20\n    room: 1\n    max: 2\n  - item: 12\n    room: 1\n"
	if out, err := removeResets([]byte(block), []int{0}); err != nil || string(out) != "resets:\n  - item: 12\n    room: 1\n" {
		t.Fatalf("block style: %v\n%s", err, out)
	}
}

func TestPlaceAndUnplace(t *testing.T) {
	srv, api, dir := newEditTest(t)
	// A new item in the hall: next free item vnum in the block, a stub
	// file, and a reset, with the area's resets run at once.
	code, res := send(t, srv, "POST", "/api/place", PlaceRequest{Room: 2, Kind: "item", Name: "a brass key"}, nil)
	if code != 200 || res.Vnum != 1 || !strings.HasSuffix(res.Created, filepath.Join("a", "items", "1.yaml")) {
		t.Fatalf("place new: %d %+v", code, res)
	}
	if got := read(t, res.Created); !strings.Contains(got, "name: a brass key\nkeywords: [brass, key]\ndescription: A brass key lies here.\n") {
		t.Fatalf("new item:\n%s", got)
	}
	resets := filepath.Join(dir, "a", "resets.yaml")
	if got := read(t, resets); got != "resets:\n  - { item: 1, room: 2 }   # a brass key\n" {
		t.Fatalf("resets:\n%s", got)
	}
	if r := api.reloaded(); r[len(r)-1] != "a" {
		t.Fatalf("area resets not run: %v", r)
	}
	// An existing prototype, as a mob with a cap; then a bad kind.
	if err := os.MkdirAll(filepath.Join(dir, "a", "mobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "mobs", "7.yaml"), []byte("vnum: 7\nname: a rat\nkeywords: [rat]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, res := send(t, srv, "POST", "/api/place", PlaceRequest{Room: 1, Kind: "mob", Vnum: 7, Max: 3}, nil); code != 200 {
		t.Fatalf("place mob: %d %+v", code, res)
	}
	if got := read(t, resets); !strings.Contains(got, "  - { mob: 7, room: 1, max: 3 }   # a rat\n") {
		t.Fatalf("mob reset:\n%s", got)
	}
	if code, _ := send(t, srv, "POST", "/api/place", PlaceRequest{Room: 1, Kind: "shoe", Vnum: 7}, nil); code != 409 {
		t.Fatalf("bad kind: %d", code)
	}
	if code, _ := send(t, srv, "POST", "/api/place", PlaceRequest{Room: 1, Kind: "mob", Vnum: 99}, nil); code != 409 {
		t.Fatalf("missing prototype: %d", code)
	}

	// Unplace checks the reset is the one the page saw.
	if code, _ := send(t, srv, "POST", "/api/unplace", UnplaceRequest{Area: "a", Index: 0, Room: 1, Vnum: 7}, nil); code != 409 {
		t.Fatalf("stale unplace: %d", code)
	}
	if code, res := send(t, srv, "POST", "/api/unplace", UnplaceRequest{Area: "a", Index: 1, Room: 1, Vnum: 7}, nil); code != 200 {
		t.Fatalf("unplace: %d %+v", code, res)
	}
	if got := read(t, resets); strings.Contains(got, "mob: 7") || !strings.Contains(got, "item: 1") {
		t.Fatalf("after unplace:\n%s", got)
	}

	// Deleting a room takes its resets with it.
	if code, res := send(t, srv, "POST", "/api/dig", DigRequest{From: 2, Dir: "east", Name: "Shed"}, nil); code != 200 || res.Vnum != 3 {
		t.Fatalf("dig: %d %+v", code, res)
	}
	if code, _ := send(t, srv, "POST", "/api/place", PlaceRequest{Room: 3, Kind: "item", Vnum: 1}, nil); code != 200 {
		t.Fatal("place in the shed")
	}
	if code, res := send(t, srv, "POST", "/api/delete", DeleteRequest{Vnum: 3}, nil); code != 200 {
		t.Fatalf("delete: %d %+v", code, res)
	}
	if got := read(t, resets); got != "resets:\n  - { item: 1, room: 2 }   # a brass key\n" {
		t.Fatalf("resets after delete:\n%s", got)
	}
}
