package cyclebound

import (
	"os"
	"testing"
)

// Commercial cartridges, used here only as inputs — nothing is read from them
// but their own bytes. 2K and 4K images both appear because the address a 2K
// cartridge is seen at is exactly what this file is about.
// The two `reference/` paths were written with TWO levels of `..` and the umbrella
// tree is THREE up from this package, so both entries pointed at
// harness/reference/… — a directory that does not exist. They took the skip on every
// run since they were added: measured here, VideoOlympics and Stampede reported "ROM
// unavailable" while Outlaw, Combat and Frogger passed, and the test was green the
// whole time. Two of the five subjects of a decoder test were absent and it read as a
// pass, which is the same failure this file's own doc comment warns about one
// paragraph down ("an analysis that finds no instructions does not look wrong").
var romBins = map[string]string{
	"VideoOlympics(2K)": "../../../reference/roms-study/VideoOlympics.bin",
	"Outlaw(2K)":        "../../../sandbox/studies/outlaw/Outlaw.bin",
	"Combat(2K)":        "../../../sandbox/studies/combat/Combat_1977_Atari.bin",
	"Stampede(2K)":      "../../../reference/pizza-boy/Samples for Pizza Boy/Stampede.bin",
	// frogger.bin is a BUILD PRODUCT and gitignored (roms/.gitignore), so this entry silently
	// skips on any machine that has not run `go run ./frogger/gen` + dasm. It is listed anyway
	// because the corpus is meant to include the flagship; the skip is the honest state, not a
	// pass. Regenerate with: cd roms && go run ./frogger/gen && dasm frogger/frogger.asm -f3 -ofrogger/frogger.bin
	"Frogger(4K)": "../../../roms/260610_frogger/frogger.bin",
}

func loadROMBin(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("ROM unavailable (%s): %v", path, err)
	}
	return b
}

// The prover has to be able to see the code in a cartridge it is handed. A 2K
// cartridge occupies only half the 4K window, so the console sees it at two
// addresses and a game's vectors may point at either — the decoder has to accept
// both or it decodes an empty program and reports nothing, confidently.
//
// Silence is the dangerous failure here: an analysis that finds no instructions
// does not look wrong, it looks like a clean ROM.
func TestDecodeReachesCodeInCommercialROMs(t *testing.T) {
	for name, path := range romBins {
		t.Run(name, func(t *testing.T) {
			rom := loadROMBin(t, path)
			p := newProgram(rom)
			instrs, entries := p.decodeFromVectors()
			if len(entries) == 0 {
				t.Fatalf("%s: no reset/NMI/IRQ vector pointed into the cartridge", name)
			}
			if len(instrs) < 50 {
				t.Errorf("%s (%d bytes): decoded only %d instructions from %d entry point(s) — "+
					"a real game has hundreds; the decoder is not seeing the cartridge",
					name, len(rom), len(instrs), len(entries))
			}
			t.Logf("%s: %d bytes, %d entries, %d instructions decoded", name, len(rom), len(entries), len(instrs))
		})
	}
}
