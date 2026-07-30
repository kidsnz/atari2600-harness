package cyclebound

import (
	"strings"
	"testing"
)

// TestEdgeSemanticsAreNamedPerMapper is Stage 4 of the bank work: the cross-bank
// edge model is Atari's, and until now it was applied to any cartridge that merely
// LOOKED like Atari's.
//
// Every gate before this one is geometric — bank count, 4K per bank, one origin at
// $F000, no mapped RAM — and geometry says nothing about how a mapper picks its
// target bank. WF8 is the measured proof: 8K, two 4K banks at $F000, no RAM, and it
// publishes $1FF8:BANK0 / $1FF9:BANK1, so it clears every geometric gate. Its
// bankswitch (mapper_atari_wf8.go) responds only to $0FF8 and takes the bank from
// DATA BUS BIT 2. Modelled the old way, $1FF9 becomes an edge to bank 1 on a
// cartridge where that address does nothing, and $1FF8 becomes an edge to bank 0
// when the hardware may go to either. A wrong edge can SHORTEN the longest path.
func TestEdgeSemanticsAreNamedPerMapper(t *testing.T) {
	// The mapper IDs the engine can produce, collected from
	// Gopher2600/hardware/memory/cartridge/mapper_*.go. Single-bank and
	// RAM-overlaying mappers never reach this check (they are declined or taken as
	// flat earlier); they are listed so the coverage is visible rather than assumed.
	engineIDs := []string{
		"2K", "2KSC", "3E", "3E+", "3F", "4K", "4KSC", "BF", "BFSC", "CV", "DF", "DPC",
		"E0", "E7", "EF", "EFSC", "F4", "F4SC", "F6", "F6SC", "F8", "F8SC", "FA", "FA2",
		"FE", "JANE", "SB", "UA", "UASW", "WD", "WF8", "WFSC",
	}

	verified, declined := 0, 0
	for _, id := range engineIDs {
		why := unverifiedEdgeSemantics(id)
		if _, ok := verifiedEdgeSemantics[id]; ok {
			verified++
			if why != "" {
				t.Errorf("%s is in the verified table but was declined: %s", id, why)
			}
			continue
		}
		declined++
		if why == "" {
			t.Errorf("%s is NOT verified and was accepted anyway — the edge model would be applied to a "+
				"mapper whose switching rule nobody has read", id)
		}
		if !strings.Contains(why, id) {
			t.Errorf("%s: the decline does not name the mapper: %s", id, why)
		}
	}
	t.Logf("engine mapper IDs: %d total, %d with verified edge semantics, %d declined",
		len(engineIDs), verified, declined)

	if verified == 0 {
		t.Fatal("no mapper is verified — every banked cartridge would be declined and this test would " +
			"pass while the feature does nothing")
	}
	if declined == 0 {
		t.Fatal("no mapper is declined — the table is accepting everything, which is the state this " +
			"test exists to prevent")
	}

	// An ID the engine does not have today must still decline. This is the property
	// that survives an upstream release adding a mapper.
	if why := unverifiedEdgeSemantics("SOMETHING-NEW"); why == "" {
		t.Error("an unknown mapper ID was accepted; a new mapper upstream would be analysed under Atari's " +
			"switching rule without anyone deciding that")
	} else if !strings.Contains(why, "checked against the engine's source") {
		t.Errorf("the unknown-mapper decline does not explain itself: %s", why)
	}

	// WF8 must be declined for its ACTUAL reason, not the generic one — the whole
	// point is that it passes every other test.
	why := unverifiedEdgeSemantics("WF8")
	for _, want := range []string{"DATA BUS BIT 2", "$1FF9 does nothing"} {
		if !strings.Contains(why, want) {
			t.Errorf("the WF8 decline should say %q; got: %s", want, why)
		}
	}

	// The evidence must be evidence. Every verified entry names the engine file the
	// rule was read in, so "verified" cannot degrade into a list of hopeful IDs.
	for id, ev := range verifiedEdgeSemantics {
		if !strings.Contains(ev, ".go") {
			t.Errorf("%s's justification does not cite an engine source file: %q", id, ev)
		}
		if !strings.Contains(ev, "bankswitch") {
			t.Errorf("%s's justification does not cite the bankswitch function: %q", id, ev)
		}
	}
}

// TestCorpusBankROMsUseVerifiedMappers closes the loop through the real path: the
// mappers this repo actually analyses must be in the verified table, so the table is
// exercised rather than merely present.
func TestCorpusBankROMsUseVerifiedMappers(t *testing.T) {
	for _, c := range []struct {
		asm   string
		banks int
	}{
		{"../../roms/techniques/banked_game.asm", 2},
		{"../../roms/litmus/litmus_bank.asm", 2},
		{"../../roms/litmus/litmus_bank_f6.asm", 4},
		{"../../roms/litmus/litmus_bank_f4.asm", 8},
	} {
		rep := mustProve(t, c.asm, 76)
		if rep.BankedDeclined != "" {
			t.Errorf("%s: declined after the per-mapper gate: %s", c.asm, rep.BankedDeclined)
			continue
		}
		if rep.Banks != c.banks {
			t.Errorf("%s: analysed %d banks, expected %d", c.asm, rep.Banks, c.banks)
		}
	}
}
