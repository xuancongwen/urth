// Package reset loads an area's repopulation script: which mobs and items to
// place where, with limits, on a schedule. Executing resets is the world's
// job; this package only describes them.
package reset

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultIntervalSeconds is used when an area does not say.
const DefaultIntervalSeconds = 600

// Area is one area's reset schedule.
type Area struct {
	// IntervalSeconds between repopulations. Resets also run once at boot.
	IntervalSeconds int     `yaml:"interval_seconds"`
	Resets          []Reset `yaml:"resets"`
}

// Reset is one placement. Exactly one of Mob or Item is set.
type Reset struct {
	// Mob places a mob prototype in Room. Max caps live instances of that
	// prototype in the whole world (0 means 1).
	Mob int `yaml:"mob,omitempty"`
	// Item places an item prototype in Room, or inside the container item
	// prototype Into which must already be in Room. Skipped if one is
	// already there.
	Item int `yaml:"item,omitempty"`
	Into int `yaml:"into,omitempty"`
	Room int `yaml:"room"`
	Max  int `yaml:"max,omitempty"`
	// Equip lists items a spawned mob carries or wears.
	Equip []Equip `yaml:"equip,omitempty"`
}

// Equip is an item given to a mob at spawn. Slot empty means inventory.
type Equip struct {
	Item int    `yaml:"item"`
	Slot string `yaml:"slot,omitempty"`
}

// LoadArea reads dir/resets.yaml. A missing file means no resets.
func LoadArea(dir string) (*Area, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "resets.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return &Area{IntervalSeconds: DefaultIntervalSeconds}, nil
	}
	if err != nil {
		return nil, err
	}
	var a Area
	if err := yaml.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, "resets.yaml"), err)
	}
	if a.IntervalSeconds <= 0 {
		a.IntervalSeconds = DefaultIntervalSeconds
	}
	for i := range a.Resets {
		r := &a.Resets[i]
		if (r.Mob == 0) == (r.Item == 0) {
			return nil, fmt.Errorf("%s: reset %d must set exactly one of mob or item", dir, i)
		}
		if r.Room <= 0 {
			return nil, fmt.Errorf("%s: reset %d needs a room", dir, i)
		}
		if r.Mob != 0 && r.Max <= 0 {
			r.Max = 1
		}
		if r.Item != 0 && len(r.Equip) > 0 {
			return nil, fmt.Errorf("%s: reset %d: equip only applies to mobs", dir, i)
		}
	}
	return &a, nil
}
