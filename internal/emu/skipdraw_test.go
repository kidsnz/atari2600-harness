package emu

import "testing"

// Grading for the skipdraw cost in `roms/techniques/vertical_pos_dcp.asm`.
//
// `docs/fundamentals-audit.md` carried "skipdraw/DoDraw **constant**-18-cycle draw" as 📖
// and added "worth a cycle litmus" — an accurate self-assessment. Measured here, the
// draw is neither constant nor 18:
//
//	sprite in range  (bcs taken)      WSYNC -> GRP0 = 20 cycles
//	sprite out of range (not taken)   WSYNC -> GRP0 = 17 cycles
//
// The ROM's own comment already said `~17-20`; the audit line said constant-18. The
// difference matters because a kernel budgeted at a constant loses three cycles on every
// line that actually draws — the lines that are already the tightest.
//
// The two paths are told apart by the branch's own cycle count rather than by reading the
// sprite state: a taken branch costs 3 (or 4 across a page), a fallthrough 2.

const (
	skipDrawInRange    = 20 // bcs taken: ldx sprDraw / lda ArtRev,x / sta GRP0
	skipDrawOutOfRange = 17 // fallthrough: lda #0 / beq / sta GRP0
	skipDrawWSYNC      = 0x02
	skipDrawGRP0       = 0x1B
	skipDrawSTAzp      = 0x85
	skipDrawBCS        = 0xB0
)

// TestSkipdrawIsSeventeenOrTwentyNotAConstantEighteen walks the kernel and times every
// WSYNC-to-GRP0 stretch, splitting them by which way the range branch went.
func TestSkipdrawIsSeventeenOrTwentyNotAConstantEighteen(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/techniques/vertical_pos_dcp.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(8); err != nil {
		t.Fatal(err)
	}
	in, out := map[int]int{}, map[int]int{}
	armed, cycles, taken := false, 0, false
	for step := 0; step < 20000; step++ {
		pc := e.PC()
		op, err := e.VCS.Mem.Read(pc)
		if err != nil {
			t.Fatalf("read opcode at $%04X: %v", pc, err)
		}
		operand, err := e.VCS.Mem.Read(pc + 1)
		if err != nil {
			t.Fatalf("read operand at $%04X: %v", pc+1, err)
		}
		tr, err := e.TraceClocks(1)
		if err != nil || len(tr) == 0 {
			break
		}
		switch {
		case op == skipDrawSTAzp && operand == skipDrawWSYNC:
			armed, cycles, taken = true, 0, false
			continue
		case !armed:
			continue
		}
		cycles += tr[0].Cycles
		if op == skipDrawBCS {
			// A taken branch costs three cycles; a fallthrough two.
			taken = tr[0].Cycles >= 3
		}
		if op == skipDrawSTAzp && operand == skipDrawGRP0 {
			if taken {
				in[cycles]++
			} else {
				out[cycles]++
			}
			armed = false
		}
	}
	if len(in) == 0 || len(out) == 0 {
		t.Fatalf("only one branch path was exercised (in=%v out=%v) — the fixture has to draw on some "+
			"lines and skip on others for this measurement to mean anything", in, out)
	}
	if len(in) != 1 {
		t.Errorf("the drawing path took %d different cycle counts (%v); it must be one", len(in), in)
	}
	if len(out) != 1 {
		t.Errorf("the skipping path took %d different cycle counts (%v); it must be one", len(out), out)
	}
	for c := range in {
		if c != skipDrawInRange {
			t.Errorf("drawing path is %d cycles from WSYNC to GRP0, want %d", c, skipDrawInRange)
		}
	}
	for c := range out {
		if c != skipDrawOutOfRange {
			t.Errorf("skipping path is %d cycles, want %d", c, skipDrawOutOfRange)
		}
	}
	if skipDrawInRange == skipDrawOutOfRange {
		t.Error("the two paths cost the same, which would make \"constant\" right — this test exists " +
			"because they do not")
	}
	t.Logf("draw %d cycles on %d lines, skip %d on %d — a spread of %d, not a constant 18",
		skipDrawInRange, in[skipDrawInRange], skipDrawOutOfRange, out[skipDrawOutOfRange],
		skipDrawInRange-skipDrawOutOfRange)
}
