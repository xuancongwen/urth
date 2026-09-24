package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"urth/internal/output"
	"urth/internal/session"
)

func startServer(t *testing.T) (*Server, chan session.Event, context.CancelFunc, chan error) {
	t.Helper()
	events := make(chan session.Event, 16)
	s := NewServer("127.0.0.1:0", events, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	return s, events, cancel, done
}

func nextEvent(t *testing.T, events chan session.Event) session.Event {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func TestPageIsServed(t *testing.T) {
	s, _, cancel, _ := startServer(t)
	defer cancel()
	resp, err := http.Get("http://" + s.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !contains(body, "new WebSocket") {
		t.Fatalf("page not served: %d %q", resp.StatusCode, body[:min(len(body), 80)])
	}
}

func TestRoundTripAndFarewell(t *testing.T) {
	s, events, cancel, done := startServer(t)
	defer cancel()

	ctx, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelDial()
	ws, _, err := websocket.Dial(ctx, "ws://"+s.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()

	conn := nextEvent(t, events).(session.Connected).Conn

	if err := ws.Write(ctx, websocket.MessageText, []byte("look\n")); err != nil {
		t.Fatal(err)
	}
	in := nextEvent(t, events).(session.Input)
	if in.Line != "look" || in.ID != conn.ID() {
		t.Fatalf("input wrong: %+v", in)
	}

	conn.Send(output.Batch{Messages: []output.Message{
		{Type: output.Room, Text: "{C}Hub{x}\n", Data: output.RoomData{Vnum: 1, Name: "Hub", Exits: []string{"north"}, Players: []string{}}},
		{Type: output.Prompt, Text: "> "},
	}})
	conn.Close()

	_, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("read batch: %v", err)
	}
	var msgs []wireMessage
	if err := json.Unmarshal(data, &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Type != output.Room || msgs[0].Text != "Hub\n" ||
		msgs[0].HTML != `<span class="c-C">Hub</span>`+"\n" || msgs[1].Type != output.Prompt {
		t.Fatalf("wire batch wrong: %+v", msgs)
	}
	if rd, ok := msgs[0].Data.(map[string]any); !ok || rd["name"] != "Hub" {
		t.Fatalf("room data missing: %#v", msgs[0].Data)
	}

	if _, _, err := ws.Read(ctx); err == nil {
		t.Fatal("expected close after farewell")
	} else if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatalf("expected normal closure, got %v", err)
	}
	disc := nextEvent(t, events).(session.Disconnected)
	if disc.ID != conn.ID() || disc.Reason != "closed by server" {
		t.Fatalf("disconnect wrong: %+v", disc)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestShutdownClosesClients(t *testing.T) {
	s, events, cancel, done := startServer(t)
	ctx, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelDial()
	ws, _, err := websocket.Dial(ctx, "ws://"+s.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	nextEvent(t, events)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve hung with an open client")
	}
	if _, _, err := ws.Read(ctx); err == nil {
		t.Fatal("client should have been closed")
	}
	if disc := nextEvent(t, events).(session.Disconnected); disc.Reason != "server shutdown" {
		t.Fatalf("reason: %q", disc.Reason)
	}
}

func contains(b []byte, s string) bool {
	return len(b) > 0 && string(b) != "" && indexOf(string(b), s) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
