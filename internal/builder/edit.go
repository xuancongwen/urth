package builder

// Editing: the page writes the same YAML files an editor would, in the
// checkout it runs against (docs/DECISIONS.md D20). Every change is loaded
// in a scratch copy of the world first, so a save that would not load is
// refused (unless forced) instead of leaving a broken file on disk. After
// a write the world reloads at once and the watcher is told, so it does
// not reload a second time for the same change.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"urth/internal/content"
	"urth/internal/room"
)

// maxFileBytes bounds one saved file.
const maxFileBytes = 1 << 20

// EditResult is the answer to every change the page makes.
type EditResult struct {
	OK bool `json:"ok"`
	// Error is why nothing was written. Invalid is set when the reason is
	// that the world would not load, which a forced save may override.
	Error   string `json:"error,omitempty"`
	Invalid bool   `json:"invalid,omitempty"`
	// Hash is the saved file's new content hash, for the next save.
	Hash string `json:"hash,omitempty"`
	// Vnum is the room a dig created, or the prototype a place used.
	Vnum int `json:"vnum,omitempty"`
	// Created is a new prototype's file, for the page to open.
	Created string `json:"created,omitempty"`
	// Files lists what was written or removed.
	Files []string `json:"files,omitempty"`
	// Warnings are map layout conflicts the change introduced.
	Warnings []string      `json:"warnings,omitempty"`
	Reload   *ReloadResult `json:"reload,omitempty"`
}

// errInvalid marks a change the world would not load.
type errInvalid struct{ err error }

func (e errInvalid) Error() string { return e.err.Error() }

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}

// guard admits a change: the right method, the page's own header (a
// cross-origin form or fetch cannot send it without a preflight this
// server never answers), and a Host that is an address or localhost, so a
// rebound DNS name pointed at loopback is turned away.
func guard(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, method+" only", http.StatusMethodNotAllowed)
		return false
	}
	if r.Header.Get("X-Urth-Builder") == "" {
		http.Error(w, "missing X-Urth-Builder header", http.StatusForbidden)
		return false
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host != "localhost" && net.ParseIP(host) == nil {
		http.Error(w, "the builder answers changes only on an address or localhost", http.StatusForbidden)
		return false
	}
	return true
}

// editable reports whether abs is a file the page may write: area.yaml or
// resets.yaml in an area directory, or one entity under rooms, items, or
// mobs.
func (s *Server) editable(abs string) bool {
	root, err := filepath.Abs(s.worldDir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	switch len(parts) {
	case 2:
		return parts[1] == "area.yaml" || parts[1] == "resets.yaml"
	case 3:
		switch parts[1] {
		case "rooms", "items", "mobs":
			return strings.HasSuffix(parts[2], ".yaml") && !strings.HasPrefix(parts[2], ".")
		}
	}
	return false
}

// handleSave writes one file: PUT /api/file?path=...[&force=1], body the
// new content, If-Match the hash the page last read ("new" to create).
func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if !guard(w, r, http.MethodPut) {
		return
	}
	path, ok := s.insideWorld(r.URL.Query().Get("path"))
	if !ok || !s.editable(path) {
		http.Error(w, "not an editable content file", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFileBytes))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	s.edit.Lock()
	defer s.edit.Unlock()

	base := r.Header.Get("If-Match")
	current, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if base != "new" {
			writeEdit(w, http.StatusConflict, &EditResult{Error: "the file no longer exists on disk"})
			return
		}
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			writeEdit(w, http.StatusBadRequest, &EditResult{Error: "no directory " + filepath.Dir(path)})
			return
		}
	case err != nil:
		writeEdit(w, http.StatusInternalServerError, &EditResult{Error: err.Error()})
		return
	case base != hashOf(current):
		writeEdit(w, http.StatusConflict, &EditResult{Error: "the file changed on disk since it was opened"})
		return
	}
	res, err := s.apply(map[string][]byte{path: body}, r.URL.Query().Get("force") != "", s.resetsArea(path))
	if err != nil {
		writeEditErr(w, err)
		return
	}
	res.Hash = hashOf(body)
	writeEdit(w, http.StatusOK, res)
}

// DigRequest makes an exit from a room: to a new room when To is zero
// (numbered Vnum, or the next free number in the area), or to an existing
// room, with the way back added on the far side either way.
type DigRequest struct {
	From        int    `json:"from"`
	Dir         string `json:"dir"`
	To          int    `json:"to,omitempty"`
	Vnum        int    `json:"vnum,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// OneWay leaves the far side alone.
	OneWay bool `json:"oneWay,omitempty"`
}

func (s *Server) handleDig(w http.ResponseWriter, r *http.Request) {
	if !guard(w, r, http.MethodPost) {
		return
	}
	var req DigRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.edit.Lock()
	defer s.edit.Unlock()
	res, err := s.dig(req)
	if err != nil {
		writeEditErr(w, err)
		return
	}
	writeEdit(w, http.StatusOK, res)
}

func (s *Server) dig(req DigRequest) (*EditResult, error) {
	disk, err := s.loadDisk()
	if err != nil {
		return nil, err
	}
	from, ok := disk.Rooms.Get(req.From)
	if !ok {
		return nil, fmt.Errorf("no room %d on disk", req.From)
	}
	back := room.Opposite[req.Dir]
	if back == "" {
		return nil, fmt.Errorf("%q is not a direction", req.Dir)
	}
	if to, taken := from.Exits[req.Dir]; taken {
		return nil, fmt.Errorf("room %d already has an exit %s, to %d", from.Vnum, req.Dir, to)
	}
	changes := map[string][]byte{}
	edit := func(r *room.Room, fn func(*yaml.Node) error) error {
		src, ok := changes[r.File]
		if !ok {
			if src, err = os.ReadFile(r.File); err != nil {
				return err
			}
		}
		out, err := editYAML(src, fn)
		if err != nil {
			return fmt.Errorf("%s: %w", r.File, err)
		}
		changes[r.File] = out
		return nil
	}

	var vnum int
	if req.To != 0 {
		to, ok := disk.Rooms.Get(req.To)
		if !ok {
			return nil, fmt.Errorf("no room %d on disk", req.To)
		}
		vnum = to.Vnum
		if err := edit(from, setExit(req.Dir, to.Vnum)); err != nil {
			return nil, err
		}
		if !req.OneWay {
			if other, taken := to.Exits[back]; taken && other != from.Vnum {
				return nil, fmt.Errorf("room %d already has an exit %s, to %d; unlink it first or make this one way", to.Vnum, back, other)
			}
			if err := edit(to, setExit(back, from.Vnum)); err != nil {
				return nil, err
			}
		}
	} else {
		vnum = req.Vnum
		if vnum == 0 {
			if vnum, err = nextVnum(disk, from.Area, "rooms"); err != nil {
				return nil, err
			}
		}
		if _, taken := disk.Rooms.Get(vnum); taken || vnum <= 0 {
			return nil, fmt.Errorf("room vnum %d is taken", vnum)
		}
		file := filepath.Join(filepath.Dir(from.File), strconv.Itoa(vnum)+".yaml")
		if _, err := os.Stat(file); err == nil {
			return nil, fmt.Errorf("%s already exists", file)
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "An Unfinished Room"
		}
		exits := map[string]int{}
		if !req.OneWay {
			exits[back] = from.Vnum
		}
		changes[file] = newRoomYAML(vnum, name, req.Description, exits)
		if err := edit(from, setExit(req.Dir, vnum)); err != nil {
			return nil, err
		}
	}
	res, err := s.apply(changes, false, "")
	if err != nil {
		return nil, err
	}
	res.Vnum = vnum
	return res, nil
}

// UnlinkRequest removes the exit Dir from room From, and its door, and the
// way back from the far room if it leads here.
type UnlinkRequest struct {
	From int    `json:"from"`
	Dir  string `json:"dir"`
}

func (s *Server) handleUnlink(w http.ResponseWriter, r *http.Request) {
	if !guard(w, r, http.MethodPost) {
		return
	}
	var req UnlinkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.edit.Lock()
	defer s.edit.Unlock()
	res, err := s.unlink(req)
	if err != nil {
		writeEditErr(w, err)
		return
	}
	writeEdit(w, http.StatusOK, res)
}

func (s *Server) unlink(req UnlinkRequest) (*EditResult, error) {
	disk, err := s.loadDisk()
	if err != nil {
		return nil, err
	}
	from, ok := disk.Rooms.Get(req.From)
	if !ok {
		return nil, fmt.Errorf("no room %d on disk", req.From)
	}
	toVnum, ok := from.Exits[req.Dir]
	if !ok {
		return nil, fmt.Errorf("room %d has no exit %s", from.Vnum, req.Dir)
	}
	changes := map[string][]byte{}
	src, err := os.ReadFile(from.File)
	if err != nil {
		return nil, err
	}
	if changes[from.File], err = editYAML(src, removeExit(req.Dir)); err != nil {
		return nil, err
	}
	back := room.Opposite[req.Dir]
	if to, ok := disk.Rooms.Get(toVnum); ok && to.Vnum != from.Vnum && to.Exits[back] == from.Vnum {
		src, err := os.ReadFile(to.File)
		if err != nil {
			return nil, err
		}
		if changes[to.File], err = editYAML(src, removeExit(back)); err != nil {
			return nil, err
		}
	}
	return s.apply(changes, false, "")
}

// DeleteRequest removes a room's file, every exit that leads to it, and
// every reset that places something in it.
type DeleteRequest struct {
	Vnum int `json:"vnum"`
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !guard(w, r, http.MethodPost) {
		return
	}
	var req DeleteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.edit.Lock()
	defer s.edit.Unlock()
	res, err := s.deleteRoom(req.Vnum)
	if err != nil {
		writeEditErr(w, err)
		return
	}
	writeEdit(w, http.StatusOK, res)
}

func (s *Server) deleteRoom(vnum int) (*EditResult, error) {
	disk, err := s.loadDisk()
	if err != nil {
		return nil, err
	}
	target, ok := disk.Rooms.Get(vnum)
	if !ok {
		return nil, fmt.Errorf("no room %d on disk", vnum)
	}
	if snap, err := s.api.Snapshot(); err == nil && snap.StartRoom == vnum {
		return nil, fmt.Errorf("room %d is the start room", vnum)
	}
	changes := map[string][]byte{target.File: nil}
	// Resets that place things in the room go with it.
	for area, a := range disk.Resets {
		var drop []int
		for i, rs := range a.Resets {
			if rs.Room == vnum {
				drop = append(drop, i)
			}
		}
		if len(drop) == 0 {
			continue
		}
		file := filepath.Join(s.worldDir, area, "resets.yaml")
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if src, err = removeResets(src, drop); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		changes[file] = src
	}
	for _, r := range disk.Rooms.Rooms {
		if r.Vnum == vnum {
			continue
		}
		var dirs []string
		for dir, to := range r.Exits {
			if to == vnum {
				dirs = append(dirs, dir)
			}
		}
		if len(dirs) == 0 {
			continue
		}
		src, err := os.ReadFile(r.File)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			if src, err = editYAML(src, removeExit(dir)); err != nil {
				return nil, fmt.Errorf("%s: %w", r.File, err)
			}
		}
		changes[r.File] = src
	}
	return s.apply(changes, false, "")
}

// loadDisk loads the world as the files stand now, which is what an edit
// changes; it can differ from what the world has loaded.
func (s *Server) loadDisk() (*content.World, error) {
	w, err := content.Load(s.worldDir)
	if err != nil {
		return nil, fmt.Errorf("the files on disk do not load, so fix that first: %w", err)
	}
	return w, nil
}

// apply checks changes (path to new content, nil to remove) against a
// scratch copy of the world, then writes them and reloads. force skips
// the check. A non-empty area has its resets run at once, so something
// just placed shows up.
func (s *Server) apply(changes map[string][]byte, force bool, area string) (*EditResult, error) {
	res := &EditResult{}
	if !force {
		before, _ := content.Load(s.worldDir)
		after, err := s.dryRun(changes)
		if err != nil {
			return nil, errInvalid{err}
		}
		known := map[string]bool{}
		if before != nil {
			for _, w := range before.Rooms.Warnings {
				known[w] = true
			}
		}
		for _, w := range after.Rooms.Warnings {
			if !known[w] {
				res.Warnings = append(res.Warnings, w)
			}
		}
	}
	paths := make([]string, 0, len(changes))
	for p := range changes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		var err error
		if changes[p] == nil {
			err = os.Remove(p)
		} else {
			err = writeAtomic(p, changes[p])
		}
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, p)
	}
	res.Reload = s.reload(area, "page")
	s.setSeen(s.signature())
	res.OK = true
	return res, nil
}

// dryRun loads the world with changes laid over a scratch copy of it.
// Error text names the real files, not the copies.
func (s *Server) dryRun(changes map[string][]byte) (*content.World, error) {
	root, err := filepath.Abs(s.worldDir)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "urth-builder-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	over := map[string][]byte{}
	for p, b := range changes {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return nil, err
		}
		over[rel] = b
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		dst := filepath.Join(tmp, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if _, changed := over[rel]; changed || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, raw, 0o644)
	})
	if err != nil {
		return nil, err
	}
	for rel, b := range over {
		if b == nil {
			continue
		}
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return nil, err
		}
	}
	w, err := content.Load(tmp)
	if err != nil {
		return nil, errors.New(strings.ReplaceAll(err.Error(), tmp, s.worldDir))
	}
	return w, nil
}

// writeAtomic replaces path through a temporary file in the same
// directory, so the watcher and the world never read half a file.
func writeAtomic(path string, b []byte) error {
	// An area's first item or mob makes its directory.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".urth-*.tmp")
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}

// nextVnum is the vnum a new thing of kind gets in area: the next free
// one in the area's block (content.Blocks), or an error when the block is
// full.
func nextVnum(w *content.World, area, kind string) (int, error) {
	b := content.Blocks(w, area)[kind]
	if b.Next == 0 {
		return 0, fmt.Errorf("area %s has no free %s vnums left in %d to %d", area, strings.TrimSuffix(kind, "s"), b.First, b.Last)
	}
	return b.Next, nil
}

// newRoomYAML is the file for a new room, laid out like the hand-written
// ones.
func newRoomYAML(vnum int, name, desc string, exits map[string]int) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "vnum: %d\n", vnum)
	nameNode := yaml.Node{Kind: yaml.ScalarNode, Value: name}
	enc, _ := yaml.Marshal(&nameNode)
	fmt.Fprintf(&b, "name: %s", enc)
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = "Nothing has been written here yet."
	}
	b.WriteString("description: |\n")
	for _, line := range strings.Split(desc, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n")
		} else {
			b.WriteString("  " + strings.TrimRight(line, " \t") + "\n")
		}
	}
	if len(exits) == 0 {
		b.WriteString("exits: {}\n")
		return b.Bytes()
	}
	b.WriteString("exits:\n")
	for _, dir := range room.Directions {
		if to, ok := exits[dir]; ok {
			fmt.Fprintf(&b, "  %s: %d\n", dir, to)
		}
	}
	return b.Bytes()
}

// editYAML parses src, lets fn change the document, and writes it back
// with two-space indent. yaml.v3 keeps comments, key order, and styles, so
// only the edited lines differ.
func editYAML(src []byte, fn func(doc *yaml.Node) error) ([]byte, error) {
	var file yaml.Node
	if err := yaml.Unmarshal(src, &file); err != nil {
		return nil, err
	}
	if file.Kind != yaml.DocumentNode || len(file.Content) == 0 || file.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("not a YAML mapping")
	}
	if err := fn(file.Content[0]); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(&file); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// mapGet returns the value under key in a mapping node, and its index.
func mapGet(m *yaml.Node, key string) (*yaml.Node, int) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], i
		}
	}
	return nil, -1
}

func mapDelete(m *yaml.Node, key string) {
	if _, i := mapGet(m, key); i >= 0 {
		m.Content = append(m.Content[:i], m.Content[i+2:]...)
	}
}

// setExit adds or repoints an exit, keeping the exits in display order
// when it adds one.
func setExit(dir string, to int) func(*yaml.Node) error {
	return func(doc *yaml.Node) error {
		exits, _ := mapGet(doc, "exits")
		if exits == nil || exits.Kind != yaml.MappingNode {
			exits = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			mapDelete(doc, "exits")
			// After description, or name, or at the end.
			at := len(doc.Content)
			for _, key := range []string{"description", "name"} {
				if _, i := mapGet(doc, key); i >= 0 {
					at = i + 2
					break
				}
			}
			key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "exits"}
			doc.Content = append(doc.Content[:at], append([]*yaml.Node{key, exits}, doc.Content[at:]...)...)
		}
		value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(to)}
		if old, _ := mapGet(exits, dir); old != nil {
			*old = *value
			return nil
		}
		if len(exits.Content) == 0 {
			exits.Style = 0 // "exits: {}" becomes a block
		}
		order := map[string]int{}
		for i, d := range room.Directions {
			order[d] = i
		}
		at := len(exits.Content)
		for i := 0; i+1 < len(exits.Content); i += 2 {
			if o, ok := order[exits.Content[i].Value]; ok && o > order[dir] {
				at = i
				break
			}
		}
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: dir}
		exits.Content = append(exits.Content[:at], append([]*yaml.Node{key, value}, exits.Content[at:]...)...)
		return nil
	}
}

// removeExit drops an exit and any door on it.
func removeExit(dir string) func(*yaml.Node) error {
	return func(doc *yaml.Node) error {
		exits, _ := mapGet(doc, "exits")
		if exits == nil || exits.Kind != yaml.MappingNode {
			return fmt.Errorf("no exit %s", dir)
		}
		mapDelete(exits, dir)
		if len(exits.Content) == 0 {
			exits.Style = yaml.FlowStyle
		}
		if doors, _ := mapGet(doc, "doors"); doors != nil && doors.Kind == yaml.MappingNode {
			mapDelete(doors, dir)
			if len(doors.Content) == 0 {
				mapDelete(doc, "doors")
			}
		}
		return nil
	}
}

func writeEdit(w http.ResponseWriter, code int, res *EditResult) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(res)
}

func writeEditErr(w http.ResponseWriter, err error) {
	var inv errInvalid
	if errors.As(err, &inv) {
		writeEdit(w, http.StatusUnprocessableEntity, &EditResult{Error: err.Error(), Invalid: true})
		return
	}
	writeEdit(w, http.StatusConflict, &EditResult{Error: err.Error()})
}
