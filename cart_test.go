package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestProjectLocationStaysOutsideCart(t *testing.T) {
	resetTestState(t)
	const id = "project-details"
	t.Cleanup(func() { delete(projectLocations, id) })
	w := httptest.NewRecorder()
	partForm[ProjectLocation](w, formRequest("/projects/location", url.Values{
		"Name": {"Warehouse"}, "Address": {"123 Main Street"}, "Notes": {"Loading dock"},
	}, id))
	if w.Code != http.StatusSeeOther || len(carts[id]) != 0 {
		t.Fatalf("location added to cart or save failed: status=%d cart=%v", w.Code, carts[id])
	}
	location := projectLocations[id]
	if location.Name != "Warehouse" || location.Address != "123 Main Street" {
		t.Fatalf("project details not saved: %+v", location)
	}
	text := (order{items: []part{Nipple{Size: "close", Quantity: 2}}, location: &location}).String()
	if !strings.Contains(text, "Warehouse") || !strings.Contains(text, "Nipple:") {
		t.Fatalf("order missing location or parts: %s", text)
	}
	r := httptest.NewRequest(http.MethodGet, "/cart", nil)
	r.AddCookie(&http.Cookie{Name: "sid", Value: id})
	w = httptest.NewRecorder()
	index(w, r)
	if !strings.Contains(w.Body.String(), "Project: Warehouse") || !strings.Contains(w.Body.String(), "Your cart is empty.") {
		t.Fatal("project details should display separately from the empty cart")
	}
}

func TestCartDisplay(t *testing.T) {
	resetTestState(t)
	carts["display"] = []part{Nipple{Size: "<script>bad()</script>", Quantity: 3}}
	r := httptest.NewRequest(http.MethodGet, "/cart", nil)
	r.AddCookie(&http.Cookie{Name: "sid", Value: "display"})
	w := httptest.NewRecorder()
	index(w, r)
	body := w.Body.String()
	if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;bad()&lt;/script&gt;") {
		t.Fatal("cart contents were not HTML-escaped")
	}
	if !strings.Contains(body, "Quantity: 3") || !strings.Contains(body, `name="index" value="0"`) {
		t.Fatal("cart contents or delete button missing")
	}
	if !strings.Contains(body, `href="/review"`) || strings.Contains(body, `action="/submit"`) {
		t.Fatal("cart should link to review before submission")
	}
	r.URL.Path = "/review"
	w = httptest.NewRecorder()
	index(w, r)
	if !strings.Contains(w.Body.String(), `action="/submit"`) || strings.Contains(w.Body.String(), `action="/remove"`) {
		t.Fatal("review must allow submission and link back for cart edits")
	}
	r.URL.Path = "/"
	w = httptest.NewRecorder()
	index(w, r)
	if !strings.Contains(w.Body.String(), "Cart (1)") || strings.Contains(w.Body.String(), `id="cart"`) {
		t.Fatal("catalog should show cart button without sidebar")
	}
	r.URL.Path = "/cart"
	delete(carts, "display")
	w = httptest.NewRecorder()
	index(w, r)
	if strings.Contains(w.Body.String(), `action="/submit"`) {
		t.Fatal("empty cart has a submission button")
	}
}

func TestRemovePart(t *testing.T) {
	resetTestState(t)
	original := []part{Nipple{Size: "close", Quantity: 1}, PipeStand{Size: "6″", Quantity: 2}}
	carts["mine"] = append([]part(nil), original...)
	carts["other"] = []part{PipeStand{Size: "4″", Quantity: 3}}
	for _, tc := range []struct {
		index  string
		status int
	}{{"", 400}, {"bad", 400}, {"-1", 404}, {"2", 404}} {
		w := httptest.NewRecorder()
		removePart(w, formRequest("/remove", url.Values{"index": {tc.index}}, "mine"))
		if w.Code != tc.status || !reflect.DeepEqual(carts["mine"], original) {
			t.Fatalf("index=%q: status=%d cart=%v", tc.index, w.Code, carts["mine"])
		}
	}
	w := httptest.NewRecorder()
	removePart(w, formRequest("/remove", url.Values{"index": {"0"}}, "mine"))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("remove: status=%d headers=%v", w.Code, w.Header())
	}
	if !reflect.DeepEqual(carts["mine"], original[1:]) || len(carts["other"]) != 1 {
		t.Fatal("wrong item or another session's cart was modified")
	}
}

func TestConcurrentAddAndRemove(t *testing.T) {
	resetTestState(t)
	const id = "concurrent"
	sessions[id] = "test-user"
	for range 20 {
		carts[id] = append(carts[id], Nipple{Size: "close", Quantity: 1})
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			partForm[Nipple](w, formRequest("/nipple", url.Values{"Size": {"close"}, "Quantity": {"1"}}, id))
			if w.Code != http.StatusSeeOther {
				t.Errorf("add status=%d", w.Code)
			}
		}()
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			removePart(w, formRequest("/remove", url.Values{"index": {"0"}}, id))
			if w.Code != http.StatusSeeOther {
				t.Errorf("remove status=%d", w.Code)
			}
		}()
	}
	wg.Wait()
	if len(carts[id]) != 20 {
		t.Fatalf("remaining parts=%d, want 20", len(carts[id]))
	}
}

func TestSubmitEmptyCart(t *testing.T) {
	resetTestState(t)
	w := httptest.NewRecorder()
	submit(func(string, ...any) error {
		t.Fatal("empty cart was recorded")
		return nil
	})(w, formRequest("/submit", nil, "empty"))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("empty submission: status=%d headers=%v", w.Code, w.Header())
	}
}
