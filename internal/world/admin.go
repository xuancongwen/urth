package world

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"urth/internal/copyover"
	"urth/internal/output"
	"urth/internal/session"
)

// Admin commands. Nothing here is a game rule.

func cmdShutdown(w *World, p *Player, _ string) {
	if w.deps.Shutdown == nil {
		p.Send("Shutdown is not available.\n")
		return
	}
	w.log.Warn("shutdown requested", "by", p.Name)
	w.broadcast("{R}Shutdown by " + output.Escape(p.Name) + ".{x}\n")
	w.deps.Shutdown()
}

// cmdCopyover saves everyone, hands every socket it can to a fresh copy of
// the binary, and execs it. Clients whose socket cannot be inherited
// (WebSocket) are given a token to reconnect with. Sessions still logging
// in are closed.
func cmdCopyover(w *World, p *Player, _ string) {
	if w.deps.Copyover == nil {
		p.Send("Copyover is not available.\n")
		return
	}
	w.log.Warn("copyover requested", "by", p.Name)

	var st copyover.State
	if w.deps.Listeners != nil {
		st.Listeners = w.deps.Listeners()
	}
	var files []*os.File
	for _, o := range w.players {
		if o.State != StatePlaying {
			o.SendMsg(output.Message{Type: output.System, Text: "\nThe server is restarting. Please reconnect in a moment.\n"})
			o.disconnect()
			continue
		}
		entry := copyover.Player{Name: o.Name, Room: 0}
		if o.Room != nil {
			entry.Room = o.Room.Vnum
		}
		if f, ok := o.conn.(session.Filer); ok {
			file, err := f.File()
			if err == nil {
				err = copyover.Inherit(int(file.Fd()))
			}
			if err != nil {
				w.log.Error("copyover: cannot hand over socket", "name", o.Name, "err", err)
				o.SendMsg(output.Message{Type: output.System, Text: "\nThe server is restarting. Please reconnect in a moment.\n"})
				o.disconnect()
				continue
			}
			files = append(files, file)
			entry.Kind = "telnet"
			entry.FD = int(file.Fd())
		} else {
			entry.Kind = "web"
			entry.Token = newToken()
			o.SendMsg(output.Message{Type: output.Reconnect, Data: output.ReconnectData{Token: entry.Token}})
		}
		st.Players = append(st.Players, entry)
	}

	w.saveAll()
	w.broadcast("{Y}Copyover by " + output.Escape(p.Name) + ". Please hold...{x}\n")
	w.flush()

	err := w.deps.Copyover(st)
	// Only reached on failure.
	for _, f := range files {
		f.Close()
	}
	w.log.Error("copyover failed", "err", err)
	if w.stillConnected(p) {
		p.Send("{R}Copyover failed: " + output.Escape(err.Error()) + "{x}\n")
	}
	w.broadcast("{Y}Copyover failed; carrying on.{x}\n")
}

func newToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
