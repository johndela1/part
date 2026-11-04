package main

import (
	"bytes"
	"html"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func checkPartForm[T part](t *testing.T, numberField string, values url.Values, want T) {
	t.Helper()
	resetTestState(t)
	for _, tc := range []struct {
		name, text, message string
		present             bool
	}{
		{"missing", "", "required", false},
		{"empty", "", "required", true},
		{"zero", "0", "must be greater than 0", true},
		{"negative", "-1", "must be greater than 0", true},
		{"text", "abc", "value must be a whole number within the supported range", true},
		{"decimal", "1.5", "value must be a whole number within the supported range", true},
		{"overflow", "999999999999999999999999999999", "value must be a whole number within the supported range", true},
	} {
		t.Run(numberField+"/"+tc.name, func(t *testing.T) {
			input := values.Clone()
			input.Del(numberField)
			if tc.present {
				input.Set(numberField, tc.text)
			}
			w := httptest.NewRecorder()
			partForm[T](w, formRequest("/part", input, ""))
			body := html.UnescapeString(w.Body.String())
			if w.Code != http.StatusBadRequest || !strings.Contains(body, tc.message) || strings.Count(body, `role="alert"`) != 1 {
				t.Fatalf("status=%d body=%s", w.Code, body)
			}
			if !strings.Contains(body, `name="`+numberField+`" value="`+tc.text+`"`) {
				t.Fatal("submitted number was replaced")
			}
			if len(carts) != 0 {
				t.Fatal("invalid submission added a part")
			}
		})
	}
	for name := range values {
		if name == numberField {
			continue
		}
		for _, text := range []string{"", "<script>bad()</script>"} {
			input := values.Clone()
			input.Set(name, text)
			w := httptest.NewRecorder()
			partForm[T](w, formRequest("/part", input, ""))
			body := w.Body.String()
			if w.Code != http.StatusBadRequest || strings.Count(body, `role="alert"`) != 1 {
				t.Fatalf("%s=%q: status=%d body=%s", name, text, w.Code, body)
			}
			if strings.Contains(body, "<script>") || strings.Contains(body, "form||") || len(carts) != 0 {
				t.Fatal("invalid choice was unescaped, leaked a translation key, or entered the cart")
			}
		}
	}
	w := httptest.NewRecorder()
	partForm[T](w, formRequest("/part", values, ""))
	cookies := w.Result().Cookies()
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" || len(cookies) != 1 {
		t.Fatalf("valid submission: status=%d headers=%v", w.Code, w.Header())
	}
	if !reflect.DeepEqual(carts[cookies[0].Value], []part{want}) {
		t.Fatalf("saved cart=%v, want %v", carts[cookies[0].Value], want)
	}
	delete(carts, cookies[0].Value)
	w = httptest.NewRecorder()
	partForm[T](w, httptest.NewRequest(http.MethodGet, "/part", nil))
	if w.Code != http.StatusOK || strings.Count(w.Body.String(), " required>") != len(values) {
		t.Fatal("form fields are missing or not required")
	}
}

func TestPartForms(t *testing.T) {
	t.Run("nipple", func(t *testing.T) {
		checkPartForm(t, "Quantity", url.Values{"Size": {"close"}, "Quantity": {"3"}}, Nipple{Size: "close", Quantity: 3})
	})
	t.Run("pipe", func(t *testing.T) {
		checkPartForm(t, "Feet", url.Values{
			"ThicknessType": {"sch 10"}, "CoatingType": {"regular"},
			"EndType": {"plain end"}, "Size": {"8″"}, "Feet": {"12"},
		}, Pipe{ThicknessType: "sch 10", CoatingType: "regular", EndType: "plain end", Size: "8″", Feet: 12})
	})
	t.Run("mechanical tee", func(t *testing.T) {
		checkPartForm(t, "Quantity", url.Values{"Size": {"8″ × 4″"}, "Quantity": {"3"}}, MechanicalTee{Size: "8″ × 4″", Quantity: 3})
	})
	t.Run("screwed tee", func(t *testing.T) {
		checkPartForm(t, "Quantity", url.Values{
			"RunSize": {"2½″ × 1¼″"}, "BullheadSize": {"½″"}, "Quantity": {"3"},
		}, ScrewedTee{RunSize: "2½″ × 1¼″", BullheadSize: "½″", Quantity: 3})
	})
	t.Run("pipe stand", func(t *testing.T) {
		checkPartForm(t, "Quantity", url.Values{"Size": {"6″"}, "Quantity": {"3"}}, PipeStand{Size: "6″", Quantity: 3})
	})
	t.Run("additional part without shared dispatch changes", func(t *testing.T) {
		checkPartForm(t, "Quantity", url.Values{
			"Size": {"sample"}, "Finish": {"galvanized"}, "Quantity": {"3"},
		}, testPart{Size: "sample", Finish: "galvanized", Quantity: 3})
	})
}

func TestParsePositiveInt(t *testing.T) {
	max := int(^uint(0) >> 1)
	for _, want := range []int{1, 3, max} {
		got, err := parsePositiveInt(strconv.Itoa(want))
		if err != nil || got != want {
			t.Fatalf("number=%d: got %d, %v", want, got, err)
		}
	}
	for _, text := range []string{"", "0", "-1", "abc", "1.5", " 1", "1 ", "999999999999999999999999", "-999999999999999999999999"} {
		if _, err := parsePositiveInt(text); err == nil {
			t.Fatalf("accepted invalid positive integer %q", text)
		}
	}
}

func TestMalformedForm(t *testing.T) {
	resetTestState(t)
	r := httptest.NewRequest(http.MethodPost, "/nipple", strings.NewReader("Size=close&Quantity=1&extra=%ZZ"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	partForm[Nipple](w, r)
	if w.Code != http.StatusBadRequest || len(carts) != 0 {
		t.Fatal("malformed form was accepted")
	}
}

func TestInvalidQuantityLog(t *testing.T) {
	resetTestState(t)
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(old) })
	w := httptest.NewRecorder()
	partForm[Nipple](w, formRequest("/nipple", url.Values{"Size": {"close"}, "Quantity": {"abc"}}, ""))
	if w.Code != http.StatusBadRequest || strings.Contains(output.String(), "Quantity: 0") {
		t.Fatal("failed quantity parsing was accepted or logged as zero")
	}
}
