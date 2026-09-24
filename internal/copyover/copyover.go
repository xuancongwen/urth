// Package copyover restarts the server binary in place while keeping player
// sockets open. The running process writes a State describing every live
// socket, clears close-on-exec on those descriptors, and execs itself with
// -copyover pointing at the file. The new process adopts the descriptors and
// re-attaches players without a login. See docs/DECISIONS.md D8.
package copyover

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

// State is what survives the exec.
type State struct {
	// Listeners are bound sockets to adopt so no connection is refused
	// during the switch.
	Listeners []Listener `json:"listeners"`
	// Players are live sessions. FD is set for sockets that are inherited;
	// Token is set for clients that must reconnect (WebSocket).
	Players []Player `json:"players"`
}

// Listener is an inherited listening socket.
type Listener struct {
	Kind string `json:"kind"` // "telnet" or "web"
	FD   int    `json:"fd"`
}

// Player is one session to restore.
type Player struct {
	Kind  string `json:"kind"`
	FD    int    `json:"fd,omitempty"`
	Token string `json:"token,omitempty"`
	Name  string `json:"name"`
	Room  int    `json:"room"`
}

// Write saves the state file with owner-only permissions.
func Write(path string, st State) error {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Read loads and deletes the state file; it is single-use.
func Read(path string) (State, error) {
	var st State
	raw, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	_ = os.Remove(path)
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, fmt.Errorf("parse copyover state: %w", err)
	}
	return st, nil
}

// Inherit marks a descriptor to survive exec. Go opens everything
// close-on-exec, so this must be called for each socket being handed over.
func Inherit(fd int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_SETFD, 0)
	if errno != 0 {
		return fmt.Errorf("clear close-on-exec on fd %d: %v", fd, errno)
	}
	return nil
}

// Exec replaces the current process with a fresh copy of the binary. It only
// returns on failure.
func Exec(configPath, statePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{exe, "-config", configPath, "-copyover", statePath}
	return syscall.Exec(exe, args, os.Environ())
}
