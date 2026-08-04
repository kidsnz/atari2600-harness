package emu

import (
	"path/filepath"
	"testing"
)

// TestCartridgeRAMIsRareAndNamed pins the fact that decides where the
// "read of a write-only cartridge hotspot / SuperChip write port" trap can be
// caught at all.
//
// docs/known-traps.md marks that row "static/manual", i.e. a source-text linter
// could flag it. It cannot, and the reason is measurable rather than arguable: what
// makes a read of $F000-$F07F a trap is the cartridge MAPPING RAM there, and the
// mapper is not in the .asm text. Measured over the same 123 ROMs the trap linter
// scans, exactly ONE maps cartridge RAM (litmus_superchip), while the only source
// line that reads into that range — litmus_6502.asm:52, `lda $F010,x` — belongs to a
// plain 4K image where it is ordinary code. A naive static rule would score one false
// positive and zero true ones.
//
// The check therefore belongs at ROM level, where emu.MapsCartridgeRAM answers first.
// This test guards the premise: if a second ROM starts mapping RAM, or if
// litmus_superchip stops, the reasoning above needs re-doing and the trap row with it.
// It is also the premise of cyclebound's SD-8c decline.
//
// SCOPE, since 2026-08-04: `roms/carts` holds five more ROMs that map cartridge RAM
// (F6SC, F4SC, 3E, 3E+, DPC), and they are deliberately NOT in this glob. This test
// is about the corpus the TRAP LINTER scans, and the trap linter scans litmus and
// techniques. The cartridge-format fixtures are graded by advcart_test.go instead.
// If they were folded in here the "exactly one" premise would become "exactly six"
// and would stop saying anything about the trap row it exists to justify.
func TestCartridgeRAMIsRareAndNamed(t *testing.T) {
	var files []string
	for _, pat := range []string{"../../roms/techniques/*.bin", "../../roms/litmus/*.bin"} {
		f, _ := filepath.Glob(pat)
		files = append(files, f...)
	}
	if len(files) < 100 {
		t.Skipf("only %d ROMs found; the corpus is not present", len(files))
	}

	loaded := 0
	var mapsRAM []string
	for _, f := range files {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(f); err != nil {
			continue // not every .bin in the tree is a loadable cartridge
		}
		loaded++
		ok, err := e.MapsCartridgeRAM()
		if err != nil {
			t.Errorf("%s: MapsCartridgeRAM: %v", filepath.Base(f), err)
			continue
		}
		if ok {
			mapsRAM = append(mapsRAM, filepath.Base(f))
		}
	}

	if loaded < 100 {
		t.Fatalf("only %d of %d ROMs loaded; the measurement below would be over a corpus that is "+
			"mostly missing", loaded, len(files))
	}
	if len(mapsRAM) != 1 || mapsRAM[0] != "litmus_superchip.bin" {
		t.Errorf("ROMs mapping cartridge RAM = %v, expected exactly [litmus_superchip.bin]. If a second "+
			"one has appeared, the write-port trap is now reachable in the corpus and the known-traps "+
			"row needs re-deciding; if the list is empty, litmus_superchip has stopped being a superchip "+
			"and cyclebound's SD-8c decline has no witness", mapsRAM)
	}
	t.Logf("%d ROMs loaded, %d map cartridge RAM: %v", loaded, len(mapsRAM), mapsRAM)
}
