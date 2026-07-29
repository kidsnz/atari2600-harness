package ramtrace

import "testing"

// A free-running counter uniquely identifies the frame, so keying on it "explains"
// every other byte perfectly and the probe reports that RAM has arity 1 —
// memorisation wearing a model's clothes. frameIndexLike exists to find such
// features and try them last.
//
// Its first definition required the value to be distinct on EVERY transition, and
// a counter that WRAPS does not satisfy that. Measured on Outlaw: $DA takes 256
// distinct values, changes on all 4266 transitions, and its only deltas are +1 and
// -255 — a frame counter that cycles. It failed the test purely because the
// recording outlives its period, and 27 of the 35 bytes reported as resolved were
// then explained by keying on it.
//
// With wrapping counters included, Outlaw's honest figures move from
// resolved 35 / unresolved 9 to resolved 23 / unresolved 21. The tool now names
// $DA in its output as a byte that can key anything.
func TestWrappingCounterCountsAsAClock(t *testing.T) {
	// A byte that visits nearly every value and changes every frame, but wraps:
	// 0,1,2,...,255,0,1,... over more transitions than its period.
	const n = 600
	mk := func(f func(i int) uint8) []transition {
		out := make([]transition, 0, n)
		for i := 0; i < n; i++ {
			var t transition
			t.prev[0] = f(i)
			out = append(out, t)
		}
		return out
	}
	ts := mk(func(i int) uint8 { return uint8(i % 256) })
	if !frameIndexLike(ts, 0) {
		t.Error("a byte cycling 0..255 on every one of 600 transitions was not recognised as a clock; " +
			"keying on it explains every other byte and the probe reports arity 1 for all of RAM")
	}

	// A byte with a small repeating pattern is NOT a clock: it carries real state and
	// must stay usable as a companion, or the detector would refuse every ordinary
	// game variable.
	if frameIndexLike(mk(func(i int) uint8 { return uint8(i % 4) }), 0) {
		t.Error("a byte cycling over 4 values was called a clock; an ordinary state variable would then " +
			"be pushed to the back of every search")
	}

	// A constant is not a clock either.
	if frameIndexLike(mk(func(int) uint8 { return 7 }), 0) {
		t.Error("a constant byte was called a clock")
	}
}
