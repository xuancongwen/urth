// Package admin serves the admin page: server status, every character on
// disk with who is online, one character in full, live connections, and
// the account actions the in-game admin commands have (promote, demote,
// deny, allow, passwd) plus kick and delete. From loopback (this machine
// or an ssh tunnel) it needs no login; from any other address, such as
// the LAN, it needs a sign-in as an admin character (auth.go). Every
// request must name an address, localhost, or a .local name as its Host,
// which turns away DNS rebinding, and every change must carry the page's
// own header, which a cross-site form cannot.
package admin

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"urth/internal/effect"
	"urth/internal/store"
)

//go:embed static
var static embed.FS

// API is what the page needs from the world. Implementations run world
// state reads and changes on the world goroutine.
type API interface {
	Status() (*Status, error)
	Characters() ([]CharacterRow, error)
	Character(name string) (*CharacterView, error)
	Connections() ([]Connection, error)
	// Act runs one account action on a character by name and returns the
	// reply: promote, demote, deny, allow, kick, delete. by is the
	// signed-in admin, or empty from loopback without a sign-in.
	Act(by, action, name string) (string, error)
	// SetPassword replaces a character's password.
	SetPassword(by, name, password string) (string, error)
	// Authenticate checks an admin character's name and game password and
	// returns the name as stored. It is slow on purpose (bcrypt).
	Authenticate(name, password string) (string, error)
	// IsAdmin reports whether name is an admin and not denied, now.
	IsAdmin(name string) bool
}

// ErrNotFound is returned for a name with no character.
var ErrNotFound = errors.New("there is no character called that")

// ErrRefused is wrapped by an action that cannot be done as asked, such
// as deleting a character who is online.
var ErrRefused = errors.New("refused")

// Status is the header of the page.
type Status struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Commit      string    `json:"commit"`
	Started     time.Time `json:"started"`
	Playing     int       `json:"playing"`
	Connections int       `json:"connections"`
	Characters  int       `json:"characters"`
	Areas       int       `json:"areas"`
	Rooms       int       `json:"rooms"`
	Items       int       `json:"items"`
	Mobs        int       `json:"mobs"`
	StartRoom   int       `json:"startRoom"`
	RespawnRoom int       `json:"respawnRoom"`
}

// CharacterRow is one line of the character list. For someone online the
// numbers are live; otherwise they are the file's.
type CharacterRow struct {
	Name      string    `json:"name"`
	Level     int       `json:"level"`
	Admin     bool      `json:"admin,omitempty"`
	Denied    bool      `json:"denied,omitempty"`
	Online    bool      `json:"online,omitempty"`
	Addr      string    `json:"addr,omitempty"`
	Room      int       `json:"room"`
	RoomName  string    `json:"roomName,omitempty"`
	Area      string    `json:"area,omitempty"`
	Created   time.Time `json:"created"`
	LastLogin time.Time `json:"lastLogin"`
	// Error is set when the file could not be read.
	Error string `json:"error,omitempty"`
}

// CharacterView is one character in full, minus the password hash.
type CharacterView struct {
	CharacterRow
	Experience  int             `json:"experience"`
	Stats       map[string]int  `json:"stats,omitempty"`
	StatPoints  int             `json:"statPoints,omitempty"`
	FeatPoints  int             `json:"featPoints,omitempty"`
	Silver      int             `json:"silver"`
	Health      int             `json:"health"`
	HealthMax   int             `json:"healthMax,omitempty"`
	Mana        int             `json:"mana"`
	ManaMax     int             `json:"manaMax,omitempty"`
	Fighting    string          `json:"fighting,omitempty"`
	Schools     []string        `json:"schools,omitempty"`
	Deity       string          `json:"deity,omitempty"`
	QuestPoints int             `json:"questPoints,omitempty"`
	QuestWait   int             `json:"questWait,omitempty"`
	Quest       *QuestView      `json:"quest,omitempty"`
	Visited     int             `json:"visited"`
	Effects     []effect.Active `json:"effects,omitempty"`
	Inventory   []ItemView      `json:"inventory"`
	Equipment   []ItemView      `json:"equipment"`
	Color       bool            `json:"color"`
}

// QuestView is a quest under way, with names resolved.
type QuestView struct {
	store.SavedQuest
	TargetName string `json:"targetName"`
	RoomName   string `json:"roomName"`
}

// ItemView is a carried or worn item. Slot is set for equipment.
type ItemView struct {
	Vnum     int        `json:"vnum"`
	Name     string     `json:"name"`
	Slot     string     `json:"slot,omitempty"`
	Contents []ItemView `json:"contents,omitempty"`
}

// Connection is one live session, including those still logging in.
type Connection struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Addr  string `json:"addr"`
	Room  int    `json:"room,omitempty"`
}

// Server serves the page and its API.
type Server struct {
	addr string
	api  API
	log  *slog.Logger
	ln   net.Listener
	auth *auth
}

// NewServer creates a server on addr.
func NewServer(addr string, api API, log *slog.Logger) *Server {
	return &Server{addr: addr, api: api, log: log, auth: newAuth()}
}

// Listen binds the address.
func (s *Server) Listen() error {
	if s.ln != nil {
		return nil
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	s.log.Info("admin listening", "addr", ln.Addr().String(), "url", "http://"+ln.Addr().String()+"/")
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok && !tcp.IP.IsLoopback() {
		s.log.Warn("admin page is reachable from other machines: they must sign in as an admin character, over plain HTTP", "addr", ln.Addr().String())
	}
	return nil
}

// Addr is the bound address, valid after Listen.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

// Handler is the page and API behind the Host check.
func (s *Server) Handler() http.Handler {
	pages, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(pages)))
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/status", s.authed(s.handleStatus))
	mux.HandleFunc("/api/characters", s.authed(s.handleCharacters))
	mux.HandleFunc("/api/character", s.authed(s.handleCharacter))
	mux.HandleFunc("/api/connections", s.authed(s.handleConnections))
	mux.HandleFunc("/api/act", s.authed(s.handleAct))
	mux.HandleFunc("/api/password", s.authed(s.handlePassword))
	return hostGuard(mux)
}

// Serve runs until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(s.ln) }()
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
	s.log.Info("admin stopped")
	return nil
}

// hostGuard refuses a request whose Host is a DNS name other than
// localhost or a .local (mDNS) name. A page on another site that rebinds
// its own name to this machine would otherwise be same-origin with this
// one; a .local name cannot be registered in public DNS, so it is safe.
func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if host != "localhost" && !strings.HasSuffix(host, ".local") && net.ParseIP(host) == nil {
			http.Error(w, "the admin page answers only on an address, localhost, or a .local name", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// change admits a POST carrying the page's header.
func change(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return false
	}
	if r.Header.Get("X-Urth-Admin") == "" {
		http.Error(w, "missing X-Urth-Admin header", http.StatusForbidden)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// result is the answer to a change.
type result struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusServiceUnavailable
	switch {
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrRefused):
		code = http.StatusConflict
	}
	writeJSON(w, code, result{Error: err.Error()})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, _ string) {
	st, err := s.api.Status()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleCharacters(w http.ResponseWriter, r *http.Request, _ string) {
	rows, err := s.api.Characters()
	if err != nil {
		writeErr(w, err)
		return
	}
	if rows == nil {
		rows = []CharacterRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleCharacter(w http.ResponseWriter, r *http.Request, _ string) {
	v, err := s.api.Character(r.URL.Query().Get("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request, _ string) {
	rows, err := s.api.Connections()
	if err != nil {
		writeErr(w, err)
		return
	}
	if rows == nil {
		rows = []Connection{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// actions are what /api/act accepts.
var actions = map[string]bool{"promote": true, "demote": true, "deny": true, "allow": true, "kick": true, "delete": true}

func (s *Server) handleAct(w http.ResponseWriter, r *http.Request, by string) {
	if !change(w, r) {
		return
	}
	var req struct {
		Action string `json:"action"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || !actions[req.Action] {
		writeJSON(w, http.StatusBadRequest, result{Error: "want {action, name} with action one of promote, demote, deny, allow, kick, delete"})
		return
	}
	msg, err := s.api.Act(by, req.Action, req.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.log.Warn("admin page action", "action", req.Action, "name", req.Name, "by", by)
	writeJSON(w, http.StatusOK, result{OK: true, Message: msg})
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request, by string) {
	if !change(w, r) {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, result{Error: "want {name, password}"})
		return
	}
	msg, err := s.api.SetPassword(by, req.Name, req.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result{OK: true, Message: msg})
}
