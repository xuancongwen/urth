// Package effect holds the engine's share of the effect model (docs/RULES.md
// 5.4): a kind, its parameters, mutable state, and a lifetime. The engine
// stores all of it blindly and only manages lifetimes; what a kind does is
// defined in the rules scripts.
package effect

// Spec is an effect as a builder writes it on a prototype: a kind and its
// parameters. Intrinsic effects are Specs.
type Spec struct {
	Kind   string         `yaml:"kind" json:"kind"`
	Params map[string]any `yaml:"params,omitempty" json:"params"`
}

// Active is an effect attached to a character or an item instance. State is
// whatever the scripts have recorded about it (charges, stacks); Rounds is
// how many rounds remain, with 0 meaning permanent.
type Active struct {
	Spec   `yaml:",inline"`
	State  map[string]any `yaml:"state,omitempty" json:"state"`
	Rounds int            `yaml:"rounds,omitempty" json:"rounds"`
}

// View renders an Active for scripts.
func (a Active) View() map[string]any {
	return map[string]any{"kind": a.Kind, "params": copyMap(a.Params), "state": copyMap(a.State), "rounds": a.Rounds}
}

// SpecView renders a Spec for scripts, as a permanent effect.
func (s Spec) View() map[string]any {
	return map[string]any{"kind": s.Kind, "params": copyMap(s.Params), "state": map[string]any{}, "rounds": 0}
}

// Views renders a list of specs followed by a list of actives as one list.
func Views(specs []Spec, actives []Active) []any {
	out := make([]any, 0, len(specs)+len(actives))
	for _, s := range specs {
		out = append(out, s.View())
	}
	for _, a := range actives {
		out = append(out, a.View())
	}
	return out
}

// Tick counts down every timed effect and drops the expired ones.
func Tick(list []Active) []Active {
	out := list[:0]
	for _, a := range list {
		if a.Rounds > 0 {
			a.Rounds--
			if a.Rounds == 0 {
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func copyMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
