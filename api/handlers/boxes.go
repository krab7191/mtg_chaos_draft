package handlers

import (
	"regexp"
	"strconv"
	"strings"
)

// Booster boxes ("displays") are tracked for the price trends page, next to
// single packs, so a box's price per pack can be compared with loose packs.

// boxKinds maps the pack type a box holds to how many packs a standard box
// has. A pack count in the name ("[12 Packs]") overrides it.
var boxKinds = []struct {
	word, label string
	packs       int
}{
	{"collector", "Collector Booster Box", 12},
	{"jumpstart", "Jumpstart Booster Box", 24},
	{"set", "Set Booster Box", 30},
	{"play", "Play Booster Box", 36},
	{"draft", "Draft Booster Box", 36},
}

// notBoosterBox: displays of decks, theme boosters and the like, cases of
// several boxes ("Case", "MasterCase"), and foreign-language boxes, which
// are priced differently from the English box.
var notBoosterBox = regexp.MustCompile(`case\b|\b(theme|deck|decks|commander|starter|intro|tournament|planeswalker|battle|prerelease|beginner|gift|scene|secret|topper|bundle|sleeved|japanese|chinese|korean|german|french|italian|spanish|portuguese|russian)\b`)

var packCountInName = regexp.MustCompile(`(\d+)\s*packs?\b`)

// deriveBoxType says whether p is a booster box, which kind, and how many
// packs it holds.
func deriveBoxType(p msProduct) (label string, packs int, ok bool) {
	name := strings.ToLower(p.Name)
	typed := p.Type != nil && *p.Type == "boosterbox"
	looksLikeBox := strings.Contains(name, "booster box") || strings.Contains(name, "booster display")
	if !typed && !looksLikeBox {
		return "", 0, false
	}
	if notBoosterBox.MatchString(name) {
		return "", 0, false
	}

	w := words(name)
	label, packs = "Booster Box", 36
	for _, k := range boxKinds {
		if w[k.word] {
			label, packs = k.label, k.packs
			break
		}
	}
	if m := packCountInName.FindStringSubmatch(name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 1 {
			packs = n
		}
	}
	return label, packs, true
}
