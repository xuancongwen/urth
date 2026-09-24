package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "players"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if s.Exists("Bob") || s.Count() != 0 {
		t.Fatal("empty store reports players")
	}
	hash, err := s.HashPassword("secret5")
	if err != nil {
		t.Fatal(err)
	}
	rec := &Record{Name: "Bob", PasswordHash: hash, Admin: true, Created: time.Now().UTC().Truncate(time.Second), Room: 7, Color: true}
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
	if !s.Exists("bob") || !s.Exists("BOB") || s.Count() != 1 {
		t.Fatal("saved record not found case-insensitively")
	}
	got, err := s.Load("bob")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Bob" || got.Room != 7 || !got.Admin || !got.Color || !got.Created.Equal(rec.Created) {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if !CheckPassword(got.PasswordHash, "secret5") || CheckPassword(got.PasswordHash, "wrong") {
		t.Fatal("password check wrong")
	}
	info, _ := os.Stat(filepath.Join(s.dir, "bob.yaml"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("player file mode %o, want 600", info.Mode().Perm())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(s.dir, "*.tmp")); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

func TestLoadMissing(t *testing.T) {
	s, _ := New(t.TempDir(), bcrypt.MinCost)
	if _, err := s.Load("Nobody"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestNamesAreValidated(t *testing.T) {
	s, _ := New(t.TempDir(), bcrypt.MinCost)
	for _, bad := range []string{"ab", "toolongofaname", "b0b", "../etc", "bob smith", ""} {
		if ValidName(bad) {
			t.Errorf("ValidName(%q) = true", bad)
		}
		if err := s.Save(&Record{Name: bad}); err == nil {
			t.Errorf("Save(%q) succeeded", bad)
		}
		if _, err := s.Load(bad); err != ErrNotFound {
			t.Errorf("Load(%q) err = %v", bad, err)
		}
	}
	if Canonical("bOB") != "Bob" || Canonical("  alice ") != "Alice" {
		t.Fatal("Canonical wrong")
	}
}
