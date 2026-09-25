package world

import (
	"strings"
	"time"

	"urth/internal/output"
	"urth/internal/store"
)

// The login state machine, after ROM's nanny(). Password work runs off the
// world goroutine; while it is in flight the session is StateBusy and input
// is ignored.

const (
	minPasswordLen   = 5
	maxPasswordLen   = 64
	maxPasswordTries = 3
)

func (w *World) handleLogin(p *Player, line string) {
	switch p.State {
	case StateGetName:
		w.loginName(p, line)
	case StateGetOldPassword:
		w.loginOldPassword(p, line)
	case StateConfirmNewName:
		w.loginConfirmName(p, line)
	case StateGetNewPassword:
		w.loginNewPassword(p, line)
	case StateConfirmNewPassword:
		w.loginConfirmPassword(p, line)
	case StateBusy:
		// Ignore input while a password check or hash is in flight.
	}
}

func (w *World) loginName(p *Player, line string) {
	name := store.Canonical(line)
	if !store.ValidName(name) {
		p.SendPrompt("Names are 3 to 12 letters. Try again: ")
		return
	}
	p.pendingName = name
	if w.store.Exists(name) {
		p.State = StateGetOldPassword
		p.SendMsg(output.Message{Type: output.EchoOff})
		p.SendPrompt("Password: ")
		return
	}
	p.State = StateConfirmNewName
	p.SendPrompt("Did I get that right, " + name + " (Y/N)? ")
}

func (w *World) loginOldPassword(p *Player, line string) {
	name := p.pendingName
	p.State = StateBusy
	p.SendMsg(output.Message{Type: output.EchoOn})
	p.Send("\n")
	go func() {
		rec, err := w.store.Load(name)
		ok := err == nil && store.CheckPassword(rec.PasswordHash, line)
		w.post(func() {
			if !w.stillConnected(p) {
				return
			}
			if !ok {
				p.passwordTries++
				w.log.Warn("wrong password", "name", name, "addr", p.conn.RemoteAddr(), "tries", p.passwordTries)
				if p.passwordTries >= maxPasswordTries {
					p.Send("Wrong password. Goodbye.\n")
					p.disconnect()
					return
				}
				p.State = StateGetOldPassword
				p.Send("Wrong password.\n")
				p.SendMsg(output.Message{Type: output.EchoOff})
				p.SendPrompt("Password: ")
				return
			}
			if rec.Denied {
				w.log.Warn("denied login", "name", name, "addr", p.conn.RemoteAddr())
				p.Send("Your account has been denied. Goodbye.\n")
				p.disconnect()
				return
			}
			w.enterGame(p, rec)
		})
	}()
}

func (w *World) loginConfirmName(p *Player, line string) {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		p.State = StateGetNewPassword
		p.SendMsg(output.Message{Type: output.EchoOff})
		p.SendPrompt("New character.\nGive me a password for " + p.pendingName + ": ")
	case "n", "no":
		p.State = StateGetName
		p.pendingName = ""
		p.SendPrompt("Ok, what IS it, then? ")
	default:
		p.SendPrompt("Please type Yes or No? ")
	}
}

func (w *World) loginNewPassword(p *Player, line string) {
	p.Send("\n")
	if len(line) < minPasswordLen || len(line) > maxPasswordLen {
		p.SendPrompt("Password must be 5 to 64 characters.\nPassword: ")
		return
	}
	p.pendingPassword = line
	p.State = StateConfirmNewPassword
	p.SendPrompt("Please retype password: ")
}

func (w *World) loginConfirmPassword(p *Player, line string) {
	p.Send("\n")
	if line != p.pendingPassword {
		p.State = StateGetNewPassword
		p.pendingPassword = ""
		p.SendPrompt("Passwords don't match.\nRetype password: ")
		return
	}
	password := p.pendingPassword
	name := p.pendingName
	p.pendingPassword = ""
	p.State = StateBusy
	p.SendMsg(output.Message{Type: output.EchoOn})
	go func() {
		hash, err := w.store.HashPassword(password)
		w.post(func() {
			if !w.stillConnected(p) {
				return
			}
			if err != nil {
				w.log.Error("hash password", "err", err)
				p.SendMsg(output.Message{Type: output.System, Text: "Something went wrong creating your character.\n"})
				p.disconnect()
				return
			}
			// The name may have been taken while we were hashing.
			if w.store.Exists(name) || w.playingByName(name, p) != nil {
				p.State = StateGetName
				p.SendPrompt("That name was just taken. By what name do you wish to be known? ")
				return
			}
			now := time.Now().UTC()
			rec := &store.Record{
				Name:         name,
				PasswordHash: hash,
				Admin:        w.cfg.World.FirstPlayerIsAdmin && w.store.Count() == 0,
				Created:      now,
				LastLogin:    now,
				Room:         w.cfg.World.StartRoom,
				Color:        true,
			}
			if err := w.store.Save(rec); err != nil {
				w.log.Error("save new player", "name", name, "err", err)
				p.SendMsg(output.Message{Type: output.System, Text: "Something went wrong creating your character.\n"})
				p.disconnect()
				return
			}
			w.log.Info("new character", "name", name, "admin", rec.Admin)
			if rec.Admin {
				p.Send("{Y}You are the first character on this server and have been made an admin.{x}\n")
			}
			w.enterGame(p, rec)
		})
	}()
}

// enterGame attaches a loaded record to the session and puts it in the
// world. If the character is already playing, that session is taken over.
func (w *World) enterGame(p *Player, rec *store.Record) {
	p.rec = rec
	p.setName(rec.Name)
	p.Admin = rec.Admin
	p.Color = rec.Color
	p.State = StatePlaying
	p.pendingName = ""
	rec.LastLogin = time.Now().UTC()

	if old := w.playingByName(rec.Name, p); old != nil {
		// Reconnect: take over the existing character in place, including
		// whatever it is carrying right now.
		p.Room = old.Room
		p.Inventory = old.Inventory
		p.Equipment = old.Equipment
		p.Level, p.Experience, p.Stats = old.Level, old.Experience, old.Stats
		p.Health, p.Mana = old.Health, old.Mana
		w.recalc(p.Character)
		for _, o := range w.allCharacters() {
			if o.Fighting == old.Character {
				o.Fighting = p.Character
			}
		}
		p.Fighting = old.Fighting
		old.Room = nil // suppress "has left the game"
		old.Inventory = nil
		old.Equipment = nil
		old.SendMsg(output.Message{Type: output.System, Text: "{R}This character has been reconnected from elsewhere.{x}\n"})
		old.disconnect()
		w.log.Info("reconnected", "session", p.conn.ID(), "name", p.Name, "old_session", old.conn.ID())
		p.Send("\nReconnecting.\n")
		w.act("$n has reconnected.", p.Character, nil, "", toRoom)
		w.sendCommands(p)
		w.look(p)
		w.save(p)
		return
	}
	w.loadCharacterItems(p)
	w.loadSheet(p)
	w.resumeQuest(p)

	room, ok := w.content.Rooms.Get(rec.Room)
	if !ok {
		room, ok = w.content.Rooms.Get(w.cfg.World.StartRoom)
		if !ok {
			p.SendMsg(output.Message{Type: output.System, Text: "The world has no start room. Try again later.\n"})
			p.disconnect()
			return
		}
	}
	p.Room = room
	w.log.Info("logged in", "session", p.conn.ID(), "name", p.Name, "addr", p.conn.RemoteAddr())
	p.Send("\nWelcome, " + p.Name + ".\n\n")
	w.act("$n has entered the game.", p.Character, nil, "", toRoom)
	w.sendCommands(p)
	w.look(p)
	w.save(p)
}

// restore re-attaches a player after a copyover without a login.
func (w *World) restore(p *Player, r Restore) {
	rec, err := w.store.Load(r.Name)
	if err != nil {
		w.log.Warn("copyover restore failed, falling back to login", "name", r.Name, "err", err)
		w.greet(p)
		return
	}
	if rec.Denied {
		p.Send("Your account has been denied. Goodbye.\n")
		p.disconnect()
		return
	}
	p.rec = rec
	p.setName(rec.Name)
	p.Admin = rec.Admin
	p.Color = rec.Color
	p.State = StatePlaying
	w.loadCharacterItems(p)
	w.loadSheet(p)
	w.resumeQuest(p)
	room, ok := w.content.Rooms.Get(r.Room)
	if !ok {
		room, ok = w.content.Rooms.Get(rec.Room)
		if !ok {
			room, _ = w.content.Rooms.Get(w.cfg.World.StartRoom)
		}
	}
	p.Room = room
	w.log.Info("restored", "session", p.conn.ID(), "name", p.Name)
	w.sendCommands(p)
	p.Send("{G}Copyover complete.{x}\n")
}

func (w *World) greet(p *Player) {
	p.State = StateGetName
	p.SendMsg(output.Message{Type: output.System, Text: w.banner})
	p.Send("Welcome to {C}" + output.Escape(w.cfg.Server.Name) + "{x}.\n")
	p.SendPrompt("By what name do you wish to be known? ")
}

// stillConnected reports whether p is still the session the world knows
// under its ID, for callbacks that outlive a disconnect.
func (w *World) stillConnected(p *Player) bool {
	cur, ok := w.players[p.conn.ID()]
	return ok && cur == p
}

// playingByName finds an in-game character by name, ignoring except.
func (w *World) playingByName(name string, except *Player) *Player {
	for _, o := range w.players {
		if o != except && o.State == StatePlaying && o.Name == name {
			return o
		}
	}
	return nil
}

// save writes a playing character to disk.
func (w *World) save(p *Player) {
	if p.State != StatePlaying || p.rec == nil {
		return
	}
	p.rec.Color = p.Color
	p.rec.Admin = p.Admin
	if p.Room != nil {
		p.rec.Room = p.Room.Vnum
	}
	w.saveCharacterItems(p)
	w.saveSheet(p)
	if err := w.store.Save(p.rec); err != nil {
		w.log.Error("save player", "name", p.Name, "err", err)
	}
}

// saveAll writes every playing character.
func (w *World) saveAll() int {
	n := 0
	for _, p := range w.players {
		if p.State == StatePlaying {
			w.save(p)
			n++
		}
	}
	return n
}
