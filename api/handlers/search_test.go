package handlers

import (
	"testing"
)

func strptr(s string) *string { return &s }

// ── deriveProductType ─────────────────────────────────────────────────────────

func TestDeriveProductType_KnownTypes(t *testing.T) {
	cases := []struct {
		typ      string
		wantType string
	}{
		{"draft_booster", "Draft Booster"},
		{"collector_booster", "Collector Booster"},
		{"set_booster", "Set Booster"},
		{"play_booster", "Play Booster"},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			p := msProduct{Name: "Some Set", Type: strptr(tc.typ)}
			got, ok := deriveProductType(p)
			if !ok {
				t.Fatalf("want ok=true")
			}
			if got != tc.wantType {
				t.Errorf("want %q, got %q", tc.wantType, got)
			}
		})
	}
}

func TestDeriveProductType_ExcludedTypes(t *testing.T) {
	for _, typ := range []string{"case", "bundle", "boosterbox"} {
		t.Run(typ, func(t *testing.T) {
			p := msProduct{Name: "Some Set", Type: strptr(typ)}
			_, ok := deriveProductType(p)
			if ok {
				t.Errorf("type %q should be excluded, want ok=false", typ)
			}
		})
	}
}

func TestDeriveProductType_SleevedPrefix(t *testing.T) {
	cases := []struct {
		name     string
		typ      string
		wantType string
	}{
		{"Sleeved Draft Booster Pack", "draft_booster", "Sleeved Draft Booster"},
		{"Sleeved Collector Booster", "collector_booster", "Sleeved Collector Booster"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := msProduct{Name: tc.name, Type: strptr(tc.typ)}
			got, ok := deriveProductType(p)
			if !ok {
				t.Fatalf("want ok=true")
			}
			if got != tc.wantType {
				t.Errorf("want %q, got %q", tc.wantType, got)
			}
		})
	}
}

func TestDeriveProductType_NullType_BoosterPackByName(t *testing.T) {
	cases := []struct {
		name     string
		wantType string
	}{
		{"Jumpstart Booster Pack", "Jumpstart Booster"},
		{"Play Booster Pack", "Play Booster"},
		{"Set Booster Pack", "Set Booster"},
		{"Draft Booster Pack", "Draft Booster"},
		{"Collector Booster Pack", "Collector Booster"},
		{"Some Generic Booster Pack", "Some Generic Booster Pack"}, // default: returns full name
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := msProduct{Name: tc.name, Type: nil}
			got, ok := deriveProductType(p)
			if !ok {
				t.Fatalf("want ok=true for %q", tc.name)
			}
			if got != tc.wantType {
				t.Errorf("want %q, got %q", tc.wantType, got)
			}
		})
	}
}

func TestDeriveProductType_NullType_NoBoosterPackInName(t *testing.T) {
	for _, name := range []string{"Gift Bundle", "Commander Deck", "Fat Pack"} {
		t.Run(name, func(t *testing.T) {
			p := msProduct{Name: name, Type: nil}
			_, ok := deriveProductType(p)
			if ok {
				t.Errorf("non-booster-pack name %q should be excluded", name)
			}
		})
	}
}

func TestDeriveProductType_UnknownType_FallsThroughToNameCheck(t *testing.T) {
	// A type not in knownPackTypes and not in excludePackTypes falls through
	// to the name-based check (same as nil type).
	p := msProduct{Name: "Jumpstart Booster Pack", Type: strptr("unknown_type")}
	got, ok := deriveProductType(p)
	if !ok {
		t.Fatal("want ok=true: unknown type with booster-pack name should be included")
	}
	if got != "Jumpstart Booster" {
		t.Errorf("want %q, got %q", "Jumpstart Booster", got)
	}
}

func TestDeriveProductType_SleevedNullType(t *testing.T) {
	p := msProduct{Name: "Sleeved Draft Booster Pack", Type: nil}
	got, ok := deriveProductType(p)
	if !ok {
		t.Fatal("want ok=true")
	}
	if got != "Sleeved Draft Booster" {
		t.Errorf("want %q, got %q", "Sleeved Draft Booster", got)
	}
}
