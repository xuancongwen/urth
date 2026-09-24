// Package telnet is the line-oriented TCP transport. It accepts connections,
// strips telnet negotiation, and forwards lines to the world as session
// events. It never negotiates options and knows nothing about the game.
package telnet

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"urth/internal/output"
	"urth/internal/session"
)

// outboundBuffer is how many Send calls may be pending before a client is
// considered too slow and dropped. The world never blocks on a client.
const outboundBuffer = 256

// writeTimeout bounds a single write to a client.
const writeTimeout = 10 * time.Second

// Listener accepts TCP connections and turns them into sessions.
type Listener struct {
	addr   string
	events chan<- session.Event
	log    *slog.Logger
	wg     sync.WaitGroup

	mu    sync.Mutex
	conns map[session.ID]*conn
	ln    net.Listener
}

// NewListener creates a listener that reports to events.
func NewListener(addr string, events chan<- session.Event, log *slog.Logger) *Listener {
	return &Listener{addr: addr, events: events, log: log, conns: map[session.ID]*conn{}}
}

// Listen binds the address. Serve calls it if it has not been called; tests
// call it first so they can read Addr before connecting.
func (l *Listener) Listen() error {
	if l.ln != nil {
		return nil
	}
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}
	l.ln = ln
	l.log.Info("telnet listening", "addr", ln.Addr().String())
	return nil
}

// Addr is the bound address, valid after Listen.
func (l *Listener) Addr() net.Addr { return l.ln.Addr() }

// SetListener adopts an already-bound socket, used after a copyover.
func (l *Listener) SetListener(ln net.Listener) {
	l.ln = ln
	l.log.Info("telnet listening (inherited)", "addr", ln.Addr().String())
}

// File duplicates the listening socket for handoff to a new process.
func (l *Listener) File() (*os.File, error) {
	tl, ok := l.ln.(*net.TCPListener)
	if !ok {
		return nil, errors.New("listener is not TCP")
	}
	return tl.File()
}

// Adopt takes over an inherited client socket after a copyover and reports
// it to the world as Connected with restore information.
func (l *Listener) Adopt(nc net.Conn, restore *session.Restore) {
	c := newConn(session.NextID(), nc)
	l.mu.Lock()
	l.conns[c.id] = c
	l.mu.Unlock()
	l.wg.Add(2)
	go l.writeLoop(c)
	go l.readLoop(c, restore)
}

// Serve accepts connections until ctx is cancelled. It returns after all
// connection goroutines have exited.
func (l *Listener) Serve(ctx context.Context) error {
	if err := l.Listen(); err != nil {
		return err
	}
	ln := l.ln

	go func() {
		<-ctx.Done()
		ln.Close()
		// The world closes every player it knows about on shutdown, but a
		// client that connected in the last instant may not be known yet.
		l.mu.Lock()
		for _, c := range l.conns {
			c.closeWith("server shutdown")
		}
		l.mu.Unlock()
	}()

	for {
		nc, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			l.log.Warn("accept failed", "err", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if tc, ok := nc.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(2 * time.Minute)
			_ = tc.SetNoDelay(true)
		}
		c := newConn(session.NextID(), nc)
		l.mu.Lock()
		l.conns[c.id] = c
		l.mu.Unlock()
		l.wg.Add(2)
		go l.writeLoop(c)
		go l.readLoop(c, nil)
	}

	l.wg.Wait()
	l.log.Info("telnet stopped")
	return nil
}

// conn is one client. It implements session.Conn.
type conn struct {
	id        session.ID
	nc        net.Conn
	out       chan string
	closeOnce sync.Once
	closed    chan struct{}
	reason    atomic.Pointer[string]
}

func newConn(id session.ID, nc net.Conn) *conn {
	return &conn{
		id:     id,
		nc:     nc,
		out:    make(chan string, outboundBuffer),
		closed: make(chan struct{}),
	}
}

func (c *conn) ID() session.ID     { return c.id }
func (c *conn) RemoteAddr() string { return c.nc.RemoteAddr().String() }

// File duplicates the socket for copyover. Implements session.Filer.
func (c *conn) File() (*os.File, error) {
	tc, ok := c.nc.(*net.TCPConn)
	if !ok {
		return nil, errors.New("connection is not TCP")
	}
	return tc.File()
}

// Send renders a batch for a terminal and queues it without blocking. A
// client that has fallen outboundBuffer batches behind is dropped.
func (c *conn) Send(b output.Batch) {
	c.enqueue(output.RenderText(b))
}

func (c *conn) enqueue(text string) {
	select {
	case c.out <- text:
	case <-c.closed:
	default:
		c.closeWith("output overflow")
	}
}

// Close disconnects the client. Queued output is flushed first, then the
// socket is closed, which unblocks the read loop, which emits Disconnected.
func (c *conn) Close() { c.closeWith("closed by server") }

// closeWith records why we are closing and signals the write loop. It does
// not close the socket; writeLoop does that once pending output is flushed.
func (c *conn) closeWith(reason string) {
	c.closeOnce.Do(func() {
		c.reason.Store(&reason)
		close(c.closed)
	})
}

func (l *Listener) readLoop(c *conn, restore *session.Restore) {
	defer l.wg.Done()
	l.events <- session.Connected{Conn: c, Restore: restore}

	lr := NewLineReader(c.nc)
	for {
		line, err := lr.ReadLine()
		if err != nil && !errors.Is(err, ErrLineTooLong) {
			break
		}
		select {
		case <-c.closed:
			// Discard input typed after the server closed us.
		default:
			l.events <- session.Input{ID: c.id, Line: line}
		}
	}

	reason := "client disconnected"
	if r := c.reason.Load(); r != nil {
		reason = *r
	}
	c.closeWith(reason)
	l.mu.Lock()
	delete(l.conns, c.id)
	l.mu.Unlock()
	l.events <- session.Disconnected{ID: c.id, Reason: reason}
}

func (l *Listener) writeLoop(c *conn) {
	defer l.wg.Done()
	// Closing the socket is the write loop's job so that a farewell queued
	// just before Close still reaches the client.
	defer c.nc.Close()
	for {
		select {
		case <-c.closed:
			// Flush anything already queued, then let the deferred close
			// unblock the read loop.
			for {
				select {
				case text := <-c.out:
					if !c.write(text) {
						return
					}
				default:
					return
				}
			}
		case text := <-c.out:
			if !c.write(text) {
				return
			}
		}
	}
}

// write sends one message, translating "\n" to "\r\n" for telnet clients.
func (c *conn) write(text string) bool {
	_ = c.nc.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := c.nc.Write([]byte(strings.ReplaceAll(text, "\n", "\r\n"))); err != nil {
		c.closeWith("write failed")
		return false
	}
	return true
}
