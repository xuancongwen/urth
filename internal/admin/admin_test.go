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

type fakeAPI struct {
	acts    []string
	demoted bool
}

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
func (f *fakeAPI) Act(by, action, name string) (string, error) {
	if action == "delete" {
		return "", fmt.Errorf("%w: online", ErrRefused)
	}
	f.acts = append(f.acts, action+" "+name)
	return name + " done", nil
}
func (f *fakeAPI) SetPassword(by, name, password string) (string, error) { return "changed", nil }
func (f *fakeAPI) Authenticate(name, password string) (string, error) {
	if name == "alice" && password == "secret5" && !f.demoted {
		return "Alice", nil
	}
	return "", ErrBadLogin
}
func (f *fakeAPI) IsAdmin(name string) bool { return name == "Alice" && !f.demoted }

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

// TestSignInFromTheLAN drives the handler with a LAN RemoteAddr: data
// needs a session, a session comes from an admin's password, failures
// lock the address out, and a demotion ends the session.
func TestSignInFromTheLAN(t *testing.T) {
	api := &fakeAPI{}
	s := NewServer("0.0.0.0:0", api, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := s.Handler()
	var cookie *http.Cookie
	do := func(method, path, body, from string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://192.168.1.5:4003"+path, strings.NewReader(body))
		req.RemoteAddr = from
		req.Header.Set("X-Urth-Admin", "1")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	lan := "192.168.1.9:50000"
	if rec := do("GET", "/api/characters", "", lan); rec.Code != 401 {
		t.Fatalf("LAN read without a session: %d", rec.Code)
	}
	if rec := do("GET", "/api/characters", "", "127.0.0.1:50000"); rec.Code != 200 {
		t.Fatalf("loopback read: %d", rec.Code)
	}
	if rec := do("GET", "/api/session", "", lan); !strings.Contains(rec.Body.String(), `"needLogin":true`) {
		t.Fatalf("session: %s", rec.Body)
	}
	if rec := do("POST", "/api/login", `{"name":"alice","password":"wrong"}`, lan); rec.Code != 401 {
		t.Fatalf("bad password: %d", rec.Code)
	}
	rec := do("POST", "/api/login", `{"name":"alice","password":"secret5"}`, lan)
	if rec.Code != 200 {
		t.Fatalf("sign in: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie: %+v", cookie)
	}
	if rec := do("GET", "/api/characters", "", lan); rec.Code != 200 {
		t.Fatalf("read with a session: %d", rec.Code)
	}
	if rec := do("GET", "/api/session", "", lan); !strings.Contains(rec.Body.String(), `"name":"Alice"`) {
		t.Fatalf("session name: %s", rec.Body)
	}
	// Demoted: the session stops working.
	api.demoted = true
	if rec := do("GET", "/api/characters", "", lan); rec.Code != 401 {
		t.Fatalf("read after demotion: %d", rec.Code)
	}
	api.demoted = false

	// Five failures lock the address out, even for the right password.
	cookie = nil
	other := "192.168.1.10:1"
	for i := 0; i < maxFails; i++ {
		do("POST", "/api/login", `{"name":"alice","password":"nope"}`, other)
	}
	if rec := do("POST", "/api/login", `{"name":"alice","password":"secret5"}`, other); rec.Code != 429 {
		t.Fatalf("after %d failures: %d", maxFails, rec.Code)
	}
	if rec := do("POST", "/api/login", `{"name":"alice","password":"secret5"}`, "192.168.1.11:1"); rec.Code != 200 {
		t.Fatalf("another address locked out too: %d", rec.Code)
	}
	// A .local name passes the Host check; a public name does not.
	req := httptest.NewRequest("GET", "http://urth.local:4003/api/session", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf(".local host: %d", rr.Code)
	}
}
