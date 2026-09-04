package emu

import "testing"

// TestAskDontWaitMaskIsTheBudgetYouThrowAway measures the other way to use the RIOT timer, and the
// cost of the shortcut everyone takes when using it.
//
// **Every INTIM in this repository waits.** `timerwrap_clean.asm`, `sound_driver.asm` and three more
// all run `lda INTIM / bne loop`: ask whether the interval has finished, and stand still until it
// has. stella-list 2002 (Roger Williams) uses it the other way — **ask whether there is room for one
// more unit of work, and if not, drop the unit and carry on**:
//
//	lda #$FC / and INTIM / beq NoTime / <one unit of work> / jmp back
//
// The masked read is what makes the question cheap, and it is also what makes it lossy, because
// **the bits the mask hides are budget the program can no longer see**. Measured over a 20-unit
// `TIM64T` interval (one unit = 64 cycles):
//
//	mask   hidden bits   work done   thrown away
//	$FF        0          58 units      0 cycles
//	$FE        1          55            64      (0.8 scanlines)
//	$FC        2          49           192      (2.5 scanlines)
//	$F8        3          36           448      (5.9)
//	$F0        4          12           960      (12.6)
//	$E0        5           0          1216      (16.0)  <- never runs anything
//
// **The waste is exactly (2^k − 1) × 64 cycles**, so the 2002 post's own `$FC` gives away **2.5
// scanlines of a 16-line interval — 15 %** — to save the two cycles a full compare would cost.
//
// **And past a threshold the idiom silently does nothing.** `$E0` cannot see a value below 32; with
// a 20-unit interval the answer is "no time" from the very first poll, so the loop never executes a
// single unit — **while the frame is still 262 lines and everything else looks correct.** The rule
// that prevents it: *the mask must be able to resolve a value smaller than the interval*
// (`2^k <= INTIM at the start`). Nothing in this repository stated it, because nothing here used the
// idiom at all.
//
// Found by the mailing-list distillation (helper-1), who ranked it first among the timing items and
// gave the reason: harness's five INTIM sites are all "wait" or "measure elapsed", and none is "ask".
func TestAskDontWaitMaskIsTheBudgetYouThrowAway(t *testing.T) {
	const (
		addrUnits  = 0x84 // units completed, latched after the loop
		addrStop   = 0x81 // INTIM when the loop gave up
		addrMask   = 0x82
		addrInit   = 0x83 // INTIM immediately after writing TIM64T
		unitCycles = 64
	)
	run := func(t *testing.T, mask uint8) (units, stop, init uint8) {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM("../../roms/litmus/litmus_askdontwait.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(addrMask, mask); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		u, err := e.PeekRAM(addrUnits)
		if err != nil {
			t.Fatal(err)
		}
		s, err := e.PeekRAM(addrStop)
		if err != nil {
			t.Fatal(err)
		}
		i, err := e.PeekRAM(addrInit)
		if err != nil {
			t.Fatal(err)
		}
		return u, s, i
	}

	// The ROM writes 20 to TIM64T. Reading INTIM straight afterwards gives 19 — the first decrement
	// has already happened. `fundamentals-audit` carries the exact first-decrement offset as ⬜; this
	// does not close it, but it does pin the value a program actually sees one instruction later.
	if _, _, init := run(t, 0xFF); init != 19 {
		t.Errorf("INTIM reads %d immediately after writing 20 to TIM64T, want 19 — if this moved, "+
			"every waste figure below shifts with it", init)
	}

	// The waste is the mask's hidden range, exactly.
	for _, tc := range []struct {
		mask       uint8
		hiddenBits int
		wantStop   uint8
	}{
		{0xFF, 0, 0}, {0xFE, 1, 1}, {0xFC, 2, 3}, {0xF8, 3, 7}, {0xF0, 4, 15},
	} {
		_, stop, _ := run(t, tc.mask)
		want := uint8(1<<tc.hiddenBits) - 1
		if stop != want {
			t.Errorf("mask $%02X hides %d bits, so the poll should stop with INTIM=%d and give away "+
				"%d cycles; it stopped at %d", tc.mask, tc.hiddenBits, want, int(want)*unitCycles, stop)
		}
		if stop != tc.wantStop {
			t.Errorf("mask $%02X stopped at INTIM=%d, want %d", tc.mask, stop, tc.wantStop)
		}
	}

	// The trap: a mask coarser than the interval never sees any time at all, and fails silently.
	units, stop, init := run(t, 0xE0)
	if units != 0 {
		t.Errorf("mask $E0 cannot resolve below 32 units and the interval is %d, so the poll must "+
			"answer 'no time' on its first read and complete NO work; it completed %d units",
			init, units)
	}
	if stop != init {
		t.Errorf("mask $E0 should give up with the interval untouched (INTIM=%d), got %d", init, stop)
	}

	// Control: the fine mask must do a lot of work, or "the coarse mask does none" is not a
	// statement about the mask — it would just mean the fixture never works under any setting.
	fine, _, _ := run(t, 0xFF)
	if fine < 40 {
		t.Errorf("with the full mask the loop completed only %d units; the fixture is not exercising "+
			"the idiom and the zero above proves nothing", fine)
	}
	if fine <= units {
		t.Errorf("the fine mask (%d units) did no more work than the coarse one (%d) — the mask is "+
			"not the variable being measured", fine, units)
	}
}
