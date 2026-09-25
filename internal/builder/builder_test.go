package builder

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"urth/internal/content"
)

type fakeAPI struct {
	mu      sync.Mutex
	reloads []string
	fail    bool
	gen     uint64
}

func (f *fakeAPI) Snapshot() (*Snapshot, error) {
	return &Snapshot{Areas: []AreaSummary{{Name: "a", Title: "A", Rooms: 2}}, Problems: []content.Problem{}, StartRoom: 1, Generation: f.Generation()}, nil
}
func (f *fakeAPI) Area(name string) (*AreaView, error) {
	if name != "a" {
		return nil, errors.New("no area named " + name)
	}
	return &AreaView{Name: "a", Title: "A", Rooms: []RoomView{{Vnum: 1, Name: "Hub", Placed: true, Exits: map[string]int{}}}}, nil
}
func (f *fakeAPI) Reload(area string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloads = append(f.reloads, area)
	if f.fail {
		return "", errors.New("room 9: bad yaml")
	}
	f.gen++
	return "World reloaded", nil
}
func (f *fakeAPI) Generation() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gen
}
func (f *fakeAPI) reloaded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.reloads...)
}

func newTest(t *testing.T) (*Server, *fakeAPI, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "rooms"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "rooms", "1.yaml"), []byte("vnum: 1\nname: Hub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{}
	s := NewServer("127.0.0.1:0", dir, api, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s, api, dir
}

func TestPageAndAPI(t *testing.T) {
	s, api, dir := newTest(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := func(path string) (int, string) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, b := body("/"); code != 200 || !strings.Contains(b, "Urth builder") {
		t.Fatalf("page: %d %q", code, b[:min(len(b), 80)])
	}
	if code, b := body("/api/world"); code != 200 || !strings.Contains(b, `"name":"a"`) {
		t.Fatalf("world: %d %s", code, b)
	}
	if code, b := body("/api/area?name=a"); code != 200 || !strings.Contains(b, `"Hub"`) {
		t.Fatalf("area: %d %s", code, b)
	}
	if code, _ := body("/api/area?name=zz"); code != 404 {
		t.Fatalf("unknown area: %d", code)
	}
	if code, _ := body("/api/area"); code != 400 {
		t.Fatalf("missing name: %d", code)
	}

	// Files: only YAML under the world directory.
	file := filepath.Join(dir, "a", "rooms", "1.yaml")
	if code, b := body("/api/file?path=" + file); code != 200 || !strings.Contains(b, "name: Hub") {
		t.Fatalf("file: %d %s", code, b)
	}
	for _, bad := range []string{"", "/etc/passwd", dir + "/../x.yaml", filepath.Join(dir, "..", "other.yaml"), dir, filepath.Join(dir, "a", "rooms", "1.txt")} {
		if code, _ := body("/api/file?path=" + bad); code != 400 && code != 404 {
			t.Fatalf("file %q served: %d", bad, code)
		}
	}
	outside := filepath.Join(filepath.Dir(dir), "escape.yaml")
	if err := os.WriteFile(outside, []byte("x: 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _ := body("/api/file?path=" + filepath.Join(dir, "..", "escape.yaml")); code != 400 {
		t.Fatalf("traversal served: %d", code)
	}

	// Reload: POST only, records the outcome, version reports it.
	if code, _ := body("/api/reload"); code != 405 {
		t.Fatalf("GET reload: %d", code)
	}
	resp, err := http.Post(srv.URL+"/api/reload?area=a", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var res ReloadResult
	_ = json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	if !res.OK || res.Report != "World reloaded" || res.Source != "page" || api.reloaded()[0] != "a" {
		t.Fatalf("reload: %+v %v", res, api.reloaded())
	}
	if code, b := body("/api/version"); code != 200 || !strings.Contains(b, `"generation":1`) || !strings.Contains(b, `"page"`) {
		t.Fatalf("version: %d %s", code, b)
	}
	api.fail = true
	resp, _ = http.Post(srv.URL+"/api/reload", "", nil)
	_ = json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	if res.OK || !strings.Contains(res.Report, "bad yaml") {
		t.Fatalf("failed reload not reported: %+v", res)
	}
}

func TestWatcherReloadsOnSave(t *testing.T) {
	s, api, dir := newTest(t)
	ctx, cancel := contextWithCancel()
	defer cancel()
	go s.watchFrom(ctx, s.signature())
	if err := os.WriteFile(filepath.Join(dir, "a", "rooms", "2.yaml"), []byte("vnum: 2\nname: New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if r := api.reloaded(); len(r) == 1 && r[0] == "" {
			if last := s.lastReload(); last == nil || !last.OK || last.Source != "watch" {
				t.Fatalf("watch outcome: %+v", last)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("watcher never reloaded: %v", api.reloaded())
}

func contextWithCancel() (ctx context.Context, cancel func()) {
	return context.WithCancel(context.Background())
}
