package script

import (
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type attackResult struct {
	Hit    bool   `json:"hit"`
	Damage int    `json:"damage"`
	Verb   string `json:"verb"`
}

func newEngine(t *testing.T, src string) (*Engine, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.js")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(dir, slog.New(slog.NewTextHandler(io.Discard, nil)), 100*time.Millisecond)
	if err := e.Load(); err != nil {
		t.Fatal(err)
	}
	return e, path
}

func TestCallExportsStruct(t *testing.T) {
	e, _ := newEngine(t, `function resolveAttack(a, d, w) { return { hit: a.level > d.level, damage: w ? w.damage * 2 : 1, verb: "hit" }; }`)
	var r attackResult
	err := e.Call("resolveAttack", &r,
		map[string]any{"level": 5}, map[string]any{"level": 2}, map[string]any{"damage": 3})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Hit || r.Damage != 6 || r.Verb != "hit" {
		t.Fatalf("result wrong: %+v", r)
	}
	var n int
	if err := e.Call("resolveAttack", &n, nil, nil, nil); err == nil {
		t.Fatal("exporting an object into an int should fail")
	}
}

func TestMissingHook(t *testing.T) {
	e, _ := newEngine(t, `function a() { return 1; }`)
	var n int
	err := e.Call("nope", &n)
	if !errors.Is(err, ErrNoHook) {
		t.Fatalf("expected ErrNoHook, got %v", err)
	}
	if !e.Has("a") || e.Has("nope") {
		t.Fatal("Has wrong")
	}
}

func TestBudgetInterrupts(t *testing.T) {
	e, _ := newEngine(t, `function spin() { while (true) {} } function fine() { return 7; }`)
	var n int
	start := time.Now()
	err := e.Call("spin", &n)
	if err == nil || !strings.Contains(err.Error(), "time budget") {
		t.Fatalf("expected budget error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("interrupt took too long")
	}
	// The runtime is usable afterwards.
	if err := e.Call("fine", &n); err != nil || n != 7 {
		t.Fatalf("runtime broken after interrupt: %v %d", err, n)
	}
}

func TestReloadOnChangeKeepsOldOnError(t *testing.T) {
	e, path := newEngine(t, `function dmg() { return 1; }`)
	var n int
	e.Call("dmg", &n)
	if n != 1 {
		t.Fatal("initial")
	}
	if changed, err := e.Reload(); changed || err != nil {
		t.Fatalf("unchanged file reported changed=%v err=%v", changed, err)
	}
	// mtime resolution can be coarse; force a distinct time.
	future := time.Now().Add(2 * time.Second)
	os.WriteFile(path, []byte(`function dmg() { return 5; }`), 0o644)
	os.Chtimes(path, future, future)
	if changed, err := e.Reload(); !changed || err != nil {
		t.Fatalf("expected reload, got changed=%v err=%v", changed, err)
	}
	e.Call("dmg", &n)
	if n != 5 {
		t.Fatalf("new value not live: %d", n)
	}
	later := future.Add(2 * time.Second)
	os.WriteFile(path, []byte(`function dmg() { return `), 0o644)
	os.Chtimes(path, later, later)
	if changed, err := e.Reload(); !changed || err == nil {
		t.Fatalf("syntax error should be reported: changed=%v err=%v", changed, err)
	}
	e.Call("dmg", &n)
	if n != 5 {
		t.Fatalf("old runtime not kept after bad reload: %d", n)
	}
	// The same broken file is not retried.
	if changed, _ := e.Reload(); changed {
		t.Fatal("broken file retried without a change")
	}
	fixed := later.Add(2 * time.Second)
	os.WriteFile(path, []byte(`function dmg() { return 9; }`), 0o644)
	os.Chtimes(path, fixed, fixed)
	if changed, err := e.Reload(); !changed || err != nil {
		t.Fatalf("fix not picked up: changed=%v err=%v", changed, err)
	}
	e.Call("dmg", &n)
	if n != 9 {
		t.Fatalf("fixed value not live: %d", n)
	}
}

func TestSeededRandom(t *testing.T) {
	e, _ := newEngine(t, `function r() { return random.int(1000) * 1000 + random.roll(2, 6); }`)
	var a, b int
	e.SetRandom(rand.New(rand.NewPCG(1, 2)))
	e.Call("r", &a)
	e.SetRandom(rand.New(rand.NewPCG(1, 2)))
	e.Call("r", &b)
	if a != b {
		t.Fatalf("seeded runs differ: %d %d", a, b)
	}
	if a%1000 < 2 || a%1000 > 12 {
		t.Fatalf("roll out of range: %d", a%1000)
	}
}

func TestMultipleFilesShareGlobals(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a_helpers.js"), []byte(`function twice(x) { return x * 2; }`), 0o644)
	os.WriteFile(filepath.Join(dir, "b_rules.js"), []byte(`function dmg() { return twice(4); }`), 0o644)
	e := New(dir, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	if err := e.Load(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := e.Call("dmg", &n); err != nil || n != 8 {
		t.Fatalf("cross-file call failed: %v %d", err, n)
	}
	if len(e.Files()) != 2 {
		t.Fatal("files not listed")
	}
}
