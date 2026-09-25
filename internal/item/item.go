package item

import (
	"strconv"
	"strings"
	"sync/atomic"

	"urth/internal/effect"
)

var lastID atomic.Uint64

// Item is one instance of a prototype. Where it is (a room, a character's
// inventory, an equipment slot, inside a container) is recorded by whoever
// holds it, not here.
type Item struct {
	ID    uint64
	Proto *Proto
	// Contents are the items inside, for containers.
	Contents []*Item
	// Effects applied to this instance (an enchantment), on top of the
	// prototype's intrinsic ones.
	Effects []effect.Active
	// Decay is rounds until this item is destroyed; 0 means never. Corpses
	// use it.
	Decay int
}

// AllEffects renders intrinsic and applied effects for scripts.
func (i *Item) AllEffects() []any { return effect.Views(i.Proto.Effects, i.Effects) }

// New creates an instance of p.
func New(p *Proto) *Item {
	return &Item{ID: lastID.Add(1), Proto: p}
}

// Name is the short description.
func (i *Item) Name() string { return i.Proto.Name }

// IsContainer reports whether things can be put inside.
func (i *Item) IsContainer() bool { return i.Proto.Type == Container }

// Matches reports whether a typed word refers to this item: the word is a
// prefix of one of its keywords, case-insensitively.
func (i *Item) Matches(word string) bool {
	return MatchKeywords(i.Proto.Keywords, word)
}

// MatchKeywords is the shared keyword test used for items and mobs.
func MatchKeywords(keywords []string, word string) bool {
	word = strings.ToLower(word)
	if word == "" {
		return false
	}
	for _, k := range keywords {
		if strings.HasPrefix(k, word) {
			return true
		}
	}
	return false
}

// Target is a parsed object reference: "sword", "2.sword", "all", "all.sword".
type Target struct {
	Word  string
	Index int  // 1-based; 0 with All means every match
	All   bool // "all" or "all.<word>"
}

// ParseTarget splits ROM-style references.
func ParseTarget(s string) Target {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "all" {
		return Target{All: true}
	}
	if rest, ok := strings.CutPrefix(s, "all."); ok {
		return Target{Word: rest, All: true}
	}
	if n, word, ok := strings.Cut(s, "."); ok {
		if idx, err := strconv.Atoi(n); err == nil && idx > 0 {
			return Target{Word: word, Index: idx}
		}
	}
	return Target{Word: s, Index: 1}
}

// Find returns the items in list that t selects, in list order.
func Find(list []*Item, t Target) []*Item {
	var out []*Item
	n := 0
	for _, it := range list {
		if t.Word != "" && !it.Matches(t.Word) {
			continue
		}
		n++
		if t.All {
			out = append(out, it)
		} else if n == t.Index {
			return []*Item{it}
		}
	}
	if t.All {
		return out
	}
	return nil
}

// Remove deletes it from list, returning the new list.
func Remove(list []*Item, it *Item) []*Item {
	for i, x := range list {
		if x == it {
			return append(list[:i:i], list[i+1:]...)
		}
	}
	return list
}

// Group counts consecutive-agnostic duplicates by prototype, preserving
// first-seen order, for "(2) a rusty sword" listings.
func Group(list []*Item) []Grouped {
	var out []Grouped
	index := map[*Proto]int{}
	for _, it := range list {
		if i, ok := index[it.Proto]; ok {
			out[i].Count++
			continue
		}
		index[it.Proto] = len(out)
		out = append(out, Grouped{Item: it, Count: 1})
	}
	return out
}

// Grouped is one line of a listing.
type Grouped struct {
	Item  *Item
	Count int
}
