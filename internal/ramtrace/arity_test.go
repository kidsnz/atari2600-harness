package ramtrace

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// A synthetic machine whose per-byte arity is known by construction, so the probe
// is checked against ground truth rather than against a plausible-looking table.
// Each byte is a deliberately different shape:
//
//	$80 counter    next = (own + 1) mod 8                 → self alone
//	$81 toggle     next = own ^ 1                         → self alone
//	$82 walker-X   next = own ± 1 on left/right           → self + input
//	$87 walker-Y   next = own ± 1 on up/down              → self + input (independent axis)
//	$83 derived    next = $82 + 5                         → 1 companion ($82)
//	$85 sum-of-two next = $82 + $87                       → 2 companions (the beam path)
//	$86 sum-of-3   next = $82 + $87 + $80                 → beyond the search bound
//	$84 constant   never written                          → constant
//
// Two deliberate choices, both learned from this test failing:
//   - the counter wraps at 8, because an un-wrapped one is unique per frame and
//     would let the probe "explain" every byte by keying on it (own test below);
//   - the two walkers are driven by independent axes and the input word is
//     aperiodic, because on a short periodic input word bytes become accidentally
//     predictable from fewer features than they truly depend on — a corpus
//     artifact rather than a property of the machine.
func simulate(inputs []string) *behavmatch.Trace {
	var st [emu.RAMSize]uint8
	st[0x80-emu.RAMBase] = 7
	st[0x82-emu.RAMBase] = 40
	st[0x84-emu.RAMBase] = 0xAA
	st[0x87-emu.RAMBase] = 90

	tr := &behavmatch.Trace{Scenario: "synthetic"}
	at := func(a uint16) uint8 { return st[a-emu.RAMBase] }

	// Frame 0 is the initial state with no input, so the first transition is
	// frame 0 -> frame 1 under inputs[0].
	tr.Samples = append(tr.Samples, behavmatch.Sample{AllRAM: st, SP: 0xFF})

	for _, in := range inputs {
		old80, old82, old87 := at(0x80), at(0x82), at(0x87)
		next := st

		next[0x80-emu.RAMBase] = (old80 + 1) & 7
		next[0x81-emu.RAMBase] = at(0x81) ^ 1
		switch in {
		case "right":
			next[0x82-emu.RAMBase] = old82 + 1
		case "left":
			next[0x82-emu.RAMBase] = old82 - 1
		case "up":
			next[0x87-emu.RAMBase] = old87 + 1
		case "down":
			next[0x87-emu.RAMBase] = old87 - 1
		}
		next[0x83-emu.RAMBase] = old82 + 5
		next[0x85-emu.RAMBase] = old82 + old87
		next[0x86-emu.RAMBase] = old82 + old87 + old80

		st = next
		smp := behavmatch.Sample{AllRAM: st, SP: 0xFF}
		if in != "" {
			smp.Inputs = behavmatch.InputState{P0: []string{in}}
		}
		tr.Samples = append(tr.Samples, smp)
	}
	return tr
}

// inputWord is an aperiodic script from a fixed 16-bit LFSR — deterministic
// across runs, but with no short period for a byte to accidentally ride on.
func inputWord(n int) []string {
	actions := []string{"", "right", "left", "up", "down", "right", "up", ""}
	lfsr := uint16(0xACE1)
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		bit := (lfsr ^ (lfsr >> 2) ^ (lfsr >> 3) ^ (lfsr >> 5)) & 1
		lfsr = (lfsr >> 1) | (bit << 15)
		out = append(out, actions[lfsr&7])
	}
	return out
}

func arityOf(t *testing.T, rep *ArityReport, addr string) ByteArity {
	t.Helper()
	for _, b := range rep.Bytes {
		if b.Addr == addr {
			return b
		}
	}
	t.Fatalf("no arity entry for %s", addr)
	return ByteArity{}
}

func hasFeature(b ByteArity, want string) bool {
	for _, f := range b.Features {
		if f == want {
			return true
		}
	}
	return false
}

func syntheticReport() *ArityReport {
	traces := map[string]*behavmatch.Trace{"synthetic": simulate(inputWord(400))}
	return Arity(Provenance{ROM: "synthetic"}, traces)
}

func TestArityRecoversKnownArities(t *testing.T) {
	rep := syntheticReport()

	// Self-determined bytes must not be charged for the input or a companion —
	// over-reporting arity would make every byte look harder than it is.
	for _, addr := range []string{"$80", "$81"} {
		b := arityOf(t, rep, addr)
		if !b.Resolved || b.Companions != 0 || b.NeedsInput {
			t.Errorf("%s: resolved=%v companions=%d needs_input=%v method=%q — want self alone",
				addr, b.Resolved, b.Companions, b.NeedsInput, b.Method)
		}
	}

	// The walkers cannot be explained without knowing what the player held.
	for _, addr := range []string{"$82", "$87"} {
		b := arityOf(t, rep, addr)
		if !b.Resolved || b.Companions != 0 || !b.NeedsInput {
			t.Errorf("%s: resolved=%v companions=%d needs_input=%v method=%q — want self+input",
				addr, b.Resolved, b.Companions, b.NeedsInput, b.Method)
		}
	}

	// A derived byte must be traced to the byte it derives from, by name.
	if b := arityOf(t, rep, "$83"); !b.Resolved || b.Companions != 1 || !hasFeature(b, "$82") {
		t.Errorf("$83: resolved=%v companions=%d features=%v — want 1 companion, $82",
			b.Resolved, b.Companions, b.Features)
	}

	// Two-companion bytes must be found by the pair search, naming both sources.
	if b := arityOf(t, rep, "$85"); !b.Resolved || b.Companions != 2 ||
		!hasFeature(b, "$82") || !hasFeature(b, "$87") {
		t.Errorf("$85: resolved=%v companions=%d features=%v — want 2 companions, $82 and $87",
			b.Resolved, b.Companions, b.Features)
	}

	// Beyond the search bound the probe must say so rather than inventing a fit.
	if b := arityOf(t, rep, "$86"); b.Resolved {
		t.Errorf("$86 needs three companions but was reported resolved: %v %v", b.Method, b.Features)
	}

	// A byte nobody writes is constant, not "resolved with 0 companions" noise.
	if b := arityOf(t, rep, "$84"); !b.Constant {
		t.Errorf("$84: constant=%v — want constant", b.Constant)
	}
	if rep.Live != 7 {
		t.Errorf("live bytes = %d, want 7 ($80 $81 $82 $83 $85 $86 $87)", rep.Live)
	}
	if rep.Unresolved != 1 {
		t.Errorf("unresolved = %d, want 1 ($86)", rep.Unresolved)
	}
}

// The probe must not claim a resolution it cannot back up: an unresolved byte
// carries the residual conflict count and a method that names its own bound.
func TestArityUnresolvedIsHonest(t *testing.T) {
	b := arityOf(t, syntheticReport(), "$86")
	if b.Companions != -1 {
		t.Errorf("unresolved byte reports companions=%d, want -1", b.Companions)
	}
	if b.Residual <= 0 {
		t.Errorf("unresolved byte reports residual=%d, want >0", b.Residual)
	}
	if b.Method == "" || b.Features != nil {
		t.Errorf("unresolved byte should name its bound and claim no features: method=%q features=%v",
			b.Method, b.Features)
	}
}

// The trap this probe was rewritten to survive. A byte that takes a fresh value
// every single frame identifies the frame, so keying on it reproduces the whole
// recording — and a naive search then declares that all of RAM has arity 1. The
// probe must name such bytes, prefer any other explanation, and mark a resolution
// that had no choice but to lean on one.
func TestArityDetectsFrameCounterTrap(t *testing.T) {
	var st [emu.RAMSize]uint8
	st[0x91-emu.RAMBase] = 1
	tr := &behavmatch.Trace{Scenario: "counter"}
	tr.Samples = append(tr.Samples, behavmatch.Sample{AllRAM: st, SP: 0xFF})
	for f := 1; f < 100; f++ {
		next := st
		next[0x80-emu.RAMBase] = uint8(f)                 // free-running frame counter
		next[0x90-emu.RAMBase] = uint8(f * 3)             // only the counter explains this
		next[0x91-emu.RAMBase] = st[0x91-emu.RAMBase] ^ 1 // an honest self-contained rule
		st = next
		tr.Samples = append(tr.Samples, behavmatch.Sample{AllRAM: st, SP: 0xFF})
	}
	rep := Arity(Provenance{ROM: "counter"}, map[string]*behavmatch.Trace{"counter": tr})

	found := false
	for _, f := range rep.FrameIndexLike {
		if f == "$80" {
			found = true
		}
	}
	if !found {
		t.Errorf("free-running counter $80 not flagged as frame-counter-like: %v", rep.FrameIndexLike)
	}

	// $90 genuinely has no other explanation, so it may resolve — but the verdict
	// must carry a warning rather than reading as a clean arity-1 result.
	if b := arityOf(t, rep, "$90"); b.Resolved && !(b.Memorising || b.UsesFrameIndex) {
		t.Errorf("$90 resolved on the counter with no warning: %+v", b)
	}

	// The byte with an honest self-rule must NOT be attributed to the counter.
	if b := arityOf(t, rep, "$91"); !b.Resolved || b.Companions != 0 || b.UsesFrameIndex {
		t.Errorf("$91 has a self-contained rule but was resolved as %v (companions=%d frame-index=%v)",
			b.Features, b.Companions, b.UsesFrameIndex)
	}
}

// Activity describes; it must not conclude. Check the numbers it reports against
// the synthetic machine.
func TestActivityDescribesTheSyntheticMachine(t *testing.T) {
	tr := simulate(inputWord(400))
	rep := Analyse(Provenance{ROM: "synthetic"}, map[string]*behavmatch.Trace{"synthetic": tr})

	if rep.LiveCount != 7 || rep.DeadCount != emu.RAMSize-7 {
		t.Errorf("live=%d dead=%d, want 7 / %d", rep.LiveCount, rep.DeadCount, emu.RAMSize-7)
	}
	if len(rep.CollisionsSeen) != 0 {
		t.Errorf("collisions seen = %v, want none in a synthetic trace", rep.CollisionsSeen)
	}
	if rep.StackLow != "$FF" {
		t.Errorf("stack low = %s, want $FF", rep.StackLow)
	}
	find := func(addr string) Activity {
		for _, a := range rep.Bytes {
			if a.Addr == addr {
				return a
			}
		}
		t.Fatalf("no activity entry for %s", addr)
		return Activity{}
	}
	// The toggle takes exactly two values and moves by exactly +1 and -1.
	if a := find("$81"); a.Distinct != 2 || len(a.Deltas) != 2 {
		t.Errorf("$81: distinct=%d deltas=%v — want 2 values, 2 deltas", a.Distinct, a.Deltas)
	}
	// The walker only ever steps by one in either direction.
	if a := find("$82"); len(a.Deltas) != 2 || a.Deltas[0] != -1 || a.Deltas[1] != 1 {
		t.Errorf("$82 deltas = %v, want [-1 1]", a.Deltas)
	}
	// An untouched byte is constant and is not counted as changing.
	if a := find("$84"); !a.Constant || a.FramesChanged != 0 || a.FirstChange != -1 {
		t.Errorf("$84: constant=%v changed=%d first=%d", a.Constant, a.FramesChanged, a.FirstChange)
	}
}
