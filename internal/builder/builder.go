// Package builder serves the builder page: every area drawn whole, what
// each room holds right now, the content check's findings, and a reload
// that fires on its own when a file under data/world changes. It is a
// window onto the same YAML files the editor writes, never a second
// editor (docs/DECISIONS.md D19). It has no login: bind it to loopback on
// the dev machine and leave it off in production.
package builder

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"urth/internal/content"
)

//go:embed static
var static embed.FS

// API is what the page needs from the world. Every call runs on the world
// goroutine; implementations block the caller until it has.
type API interface {
	// Snapshot is the overview: areas, findings, and the reload state.
	Snapshot() (*Snapshot, error)
	// Area is one area in full, with live contents.
	Area(name string) (*AreaView, error)
	// Reload re-reads one area, or the world when area is empty, and
	// returns the report the in-game command prints.
	Reload(area string) (string, error)
	// Generation counts successful content loads, including in-game
	// reloads, so the page knows when to refetch. It must be cheap.
	Generation() uint64
}

// Snapshot is the world overview.
type Snapshot struct {
	Areas      []AreaSummary     `json:"areas"`
	Problems   []content.Problem `json:"problems"`
	StartRoom  int               `json:"startRoom"`
	Generation uint64            `json:"generation"`
	LastReload *ReloadResult     `json:"lastReload,omitempty"`
}

// AreaSummary is one row of the area list.
type AreaSummary struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Detached bool   `json:"detached,omitempty"`
	Rooms    int    `json:"rooms"`
	Items    int    `json:"items"`
	Mobs     int    `json:"mobs"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// ReloadResult is the outcome of the last reload, by hand or by the
// watcher.
type ReloadResult struct {
	At     time.Time `json:"at"`
	OK     bool      `json:"ok"`
	Report string    `json:"report"`
	Source string    `json:"source"` // "watch" or "page"
}

// AreaView is one area in full.
type AreaView struct {
	Name     string            `json:"name"`
	Title    string            `json:"title"`
	Detached bool              `json:"detached,omitempty"`
	Rooms    []RoomView        `json:"rooms"`
	Items    []ProtoView       `json:"items"`
	Mobs     []ProtoView       `json:"mobs"`
	Resets   []ResetView       `json:"resets"`
	Problems []content.Problem `json:"problems"`
}

// RoomView is a room with its layout position and live contents.
type RoomView struct {
	Vnum        int            `json:"vnum"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	File        string         `json:"file"`
	X           int            `json:"x"`
	Y           int            `json:"y"`
	Z           int            `json:"z"`
	Placed      bool           `json:"placed"`
	Exits       map[string]int `json:"exits"`
	// Links describes exits that leave the area, by direction.
	Links   map[string]Link `json:"links,omitempty"`
	Flags   []string        `json:"flags,omitempty"`
	Temple  string          `json:"temple,omitempty"`
	Players []string        `json:"players,omitempty"`
	Mobs    []string        `json:"mobs,omitempty"`
	Items   []string        `json:"items,omitempty"`
}

// Link is the far end of an exit into another area.
type Link struct {
	Vnum int    `json:"vnum"`
	Name string `json:"name"`
	Area string `json:"area"`
}

// ProtoView is an item or mob prototype in a listing.
type ProtoView struct {
	Vnum  int    `json:"vnum"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Type  string `json:"type,omitempty"`
	Slot  string `json:"slot,omitempty"`
	File  string `json:"file"`
	// Live is how many instances exist in the world right now.
	Live int `json:"live"`
}

// ResetView is one reset line, with names resolved.
type ResetView struct {
	Index    int         `json:"index"`
	Room     int         `json:"room"`
	RoomName string      `json:"roomName"`
	Mob      int         `json:"mob,omitempty"`
	Item     int         `json:"item,omitempty"`
	Into     int         `json:"into,omitempty"`
	Name     string      `json:"name"`
	Max      int         `json:"max,omitempty"`
	Equip    []EquipView `json:"equip,omitempty"`
}

// EquipView is one item a reset gives a mob.
type EquipView struct {
	Item int    `json:"item"`
	Name string `json:"name"`
	Slot string `json:"slot,omitempty"`
}

// Server serves the page and its API on one address.
type Server struct {
	addr     string
	worldDir string
	api      API
	log      *slog.Logger
	ln       net.Listener

	mu   sync.Mutex
	last *ReloadResult
}

// NewServer creates a server for the world files under worldDir.
func NewServer(addr, worldDir string, api API, log *slog.Logger) *Server {
	return &Server{addr: addr, worldDir: worldDir, api: api, log: log}
}

// Listen binds the address; Serve calls it if needed.
func (s *Server) Listen() error {
	if s.ln != nil {
		return nil
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	s.log.Info("builder listening", "addr", ln.Addr().String(), "url", "http://"+ln.Addr().String()+"/")
	if host, _, err := net.SplitHostPort(ln.Addr().String()); err == nil {
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			s.log.Warn("builder page is reachable from other machines and has no login", "addr", ln.Addr().String())
		}
	}
	return nil
}

// Addr is the bound address, valid after Listen.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

// Handler is the page and API, for tests and for mounting elsewhere.
func (s *Server) Handler() http.Handler {
	pages, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(pages)))
	mux.HandleFunc("/api/world", s.handleWorld)
	mux.HandleFunc("/api/area", s.handleArea)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/api/reload", s.handleReload)
	mux.HandleFunc("/api/version", s.handleVersion)
	return mux
}

// Serve runs the page and the file watcher until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(s.ln) }()
	go s.watchFrom(ctx, s.signature())

	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	s.log.Info("builder stopped")
	return nil
}

// Watch settings: how often the tree is stat'ed, and how long a burst of
// saves is allowed to settle before one reload picks all of them up.
const (
	watchInterval = time.Second
	watchSettle   = 400 * time.Millisecond
)

// watchFrom polls the world tree for changes against last, the signature
// taken before it started. A poll stats every file, which for a few
// hundred files is well under a millisecond; no inotify dependency is
// worth that. Every change reloads the whole world, since a room can
// point at another area.
func (s *Server) watchFrom(ctx context.Context, last string) {
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		sig := s.signature()
		if sig == last {
			continue
		}
		// Wait for the burst to settle: an editor may write several files,
		// or a file twice (temp then rename).
		for {
			time.Sleep(watchSettle)
			next := s.signature()
			if next == sig {
				break
			}
			sig = next
		}
		last = sig
		s.reload("", "watch")
	}
}

// signature summarises the tree: one string that changes whenever any
// file is added, removed, touched, or resized.
func (s *Server) signature() string {
	var b strings.Builder
	_ = filepath.WalkDir(s.worldDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "%s:%d:%d;", path, info.ModTime().UnixNano(), info.Size())
		return nil
	})
	return b.String()
}

// reload asks the world to re-read content and records the outcome.
func (s *Server) reload(area, source string) *ReloadResult {
	report, err := s.api.Reload(area)
	res := &ReloadResult{At: time.Now(), OK: err == nil, Report: report, Source: source}
	if err != nil {
		res.Report = err.Error()
		s.log.Warn("builder reload failed", "area", area, "source", source, "err", err)
	} else {
		s.log.Info("builder reload", "area", area, "source", source)
	}
	s.mu.Lock()
	s.last = res
	s.mu.Unlock()
	return res
}

func (s *Server) lastReload() *ReloadResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleWorld(w http.ResponseWriter, r *http.Request) {
	snap, err := s.api.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	snap.LastReload = s.lastReload()
	writeJSON(w, snap)
}

func (s *Server) handleArea(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	view, err := s.api.Area(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, view)
}

// handleFile returns one content file as it is on disk, which is what
// the editor holds and may differ from what the world loaded.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	clean, ok := s.insideWorld(path)
	if !ok {
		http.Error(w, "not a content file", http.StatusBadRequest)
		return
	}
	raw, err := os.ReadFile(clean)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

// insideWorld resolves path and reports whether it is a YAML file under
// the world directory, so the page can never read anything else.
func (s *Server) insideWorld(path string) (string, bool) {
	if path == "" || !strings.HasSuffix(path, ".yaml") {
		return "", false
	}
	root, err := filepath.Abs(s.worldDir)
	if err != nil {
		return "", false
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs, err = filepath.Abs(abs)
		if err != nil {
			return "", false
		}
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return abs, true
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.reload(r.URL.Query().Get("area"), "page"))
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"generation": s.api.Generation(), "lastReload": s.lastReload()})
}
