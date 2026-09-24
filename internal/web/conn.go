package web

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"urth/internal/output"
	"urth/internal/session"
	"urth/internal/telnet"
)

// outboundBuffer is how many batches may be pending before a slow browser is
// dropped.
const outboundBuffer = 256

const writeTimeout = 10 * time.Second

// wireMessage is one output.Message as sent to the browser: plain text for
// clients that want it, HTML with color spans for the bare page, and any
// structured data.
type wireMessage struct {
	Type output.Type `json:"type"`
	Text string      `json:"text"`
	HTML string      `json:"html"`
	Data any         `json:"data,omitempty"`
}

// conn is one browser tab. It implements session.Conn.
type conn struct {
	id        session.ID
	ws        *websocket.Conn
	remote    string
	out       chan []byte
	closeOnce sync.Once
	closed    chan struct{}
	reason    atomic.Pointer[string]
}

func newConn(id session.ID, ws *websocket.Conn, remote string) *conn {
	return &conn{
		id:     id,
		ws:     ws,
		remote: remote,
		out:    make(chan []byte, outboundBuffer),
		closed: make(chan struct{}),
	}
}

func (c *conn) ID() session.ID     { return c.id }
func (c *conn) RemoteAddr() string { return c.remote }

// Send renders a batch as a JSON array of wire messages and queues it.
func (c *conn) Send(b output.Batch) {
	msgs := make([]wireMessage, 0, len(b.Messages))
	for _, m := range b.Messages {
		msgs = append(msgs, wireMessage{
			Type: m.Type,
			Text: output.Strip(m.Text),
			HTML: output.HTML(m.Text),
			Data: m.Data,
		})
	}
	data, err := json.Marshal(msgs)
	if err != nil {
		return
	}
	select {
	case c.out <- data:
	case <-c.closed:
	default:
		c.closeWith("output overflow")
	}
}

// Close disconnects after pending output is flushed.
func (c *conn) Close() { c.closeWith("closed by server") }

func (c *conn) closeWith(reason string) {
	c.closeOnce.Do(func() {
		c.reason.Store(&reason)
		close(c.closed)
	})
}

// readLoop turns each text frame into one input line. It emits Connected
// first and Disconnected exactly once at the end.
func (s *Server) readLoop(c *conn) {
	defer s.wg.Done()
	s.events <- session.Connected{Conn: c}

	ctx := context.Background()
	for {
		typ, data, err := c.ws.Read(ctx)
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			continue
		}
		line := strings.TrimRight(string(data), "\r\n")
		if len(line) > telnet.MaxLineLen {
			line = line[:telnet.MaxLineLen]
		}
		select {
		case <-c.closed:
		default:
			s.events <- session.Input{ID: c.id, Line: line}
		}
	}

	reason := "client disconnected"
	if r := c.reason.Load(); r != nil {
		reason = *r
	}
	c.closeWith(reason)
	s.mu.Lock()
	delete(s.conns, c.id)
	s.mu.Unlock()
	s.events <- session.Disconnected{ID: c.id, Reason: reason}
}

// writeLoop owns the socket's close so a farewell queued just before Close
// still reaches the browser.
func (s *Server) writeLoop(c *conn) {
	defer s.wg.Done()
	defer func() {
		reason := "bye"
		if r := c.reason.Load(); r != nil {
			reason = *r
		}
		if reason == "closed by server" {
			// A player quit: do the clean close handshake so the browser sees
			// the reason. The library waits up to 5s for the peer's reply.
			_ = c.ws.Close(websocket.StatusNormalClosure, reason)
			return
		}
		// Shutdown or a broken peer: do not wait on the other side.
		_ = c.ws.CloseNow()
	}()
	for {
		select {
		case <-c.closed:
			for {
				select {
				case data := <-c.out:
					if !c.write(data) {
						return
					}
				default:
					return
				}
			}
		case data := <-c.out:
			if !c.write(data) {
				return
			}
		}
	}
}

func (c *conn) write(data []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
		c.closeWith("write failed")
		return false
	}
	return true
}
