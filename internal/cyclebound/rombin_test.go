package cyclebound

import (
	"os"
	"testing"
)

// Commercial cartridges, used here only as inputs — nothing is read from them
// but their own bytes. 2K and 4K images both appear because the address a 2K
// cartridge is seen at is exactly what this file is about.
var romBins = map[string]string{
	"VideoOlympics(2K)": "../../reference/roms-study/VideoOlympics.bin",
	"Outlaw(2K)":        "../../../sandbox/studies/outlaw/Outlaw.bin",
	"Combat(2K)":        "../../../sandbox/studies/combat/Combat_1977_Atari.bin",
	"Stampede(4K)":      "../../reference/pizza-boy/Samples for Pizza Boy/Stampede.bin",
	"Frogger(4K)":       "../../../roms/frogger/frogger.bin",
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
