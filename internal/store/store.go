// Package store persists player records as one YAML file each under
// data/players/. A character is the account, as in ROM: the name and password
// hash live on the same record as the character's state.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"

	"urth/internal/effect"
)

// Record is everything saved about a player.
type Record struct {
	Name         string    `yaml:"name"`
	PasswordHash string    `yaml:"password_hash"`
	Admin        bool      `yaml:"admin"`
	Created      time.Time `yaml:"created"`
	LastLogin    time.Time `yaml:"last_login"`
	Room         int       `yaml:"room"`
	Color        bool      `yaml:"color"`
	// Sheet. Stats is free-form because the stat set is a rules decision.
	Level      int            `yaml:"level"`
	Experience int            `yaml:"experience"`
	Stats      map[string]int `yaml:"stats,omitempty"`
	StatPoints int            `yaml:"stat_points,omitempty"`
	FeatPoints int            `yaml:"feat_points,omitempty"`
	Silver     int            `yaml:"silver,omitempty"`
	// Magic access (docs/RULES.md 6.1): unlocked arcane schools and the
	// deity served, if any.
	Schools []string `yaml:"schools,omitempty"`
	Deity   string   `yaml:"deity,omitempty"`
	Health  int      `yaml:"health"`
	Mana    int      `yaml:"mana"`
	// Effects are the character's active effects: feats (permanent) and
	// anything timed that was running at save.
	Effects []effect.Active `yaml:"effects,omitempty"`
	// Inventory and Equipment are recreated from prototypes at login.
	Inventory []SavedItem          `yaml:"inventory,omitempty"`
	Equipment map[string]SavedItem `yaml:"equipment,omitempty"`
}

// SavedItem is an item instance on disk: its prototype and, for
// containers, what was inside.
type SavedItem struct {
	Vnum     int             `yaml:"vnum"`
	Contents []SavedItem     `yaml:"contents,omitempty"`
	Effects  []effect.Active `yaml:"effects,omitempty"`
	Burn     int             `yaml:"burn,omitempty"`
}

// ErrNotFound is returned by Load for an unknown name.
var ErrNotFound = errors.New("player not found")

// Store reads and writes records in a directory.
type Store struct {
	dir  string
	cost int
}

// New creates a store rooted at dir, creating it if needed. cost is the
// bcrypt cost; bcrypt.DefaultCost in production, bcrypt.MinCost in tests.
func New(dir string, cost int) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create player dir: %w", err)
	}
	return &Store{dir: dir, cost: cost}, nil
}

// ValidName reports whether a name is acceptable: 3 to 12 ASCII letters. It
// is also what keeps file names safe.
func ValidName(name string) bool {
	if len(name) < 3 || len(name) > 12 {
		return false
	}
	for _, r := range name {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// Canonical capitalises a name the way it is stored and displayed.
func Canonical(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, strings.ToLower(name)+".yaml")
}

// Exists reports whether a record for name is on disk.
func (s *Store) Exists(name string) bool {
	if !ValidName(name) {
		return false
	}
	_, err := os.Stat(s.path(name))
	return err == nil
}

// Count returns how many records exist.
func (s *Store) Count() int {
	matches, _ := filepath.Glob(filepath.Join(s.dir, "*.yaml"))
	return len(matches)
}

// Load reads a record by name.
func (s *Store) Load(name string) (*Record, error) {
	if !ValidName(name) {
		return nil, ErrNotFound
	}
	raw, err := os.ReadFile(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var rec Record
	if err := yaml.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path(name), err)
	}
	return &rec, nil
}

// Save writes a record atomically: to a temp file, then renamed into place,
// so a crash mid-write never leaves a truncated player file.
func (s *Store) Save(rec *Record) error {
	if !ValidName(rec.Name) {
		return fmt.Errorf("invalid name %q", rec.Name)
	}
	raw, err := yaml.Marshal(rec)
	if err != nil {
		return err
	}
	final := s.path(rec.Name)
	tmp, err := os.CreateTemp(s.dir, "."+strings.ToLower(rec.Name)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, final)
}

// HashPassword returns a bcrypt hash. It is slow by design; callers should
// not run it on the world goroutine.
func (s *Store) HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether password matches hash. Slow by design.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
