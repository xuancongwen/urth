package admin

// Signing in. A request from loopback (this machine, or an ssh tunnel to
// it) needs no login, as before. A request from any other address must
// carry a session made by signing in as an admin character, with that
// character's game password. Sessions live in memory, so a restart signs
// everyone out; each request re-checks that the character is still an
// admin and not denied, so a demotion ends its sessions at once.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// ErrBadLogin is the one answer to a failed sign-in, whatever failed.
var ErrBadLogin = errors.New("wrong name or password, or not an admin")

const (
	sessionCookie = "urth_admin"
	sessionLife   = 12 * time.Hour
	// A remote address that fails this many sign-ins in failWindow waits
	// failLockout before it may try again.
	maxFails    = 5
	failWindow  = 10 * time.Minute
	failLockout = time.Minute
)

type session struct {
	name    string
	expires time.Time
}

type fails struct {
	count int
	first time.Time
	until time.Time
}

type auth struct {
	mu       sync.Mutex
	sessions map[string]*session
	fails    map[string]*fails
	now      func() time.Time
}

func newAuth() *auth {
	return &auth{sessions: map[string]*session{}, fails: map[string]*fails{}, now: time.Now}
}

// remoteIP is the address the request came from, without the port.
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// fromLoopback reports whether the request came from this machine.
func fromLoopback(r *http.Request) bool {
	ip := net.ParseIP(remoteIP(r))
	return ip != nil && ip.IsLoopback()
}

// signedIn returns the character a request's session belongs to, if it
// is live.
func (a *auth) signedIn(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[c.Value]
	if !ok {
		return "", false
	}
	if a.now().After(s.expires) {
		delete(a.sessions, c.Value)
		return "", false
	}
	s.expires = a.now().Add(sessionLife)
	return s.name, true
}

func (a *auth) start(name string) string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(b[:])
	a.mu.Lock()
	defer a.mu.Unlock()
	// Drop expired sessions while here.
	for t, s := range a.sessions {
		if a.now().After(s.expires) {
			delete(a.sessions, t)
		}
	}
	a.sessions[token] = &session{name: name, expires: a.now().Add(sessionLife)}
	return token
}

func (a *auth) end(r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
}

// locked reports whether ip must wait before trying again.
func (a *auth) locked(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.fails[ip]
	return f != nil && a.now().Before(f.until)
}

func (a *auth) failed(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	f := a.fails[ip]
	if f == nil || now.Sub(f.first) > failWindow {
		f = &fails{first: now}
		a.fails[ip] = f
	}
	f.count++
	if f.count >= maxFails {
		f.until = now.Add(failLockout)
		f.count, f.first = 0, now
	}
}

func (a *auth) succeeded(ip string) {
	a.mu.Lock()
	delete(a.fails, ip)
	a.mu.Unlock()
}

// actor is who a request acts as: the signed-in admin, or, from
// loopback without a session, the page itself. ok is false when the
// request must sign in first.
func (s *Server) actor(r *http.Request) (by string, ok bool) {
	if name, live := s.auth.signedIn(r); live {
		if s.api.IsAdmin(name) {
			return name, true
		}
		s.auth.end(r)
	}
	if fromLoopback(r) {
		return "", true
	}
	return "", false
}

// authed wraps an API handler with the sign-in check.
func (s *Server) authed(h func(w http.ResponseWriter, r *http.Request, by string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		by, ok := s.actor(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, result{Error: "sign in with an admin character"})
			return
		}
		h(w, r, by)
	}
}

// handleSession says whether this browser must sign in, and as whom it
// is signed in.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	by, ok := s.actor(r)
	writeJSON(w, http.StatusOK, map[string]any{"needLogin": !ok, "name": by, "local": fromLoopback(r)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !change(w, r) {
		return
	}
	ip := remoteIP(r)
	if s.auth.locked(ip) {
		writeJSON(w, http.StatusTooManyRequests, result{Error: "too many failed sign-ins; wait a minute"})
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
	name, err := s.api.Authenticate(req.Name, req.Password)
	if err != nil {
		s.auth.failed(ip)
		s.log.Warn("admin sign-in failed", "name", req.Name, "addr", ip)
		writeJSON(w, http.StatusUnauthorized, result{Error: ErrBadLogin.Error()})
		return
	}
	s.auth.succeeded(ip)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: s.auth.start(name), Path: "/",
		MaxAge: int(sessionLife.Seconds()), HttpOnly: true, SameSite: http.SameSiteStrictMode})
	s.log.Info("admin signed in", "name", name, "addr", ip)
	writeJSON(w, http.StatusOK, result{OK: true, Message: name})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !change(w, r) {
		return
	}
	s.auth.end(r)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, result{OK: true})
}
