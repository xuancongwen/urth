package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/store"
)

// Account administration from inside the game: promote, demote, passwd,
// deny, allow, users. Each works on an online character or, failing that,
// on the record on disk, so an admin never has to wait for someone to log
// in. Nothing here is a game rule.

// editRecord applies fn to name's record and saves it. An online character
// is edited in place and saved through the world so the change survives
// autosave; an offline one is loaded, edited, and written back. The
// returned string names what was edited, for the reply.
func (w *World) editRecord(name string, fn func(rec *store.Record)) (online bool, err error) {
	if o := w.playingByName(name, nil); o != nil && o.rec != nil {
		fn(o.rec)
		o.Admin = o.rec.Admin
		w.save(o)
		return true, nil
	}
	rec, err := w.store.Load(name)
	if err != nil {
		return false, err
	}
	fn(rec)
	return false, w.store.Save(rec)
}

// oneName parses a command whose argument is a single character name.
func oneName(p *Player, args, syntax string) (string, bool) {
	name := canonicalName(args)
	if name == "" || strings.ContainsAny(name, " \t") {
		p.Send("Syntax: " + syntax + "\n")
		return "", false
	}
	if !store.ValidName(name) {
		p.Send("There is no character called that.\n")
		return "", false
	}
	return name, true
}

// errNoCharacter is the reply to an account change for a name with no
// record and nobody playing it.
var errNoCharacter = contentError("there is no character called that")

// The account changes below are shared by the in-game commands and the
// admin page. by names who asked, for the log and the notice the target
// sees; the reply is for the asker.

func (w *World) promote(name, by string) (string, error) {
	online, err := w.editRecord(name, func(rec *store.Record) { rec.Admin = true })
	if err != nil {
		return "", errNoCharacter
	}
	w.log.Warn("promoted", "name", name, "by", by)
	if o := w.playingByName(name, nil); online && o != nil {
		o.Send("{Y}" + output.Escape(by) + " has made you an admin.{x}\n")
		w.sendCommands(o)
	}
	return name + " is now an admin.", nil
}

func (w *World) demote(name, by string) (string, error) {
	online, err := w.editRecord(name, func(rec *store.Record) { rec.Admin = false })
	if err != nil {
		return "", errNoCharacter
	}
	w.log.Warn("demoted", "name", name, "by", by)
	if o := w.playingByName(name, nil); online && o != nil {
		o.Send("{Y}" + output.Escape(by) + " has removed your admin privileges.{x}\n")
		w.sendCommands(o)
	}
	return name + " is no longer an admin.", nil
}

// deny locks an account: its session is dropped and login refuses it
// until allow.
func (w *World) deny(name, by string) (string, error) {
	online, err := w.editRecord(name, func(rec *store.Record) { rec.Denied = true })
	if err != nil {
		return "", errNoCharacter
	}
	w.log.Warn("denied", "name", name, "by", by)
	if o := w.playingByName(name, nil); online && o != nil {
		o.SendMsg(output.Message{Type: output.System, Text: "{R}Your account has been denied.{x}\n"})
		o.disconnect()
	}
	return name + " is denied.", nil
}

func (w *World) allow(name, by string) (string, error) {
	if _, err := w.editRecord(name, func(rec *store.Record) { rec.Denied = false }); err != nil {
		return "", errNoCharacter
	}
	w.log.Warn("allowed", "name", name, "by", by)
	return name + " may log in again.", nil
}

// setPasswordHash stores a hash made off the world goroutine.
func (w *World) setPasswordHash(name, hash, by string) (string, error) {
	if _, err := w.editRecord(name, func(rec *store.Record) { rec.PasswordHash = hash }); err != nil {
		w.log.Error("passwd", "name", name, "err", err)
		return "", err
	}
	w.log.Warn("password reset", "name", name, "by", by)
	if o := w.playingByName(name, nil); o != nil {
		o.Send("{Y}" + output.Escape(by) + " has changed your password.{x}\n")
	}
	return "Password for " + name + " changed.", nil
}

// characterExists reports whether name has a record or is playing.
func (w *World) characterExists(name string) bool {
	return store.ValidName(name) && (w.playingByName(name, nil) != nil || w.store.Exists(name))
}

// reply sends the outcome of an account change to an admin in game.
func (p *Player) reply(msg string, err error) {
	if err != nil {
		if err == errNoCharacter {
			p.Send("There is no character called that.\n")
		} else {
			p.Send("Something went wrong. Nothing changed.\n")
		}
		return
	}
	p.Send(output.Escape(msg) + "\n")
}

func cmdPromote(w *World, p *Player, args string) {
	if name, ok := oneName(p, args, "promote <player>"); ok {
		p.reply(w.promote(name, p.Name))
	}
}

func cmdDemote(w *World, p *Player, args string) {
	name, ok := oneName(p, args, "demote <player>")
	if !ok {
		return
	}
	if name == p.Name {
		p.Send("You cannot demote yourself; have another admin do it, or use the urth admin command on the host.\n")
		return
	}
	p.reply(w.demote(name, p.Name))
}

// cmdPasswd sets another character's password: passwd <player> <new>.
// The hash runs off the world goroutine, as password does.
func cmdPasswd(w *World, p *Player, args string) {
	first, rest, _ := strings.Cut(strings.TrimSpace(args), " ")
	newPw := strings.TrimSpace(rest)
	name := canonicalName(first)
	if name == "" || newPw == "" {
		p.Send("Syntax: passwd <player> <new password>\n")
		return
	}
	if !w.characterExists(name) {
		p.Send("There is no character called that.\n")
		return
	}
	if len(newPw) < minPasswordLen || len(newPw) > maxPasswordLen {
		p.Send("Password must be 5 to 64 characters.\n")
		return
	}
	go func() {
		hash, err := w.store.HashPassword(newPw)
		w.post(func() {
			if !w.stillConnected(p) {
				return
			}
			msg := ""
			if err == nil {
				msg, err = w.setPasswordHash(name, hash, p.Name)
			}
			if err != nil {
				p.Send("Something went wrong. Nothing changed.\n")
				return
			}
			p.Send(output.Escape(msg) + "\n")
		})
	}()
}

func cmdDeny(w *World, p *Player, args string) {
	name, ok := oneName(p, args, "deny <player>")
	if !ok {
		return
	}
	if name == p.Name {
		p.Send("You cannot deny yourself.\n")
		return
	}
	p.reply(w.deny(name, p.Name))
}

func cmdAllow(w *World, p *Player, args string) {
	if name, ok := oneName(p, args, "allow <player>"); ok {
		p.reply(w.allow(name, p.Name))
	}
}

// cmdUsers lists every connection, including those still at the login
// prompts, with its address: what who leaves out.
func cmdUsers(w *World, p *Player, _ string) {
	type row struct{ name, state, addr string }
	var rows []row
	for _, o := range w.players {
		r := row{name: o.Name, addr: o.conn.RemoteAddr()}
		switch o.State {
		case StatePlaying:
			r.state = "playing"
			if o.Admin {
				r.state = "admin"
			}
		case StateBusy:
			r.state = "busy"
		default:
			r.state = "login"
		}
		if r.name == "" {
			r.name = "(" + o.pendingName + ")"
			if o.pendingName == "" {
				r.name = "-"
			}
		}
		rows = append(rows, r)
	}
	sortStringsBy(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	var b strings.Builder
	b.WriteString(padRight("Name", 14) + padRight("State", 9) + "Address\n")
	for _, r := range rows {
		b.WriteString(padRight(output.Escape(r.name), 14) + padRight(r.state, 9) + output.Escape(r.addr) + "\n")
	}
	b.WriteString(plural(len(rows), "connection") + ".\n")
	p.Send(b.String())
}
