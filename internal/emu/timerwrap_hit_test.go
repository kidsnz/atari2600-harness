package emu

// The timer-divider detector now has a POSITIVE witness, and the audit entry that
// said it did not can be retired.
//
// `WatchTimerDividerHazard` reports a write to TIM1T/TIM8T/TIM64T/T1024T whose own
// cycles straddle the counter's underflow — on real hardware the RIOT's divider drops
// to 1T at that instant, the requested divider is lost, and the interval comes out
// 64x or 1024x short. Gopher2600 does not reproduce the race (its `Update` assigns
// the divider unconditionally), which is why this is a detector rather than a test:
// the consequence is invisible here, only the hazard can be reported.
//
// Until now nothing in the repository satisfied its condition.
// `litmus_timerwrap_nearmiss` writes just AFTER a wrap — the ordinary safe shape —
// and measures 0. `docs/capability-gap-audit.md` said so plainly: *"the positive case
// is untested ... this detector's silence must not be read as 'no hazard here'."*
//
// A detector that has never been seen to fire is not evidence.
//
// `litmus_timerwrap_hit` arms TIM1T (one tick per CPU cycle) with N = 1..12 and
// stores a divider a small, fixed number of cycles later, so the underflow lands
// inside the second store for some N and outside it for the rest. That makes the ROM
// a witness for BOTH answers: the rows that fire and the rows that do not are the
// same shape at different distances, so the boundary is measured rather than assumed.

import "testing"

func TestTimerDividerHazardFiresOnAStoreThatStraddlesTheWrap(t *testing.T) {
	const frames = 2

	fire := func(rom string) []TimerDividerHazard {
		t.Helper()
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		hz, err := e.WatchTimerDividerHazard(frames)
		if err != nil {
			t.Fatal(err)
		}
		return hz
	}

	hit := fire("../../roms/litmus/litmus_timerwrap_hit.bin")
	if len(hit) == 0 {
		t.Fatal("litmus_timerwrap_hit produced NO hazard — it exists precisely to make the " +
			"detector fire, and a detector that has never fired is not evidence")
	}
	// Six rows per frame land inside the window, over `frames` frames.
	//
	// THE COUNT IS THE ASSERTION, and the negative control that establishes it had to
	// be built twice. Adding `nop`s at the TOP of the ROM changes nothing: every row
	// opens with `sta WSYNC`, so the beam resets and each row's INTERNAL timing is
	// untouched — measured, the same six rows fire and only their addresses shift by
	// four ($F04A -> $F04E). The control that works puts the `nop`s INSIDE rows N1..N4,
	// between arming and the store, which pushes their underflow past the store's four
	// cycles: 12 hazards become 8.
	if want := 6 * frames; len(hit) != want {
		t.Errorf("%d hazards over %d frames, expected %d (6 per frame) — if the count moved, "+
			"either the RIOT's tick phase changed or the sweep's rows no longer straddle",
			len(hit), frames, want)
	}

	// THE CONDITION ITSELF, checked on every report rather than trusted: the detector
	// claims the underflow falls inside the store's own cycles.
	for _, h := range hit {
		if h.UntilWrap > h.StoreCycles {
			t.Errorf("scanline %d reports untilWrap=%d against a %d-cycle store — that is "+
				"OUTSIDE the store, so the write does not race the wrap and should not have "+
				"been reported", h.Scanline, h.UntilWrap, h.StoreCycles)
		}
		if h.UntilWrap < 0 {
			t.Errorf("scanline %d reports untilWrap=%d; a negative distance means the "+
				"pre-state was read after the wrap, not before it", h.Scanline, h.UntilWrap)
		}
		if h.Reg == "" || h.PC == 0 {
			t.Errorf("a hazard at scanline %d names neither register nor PC (%q, $%04X) — "+
				"a report a builder cannot act on", h.Scanline, h.Reg, h.PC)
		}
	}

	// THE ROWS THAT DO NOT FIRE ARE THE OTHER HALF OF THE WITNESS. The ROM sweeps
	// N = 1..12; only the ones whose underflow lands within the 4-cycle store are
	// hazards. If every row fired, the detector would be reporting the SHAPE
	// (`sta TIMxxT` soon after arming) rather than the RACE.
	lines := map[int]bool{}
	for _, h := range hit {
		lines[h.Scanline] = true
	}
	if len(lines) >= 12 {
		t.Errorf("%d distinct scanlines fired; the sweep has 12 rows and only some are "+
			"supposed to straddle. Firing on all of them means the condition has stopped "+
			"discriminating", len(lines))
	}

	// THE NEGATIVE CONTROL ROM. A write just after the wrap is the ordinary safe
	// shape every timer-driven kernel uses; flagging it would cry wolf on all of them.
	near := fire("../../roms/litmus/litmus_timerwrap_nearmiss.bin")
	if len(near) != 0 {
		t.Errorf("litmus_timerwrap_nearmiss reports %d hazard(s) (first at scanline %d, "+
			"untilWrap=%d) — it writes AFTER the wrap, which is what every timer kernel does",
			len(near), near[0].Scanline, near[0].UntilWrap)
	}

	var lo, hi int = 1 << 30, -1
	for _, h := range hit {
		if h.UntilWrap < lo {
			lo = h.UntilWrap
		}
		if h.UntilWrap > hi {
			hi = h.UntilWrap
		}
	}
	t.Logf("%d hazards over %d frames across %d distinct scanlines; untilWrap ranges %d..%d "+
		"against a %d-cycle store; near-miss ROM reports %d",
		len(hit), frames, len(lines), lo, hi, hit[0].StoreCycles, len(near))
}
