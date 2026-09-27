package builder

// Placing things in rooms. The page adds an item or mob to a room by
// adding a reset for it to the room's area, making a new prototype first
// if asked, and removes one by index. resets.yaml files are hand-aligned
// one reset to a line with trailing comments, which yaml.v3 would
// re-space, so they are edited as text: one line in, or one reset's lines
// out, found from the parser's line numbers.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"urth/internal/reset"
)

// resetsArea is the area whose resets a saved file changes, so they run
// at once: the area of a resets.yaml, else none.
func (s *Server) resetsArea(path string) string {
	if filepath.Base(path) != "resets.yaml" {
		return ""
	}
	return filepath.Base(filepath.Dir(path))
}

// PlaceRequest puts an item or mob in a room through a reset in the
// room's area. Vnum names an existing prototype; zero makes a new one
// called Name with the area's next free vnum.
type PlaceRequest struct {
	Room int    `json:"room"`
	Kind string `json:"kind"` // "item" or "mob"
	Vnum int    `json:"vnum,omitempty"`
	Name string `json:"name,omitempty"`
	// Max caps a mob's live count (reset.Reset.Max); zero means one.
	Max int `json:"max,omitempty"`
}

func (s *Server) handlePlace(w http.ResponseWriter, r *http.Request) {
	if !guard(w, r, http.MethodPost) {
		return
	}
	var req PlaceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.edit.Lock()
	defer s.edit.Unlock()
	res, err := s.place(req)
	if err != nil {
		writeEditErr(w, err)
		return
	}
	writeEdit(w, http.StatusOK, res)
}

func (s *Server) place(req PlaceRequest) (*EditResult, error) {
	if req.Kind != "item" && req.Kind != "mob" {
		return nil, fmt.Errorf("kind must be item or mob, not %q", req.Kind)
	}
	disk, err := s.loadDisk()
	if err != nil {
		return nil, err
	}
	rm, ok := disk.Rooms.Get(req.Room)
	if !ok {
		return nil, fmt.Errorf("no room %d on disk", req.Room)
	}
	changes := map[string][]byte{}
	res := &EditResult{}
	vnum, name := req.Vnum, ""
	if vnum == 0 {
		name = strings.TrimSpace(req.Name)
		if name == "" {
			return nil, fmt.Errorf("a new %s needs a name, like %q", req.Kind, map[string]string{"item": "a brass key", "mob": "a sleepy guard"}[req.Kind])
		}
		if vnum, err = nextVnum(disk, rm.Area, req.Kind+"s"); err != nil {
			return nil, err
		}
		file := filepath.Join(s.worldDir, rm.Area, req.Kind+"s", strconv.Itoa(vnum)+".yaml")
		if _, err := os.Stat(file); err == nil {
			return nil, fmt.Errorf("%s already exists", file)
		}
		changes[file] = newProtoYAML(req.Kind, vnum, name)
		res.Created = file
	} else if req.Kind == "item" {
		p, ok := disk.Items[vnum]
		if !ok {
			return nil, fmt.Errorf("no item %d", vnum)
		}
		name = p.Name
	} else {
		p, ok := disk.Mobs[vnum]
		if !ok {
			return nil, fmt.Errorf("no mob %d", vnum)
		}
		name = p.Name
	}

	line := fmt.Sprintf("{ %s: %d, room: %d", req.Kind, vnum, rm.Vnum)
	if req.Kind == "mob" && req.Max > 1 {
		line += fmt.Sprintf(", max: %d", req.Max)
	}
	line += " }   # " + strings.ReplaceAll(name, "\n", " ")
	file := filepath.Join(s.worldDir, rm.Area, "resets.yaml")
	src, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if changes[file], err = addReset(src, line, rm.Vnum); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	out, err := s.apply(changes, false, rm.Area)
	if err != nil {
		return nil, err
	}
	out.Vnum, out.Created = vnum, res.Created
	return out, nil
}

// UnplaceRequest removes reset Index from an area's resets. Room and
// Vnum must match it, so a stale page cannot remove the wrong one.
type UnplaceRequest struct {
	Area  string `json:"area"`
	Index int    `json:"index"`
	Room  int    `json:"room"`
	Vnum  int    `json:"vnum"`
}

func (s *Server) handleUnplace(w http.ResponseWriter, r *http.Request) {
	if !guard(w, r, http.MethodPost) {
		return
	}
	var req UnplaceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.edit.Lock()
	defer s.edit.Unlock()
	res, err := s.unplace(req)
	if err != nil {
		writeEditErr(w, err)
		return
	}
	writeEdit(w, http.StatusOK, res)
}

func (s *Server) unplace(req UnplaceRequest) (*EditResult, error) {
	disk, err := s.loadDisk()
	if err != nil {
		return nil, err
	}
	a, ok := disk.Resets[req.Area]
	if !ok || req.Index < 0 || req.Index >= len(a.Resets) {
		return nil, fmt.Errorf("area %s has no reset %d", req.Area, req.Index)
	}
	rs := a.Resets[req.Index]
	if rs.Room != req.Room || (rs.Mob != req.Vnum && rs.Item != req.Vnum) {
		return nil, errors.New("the resets changed on disk; reload the page and try again")
	}
	file := filepath.Join(s.worldDir, req.Area, "resets.yaml")
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	out, err := removeResets(src, []int{req.Index})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return s.apply(map[string][]byte{file: out}, false, "")
}

// ProtoRow is one prototype in the page's picker.
type ProtoRow struct {
	Vnum int    `json:"vnum"`
	Name string `json:"name"`
	Area string `json:"area"`
}

// handleProtos lists every item and mob prototype on disk, for placing.
func (s *Server) handleProtos(w http.ResponseWriter, r *http.Request) {
	disk, err := s.loadDisk()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	out := map[string][]ProtoRow{"items": {}, "mobs": {}}
	for _, p := range disk.Items {
		out["items"] = append(out["items"], ProtoRow{p.Vnum, p.Name, p.Area})
	}
	for _, p := range disk.Mobs {
		out["mobs"] = append(out["mobs"], ProtoRow{p.Vnum, p.Name, p.Area})
	}
	for _, list := range out {
		sort.Slice(list, func(i, j int) bool { return list[i].Vnum < list[j].Vnum })
	}
	writeJSON(w, out)
}

// resetsDoc parses a resets file for line surgery: the lines, the
// resets key, and its sequence.
func resetsDoc(src []byte) (lines []string, key, seq *yaml.Node, err error) {
	lines = strings.Split(string(src), "\n")
	var file yaml.Node
	if err := yaml.Unmarshal(src, &file); err != nil {
		return nil, nil, nil, err
	}
	if file.Kind != yaml.DocumentNode || len(file.Content) == 0 {
		return lines, nil, nil, nil
	}
	doc := file.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, nil, nil, errors.New("not a YAML mapping")
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "resets" {
			key, seq = doc.Content[i], doc.Content[i+1]
		}
	}
	if seq != nil && seq.Kind != yaml.SequenceNode && !(seq.Kind == yaml.ScalarNode && seq.Tag == "!!null") {
		return nil, nil, nil, errors.New("resets is not a list")
	}
	if seq != nil && seq.Kind == yaml.SequenceNode && seq.Style&yaml.FlowStyle != 0 && len(seq.Content) > 0 {
		return nil, nil, nil, errors.New("resets is written as one flow list; put one reset on each line to edit it here")
	}
	return lines, key, seq, nil
}

// lastLine is the last source line a node covers.
func lastLine(n *yaml.Node) int {
	end := n.Line
	for _, c := range n.Content {
		end = max(end, lastLine(c))
	}
	return end
}

// addReset inserts a reset line: after the last reset for the same room,
// else after the last reset, else as the first.
func addReset(src []byte, line string, room int) ([]byte, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return []byte("resets:\n  - " + line + "\n"), nil
	}
	lines, key, seq, err := resetsDoc(src)
	if err != nil {
		return nil, err
	}
	if key == nil {
		out := strings.TrimRight(string(src), "\n") + "\nresets:\n  - " + line + "\n"
		return []byte(out), nil
	}
	if seq.Kind != yaml.SequenceNode || len(seq.Content) == 0 {
		// "resets:" or "resets: []" on one line becomes a block list.
		at := key.Line - 1
		indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " "))]
		lines[at] = indent + "resets:"
		lines = insert(lines, at+1, indent+"  - "+line)
		return []byte(strings.Join(lines, "\n")), nil
	}
	anchor := seq.Content[len(seq.Content)-1]
	for _, item := range seq.Content {
		if v, _ := mapGet(item, "room"); v != nil && v.Value == strconv.Itoa(room) {
			anchor = item
		}
	}
	first := lines[anchor.Line-1]
	dash := strings.Index(first, "-")
	if dash < 0 {
		return nil, errors.New("cannot find the list's indentation")
	}
	lines = insert(lines, lastLine(anchor), first[:dash]+"- "+line)
	return []byte(strings.Join(lines, "\n")), nil
}

// removeResets deletes the lines of the resets at the given indices.
func removeResets(src []byte, indices []int) ([]byte, error) {
	lines, key, seq, err := resetsDoc(src)
	if err != nil {
		return nil, err
	}
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, errors.New("no resets to remove")
	}
	type span struct{ from, to int } // 1-based, inclusive
	var spans []span
	for _, i := range indices {
		if i < 0 || i >= len(seq.Content) {
			return nil, fmt.Errorf("no reset %d", i)
		}
		item := seq.Content[i]
		sp := span{item.Line, lastLine(item)}
		if (i > 0 && lastLine(seq.Content[i-1]) >= sp.from) || (i+1 < len(seq.Content) && seq.Content[i+1].Line <= sp.to) {
			return nil, fmt.Errorf("reset %d shares a line with another; edit it by hand", i)
		}
		spans = append(spans, sp)
	}
	sort.Slice(spans, func(a, b int) bool { return spans[a].from > spans[b].from })
	for _, sp := range spans {
		lines = append(lines[:sp.from-1], lines[sp.to:]...)
	}
	if len(indices) == len(seq.Content) {
		at := key.Line - 1
		lines[at] = strings.TrimRight(lines[at], " ") + " []"
	}
	out := []byte(strings.Join(lines, "\n"))
	// The result must still parse to the same list minus those.
	var a reset.Area
	if err := yaml.Unmarshal(out, &a); err != nil || len(a.Resets) != len(seq.Content)-len(indices) {
		return nil, errors.New("could not remove the reset cleanly; edit resets.yaml by hand")
	}
	return out, nil
}

func insert(lines []string, at int, line string) []string {
	lines = append(lines, "")
	copy(lines[at+1:], lines[at:])
	lines[at] = line
	return lines
}

// newProtoYAML is the file for a new item or mob: just enough to load,
// for the builder to fill in.
func newProtoYAML(kind string, vnum int, name string) []byte {
	var keywords []string
	for _, w := range strings.Fields(strings.ToLower(name)) {
		w = strings.Trim(w, ".,;:!?'\"()")
		switch w {
		case "", "a", "an", "the", "some", "of", "pair", "set":
			continue
		}
		ok := true
		for _, r := range w {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				ok = false
			}
		}
		if ok {
			keywords = append(keywords, w)
		}
	}
	if len(keywords) == 0 {
		keywords = []string{kind}
	}
	scalar := func(s string) string {
		b, _ := yaml.Marshal(&yaml.Node{Kind: yaml.ScalarNode, Value: s})
		return strings.TrimRight(string(b), "\n")
	}
	title := strings.ToUpper(name[:1]) + name[1:]
	var b strings.Builder
	fmt.Fprintf(&b, "vnum: %d\nname: %s\nkeywords: [%s]\n", vnum, scalar(name), strings.Join(keywords, ", "))
	if kind == "item" {
		fmt.Fprintf(&b, "description: %s\nlook: |\n  Nothing has been written about it yet.\ntype: other\nlevel: 1\n", scalar(title+" lies here."))
	} else {
		fmt.Fprintf(&b, "description: %s\nlook: |\n  Nothing has been written about it yet.\nlevel: 1\n", scalar(title+" is here."))
	}
	return []byte(b.String())
}
