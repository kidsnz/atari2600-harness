package emu

import "testing"

// TestTheUnnamed2002SequenceIsAPeriod63Generator answers a question the archive left open.
//
// stella-list 200201/msg00015. Manuel Polik posts five instructions lifted from a disassembly:
//
//	LDA $82 / LSR / ADC #$00 / LSR / ROR $82
//
// and asks "Is this some sort of a random number/sequence generator, or what is in $82?", adding
// that "the carry state on entry of this part may vary". **The thread is two messages long and
// nobody answered.** It is a question about the 6507, not about anyone's opinion, so it can be
// closed by measurement twenty-four years later.
//
// The claim came from the distillation (helper-3), who enumerated the map in Python off the
// machine: a bijection whose orbits are one fixed point, one 3-cycle and **four** 63-cycles, with
// the entry carry irrelevant because the first LSR overwrites it. Arithmetic done off the machine
// is exactly the kind of claim this repository does not write down until a second implementation
// agrees, so `litmus_lsradcror.asm` runs the real instructions on the real CPU and — this is the
// part that matters — walks **all 256 seeds**, not the three that happen to be convenient.
//
// The measured answer: **period 63 from 252 of the 256 seeds**, 252/63 = exactly four cycles,
// plus one fixed point and one 3-cycle. Nothing else. So the sequence IS a generator, and a weak
// one: a given seed only ever visits 63 values and returns to its exact starting point after 63
// frames — **1.05 seconds at 60 Hz**, short enough that anything driven by it repeats visibly.
func TestTheUnnamed2002SequenceIsAPeriod63Generator(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_lsradcror.bin"); err != nil {
		t.Fatal(err)
	}
	// ★30 frames, measured not guessed: the census walks 256 seeds at up to 63 steps each with no
	// WSYNC in between, so it takes many frames to finish. Measured progress: 3->19, 8->63,
	// 15->126, 20->170, **30->256**. Reading it early is the failure mode the first assertion
	// below exists to catch — and it did catch it, at 3 frames, before this number was measured.
	if err := e.RunFrames(30); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	const (
		per1     = 0x00 // $80
		per2     = 0x01
		per3     = 0x03
		carryOK  = 0x04
		fixed    = 0x05
		cens1    = 0x08 // $88: seeds of period 1
		cens3    = 0x09
		cens63   = 0x0A
		censOthr = 0x0B
		orbit    = 0x10 // $90
	)

	// --- the census is the whole claim: every seed, classified, adding up to 256 -------------
	total := int(r[cens1]) + int(r[cens3]) + int(r[cens63]) + int(r[censOthr])
	if total != 256 {
		t.Fatalf("the census classified %d seeds, not 256 — the ROM did not finish its walk and "+
			"every number below is a fragment (period1=%d period3=%d period63=%d other=%d)",
			total, r[cens1], r[cens3], r[cens63], r[censOthr])
	}
	if got := int(r[censOthr]); got != 0 {
		t.Errorf("%d seeds have a period that is neither 1, 3 nor 63 — the orbit structure is not "+
			"what the enumeration claimed, and the 63-frame repeat cannot be stated as a fact", got)
	}
	if got := int(r[cens63]); got != 252 {
		t.Errorf("%d seeds have period 63, want 252 — 252 is four whole 63-cycles, and the count "+
			"is what makes 'almost any seed repeats after 63 frames' true rather than typical", got)
	}
	if got := int(r[cens63]) % 63; got != 0 {
		t.Errorf("the period-63 seeds do not divide into whole cycles (%d left over) — an orbit "+
			"cannot have a partial cycle, so the measurement itself is wrong", got)
	}
	if got := int(r[cens1]); got != 1 {
		t.Errorf("%d fixed points, want 1 — $00 maps to itself and nothing else should", got)
	}
	if got := int(r[cens3]); got != 3 {
		t.Errorf("%d seeds of period 3, want 3 (one 3-cycle)", got)
	}

	// --- the three sampled seeds must agree with the census -----------------------------------
	for _, tc := range []struct {
		seed string
		got  uint8
	}{{"$01", r[per1]}, {"$02", r[per2]}, {"$03", r[per3]}} {
		if tc.got != 63 {
			t.Errorf("seed %s has period %d, want 63", tc.seed, tc.got)
		}
	}

	// --- $00 is the fixed point ---------------------------------------------------------------
	if r[fixed] != 0 {
		t.Errorf("stepping $00 gives $%02X, want $00 — Polik's post says the variable starts at 0 "+
			"in the first frame, so if zero is not a fixed point the generator never starts", r[fixed])
	}

	// --- Polik's own worry: does the entry carry change the answer? ---------------------------
	if r[carryOK] != 1 {
		t.Error("stepping $01 with the carry SET gives a different result from stepping it CLEAR — " +
			"the entry carry matters after all, and Polik's 'the carry state on entry may vary' " +
			"is a real hazard rather than a false alarm. Whatever else this test says, that " +
			"sentence in the docs is now wrong")
	}

	// --- the head of the orbit, pinned so a change in the CPU's ADC/ROR shows up here ---------
	// Measured, not derived: seed $01 walks 128 64 32 16 8 4 2 129 192 96 48 24 12 6 131 65.
	want := []uint8{128, 64, 32, 16, 8, 4, 2, 129, 192, 96, 48, 24, 12, 6, 131, 65}
	for i, w := range want {
		if got := r[orbit+i]; got != w {
			t.Errorf("orbit step %d is %d, want %d — the sequence's values have moved, which means "+
				"LSR, ADC or ROR is behaving differently than when this was pinned", i+1, got, w)
			break
		}
	}
}
