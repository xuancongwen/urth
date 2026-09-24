package telnet

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func readAll(t *testing.T, input string) []string {
	t.Helper()
	lr := NewLineReader(strings.NewReader(input))
	var lines []string
	for {
		line, err := lr.ReadLine()
		if err == io.EOF {
			return lines
		}
		if err != nil && !errors.Is(err, ErrLineTooLong) {
			t.Fatalf("unexpected error: %v", err)
		}
		lines = append(lines, line)
	}
}

func TestPlainLines(t *testing.T) {
	got := readAll(t, "look\r\nnorth\n\nsay hi\r\n")
	want := []string{"look", "north", "", "say hi"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPartialLineAtEOF(t *testing.T) {
	got := readAll(t, "quit")
	if len(got) != 1 || got[0] != "quit" {
		t.Fatalf("got %q", got)
	}
}

func TestStripsNegotiation(t *testing.T) {
	// Mudlet-style opening: IAC DO TTYPE, IAC WILL NAWS, then a line, with an
	// IAC SB ... IAC SE block in the middle of it, and a NOP.
	input := string([]byte{iac, do, 24, iac, will, 31}) +
		"lo" + string([]byte{iac, sb, 31, 0, 80, 0, 24, iac, se}) + "ok" +
		string([]byte{iac, 241}) + "\r\n"
	got := readAll(t, input)
	if len(got) != 1 || got[0] != "look" {
		t.Fatalf("got %q", got)
	}
}

func TestEscapedIACDropped(t *testing.T) {
	got := readAll(t, "a"+string([]byte{iac, iac})+"b\n")
	if len(got) != 1 || got[0] != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestLineTooLong(t *testing.T) {
	long := strings.Repeat("x", MaxLineLen+50)
	lr := NewLineReader(strings.NewReader(long + "\nnext\n"))
	line, err := lr.ReadLine()
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
	if len(line) != MaxLineLen {
		t.Fatalf("expected truncation to %d, got %d", MaxLineLen, len(line))
	}
	line, err = lr.ReadLine()
	if err != nil || line != "next" {
		t.Fatalf("expected clean next line, got %q %v", line, err)
	}
}
