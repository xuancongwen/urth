package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
)

// lastState returns the most recent State message sent on c, if any.
func lastState(c *fakeConn) (output.StateData, bool) {
	for i := len(c.batches) - 1; i >= 0; i-- {
		msgs := c.batches[i].Messages
		for j := len(msgs) - 1; j >= 0; j-- {
			if msgs[j].Type == output.State {
				return msgs[j].Data.(output.StateData), true
			}
		}
	}
	return output.StateData{}, false
}

func TestClientState(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	if _, ok := lastState(bob); ok {
		t.Fatal("state sent without asking")
	}
	send(w, 1, "client state")
	s, ok := lastState(bob)
	if !ok {
		t.Fatal("no state after asking")
	}
	if s.Name != "Bob" || s.Room.Vnum != 1 || len(s.Room.Mobs) != 1 || s.Room.Mobs[0].Ref != "city" {
		t.Fatalf("state: %+v", s)
	}

	// Nothing changed, so nothing is sent.
	bob.batches = nil
	w.Tick()
	if _, ok := lastState(bob); ok {
		t.Fatal("unchanged state resent")
	}

	// A cap and a sword, named by their most distinctive keywords.
	p.Inventory = append(p.Inventory, item.New(w.content.Items[11]), item.New(w.content.Items[10]))
	w.Tick()
	s, _ = lastState(bob)
	if len(s.Inventory) != 2 || s.Inventory[0].Ref != "cap" || s.Inventory[0].Slot != "head" || s.Inventory[1].Type != "weapon" {
		t.Fatalf("inventory: %+v", s.Inventory)
	}
	send(w, 1, "wear "+s.Inventory[0].Ref)
	s, _ = lastState(bob)
	if len(s.Equipment) != 1 || s.Equipment[0].Slot != "head" || s.Equipment[0].Ref != "cap" {
		t.Fatalf("equipment: %+v", s.Equipment)
	}

	send(w, 1, "kill "+s.Room.Mobs[0].Ref)
	s, _ = lastState(bob)
	if s.Target == nil || s.Target.Name != "a city guard" || len(s.Enemies) != 1 {
		t.Fatalf("fight: target %+v enemies %+v", s.Target, s.Enemies)
	}

	send(w, 1, "client state off")
	bob.batches = nil
	p.Inventory = nil
	w.Tick()
	if _, ok := lastState(bob); ok {
		t.Fatal("state sent after off")
	}
}

func TestStateMapDetails(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.visited[3] = true
	send(w, 1, "client state")
	s, _ := lastState(bob)
	if s.Map == nil {
		t.Skip("test rooms are not placed on a map")
	}
	rooms := map[int]output.MapRoom{}
	for _, r := range s.Map.Rooms {
		rooms[r.Vnum] = r
	}
	if r := rooms[1]; r.Foes != 1 || r.Danger != 0 || !r.Items {
		t.Fatalf("own room: %+v", r)
	}
	if r := rooms[3]; r.Vnum == 3 && (r.Temple != "good" || r.Doors["west"] == "") {
		t.Fatalf("seen temple: %+v", r)
	}
}

func TestChatIsTyped(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	alice.batches = nil
	send(w, 1, "say hello")
	send(w, 1, "tell alice psst")
	var said, told bool
	for _, b := range alice.batches {
		for _, m := range b.Messages {
			if m.Type == output.Chat && strings.Contains(m.Text, "says") {
				said = true
			}
			if m.Type == output.Chat && strings.Contains(m.Text, "tells you") {
				d, ok := m.Data.(output.ChatData)
				told = ok && d.Tell && d.From == "Bob"
			}
		}
	}
	if !said || !told {
		t.Fatalf("say typed %v, tell typed with sender %v", said, told)
	}
	_ = bob
}

func TestStateCarriesEffectsAndGroup(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	login(t, w, 2, "Alice")
	send(w, 2, "follow bob")
	send(w, 1, "group alice")
	p := w.players[1]
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "attacks", Params: map[string]any{"amount": 1}}, Rounds: 5})
	send(w, 1, "client state")
	s, _ := lastState(bob)
	if len(s.Effects) != 1 || s.Effects[0].Rounds != 5 || s.Effects[0].Left == "" {
		t.Fatalf("effects: %+v", s.Effects)
	}
	if len(s.Group) != 1 || s.Group[0].Name != "Alice" || !s.Group[0].Here {
		t.Fatalf("group: %+v", s.Group)
	}
}

func TestDeathNamesTheCorpseRoom(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	send(w, 1, "north")
	bob.batches = nil
	w.die(p.Character, nil)
	w.Tick()
	for _, b := range bob.batches {
		for _, m := range b.Messages {
			if m.Type == output.Death {
				d := m.Data.(output.DeathData)
				if d.Room != 2 || d.Name != "North" || d.Rounds <= 0 || d.Seconds <= 0 {
					t.Fatalf("death: %+v", d)
				}
				return
			}
		}
	}
	t.Fatal("no death message")
}
