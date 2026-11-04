package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func resetTestState(t *testing.T) {
	t.Helper()
	oldSessions, oldCarts := sessions, carts
	sessions = make(map[string]string)
	carts = make(map[string][]part)
	t.Cleanup(func() { sessions, carts = oldSessions, oldCarts })
}

func formRequest(path string, values url.Values, sid string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sid != "" {
		r.AddCookie(&http.Cookie{Name: "sid", Value: sid})
	}
	return r
}

func TestReadUsers(t *testing.T) {
	got, err := readUsers(strings.NewReader("alice|one\nbob|two\n"))
	if err != nil || !reflect.DeepEqual(got, map[string]string{"alice": "one", "bob": "two"}) {
		t.Fatalf("users=%v error=%v", got, err)
	}
	if _, err := readUsers(strings.NewReader("alice|one\nbad line\n")); err == nil || !strings.Contains(err.Error(), "passwd:2:") {
		t.Fatalf("malformed line error=%v", err)
	}
	want := errors.New("read failed")
	if _, err := readUsers(iotest.ErrReader(want)); !errors.Is(err, want) {
		t.Fatalf("reader error=%v, want %v", err, want)
	}
}

func TestLoginGuard(t *testing.T) {
	resetTestState(t)
	sessions["known"] = "alice"
	for _, tc := range []struct {
		sid     string
		allowed bool
	}{{"", false}, {"unknown", false}, {"known", true}} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.sid != "" {
			r.AddCookie(&http.Cookie{Name: "sid", Value: tc.sid})
		}
		called := false
		w := httptest.NewRecorder()
		requireLogin(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		})(w, r)
		if called != tc.allowed {
			t.Fatalf("sid=%q: protected handler called=%v", tc.sid, called)
		}
		if !tc.allowed && (w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login") {
			t.Fatalf("sid=%q: status=%d headers=%v", tc.sid, w.Code, w.Header())
		}
	}
}

func TestAuthenticate(t *testing.T) {
	resetTestState(t)
	handler := authenticate(map[string]string{"alice": "test-password"})
	for _, values := range []url.Values{
		{"username": {"alice"}, "password": {"wrong"}},
		{"username": {"unknown"}, "password": {"test-password"}},
		{},
	} {
		w := httptest.NewRecorder()
		handler(w, formRequest("/login", values, ""))
		if w.Code != http.StatusUnauthorized || len(sessions) != 0 {
			t.Fatalf("invalid login: status=%d sessions=%v", w.Code, sessions)
		}
	}
	w := httptest.NewRecorder()
	handler(w, formRequest("/login", url.Values{"username": {"alice"}, "password": {"test-password"}}, ""))
	cookies := w.Result().Cookies()
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" || len(cookies) != 1 {
		t.Fatalf("login: status=%d headers=%v", w.Code, w.Header())
	}
	if sessions[cookies[0].Value] != "alice" {
		t.Fatal("authenticated session was not saved")
	}
}

func TestSessionID(t *testing.T) {
	w := httptest.NewRecorder()
	id := sessionID(w, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := w.Result().Cookies()
	if id == "" || len(cookies) != 1 || cookies[0].Value != id {
		t.Fatalf("id=%q cookies=%v", id, cookies)
	}
	c := cookies[0]
	if c.Name != "sid" || c.Path != "/" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie=%+v", c)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(c)
	w = httptest.NewRecorder()
	if got := sessionID(w, r); got != id || len(w.Result().Cookies()) != 0 {
		t.Fatal("existing session ID was replaced")
	}
}
