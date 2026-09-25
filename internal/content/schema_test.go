package content

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/reset"
	"urth/internal/room"
)

// The JSON schemas under data/schema are written by hand so they can carry
// descriptions and constraints. This keeps them honest: every yaml field
// on a struct must be a schema property and vice versa.

func yamlKeys(t reflect.Type) []string {
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("yaml")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

// schemaKeys loads file and returns the property names at path, a slash
// separated walk through the JSON ("items/properties/equip/items").
func schemaKeys(t *testing.T, file, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "data", "schema", file))
	if err != nil {
		t.Fatal(err)
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	if path != "" {
		for _, step := range strings.Split(path, "/") {
			m, ok := node.(map[string]any)
			if !ok {
				t.Fatalf("%s: %s is not an object at %q", file, path, step)
			}
			node = m[step]
		}
	}
	props, ok := node.(map[string]any)["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s: no properties at %q", file, path)
	}
	var keys []string
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestSchemasMatchStructs(t *testing.T) {
	cases := []struct {
		file, path string
		typ        any
	}{
		{"room.schema.json", "", room.Room{}},
		{"room.schema.json", "properties/position", room.Coord{}},
		{"room.schema.json", "$defs/door", room.Door{}},
		{"area.schema.json", "", room.Area{}},
		{"item.schema.json", "", item.Proto{}},
		{"mob.schema.json", "", mob.Proto{}},
		{"resets.schema.json", "", reset.Area{}},
		{"resets.schema.json", "properties/resets/items", reset.Reset{}},
		{"resets.schema.json", "properties/resets/items/properties/equip/items", reset.Equip{}},
		{"common.schema.json", "$defs/weapon", item.WeaponSpec{}},
		{"common.schema.json", "$defs/armor", item.ArmorSpec{}},
		{"common.schema.json", "$defs/effect", effect.Spec{}},
	}
	for _, c := range cases {
		want := yamlKeys(reflect.TypeOf(c.typ))
		got := schemaKeys(t, c.file, c.path)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s %s: struct has %v, schema has %v", c.file, c.path, want, got)
		}
	}
}

func TestSchemaEnumsMatchCode(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "data", "schema", "common.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var positions []string
	for _, s := range item.Slots {
		positions = append(positions, string(s))
	}
	sort.Strings(positions)
	got := append([]string(nil), doc.Defs["position"].Enum...)
	sort.Strings(got)
	if !reflect.DeepEqual(positions, got) {
		t.Errorf("position enum: code %v, schema %v", positions, got)
	}
	for _, s := range doc.Defs["itemSlot"].Enum {
		if !item.ValidItemSlot(item.Slot(s)) {
			t.Errorf("itemSlot enum has %q, which the code rejects", s)
		}
	}
	dirs := append([]string(nil), room.Directions...)
	sort.Strings(dirs)
	gotDirs := append([]string(nil), doc.Defs["direction"].Enum...)
	sort.Strings(gotDirs)
	if !reflect.DeepEqual(dirs, gotDirs) {
		t.Errorf("direction enum: code %v, schema %v", dirs, gotDirs)
	}
}
