// Package session defines the contract between the world loop and any
// transport (TCP, WebSocket, in-memory test harness). The world sees a Conn
// and receives Events; it never touches sockets. See docs/DECISIONS.md D3.
package session

import (
	"os"
	"sync/atomic"

	"urth/internal/output"
)

// ID uniquely identifies a connection for the life of the process, across
// every transport.
type ID uint64

var lastID atomic.Uint64

// NextID allocates a process-unique ID. Transports call it per connection.
func NextID() ID { return ID(lastID.Add(1)) }

// Conn is a live client connection as seen by the world.
//
// Send must never block the caller. A transport that cannot keep up should
// drop the connection rather than stall the world loop.
type Conn interface {
	ID() ID
	RemoteAddr() string
	// Send queues one tick's worth of output. The transport renders it for
	// its client type.
	Send(batch output.Batch)
	// Close disconnects the client. It is safe to call more than once.
	// The transport must still emit a Disconnected event afterwards.
	Close()
}

// Event is something a transport tells the world about.
type Event interface{ isEvent() }

// Connected is emitted once when a client connects. After a copyover, a
// transport that adopted an inherited socket sets Restore so the world can
// re-attach the player without a login; a client reconnecting with a
// copyover token sets Token instead and the world resolves it.
type Connected struct {
	Conn    Conn
	Restore *Restore
	Token   string
}

// Restore identifies a player to re-attach after a copyover.
type Restore struct {
	Name string
	Room int
}

// Filer is implemented by connections whose socket can be handed to a new
// process. The returned file is a duplicate descriptor; the caller owns it.
type Filer interface {
	File() (*os.File, error)
}

// Disconnected is emitted exactly once when a client goes away, whether it
// hung up, errored, or was closed by the world.
type Disconnected struct {
	ID     ID
	Reason string
}

// Input is one line typed by the client, without the line terminator.
type Input struct {
	ID   ID
	Line string
}

func (Connected) isEvent()    {}
func (Disconnected) isEvent() {}
func (Input) isEvent()        {}
