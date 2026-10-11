package world

import (
	"time"

	"urth/internal/output"
	"urth/internal/session"
)

// Link-dead characters: a playing character whose connection drops without
// a quit stays in the world for world.linkdead_minutes, as in ROM, so a
// phone that slept or a dropped train connection does not lose the player
// their place. A web client resumes with the token it was given (a Session
// message); any client can sign in again with the password. Either way the
// same character is reattached to the new connection. If nobody returns in
// time, the character is saved and leaves.

// deadConn stands in for a link-dead character's connection: it keeps the
// old session ID and swallows output.
type deadConn struct{ id session.ID }

func (d deadConn) ID() session.ID   { return d.id }
func (deadConn) RemoteAddr() string { return "link-dead" }
func (deadConn) Send(output.Batch)  {}
func (deadConn) Close()             {}
func (p *Player) linkdead() bool    { _, ok := p.conn.(deadConn); return ok }
func (w *World) linkdeadGrace() time.Duration {
	return time.Duration(w.cfg.World.LinkdeadMinutes) * time.Minute
}

// goLinkdead keeps p in the world without a connection.
func (w *World) goLinkdead(p *Player, reason string) {
	p.conn = deadConn{id: p.conn.ID()}
	p.pending = nil
	p.input = nil
	p.linkdeadUntil = time.Now().Add(w.linkdeadGrace())
	w.save(p)
	w.act("$n has lost the link.", p.Character, nil, "", toRoom)
	w.log.Info("link-dead", "session", p.conn.ID(), "name", p.Name, "reason", reason)
}

// expireLinkdead removes the link-dead characters whose time is up.
func (w *World) expireLinkdead() {
	now := time.Now()
	for id, p := range w.players {
		if p.linkdead() && now.After(p.linkdeadUntil) {
			w.save(p)
			w.leave(p)
			w.forget(p)
			delete(w.players, id)
			w.log.Info("link-dead character left", "name", p.Name)
		}
	}
}

// issueToken gives p a fresh resume token, retiring the last one.
func (w *World) issueToken(p *Player) {
	w.forget(p)
	p.resumeToken = newToken()
	w.resume[p.resumeToken] = p
	p.SendMsg(output.Message{Type: output.Session, Data: output.SessionData{Token: p.resumeToken}})
}

// forget retires p's resume token.
func (w *World) forget(p *Player) {
	if p.resumeToken != "" {
		delete(w.resume, p.resumeToken)
		p.resumeToken = ""
	}
}

// reattach moves the character old to the connection of fresh, a session
// that has just resumed or signed in, and drops fresh. If old still has a
// live connection (signed in from a second device), that one is told and
// closed.
func (w *World) reattach(old, fresh *Player) {
	if !old.linkdead() {
		old.conn.Send(output.Batch{Messages: []output.Message{{Type: output.System,
			Text: "{R}This character has been reconnected from elsewhere.{x}\n"}}, Color: old.Color})
		old.conn.Close()
	}
	delete(w.players, old.conn.ID())
	delete(w.players, fresh.conn.ID())
	old.conn = fresh.conn
	old.pending, old.input = nil, nil
	old.closing = false
	old.lastState = ""
	w.players[old.conn.ID()] = old
	w.log.Info("reconnected", "session", old.conn.ID(), "name", old.Name, "addr", old.conn.RemoteAddr())
	old.Send("\nReconnecting.\n")
	w.act("$n has reconnected.", old.Character, nil, "", toRoom)
	w.sendCommands(old)
	w.issueToken(old)
	w.look(old)
}
