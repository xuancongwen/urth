package builder

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newEditTest is a two-room area on disk, hub north to hall, with a
// comment the edits must keep.
func newEditTest(t *testing.T) (*httptest.Server, *fakeAPI, string) {
	t.Helper()
	s, api, dir := newTest(t)
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a/rooms/1.yaml", "vnum: 1\nname: Hub\ndescription: |\n  The middle.\n# the way on\nexits:\n  north: 2\nflags: [safe]\n")
	write("a/rooms/2.yaml", "vnum: 2\nname: Hall\nexits:\n  south: 1\n")
	write("a/resets.yaml", "resets: []\n")
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv, api, dir
}

func send(t *testing.T, srv *httptest.Server, method, path string, body any, header map[string]string) (int, EditResult) {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case string:
		rd = strings.NewReader(b)
	case nil:
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, srv.URL+path, rd)
	req.Header.Set("X-Urth-Builder", "1")
	for k, v := range header {
		if k == "Host" {
			req.Host = v
		} else if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res EditResult
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

func read(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSaveFile(t *testing.T) {
	srv, api, dir := newEditTest(t)
	file := filepath.Join(dir, "a", "rooms", "2.yaml")
	resp, err := http.Get(srv.URL + "/api/file?path=" + file)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on a file")
	}
	url := "/api/file?path=" + file

	// A stale hash is a conflict and writes nothing.
	if code, res := send(t, srv, "PUT", url, "vnum: 2\nname: Stale\nexits:\n  south: 1\n", map[string]string{"If-Match": "nope"}); code != 409 || res.OK {
		t.Fatalf("stale save: %d %+v", code, res)
	}
	// A file the world would not load is refused, and written when forced.
	broken := "vnum: 2\nname: Hall\nexits:\n  south: 99\n"
	code, res := send(t, srv, "PUT", url, broken, map[string]string{"If-Match": etag})
	if code != 422 || !res.Invalid || !strings.Contains(res.Error, "missing room 99") || strings.Contains(res.Error, "urth-builder-") {
		t.Fatalf("invalid save: %d %+v", code, res)
	}
	if strings.Contains(read(t, file), "99") || len(api.reloaded()) != 0 {
		t.Fatal("an invalid save was written or reloaded")
	}
	good := "vnum: 2\nname: The Great Hall\nexits:\n  south: 1\n"
	code, res = send(t, srv, "PUT", url, good, map[string]string{"If-Match": etag})
	if code != 200 || !res.OK || res.Reload == nil || res.Hash == "" || read(t, file) != good || len(api.reloaded()) != 1 {
		t.Fatalf("save: %d %+v %q", code, res, read(t, file))
	}
	code, res = send(t, srv, "PUT", url+"&force=1", broken, map[string]string{"If-Match": res.Hash})
	if code != 200 || read(t, file) != broken {
		t.Fatalf("forced save: %d %+v", code, res)
	}

	// Only content files, and only with the header on a loopback host.
	for _, bad := range []string{filepath.Join(dir, "a", "notes.yaml"), filepath.Join(dir, "a", "rooms", "x", "1.yaml"), filepath.Join(dir, "..", "x.yaml")} {
		if code, _ := send(t, srv, "PUT", "/api/file?path="+bad, "x: 1\n", map[string]string{"If-Match": "new"}); code != 400 {
			t.Fatalf("save to %s: %d", bad, code)
		}
	}
	if code, _ := send(t, srv, "PUT", url, good, map[string]string{"X-Urth-Builder": ""}); code != 403 {
		t.Fatalf("save without the header: %d", code)
	}
	if code, _ := send(t, srv, "PUT", url, good, map[string]string{"Host": "evil.example:4002"}); code != 403 {
		t.Fatalf("save through a DNS name: %d", code)
	}
}

func TestDigLinkUnlinkDelete(t *testing.T) {
	srv, _, dir := newEditTest(t)
	hub := filepath.Join(dir, "a", "rooms", "1.yaml")

	// Dig east from the hub: a new room 3 with the way back, and the hub
	// file edited in place with its comment and styles kept.
	code, res := send(t, srv, "POST", "/api/dig", DigRequest{From: 1, Dir: "east", Name: "The East Room"}, nil)
	if code != 200 || res.Vnum != 3 {
		t.Fatalf("dig: %d %+v", code, res)
	}
	if got, want := read(t, hub), "vnum: 1\nname: Hub\ndescription: |\n  The middle.\n# the way on\nexits:\n  north: 2\n  east: 3\nflags: [safe]\n"; got != want {
		t.Fatalf("hub after dig:\n%s\nwant:\n%s", got, want)
	}
	if got := read(t, filepath.Join(dir, "a", "rooms", "3.yaml")); !strings.Contains(got, "name: The East Room\n") || !strings.Contains(got, "exits:\n  west: 1\n") {
		t.Fatalf("new room:\n%s", got)
	}
	// An exit that is taken, or a bad direction, is refused.
	if code, res := send(t, srv, "POST", "/api/dig", DigRequest{From: 1, Dir: "east"}, nil); code != 409 || !strings.Contains(res.Error, "already has an exit east") {
		t.Fatalf("dig over an exit: %d %+v", code, res)
	}
	if code, _ := send(t, srv, "POST", "/api/dig", DigRequest{From: 1, Dir: "sideways"}, nil); code != 409 {
		t.Fatalf("dig sideways: %d", code)
	}

	// Dig north from 3 to make room 4, then link the hall east to it:
	// the hall's east and 4's west are both free.
	if code, res := send(t, srv, "POST", "/api/dig", DigRequest{From: 3, Dir: "north"}, nil); code != 200 || res.Vnum != 4 {
		t.Fatalf("dig north: %d %+v", code, res)
	}
	hall, four := filepath.Join(dir, "a", "rooms", "2.yaml"), filepath.Join(dir, "a", "rooms", "4.yaml")
	if code, res := send(t, srv, "POST", "/api/dig", DigRequest{From: 2, Dir: "east", To: 4}, nil); code != 200 || len(res.Files) != 2 {
		t.Fatalf("link: %d %+v", code, res)
	}
	if !strings.Contains(read(t, hall), "  east: 4\n") || !strings.Contains(read(t, four), "  west: 2\n") {
		t.Fatalf("link wrote:\n%s\n%s", read(t, hall), read(t, four))
	}
	// A link whose way back is taken is refused and writes nothing.
	if code, res := send(t, srv, "POST", "/api/dig", DigRequest{From: 3, Dir: "south", To: 1}, nil); code != 409 || !strings.Contains(res.Error, "room 1 already has an exit north") {
		t.Fatalf("link over a far exit: %d %+v", code, res)
	}
	if strings.Contains(read(t, filepath.Join(dir, "a", "rooms", "3.yaml")), "south") {
		t.Fatal("a refused link was written")
	}

	// Unlink takes both sides.
	if code, res := send(t, srv, "POST", "/api/unlink", UnlinkRequest{From: 2, Dir: "east"}, nil); code != 200 {
		t.Fatalf("unlink: %d %+v", code, res)
	}
	if strings.Contains(read(t, hall), "east") || strings.Contains(read(t, four), "west") {
		t.Fatalf("unlink left:\n%s\n%s", read(t, hall), read(t, four))
	}

	// Delete removes the file and every exit into it; the start room
	// cannot go.
	if code, res := send(t, srv, "POST", "/api/delete", DeleteRequest{Vnum: 4}, nil); code != 200 {
		t.Fatalf("delete: %d %+v", code, res)
	}
	if _, err := os.Stat(four); !os.IsNotExist(err) {
		t.Fatal("room 4 file still there")
	}
	if strings.Contains(read(t, filepath.Join(dir, "a", "rooms", "3.yaml")), "north") {
		t.Fatal("room 3 still leads to room 4")
	}
	if code, res := send(t, srv, "POST", "/api/delete", DeleteRequest{Vnum: 1}, nil); code != 409 || !strings.Contains(res.Error, "start room") {
		t.Fatalf("delete start: %d %+v", code, res)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "resets.yaml"), []byte("resets:\n  - { item: 5, room: 3 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The reset names an item that does not exist, so the disk does not
	// load and nothing can be changed until it is fixed.
	if code, res := send(t, srv, "POST", "/api/delete", DeleteRequest{Vnum: 3}, nil); code != 409 || !strings.Contains(res.Error, "fix that first") {
		t.Fatalf("delete on a broken disk: %d %+v", code, res)
	}
}

func TestEditYAMLExits(t *testing.T) {
	src := "vnum: 7\nname: Nook\nexits: {}\n"
	out, err := editYAML([]byte(src), setExit("down", 8))
	if err != nil || string(out) != "vnum: 7\nname: Nook\nexits:\n  down: 8\n" {
		t.Fatalf("set on empty: %v %q", err, out)
	}
	out, _ = editYAML(out, setExit("north", 9))
	if string(out) != "vnum: 7\nname: Nook\nexits:\n  north: 9\n  down: 8\n" {
		t.Fatalf("display order: %q", out)
	}
	src = "vnum: 7\nname: Nook\nexits:\n  north: 9\ndoors:\n  north: {name: the hatch}\n"
	out, _ = editYAML([]byte(src), removeExit("north"))
	if string(out) != "vnum: 7\nname: Nook\nexits: {}\n" {
		t.Fatalf("remove with door: %q", out)
	}
	out, _ = editYAML([]byte("vnum: 7\nname: Nook\nflags: [dark]\n"), setExit("west", 1))
	if string(out) != "vnum: 7\nname: Nook\nexits:\n  west: 1\nflags: [dark]\n" {
		t.Fatalf("no exits key: %q", out)
	}
}
