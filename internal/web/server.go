// Package web is the WebSocket transport plus a bare browser client. It
// exists in milestone 3 to prove the world is transport-agnostic: the same
// session events flow in, and output batches are rendered to HTML instead
// of ANSI.
package web

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"urth/internal/limit"
	"urth/internal/session"
)

//go:embed static
var static embed.FS

// Server serves the client page at / and the game socket at /ws.
type Server struct {
	addr   string
	events chan<- session.Event
	log    *slog.Logger
	wg     sync.WaitGroup
	ln     net.Listener

	mu    sync.Mutex
	conns map[session.ID]*conn
	gate  *limit.Gate // nil admits everything
}

// SetLimits installs connection and input caps. Call before Serve.
func (s *Server) SetLimits(cfg limit.Config) { s.gate = limit.New(cfg) }

// NewServer creates a server reporting to events.
func NewServer(addr string, events chan<- session.Event, log *slog.Logger) *Server {
	return &Server{addr: addr, events: events, log: log, conns: map[session.ID]*conn{}}
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
	s.log.Info("web listening", "addr", ln.Addr().String(), "url", "http://"+ln.Addr().String()+"/")
	return nil
}

// Addr is the bound address, valid after Listen.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

// SetListener adopts an already-bound socket, used after a copyover.
func (s *Server) SetListener(ln net.Listener) {
	s.ln = ln
	s.log.Info("web listening (inherited)", "addr", ln.Addr().String())
}

// File duplicates the listening socket for handoff to a new process.
func (s *Server) File() (*os.File, error) {
	tl, ok := s.ln.(*net.TCPListener)
	if !ok {
		return nil, errors.New("listener is not TCP")
	}
	return tl.File()
}

// Serve runs until ctx is cancelled, then closes every socket and returns
// once all connection goroutines have exited.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	pages, err := fs.Sub(static, "static")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(pages)))
	mux.HandleFunc("/ws", s.handleWS)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(s.ln) }()

	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	// Close game sockets first so their handler goroutines return, then
	// stop the HTTP server.
	s.mu.Lock()
	for _, c := range s.conns {
		c.closeWith("server shutdown")
	}
	s.mu.Unlock()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	s.wg.Wait()
	s.log.Info("web stopped")
	return nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.gate.Admit(ip, false) {
		s.log.Warn("connection refused: limit", "remote", ip)
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.gate.Release(ip)
		s.log.Warn("websocket accept failed", "err", err, "remote", ip)
		return
	}
	c := newConn(session.NextID(), ws, ip)
	c.bucket = s.gate.NewBucket(time.Now())
	s.mu.Lock()
	s.conns[c.id] = c
	s.mu.Unlock()
	s.wg.Add(2)
	go s.writeLoop(c)
	s.readLoop(c, r.URL.Query().Get("token")) // runs on the HTTP handler goroutine
}

// clientIP is the address limits and logs are keyed by. A request from
// loopback is trusted to come from a proxy on this host (cloudflared, or
// a reverse proxy), and the proxy's header names the real client; any
// other request is keyed by its socket address so headers cannot be
// spoofed from the internet.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if h := r.Header.Get("CF-Connecting-IP"); h != "" {
			return h
		}
		if h := r.Header.Get("X-Forwarded-For"); h != "" {
			first, _, _ := strings.Cut(h, ",")
			return strings.TrimSpace(first)
		}
	}
	return host
}
