package cyclebound

import (
	"strings"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestUnitsFromBanksDeclinesByName exercises the guards that decide whether a
// bank-switched cartridge can be analysed at all.
//
// Every one of them fires only on a cartridge shape no ROM in this repo has, so
// until now they were code that had never run: the SD-11a review flagged "analysisUnits
// can return an empty unit list with no decline reason" and the guard that answers it
// had no test either way. A guard nobody exercises is indistinguishable from a guard
// that does not work.
//
// The worst outcome is the empty list: every caller reads "no decline" as permission
// to proceed, finds nothing to analyse, reports zero regions, and a zero-region
// report reads as a clean one.
func TestUnitsFromBanksDeclinesByName(t *testing.T) {
	good := func(n int) emu.CartBank {
		return emu.CartBank{Number: n, Data: make([]byte, 4096), Origins: []uint16{memorymap.OriginCartFxxx}}
	}
	hot := map[uint16]string{0x1FF8: "BANK0", 0x1FF9: "BANK1"}

	cases := []struct {
		name     string
		banks    int
		contents []emu.CartBank
		wantSaid string // a phrase the decline must contain
	}{
		{
			name: "no bank could be read at all", banks: 2, contents: nil,
			wantSaid: "no bank could be turned into an analysable image",
		},
		{
			name: "a bank came back empty", banks: 2,
			contents: []emu.CartBank{good(0), {Number: 1, Data: nil, Origins: []uint16{memorymap.OriginCartFxxx}}},
			wantSaid: "bank 1 came back empty",
		},
		{
			name: "a 2K segment, not the whole window", banks: 2,
			contents: []emu.CartBank{good(0), {Number: 1, Data: make([]byte, 2048),
				Origins: []uint16{memorymap.OriginCartFxxx}}},
			wantSaid: "rather than the whole 4K window",
		},
		{
			name: "a bank mapped at a second origin", banks: 2,
			contents: []emu.CartBank{good(0), {Number: 1, Data: make([]byte, 4096),
				Origins: []uint16{memorymap.OriginCartFxxx, memorymap.OriginCartFxxx + 2048}}},
			wantSaid: "rather than $F000 alone",
		},
		{
			name: "fewer banks readable than reported", banks: 3,
			contents: []emu.CartBank{good(0), good(1)},
			wantSaid: "analysing a subset would certify on the part that happened to be available",
		},
	}

	for _, c := range cases {
		units, decline := unitsFromBanks("TESTMAPPER", c.banks, c.contents, hot)
		if decline == "" {
			t.Errorf("%s: accepted %d unit(s) with NO decline reason — a caller reads that as "+
				"permission to proceed", c.name, len(units))
			continue
		}
		if units != nil {
			t.Errorf("%s: declined AND returned %d units; a caller that ignores the reason would "+
				"analyse them", c.name, len(units))
		}
		if !strings.Contains(decline, c.wantSaid) {
			t.Errorf("%s: decline does not say why.\n got: %s\nwant it to contain: %s",
				c.name, decline, c.wantSaid)
		}
		if !strings.Contains(decline, "TESTMAPPER") {
			t.Errorf("%s: decline does not name the mapper: %s", c.name, decline)
		}
	}

	// And the accepting case, or a function that refuses everything would pass every
	// assertion above.
	units, decline := unitsFromBanks("F8", 2, []emu.CartBank{good(0), good(1)}, hot)
	if decline != "" {
		t.Fatalf("a well-formed 2x4K cartridge was declined: %s", decline)
	}
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}
	for i, u := range units {
		if u.bank != i || u.prog == nil || len(u.hotspots) != len(hot) {
			t.Errorf("unit %d is malformed: bank=%d prog=%v hotspots=%d", i, u.bank, u.prog != nil, len(u.hotspots))
		}
	}
}
