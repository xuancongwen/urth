// Package item holds item prototypes (loaded from YAML) and item instances
// (created from prototypes at reset time or restored from a player file).
// It knows nothing about rules: damage and defense are numbers a rule
// script may read, never interpreted here.
package item

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"urth/internal/effect"
)

// Type is the broad kind of an item. It decides which commands apply.
type Type string

const (
	Weapon    Type = "weapon"
	Armor     Type = "armor"
	Container Type = "container"
	Material  Type = "material" // consumed by spells (docs/RULES.md 6.2)
	Totem     Type = "totem"    // consumed to unlock a school (6.1)
	Other     Type = "other"
)

// Slot is an equipment position.
type Slot string

// Slots in display order. Wield and Hold take weapons and held items; the
// rest take armor. Which slots matter for balance is a rules question.
var Slots = []Slot{
	"light", "head", "neck", "body", "arms", "hands", "wrist", "finger",
	"waist", "legs", "feet", "shield", "wield", "hold",
}

// ValidSlot reports whether s is a known slot.
func ValidSlot(s Slot) bool {
	for _, k := range Slots {
		if k == s {
			return true
		}
	}
	return false
}

// Proto is an item definition.
type Proto struct {
	Vnum int `yaml:"vnum"`
	// Name is the short description used in sentences: "a rusty sword".
	Name string `yaml:"name"`
	// Keywords are what players type to refer to it.
	Keywords []string `yaml:"keywords"`
	// Description is shown when the item lies in a room: "A rusty sword lies here."
	Description string `yaml:"description"`
	// Look is shown by "look <item>".
	Look string `yaml:"look"`
	Type Type   `yaml:"type"`
	// Slot is where armor is worn. Weapons go in wield; held items in hold.
	Slot   Slot `yaml:"slot"`
	Weight int  `yaml:"weight"`
	Value  int  `yaml:"value"`
	// Level and Baseline select the row of the rules' item curve this item
	// was drawn from (docs/RULES.md 5.2). Level defaults to 1; an empty
	// Baseline means the rules' default for the type.
	Level    int    `yaml:"level"`
	Baseline string `yaml:"baseline"`
	// Rule inputs as the builder stated them. The engine stores them and
	// never interprets them. Fields left zero are filled from the baseline
	// by the rules' itemBaseline hook into Resolved.
	Weapon *WeaponSpec    `yaml:"weapon,omitempty"`
	Armor  *ArmorSpec     `yaml:"armor,omitempty"`
	Mods   map[string]int `yaml:"mods,omitempty"`
	// Effects are intrinsic: every instance of this prototype has them.
	Effects []effect.Spec `yaml:"effects,omitempty"`
	// Material is the pool name a material item counts as ("ash"), and
	// Rarity its tier ("common", "uncommon", "rare", "deterministic").
	Material string `yaml:"material,omitempty"`
	Rarity   string `yaml:"rarity,omitempty"`
	// School is the arcane school a totem unlocks.
	School string `yaml:"school,omitempty"`
	// Sacrifice names the deity ("good", "neutral", "evil") that accepts
	// this item as a great sacrifice at its temple.
	Sacrifice string `yaml:"sacrifice,omitempty"`
	// Resolved is the numbers the game actually uses: stated fields with
	// the baseline filling the gaps. Set by the world after load and after
	// every script reload; never read from YAML.
	Resolved Resolved `yaml:"-"`
	// Capacity is how much a container holds, in weight. 0 means unlimited.
	Capacity int `yaml:"capacity"`
	// Flags are free-form markers for rules and builders.
	Flags []string `yaml:"flags,omitempty"`

	Area string `yaml:"-"`
}

// WeaponSpec is rule input for weapons. Damage is the mean per swing,
// Spread the fraction it varies by, Speed the swings per round, Verb the
// word combat messages use ("slash", "bite").
type WeaponSpec struct {
	Damage int     `yaml:"damage" json:"damage"`
	Spread float64 `yaml:"spread" json:"spread"`
	Speed  float64 `yaml:"speed" json:"speed"`
	Hands  int     `yaml:"hands" json:"hands"`
	Kind   string  `yaml:"kind" json:"kind"` // slash, pierce, blunt, ...
	Verb   string  `yaml:"verb" json:"verb"`
}

// ArmorSpec is rule input for armor.
type ArmorSpec struct {
	Defense int     `yaml:"defense" json:"defense"`
	Spread  float64 `yaml:"spread" json:"spread"`
}

// Resolved holds the numbers in play after the baseline has been applied.
// Source says where they came from, for the builder's stat command.
type Resolved struct {
	Weapon WeaponSpec
	Armor  ArmorSpec
	Source string
}

// ResolveStated fills Resolved with the stated values alone, for use when
// no baseline hook is available.
func (p *Proto) ResolveStated() {
	p.Resolved = Resolved{Source: "stated"}
	if p.Weapon != nil {
		p.Resolved.Weapon = *p.Weapon
	}
	if p.Armor != nil {
		p.Resolved.Armor = *p.Armor
	}
	if p.Resolved.Weapon.Speed == 0 && p.Type == Weapon {
		p.Resolved.Weapon.Speed = 1
	}
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

// WearSlot is where the item goes when worn, wielded, or held.
func (p *Proto) WearSlot() Slot {
	switch p.Type {
	case Weapon:
		return "wield"
	case Armor:
		return p.Slot
	default:
		if p.Slot != "" {
			return p.Slot
		}
		return ""
	}
}

// LoadArea reads every item under dir/items/*.yaml for the area named
// area, appending to protos and rejecting duplicates and malformed files.
func LoadArea(dir, area string, protos map[int]*Proto) error {
	files, err := filepath.Glob(filepath.Join(dir, "items", "*.yaml"))
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
		if err := p.validate(); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if prev, dup := protos[p.Vnum]; dup {
			return fmt.Errorf("%s: item vnum %d already used by %s in area %s", f, p.Vnum, prev.Name, prev.Area)
		}
		p.Area = area
		protos[p.Vnum] = &p
	}
	return nil
}

func (p *Proto) validate() error {
	if p.Vnum <= 0 {
		return fmt.Errorf("vnum must be positive")
	}
	if p.Name == "" || len(p.Keywords) == 0 {
		return fmt.Errorf("item %d: name and keywords are required", p.Vnum)
	}
	for i, k := range p.Keywords {
		p.Keywords[i] = strings.ToLower(k)
	}
	if p.Type == "" {
		p.Type = Other
	}
	switch p.Type {
	case Weapon, Armor, Container, Other:
	case Material:
		if p.Material == "" {
			return fmt.Errorf("item %d: a material needs a material name", p.Vnum)
		}
		if p.Rarity == "" {
			p.Rarity = "common"
		}
	case Totem:
		if p.School == "" {
			return fmt.Errorf("item %d: a totem needs a school", p.Vnum)
		}
	default:
		return fmt.Errorf("item %d: unknown type %q", p.Vnum, p.Type)
	}
	if p.Type == Armor && !ValidSlot(p.Slot) {
		return fmt.Errorf("item %d: armor needs a valid slot, got %q", p.Vnum, p.Slot)
	}
	if p.Slot != "" && !ValidSlot(p.Slot) {
		return fmt.Errorf("item %d: unknown slot %q", p.Vnum, p.Slot)
	}
	if p.Type == Weapon && p.Slot == "" {
		p.Slot = "wield"
	}
	if p.Level <= 0 {
		p.Level = 1
	}
	for i, e := range p.Effects {
		if e.Kind == "" {
			return fmt.Errorf("item %d: effect %d has no kind", p.Vnum, i)
		}
	}
	p.ResolveStated()
	if p.Description == "" {
		p.Description = capitalize(p.Name) + " is here."
	}
	return nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
