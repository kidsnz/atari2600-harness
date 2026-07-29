package cyclebound

import (
	"strings"
	"testing"
)

// A superchip overlays 128 bytes of RAM on $F000-$F0FF, so the image is not what
// the CPU reads there. That is not a cosmetic distinction for this package:
// romTableRange folds real cartridge bytes into an EXACT value range, that range
// bounds a loop's trip count, and the trip count sets the proven worst case. Fold
// a RAM address and the result is a narrow, confident, wrong number in the one
// direction the package forbids.
//
// The hole was measured, not theorised: analysisUnits accepted any mapper with
// more than one bank that published hotspots, and asked nothing about RAM.
//
// Presence is FINGERPRINTED from the bytes rather than declared — the engine
// requires every bank's first page to mirror its own two halves — so the same
// image is F8 or F8SC depending on the engine's decision. litmus_superchip
// satisfies that pattern deliberately, which is why the analysis must ASK the
// engine instead of inferring from the size.
func TestSuperchipImageIsDeclinedByName(t *testing.T) {
	rep := mustProve(t, "../../roms/litmus/litmus_superchip.asm", 76)
	if rep.BankedDeclined == "" {
		t.Fatal("a superchip cartridge was analysed: its bottom page is RAM, so any value range " +
			"folded from the image there bounds a loop on data the hardware never holds")
	}
	if !strings.Contains(rep.BankedDeclined, "F8SC") {
		t.Errorf("the decline must name the mapper the ENGINE fingerprinted, so a size-based guess "+
			"and the emulator cannot disagree about which machine is described; got %q",
			rep.BankedDeclined)
	}
	if !strings.Contains(rep.BankedDeclined, "superchip") {
		t.Errorf("the decline must say WHY, not just that it declined; got %q", rep.BankedDeclined)
	}
	if rep.Certified {
		t.Error("a declined image must never certify")
	}
}

// The guard must not spread: a plain F8 cartridge has no overlay and must still be
// analysed per bank. A check that refuses everything is sound and useless.
func TestPlainBankedImageStillAnalysed(t *testing.T) {
	rep := mustProve(t, "../../roms/litmus/litmus_bank.asm", 76)
	if rep.BankedDeclined != "" {
		t.Fatalf("a plain F8 cartridge was declined: %q", rep.BankedDeclined)
	}
	if rep.Regions == 0 {
		t.Error("premise broken: litmus_bank should expose WSYNC regions")
	}
	if rep.Banks != 2 {
		t.Errorf("expected 2 banks, got %d", rep.Banks)
	}
}
