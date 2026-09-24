// Package mob holds mob (NPC) prototypes loaded from YAML. Instances live in
// the world package because they share the character model with players.
package mob

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Proto is a mob definition.
type Proto struct {
	Vnum int `yaml:"vnum"`
	// Name is the short description used in sentences: "a city guard".
	Name string `yaml:"name"`
	// Keywords are what players type to refer to it.
	Keywords []string `yaml:"keywords"`
	// Description is shown in a room listing: "A city guard stands here."
	Description string `yaml:"description"`
	// Look is shown by "look <mob>".
	Look  string `yaml:"look"`
	Level int    `yaml:"level"`
	// Flags: "sentinel" never moves; "stay_area" (default on) never leaves
	// its area when wandering. Others are free for rules.
	Flags []string `yaml:"flags,omitempty"`
	// Stats are rule inputs the engine stores and never interprets.
	Stats map[string]int `yaml:"stats,omitempty"`

	Area string `yaml:"-"`
}

// HasFlag reports whether the prototype carries flag.
func (p *Proto) HasFlag(flag string) bool {
	for _, f := range p.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// Sentinel mobs never wander.
func (p *Proto) Sentinel() bool { return p.HasFlag("sentinel") }

// StaysInArea reports whether wandering is confined to the home area.
func (p *Proto) StaysInArea() bool { return !p.HasFlag("roam") }

// LoadArea reads every mob under dir/mobs/*.yaml for the area named area.
func LoadArea(dir, area string, protos map[int]*Proto) error {
	files, err := filepath.Glob(filepath.Join(dir, "mobs", "*.yaml"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var p Proto
		if err := yaml.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if p.Vnum <= 0 {
			return fmt.Errorf("%s: vnum must be positive", f)
		}
		if p.Name == "" || len(p.Keywords) == 0 {
			return fmt.Errorf("%s: mob %d: name and keywords are required", f, p.Vnum)
		}
		for i, k := range p.Keywords {
			p.Keywords[i] = strings.ToLower(k)
		}
		if p.Description == "" {
			p.Description = strings.ToUpper(p.Name[:1]) + p.Name[1:] + " is here."
		}
		if p.Level <= 0 {
			p.Level = 1
		}
		if prev, dup := protos[p.Vnum]; dup {
			return fmt.Errorf("%s: mob vnum %d already used by %s in area %s", f, p.Vnum, prev.Name, prev.Area)
		}
		p.Area = area
		protos[p.Vnum] = &p
	}
	return nil
}
