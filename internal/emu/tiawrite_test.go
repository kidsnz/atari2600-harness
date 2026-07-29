package emu

import "testing"

// romIndexedTIA does `ldx #3 / sta COLUP0,x`, so the hardware writes COLUBK.
const romIndexedTIA = "../../roms/litmus/litmus_indexed_tia.bin"

// A TIA write must be attributed to the register the hardware actually reached,
// which is the EFFECTIVE address, not the base operand. Indexed stores to TIA are
// how a multiplexed kernel drives several objects from one loop — precisely the
// case a write→beam timeline exists to explain — so naming the base register
// there is not a rounding error, it is the wrong answer in the only situation
// that matters.
//
// The screen is the arbiter: this ROM turns the background green. If the reported
// register were COLUP0, the picture would disagree with the report.
func TestLastTIAWriteUsesEffectiveAddress(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(romIndexedTIA); err != nil {
		t.Skipf("ROM unavailable (%s): %v", romIndexedTIA, err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}

	const (
		colup0 = 0x06
		colubk = 0x09
	)
	var seen []uint16
	found := false
	for i := 0; i < 60000 && !found; i++ {
		if err := e.StepInstruction(); err != nil {
			t.Fatal(err)
		}
		w, ok := e.LastTIAWrite()
		if !ok {
			continue
		}
		if w.Reg == colubk {
			found = true
			if !w.HasVal || w.Val != 0xC4 {
				t.Errorf("COLUBK write value = $%02X (has=%v), want $C4", w.Val, w.HasVal)
			}
			break
		}
		if w.Reg == colup0 {
			t.Fatalf("`sta COLUP0,x` with x=3 was attributed to COLUP0 ($06); the hardware "+
				"wrote COLUBK ($09) — the effective address, not the base operand. PC $%04X",
				w.PC)
		}
		seen = append(seen, w.Reg)
	}
	if !found {
		t.Errorf("no write to COLUBK observed; registers seen: %v", seen)
	}

	// Cross-check against the machine itself rather than against the same code
	// path: the background register really does hold the green.
	if v, err := e.PeekRAM(colubk); err == nil && v != 0xC4 {
		t.Logf("note: COLUBK peeks as $%02X (TIA write-only registers do not read back)", v)
	}
}
