// Package session defines the contract between the world loop and any
// transport (TCP, WebSocket, in-memory test harness). The world sees a Conn
// and receives Events; it never touches sockets. See docs/DECISIONS.md D3.
package session

import (
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

// Connected is emitted once when a client connects.
type Connected struct{ Conn Conn }

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
