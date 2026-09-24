package telnet

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"urth/internal/output"
	"urth/internal/session"
)

func text(s string) output.Batch {
	return output.Batch{Messages: []output.Message{{Type: output.Text, Text: s}}}
}

func startListener(t *testing.T) (*Listener, chan session.Event, context.CancelFunc, chan error) {
	t.Helper()
	events := make(chan session.Event, 16)
	l := NewListener("127.0.0.1:0", events, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := l.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- l.Serve(ctx) }()
	return l, events, cancel, done
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

func TestFarewellIsFlushedBeforeClose(t *testing.T) {
	l, events, cancel, done := startListener(t)
	defer cancel()

	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	connected, ok := nextEvent(t, events).(session.Connected)
	if !ok {
		t.Fatal("expected Connected first")
	}
	conn := connected.Conn

	if _, err := client.Write([]byte("quit\r\n")); err != nil {
		t.Fatal(err)
	}
	input, ok := nextEvent(t, events).(session.Input)
	if !ok || input.Line != "quit" || input.ID != conn.ID() {
		t.Fatalf("expected Input quit, got %#v", input)
	}

	// The world's quit path: farewell, then Close, back to back.
	conn.Send(text("Alas, all good things must come to an end.\n"))
	conn.Close()

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := io.ReadAll(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("read after close: %v", err)
	}
	if string(got) != "Alas, all good things must come to an end.\r\n" {
		t.Fatalf("farewell lost or mangled: %q", got)
	}

	disc, ok := nextEvent(t, events).(session.Disconnected)
	if !ok || disc.ID != conn.ID() || disc.Reason != "closed by server" {
		t.Fatalf("expected Disconnected closed by server, got %#v", disc)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}

func TestShutdownClosesUnknownClients(t *testing.T) {
	l, events, cancel, done := startListener(t)

	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	nextEvent(t, events) // Connected; the world never acts on it.

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve hung with an open client")
	}
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("expected EOF for straggler, got %v", err)
	}
	disc, ok := nextEvent(t, events).(session.Disconnected)
	if !ok || disc.Reason != "server shutdown" {
		t.Fatalf("expected server shutdown disconnect, got %#v", disc)
	}
}

func TestSlowClientIsDropped(t *testing.T) {
	l, events, cancel, done := startListener(t)
	defer cancel()

	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	conn := nextEvent(t, events).(session.Connected).Conn

	// Never read on the client side; overflow the outbound queue. The kernel
	// buffers some, so push well past the queue depth. Send must never block.
	finished := make(chan struct{})
	go func() {
		for i := 0; i < outboundBuffer*200; i++ {
			conn.Send(text("spam spam spam spam spam spam spam spam spam spam spam spam spam\n"))
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("Send blocked the caller")
	}
	disc, ok := nextEvent(t, events).(session.Disconnected)
	if !ok {
		t.Fatalf("expected Disconnected, got %#v", disc)
	}
	if disc.Reason != "output overflow" && disc.Reason != "write failed" {
		t.Fatalf("unexpected reason %q", disc.Reason)
	}
	cancel()
	<-done
}
