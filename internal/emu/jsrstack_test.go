package emu

import "testing"

// TestJSRStackFactsStillHold re-checks two measurements that production code is built on and that
// nothing was re-running. `internal/cyclebound/absint.go` says *"Measured on litmus_jsr_stack, whose
// own header states the ground truth"*, and `defuse.go` cites the same ROM for the addresses a JSR
// writes. The ROM exists and the facts were taken once; no test named it, so the abstract
// interpreter's treatment of `JSR` rested on a claim about the day it was measured.
//
// Fact 1: a JSR pushes the return address FIRST, so a callee entered with the caller's SP at $FF
// opens its `pha` at **$01FD**, not $01FF. An interpreter that hands the callee the caller's SP
// unchanged names an address two above the one the machine writes — and names it `exact: true`.
// $01FD is the page-1 mirror of RAM $FD, so the pushed $A5 is readable from RAM.
//
// Fact 2: page 1 is the same address space the console decodes, so with SP aimed into the TIA a
// JSR's return address IS a pair of register writes and the callee's `pha` writes $0109 = COLUBK.
// The ROM's own header says the picture arbitrates this, so the assertion is on the picture: the
// background must CHANGE partway down the frame. Reading the colour rather than the register is
// deliberate — a register readout that disagreed with the screen would be the more interesting
// failure, and this way the screen is what answers.
func TestJSRStackFactsStillHold(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_jsr_stack.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}
	ram, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	at := func(a int) byte { return ram[a-0x80] }

	// Fact 1 — the callee's push landed at $01FD.
	if got := at(0xFD); got != 0xA5 {
		t.Fatalf("RAM $FD = $%02X, want $A5: `Save` runs `lda #$A5 / pha` and the JSR has already "+
			"pushed its return address, so SP is $FD and the byte belongs at $01FD — the page-1 "+
			"mirror of RAM $FD", got)
	}
	// ...and NOT at $01FF, which holds a return-address byte. Without this, the test would pass on a
	// machine that had written $A5 across the whole stack page.
	if at(0xFF) == 0xA5 {
		t.Fatal("RAM $FF is $A5 as well, so the push cannot be told apart from the return address; " +
			"fact 1 is precisely that $01FD and $01FF hold different things")
	}

	// Fact 2 — the picture arbitrates: the background must change partway down.
	bg := func(line int) (string, bool) {
		runs, _, err := e.ReadRow(line)
		if err != nil || len(runs) == 0 {
			return "", false
		}
		return runs[0].Hex, true
	}
	early, okE := bg(60)
	late, okL := bg(170)
	if !okE || !okL {
		t.Fatalf("could not read both rows (early ok=%v, late ok=%v)", okE, okL)
	}
	if early == late {
		t.Fatalf("background is %s on both scanline 60 and 170, so no COLUBK write happened where "+
			"the JSR put it. The callee's `pha` should have written $0109 with SP aimed into the "+
			"TIA — a tool reporting no COLUBK write here is contradicted by the screen", early)
	}
}
