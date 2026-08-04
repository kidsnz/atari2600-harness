package cyclebound

// A timer spin is not a loop missing a counter.
//
// `determineBound` needs a counted `dex`/`dey` or the `sbc` divide idiom. Measured
// across the sixteen-cartridge corpus, of the loops it refuses for having neither:
//
//	12   lda $0284 / bne                  <- INTIM, polled until zero
//	 7   ldy $0284 / bne                  <- the same, through Y
//	 1   sta $002A / lda $0284 / bne      <- the same, with a store in the body
//	 1   sta $xxxx,X / inx / bne          <- a genuine count, upwards
//
// Twenty of twenty-one are the same idiom: the RIOT's interval timer, spun on until
// it reaches zero. The trip count is not a property of any register this analysis
// tracks — it is whatever the hardware has left to count down, and no counter will
// ever appear however much the analysis is strengthened.
//
// So the REFUSAL is right and the REASON is wrong. "Needs a counted dex/dey" sends a
// builder looking for an analysis gap that is not there. This change names the wait
// and nothing else: no bound is invented and no region changes verdict.
//
// WHAT IT CHANGED ON THE CORPUS: nothing, by construction — 0 keys moved. The
// detector fires on 12 loops and reaches the region's reported reason on 5, the rest
// being regions that fail earlier for another reason entirely.

import (
	"strings"
	"testing"
)

func TestTimerSpinIsNamedRatherThanCalledUncounted(t *testing.T) {
	const asm = "../../roms/litmus/litmus_timerwait.asm"

	before := timerSpins
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	// PREMISE — the fixture must reach the detector.
	if got := timerSpins - before; got < 1 {
		t.Fatalf("the timer-spin detector never fired; the fixture witnesses nothing")
	}

	region := func(name string) (Region, bool) {
		for _, r := range append(append([]Region{}, rep.Lines...), rep.Unbounded...) {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				return r, true
			}
		}
		return Region{}, false
	}

	// THE ROW. Still refused — there is no bound to invent — but named.
	tr, ok := region("TimerRow")
	if !ok {
		t.Fatal("no TimerRow region")
	}
	if tr.Bounded {
		t.Errorf("TimerRow is bounded at %d; a spin on the RIOT timer has no trip count "+
			"this analysis can read, and inventing one would be an under-approximation "+
			"whenever the timer is set higher", tr.Worst)
	}
	if !strings.Contains(tr.Reason, "timer wait") || !strings.Contains(tr.Reason, "INTIM") {
		t.Errorf("TimerRow is refused for %q — it must name the wait, because "+
			"\"needs a counted dex/dey\" sends a builder looking for an analysis gap "+
			"that is not there", tr.Reason)
	}

	// CONTROL 1 — the same SHAPE on an ordinary RAM address. If this is renamed too,
	// the detector is keying on `lda abs / bne` and would call every polling loop in
	// the corpus a timer wait.
	nt, ok := region("NotTimerRow")
	if !ok {
		t.Fatal("no NotTimerRow region")
	}
	if strings.Contains(nt.Reason, "timer wait") {
		t.Errorf("NotTimerRow is called a timer wait (%q) — it polls a RAM address, and a "+
			"detector that cannot tell the two apart has stopped saying anything", nt.Reason)
	}
	if !strings.Contains(nt.Reason, "loop bound unknown") {
		t.Errorf("NotTimerRow is refused for %q, expected the generic reason", nt.Reason)
	}

	// CONTROL 2 — INTIM IS READ, BUT THE BRANCH IS NOT ABOUT IT. The `ldx` overwrites
	// Z before the latch sees it, so the loop spins on $A0 and the timer read is
	// incidental. The detector asks whether the LAST instruction to touch Z is the
	// INTIM load, precisely so this is not renamed; asking only "does the body contain
	// `lda INTIM`?" would call it a timer wait and be wrong about what the ROM does.
	fr, ok := region("FlagRow")
	if !ok {
		t.Fatal("no FlagRow region")
	}
	if strings.Contains(fr.Reason, "timer wait") {
		t.Errorf("FlagRow is called a timer wait (%q) — the latch tests the `ldx`, not the "+
			"timer, so this loop's length has nothing to do with the RIOT", fr.Reason)
	}

	// CONTROL 3 — a loop determineBound already handles. The detector runs only after
	// it has given up, so this must be untouched.
	if c, ok := region("CountedCtl"); !ok || !c.Bounded || c.Worst != 36 {
		t.Errorf("CountedCtl: bounded=%v worst=%d, expected a bound of 36 — the detector "+
			"must not intercept loops that already have a trip count", ok && c.Bounded, c.Worst)
	}
}
