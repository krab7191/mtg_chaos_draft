package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

// Tests against a real MTGStocks /sealed response (every set MTGStocks
// tracks), so parsing is checked against how MTGStocks actually names
// products, not just hand-written fixtures.
//
// Refresh the snapshot by gzipping a fresh download of
// https://api.mtgstocks.com/sealed into testdata/mtgstocks_sealed.json.gz,
// then regenerate the golden file and review its diff:
//
//	go test ./handlers -run TestRealCatalog_Golden -update

var updateGolden = flag.Bool("update", false, "rewrite golden files")

const (
	goldenProductsPath = "testdata/sealed_products.golden.json"
	goldenBoxesPath    = "testdata/sealed_boxes.golden.json"
)

func loadRealCatalog(t *testing.T) (*catalog, []msSet) {
	t.Helper()
	f, err := os.Open("testdata/mtgstocks_sealed.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	c, err := parseCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	var sets []msSet
	if err := json.Unmarshal(raw, &sets); err != nil {
		t.Fatal(err)
	}
	return c, sets
}

// TestRealCatalog_Golden pins the exact search catalog built from the real
// snapshot. Any filter change shows up as a reviewable diff of this file.
func TestRealCatalog_Golden(t *testing.T) {
	c, _ := loadRealCatalog(t)
	checkGolden(t, goldenProductsPath, c.products)
	checkGolden(t, goldenBoxesPath, c.boxes)
}

func checkGolden(t *testing.T, path string, items []SearchResult) {
	t.Helper()
	type row struct {
		Set   string   `json:"set"`
		Type  string   `json:"type"`
		Name  string   `json:"name"`
		ID    int      `json:"id"`
		Packs int      `json:"packs,omitempty"`
		Price *float64 `json:"price"`
	}
	rows := make([]row, len(items))
	for i, p := range items {
		rows[i] = row{p.SetName, p.ProductType, p.Name, p.MTGStocksID, p.PacksPerBox, p.MarketPrice}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		t.Fatal(err)
	}

	if *updateGolden {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("catalog changed; review with -update and git diff %s", path)
	}
}

// TestRealCatalog_OnlySinglePacks: nothing that holds many packs or isn't a
// draftable booster may reach search.
func TestRealCatalog_OnlySinglePacks(t *testing.T) {
	c, _ := loadRealCatalog(t)
	banned := []string{"theme booster", "display", "case", "bundle", "box", "battle pack", "3-booster", "omega", "prerelease", "set of", "sleeved"}
	for _, p := range c.products {
		name := strings.ToLower(p.Name)
		for _, b := range banned {
			if strings.Contains(name, b) {
				t.Errorf("%s: %q (%s) should not be searchable", p.SetName, p.Name, p.ProductType)
			}
		}
	}
}

// TestRealCatalog_EveryBoosterIsFound: for every set, each product whose name
// is a plain single booster (with or without set/sleeved prefixes) must give
// that set a search entry of the same pack type.
func TestRealCatalog_EveryBoosterIsFound(t *testing.T) {
	c, sets := loadRealCatalog(t)
	have := map[string]bool{}
	for _, p := range c.products {
		have[p.SetName+"|"+p.ProductType] = true
	}
	for _, s := range sets {
		for _, p := range s.Products {
			name := strings.ToLower(p.Name)
			for _, kind := range []string{"Play Booster", "Collector Booster", "Set Booster", "Draft Booster"} {
				if strings.HasSuffix(name, strings.ToLower(kind)) && !strings.Contains(name, "sleeved") && !have[s.Name+"|"+kind] {
					t.Errorf("%s: %q exists but %q is not searchable", s.Name, p.Name, kind)
				}
			}
		}
	}
}

// TestRealCatalog_KnownSets spot-checks sets that have broken before.
func TestRealCatalog_KnownSets(t *testing.T) {
	c, _ := loadRealCatalog(t)
	want := map[string][]string{
		"Reality Fracture":    {"Play Booster", "Collector Booster"},
		"Edge of Eternities":  {"Play Booster", "Collector Booster"},
		"Tarkir: Dragonstorm": {"Play Booster", "Collector Booster"},
	}
	for set, types := range want {
		for _, typ := range types {
			found := false
			for _, p := range c.products {
				if p.SetName == set && p.ProductType == typ {
					found = p.MarketPrice != nil
				}
			}
			if !found {
				t.Errorf("%s %s missing or unpriced", set, typ)
			}
		}
	}
}

// TestRealCatalog_BoxesAreSingleEnglishBoosterBoxes: no cases, deck displays
// or foreign-language boxes, and every box knows its pack count.
func TestRealCatalog_BoxesAreSingleEnglishBoosterBoxes(t *testing.T) {
	c, _ := loadRealCatalog(t)
	banned := []string{"case", "deck", "theme", "japanese", "commander", "starter"}
	for _, b := range c.boxes {
		name := strings.ToLower(b.Name)
		for _, w := range banned {
			if strings.Contains(name, w) {
				t.Errorf("%s: %q should not be a booster box", b.SetName, b.Name)
			}
		}
		if b.PacksPerBox < 2 {
			t.Errorf("%s %s: packs per box %d", b.SetName, b.Name, b.PacksPerBox)
		}
	}
	want := map[string]string{
		"Teenage Mutant Ninja Turtles|Collector Booster Box": "Collector Booster Display",
		"Bloomburrow|Play Booster Box":                       "Play Booster Display",
		"Wilds of Eldraine|Set Booster Box":                  "",
	}
	have := map[string]string{}
	for _, b := range c.boxes {
		have[b.SetName+"|"+b.ProductType] = b.Name
	}
	for k, name := range want {
		got, ok := have[k]
		if !ok || (name != "" && got != name) {
			t.Errorf("%s: want %q, got %q (found=%v)", k, name, got, ok)
		}
	}
}
