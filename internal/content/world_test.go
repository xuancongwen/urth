package content

import (
	"os"
	"testing"
)

// TestShippedWorld loads the areas under data/world and fails on any
// reference the loader rejects or any map conflict the layout reports.
// URTH_WORLD overrides the directory.
func TestShippedWorld(t *testing.T) {
	dir := os.Getenv("URTH_WORLD")
	if dir == "" {
		dir = "../../data/world"
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no world at %s", dir)
	}
	w, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, warn := range w.Rooms.Warnings {
		t.Error(warn)
	}
	t.Logf("%d rooms, %d items, %d mobs, %d areas", len(w.Rooms.Rooms), len(w.Items), len(w.Mobs), len(w.Resets))
}
