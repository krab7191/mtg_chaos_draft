package handlers

import "testing"

func TestDeriveBoxType(t *testing.T) {
	box := strptr("boosterbox")
	cases := []struct {
		name  string
		typ   *string
		label string
		packs int
		ok    bool
	}{
		{"Play Booster Display", box, "Play Booster Box", 36, true},
		{"Collector Booster Display", box, "Collector Booster Box", 12, true},
		{"Set Booster Box", nil, "Set Booster Box", 30, true},
		{"Draft Booster Box", box, "Draft Booster Box", 36, true},
		{"Jumpstart Booster Display", box, "Jumpstart Booster Box", 24, true},
		{"Booster Box", box, "Booster Box", 36, true},
		{"Collector Booster Pack Display [12 Packs]", nil, "", 0, false}, // "booster pack display" isn't "booster display"
		{"Booster Display [24 Packs]", nil, "Booster Box", 24, true},
		{"Collector Booster Display MasterCase", box, "", 0, false},
		{"Play Booster Display Case", nil, "", 0, false},
		{"Basic Booster Display (Japanese)", box, "", 0, false},
		{"Theme Booster Display Box", nil, "", 0, false},
		{"Commander Deck Display", nil, "", 0, false},
		{"Play Booster Pack", nil, "", 0, false},
	}
	for _, tc := range cases {
		label, packs, ok := deriveBoxType(msProduct{Name: tc.name, Type: tc.typ})
		if ok != tc.ok || label != tc.label || packs != tc.packs {
			t.Errorf("%q: got (%q, %d, %v), want (%q, %d, %v)", tc.name, label, packs, ok, tc.label, tc.packs, tc.ok)
		}
	}
}
