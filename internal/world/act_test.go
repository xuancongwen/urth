package world

import "testing"

func TestExpandAct(t *testing.T) {
	bob := newCharacter("Bob", []string{"bob"})
	alice := newCharacter("Alice", []string{"alice"})
	cases := map[string]string{
		"$n hits $N.":        "Bob hits Alice.",
		"$n says '$t'":       "Bob says 'hi there'",
		"cost: $$5":          "cost: $5",
		"$n gives $p to $N.": "Bob gives a sword to Alice.",
		"$z is unknown":      "$z is unknown",
		"trailing $":         "trailing $",
		"$N looks at $N":     "Alice looks at Alice",
	}
	for in, want := range cases {
		if got := expandAct(in, bob, alice, "a sword", "hi there"); got != want {
			t.Errorf("expandAct(%q) = %q, want %q", in, got, want)
		}
	}
	if got := expandAct("$N vanishes", bob, nil, "", ""); got != "someone vanishes" {
		t.Errorf("nil victim: %q", got)
	}
}
