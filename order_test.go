package main

import (
	"fmt"
	"testing"
)

// This part intentionally has no String or formatting method.
type testPart struct {
	Size     string
	Quantity int
	Finish   string
}

func (testPart) choices(field string) []string {
	switch field {
	case "Size":
		return []string{"sample"}
	case "Finish":
		return []string{"galvanized"}
	}
	return nil
}

func TestAutomaticPartFormatting(t *testing.T) {
	for _, tc := range []struct {
		p    part
		want string
	}{
		{Nipple{Size: "close", Quantity: 3}, "Nipple: Size: close, Quantity: 3"},
		{Pipe{ThicknessType: "sch 10", CoatingType: "regular", EndType: "plain end", Size: "8″", Feet: 3},
			"Pipe: ThicknessType: sch 10, CoatingType: regular, EndType: plain end, Size: 8″, Feet: 3"},
		{MechanicalTee{Size: "8″ × 4″", Quantity: 3}, "MechanicalTee: Size: 8″ × 4″, Quantity: 3"},
		{PipeStand{Size: "6″", Quantity: 3}, "PipeStand: Size: 6″, Quantity: 3"},
		{testPart{Size: "6″", Quantity: 3, Finish: "GALVANIZED"}, "testPart: Size: 6″, Quantity: 3, Finish: GALVANIZED"},
	} {
		if got := formatPart(tc.p); got != tc.want {
			t.Fatalf("%T: got %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestOrderString(t *testing.T) {
	o := order{
		items: []part{
			Nipple{Size: "close", Quantity: 2},
			PipeStand{Size: "6″", Quantity: 3},
			Pipe{ThicknessType: "sch 10", CoatingType: "regular", EndType: "plain end", Size: "8″", Feet: 12},
		},
	}
	want := "Nipple: Size: close, Quantity: 2\n" +
		"PipeStand: Size: 6″, Quantity: 3\n" +
		"Pipe: ThicknessType: sch 10, CoatingType: regular, EndType: plain end, Size: 8″, Feet: 12\n"
	if got := fmt.Sprintf("%s", o); got != want {
		t.Fatalf("order text=%q, want %q", got, want)
	}
}
