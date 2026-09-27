package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAPI struct{ acts []string }

func (f *fakeAPI) Status() (*Status, error) { return &Status{Name: "Urth", Playing: 1}, nil }
func (f *fakeAPI) Characters() ([]CharacterRow, error) {
	return []CharacterRow{{Name: "Bob", Level: 3, Online: true}}, nil
}
func (f *fakeAPI) Character(name string) (*CharacterView, error) {
	if name != "Bob" {
		return nil, ErrNotFound
	}
	return &CharacterView{CharacterRow: CharacterRow{Name: "Bob"}}, nil
}
func (f *fakeAPI) Connections() ([]Connection, error) { return nil, nil }
func (f *fakeAPI) Act(action, name string) (string, error) {
	if action == "delete" {
		return "", fmt.Errorf("%w: online", ErrRefused)
	}
	f.acts = append(f.acts, action+" "+name)
	return name + " done", nil
}
func (f *fakeAPI) SetPassword(name, password string) (string, error) { return "changed", nil }

func TestAdminPage(t *testing.T) {
	api := &fakeAPI{}
	s := NewServer("127.0.0.1:0", api, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	do := func(method, path, body string, header map[string]string) (int, string) {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		for k, v := range header {
			if k == "Host" {
				req.Host = v
			} else {
				req.Header.Set(k, v)
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, b := do("GET", "/", "", nil); code != 200 || !strings.Contains(b, "Urth admin") {
		t.Fatalf("page: %d", code)
	}
	if code, b := do("GET", "/api/characters", "", nil); code != 200 || !strings.Contains(b, `"Bob"`) {
		t.Fatalf("characters: %d %s", code, b)
	}
	if code, _ := do("GET", "/api/character?name=Zed", "", nil); code != 404 {
		t.Fatalf("unknown character: %d", code)
	}
	if code, b := do("GET", "/api/connections", "", nil); code != 200 || strings.TrimSpace(b) != "[]" {
		t.Fatalf("connections: %d %s", code, b)
	}
	// A rebound DNS name is turned away even for reads.
	if code, _ := do("GET", "/api/characters", "", map[string]string{"Host": "evil.example:4003"}); code != 403 {
		t.Fatalf("DNS name read: %d", code)
	}
	if code, _ := do("GET", "/api/characters", "", map[string]string{"Host": "localhost:4003"}); code != 200 {
		t.Fatalf("localhost read: %d", code)
	}
	// Changes need POST and the header.
	h := map[string]string{"X-Urth-Admin": "1"}
	if code, _ := do("POST", "/api/act", `{"action":"promote","name":"Bob"}`, nil); code != 403 {
		t.Fatalf("act without header: %d", code)
	}
	if code, _ := do("GET", "/api/act", "", h); code != 405 {
		t.Fatalf("GET act: %d", code)
	}
	if code, _ := do("POST", "/api/act", `{"action":"explode","name":"Bob"}`, h); code != 400 {
		t.Fatalf("unknown action: %d", code)
	}
	code, b := do("POST", "/api/act", `{"action":"promote","name":"Bob"}`, h)
	var res result
	_ = json.Unmarshal([]byte(b), &res)
	if code != 200 || !res.OK || res.Message != "Bob done" || len(api.acts) != 1 {
		t.Fatalf("promote: %d %s %v", code, b, api.acts)
	}
	if code, _ := do("POST", "/api/act", `{"action":"delete","name":"Bob"}`, h); code != 409 {
		t.Fatalf("refused delete: %d", code)
	}
	if code, _ := do("POST", "/api/password", `{"name":"Bob","password":"hunter22"}`, h); code != 200 {
		t.Fatalf("password: %d", code)
	}
}

func TestListenRefusesNonLoopback(t *testing.T) {
	s := NewServer("0.0.0.0:0", &fakeAPI{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.Listen(); err == nil {
		s.ln.Close()
		t.Fatal("listened on every address")
	}
}
