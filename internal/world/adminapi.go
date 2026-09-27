package world

import (
	"errors"
	"fmt"
	"sort"

	"urth/internal/admin"
	"urth/internal/output"
	"urth/internal/store"
	"urth/internal/version"
)

// The admin page's view of the world. Player files are read on the
// page's goroutine, which is safe because the store writes by rename;
// everything live, and every change, runs on the world goroutine through
// the same call the builder page uses.

// adminBy is who the page's changes are logged as, and what an online
// character is told did it, when nobody signed in (loopback).
const adminBy = "An admin"

// adminActor is the name an action is done in: the signed-in admin, or
// adminBy.
func adminActor(by string) string {
	if by == "" {
		return adminBy
	}
	return by
}

type adminAPI struct{ b builderAPI }

// Admin returns the adapter the admin server drives.
func (w *World) Admin() admin.API { return adminAPI{builderAPI{w}} }

func (a adminAPI) w() *World { return a.b.w }

func (a adminAPI) Status() (*admin.Status, error) {
	count := a.w().store.Count()
	var st *admin.Status
	err := a.b.call(func() {
		w := a.w()
		st = &admin.Status{Name: w.cfg.Server.Name, Version: version.Version, Commit: version.Commit, Started: w.started,
			Characters: count, Connections: len(w.players), Areas: len(w.content.Rooms.Areas), Rooms: len(w.content.Rooms.Rooms),
			Items: len(w.content.Items), Mobs: len(w.content.Mobs), StartRoom: w.cfg.World.StartRoom, RespawnRoom: w.cfg.World.Respawn()}
		for _, p := range w.players {
			if p.State == StatePlaying {
				st.Playing++
			}
		}
	})
	return st, err
}

func (a adminAPI) Characters() ([]admin.CharacterRow, error) {
	names, err := a.w().store.List()
	if err != nil {
		return nil, err
	}
	recs := map[string]*store.Record{}
	failed := map[string]string{}
	for _, n := range names {
		rec, err := a.w().store.Load(n)
		if err != nil {
			failed[n] = err.Error()
			continue
		}
		recs[n] = rec
	}
	var rows []admin.CharacterRow
	err = a.b.call(func() {
		w := a.w()
		seen := map[string]bool{}
		for _, n := range names {
			seen[n] = true
			if msg, bad := failed[n]; bad {
				rows = append(rows, admin.CharacterRow{Name: n, Error: msg})
				continue
			}
			rows = append(rows, w.adminRow(recs[n]))
		}
		// Someone playing whose file is not listed yet.
		for _, p := range w.players {
			if p.State == StatePlaying && p.rec != nil && !seen[p.Name] {
				rows = append(rows, w.adminRow(p.rec))
			}
		}
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, err
}

// adminRow is the list line for a record, with live numbers for someone
// online.
func (w *World) adminRow(rec *store.Record) admin.CharacterRow {
	row := admin.CharacterRow{Name: rec.Name, Level: rec.Level, Admin: rec.Admin, Denied: rec.Denied,
		Room: rec.Room, Created: rec.Created, LastLogin: rec.LastLogin}
	if o := w.playingByName(rec.Name, nil); o != nil {
		row.Online, row.Level, row.Admin = true, o.Level, o.Admin
		row.Addr = o.conn.RemoteAddr()
		if o.Room != nil {
			row.Room = o.Room.Vnum
		}
	}
	if r, ok := w.content.Rooms.Get(row.Room); ok {
		row.RoomName, row.Area = r.Name, r.Area
	}
	return row
}

func (a adminAPI) Character(name string) (*admin.CharacterView, error) {
	name = canonicalName(name)
	if !store.ValidName(name) {
		return nil, admin.ErrNotFound
	}
	disk, derr := a.w().store.Load(name)
	var view *admin.CharacterView
	err := a.b.call(func() {
		w := a.w()
		o := w.playingByName(name, nil)
		rec := disk
		if o != nil && o.rec != nil {
			// Bring the record up to the minute without writing it.
			w.saveCharacterItems(o)
			w.saveSheet(o)
			rec = o.rec
		}
		if rec == nil {
			return
		}
		v := &admin.CharacterView{CharacterRow: w.adminRow(rec), Experience: rec.Experience, Stats: copyStats(rec.Stats),
			StatPoints: rec.StatPoints, FeatPoints: rec.FeatPoints, Silver: rec.Silver, Health: rec.Health, Mana: rec.Mana,
			Schools: rec.Schools, Deity: rec.Deity, QuestPoints: rec.QuestPoints, QuestWait: rec.QuestWait,
			Visited: len(rec.Visited), Effects: rec.Effects, Color: rec.Color,
			Inventory: w.adminItems(rec.Inventory), Equipment: []admin.ItemView{}}
		if o != nil {
			v.Health, v.HealthMax, v.Mana, v.ManaMax = o.Health, o.HealthMax, o.Mana, o.ManaMax
			v.Visited = len(o.visited)
			if o.Fighting != nil {
				v.Fighting = o.Fighting.Name
			}
		}
		slots := make([]string, 0, len(rec.Equipment))
		for slot := range rec.Equipment {
			slots = append(slots, slot)
		}
		sort.Strings(slots)
		for _, slot := range slots {
			it := w.adminItem(rec.Equipment[slot])
			it.Slot = slot
			v.Equipment = append(v.Equipment, it)
		}
		if q := rec.Quest; q != nil {
			qv := &admin.QuestView{SavedQuest: *q}
			if q.Kind == "kill" {
				if p, ok := w.content.Mobs[q.Target]; ok {
					qv.TargetName = p.Name
				}
			} else if p, ok := w.content.Items[q.Target]; ok {
				qv.TargetName = p.Name
			}
			if r, ok := w.content.Rooms.Get(q.Room); ok {
				qv.RoomName = r.Name
			}
			v.Quest = qv
		}
		view = v
	})
	if err != nil {
		return nil, err
	}
	if view == nil {
		if derr != nil && !errors.Is(derr, store.ErrNotFound) {
			return nil, derr
		}
		return nil, admin.ErrNotFound
	}
	return view, nil
}

func (w *World) adminItems(list []store.SavedItem) []admin.ItemView {
	out := []admin.ItemView{}
	for _, it := range list {
		out = append(out, w.adminItem(it))
	}
	return out
}

func (w *World) adminItem(it store.SavedItem) admin.ItemView {
	v := admin.ItemView{Vnum: it.Vnum, Name: "(no prototype)"}
	if p, ok := w.content.Items[it.Vnum]; ok {
		v.Name = p.Name
	}
	if len(it.Contents) > 0 {
		v.Contents = w.adminItems(it.Contents)
	}
	return v
}

func (a adminAPI) Connections() ([]admin.Connection, error) {
	var rows []admin.Connection
	err := a.b.call(func() {
		for _, o := range a.w().players {
			c := admin.Connection{ID: uint64(o.conn.ID()), Name: o.Name, Addr: o.conn.RemoteAddr()}
			switch o.State {
			case StatePlaying:
				c.State = "playing"
				if o.Room != nil {
					c.Room = o.Room.Vnum
				}
			case StateBusy:
				c.State = "busy"
			default:
				c.State = "login"
				if c.Name == "" {
					c.Name = o.pendingName
				}
			}
			rows = append(rows, c)
		}
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, err
}

func (a adminAPI) Act(by, action, name string) (string, error) {
	name = canonicalName(name)
	if by != "" && name == by && (action == "demote" || action == "deny" || action == "delete") {
		return "", fmt.Errorf("%w: you cannot %s yourself; have another admin do it", admin.ErrRefused, action)
	}
	who := adminActor(by)
	var msg string
	var err error
	cerr := a.b.call(func() {
		w := a.w()
		if !w.characterExists(name) {
			err = admin.ErrNotFound
			return
		}
		switch action {
		case "promote":
			msg, err = w.promote(name, who)
		case "demote":
			msg, err = w.demote(name, who)
		case "deny":
			msg, err = w.deny(name, who)
		case "allow":
			msg, err = w.allow(name, who)
		case "kick":
			o := w.playingByName(name, nil)
			if o == nil {
				err = fmt.Errorf("%w: %s is not online", admin.ErrRefused, name)
				return
			}
			w.save(o)
			o.SendMsg(output.Message{Type: output.System, Text: "{R}You have been disconnected by an admin.{x}\n"})
			o.disconnect()
			w.log.Warn("kicked", "name", name, "by", who)
			msg = name + " was disconnected."
		case "delete":
			if w.playingByName(name, nil) != nil {
				err = fmt.Errorf("%w: %s is online; kick them first", admin.ErrRefused, name)
				return
			}
			if err = w.store.Delete(name); err == nil {
				w.log.Warn("deleted", "name", name, "by", who)
				msg = name + " was retired to players/deleted/."
			}
		default:
			err = fmt.Errorf("%w: unknown action %q", admin.ErrRefused, action)
		}
	})
	if cerr != nil {
		return "", cerr
	}
	if errors.Is(err, errNoCharacter) {
		err = admin.ErrNotFound
	}
	return msg, err
}

func (a adminAPI) SetPassword(by, name, password string) (string, error) {
	name = canonicalName(name)
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return "", fmt.Errorf("%w: a password must be %d to %d characters", admin.ErrRefused, minPasswordLen, maxPasswordLen)
	}
	var exists bool
	if err := a.b.call(func() { exists = a.w().characterExists(name) }); err != nil {
		return "", err
	}
	if !exists {
		return "", admin.ErrNotFound
	}
	// bcrypt is slow on purpose: hash here, not on the world goroutine.
	hash, err := a.w().store.HashPassword(password)
	if err != nil {
		return "", err
	}
	var msg string
	cerr := a.b.call(func() { msg, err = a.w().setPasswordHash(name, hash, adminActor(by)) })
	if cerr != nil {
		return "", cerr
	}
	return msg, err
}

// Authenticate reads the file, not the live character: account changes
// save at once (editRecord), so the file is current.
func (a adminAPI) Authenticate(name, password string) (string, error) {
	name = canonicalName(name)
	rec, err := a.w().store.Load(name)
	if err != nil || !store.CheckPassword(rec.PasswordHash, password) || !rec.Admin || rec.Denied {
		return "", admin.ErrBadLogin
	}
	return rec.Name, nil
}

func (a adminAPI) IsAdmin(name string) bool {
	rec, err := a.w().store.Load(name)
	return err == nil && rec.Admin && !rec.Denied
}
