package emu

import (
	"testing"
)

// TestTIM64TReachesZeroAfterFortyTwoIntervals settles a number this repository had wrong by a
// scanline, and pins the resolution of the measurement alongside it.
//
// Two answers were in circulation:
//
//	docs/capability-gap-audit.md    43 * 64      = 2752
//	Erik Mooney, stella-list 2004   1 + 42 * 64  = 2689
//
// The difference matters because the wait is usually spent on WSYNC: 2752/76 = 36.2 rounds to
// **37 lines**, 2689/76 = 35.4 rounds to **36**.
//
// Mooney's account is that the first decrement is immediate — write 43 and INTIM reads 42 at once,
// then 64 cycles per step down to zero. `fundamentals-audit.md` already measures the same shape for
// a different value (write 20, read 19 on the next instruction).
//
// Measured here with `roms/litmus/litmus_tim64t_zero.asm`, which writes 43, records what INTIM
// returns on the very next instruction, spins until INTIM reads zero, and parks a marker. The cycle
// count comes from the emulator rather than from a counting loop inside the ROM, because a counting
// loop measures itself.
//
//	INTIM immediately after the write   42          (not 43)
//	write -> zero observed              2692 cycles (three consecutive periods, identical)
//
// ★**2692 is not 2689, and the three-cycle gap is this measurement's resolution, not a
// disagreement.** The spin is `LDA INTIM` (4) + `BNE` (3) = **7 cycles per poll**, and what is
// timestamped is the end of the instruction that saw the zero, so the true crossing is somewhere in
// the preceding seven cycles. 2689 sits inside that window. 2752 sits **nine polls** outside it.
//
// The two markers are read seven cycles after the events they mark — `$81` is stored 7 cycles after
// the write completes, `$80` 7 cycles after the zero is read — so the offsets cancel in the
// subtraction and the difference is the interval itself.
//
// Predicted as 2689 by the mailing-list distillation (helper-2) before it was run.
func TestTIM64TReachesZeroAfterFortyTwoIntervals(t *testing.T) {
	const (
		pollCycles = 7    // LDA INTIM (4) + BNE (3): the resolution of the reading below
		mooney     = 2689 // 1 + 42*64
		measured   = 2692
	)

	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_tim64t_zero.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}

	var intervals []int64
	var afterWrite uint8
	wrote := int64(-1)
	p80, p81 := uint8(0), uint8(0)
	for i := 0; i < 4_000_000 && len(intervals) < 4; i++ {
		if err := e.StepInstruction(); err != nil {
			break
		}
		v81, _ := e.PeekRAM(0x81)
		v80, _ := e.PeekRAM(0x80)
		if v81 != 0 && p81 == 0 {
			wrote = e.TotalCycles()
			afterWrite = v81
		}
		if v80 == 1 && p80 == 0 && wrote >= 0 {
			intervals = append(intervals, e.TotalCycles()-wrote)
			wrote = -1
		}
		p80, p81 = v80, v81
	}

	if afterWrite != 42 {
		t.Fatalf("wrote 43 to TIM64T and the next instruction read INTIM = %d, want 42. The first "+
			"decrement is supposed to have happened already; if it has not, every timer budget in "+
			"this repository is off by one interval", afterWrite)
	}

	// The first interval is caught part-way through a loop already in progress, so it is short by
	// construction and is not evidence about anything. The rest must agree exactly with each other.
	if len(intervals) < 4 {
		t.Fatalf("only %d intervals observed, want 4 (one partial and three whole)", len(intervals))
	}
	whole := intervals[1:]
	for _, v := range whole {
		if v != whole[0] {
			t.Fatalf("the timer interval is not stable across periods: %v. Everything below assumes "+
				"one number", intervals)
		}
	}
	got := whole[0]

	if got != measured {
		t.Errorf("write-to-zero measured %d cycles, was %d on 2026-09-07. The number moved, so the "+
			"note in capability-gap-audit.md and the WSYNC line count derived from it are stale",
			got, measured)
	}
	if d := got - mooney; d < 0 || d > pollCycles {
		t.Errorf("measured %d, which is %d cycles from Mooney's %d — outside the %d-cycle poll "+
			"resolution, so this is a real disagreement and not the granularity of the reading",
			got, d, mooney, pollCycles)
	}
	// And the two-sided half: the old number must stay refuted, or this test stops meaning anything.
	if old := int64(43 * 64); got > old-int64(pollCycles) {
		t.Errorf("measured %d is within a poll of the old %d — the two answers are no longer "+
			"distinguishable and the correction in capability-gap-audit.md should be revisited",
			got, old)
	}
}
