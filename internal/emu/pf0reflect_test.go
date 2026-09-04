package emu

import "testing"

// TestPF0DoubleWriteUnderReflection settles the measurable half of an audit line that
// mixed two claims: "asymmetric PF under reflection via double PF0 rewrite per line is
// real-game practice". Whether real games do it is someone else's source. Whether the
// technique works, and where its boundaries are, is ours.
//
// Under reflection PF0 draws twice — cols 0-3 at the left, cols 36-39 at the right —
// so a second write landing between them changes one edge and not the other. The band
// sweeps that second write across the right copy in five-cycle steps and reads a probe
// standing on it.
//
// Most of the work here is in the instrument, not the result:
//
//   - Points E and F establish WHICH copy the probe reads. The four points before them
//     cannot: they set PF0 for the whole line, so both copies are always in the same
//     state and a probe on either one returns the same 1,0,1,0. E and F make the copies
//     disagree, by writing in the blind gap between them.
//
//   - That mattered. A first version used a quad-width P1 to be sure of reaching the
//     right copy; it reached the copy at the other end too, because the TIA's counter
//     wraps at 160 and a 32-px object at 151 draws 151-159 and then 0-22. E and F both
//     read 1, which is what "the probe is on both copies" looks like.
//
//   - The step is expected at the LAST index, and that is not a defect. The right copy
//     is drawn at cy ~70.7-75.7 and the line ends at 76, so no store can land after it:
//     the final point lands INSIDE the copy and splits it old|new. A sweep that shows
//     its step anywhere earlier is sweeping the wrong window — the first design swept
//     cy 40-55, the blind gap, and found nothing at all.
func TestPF0DoubleWriteUnderReflection(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_pf0_reflect.bin"); err != nil {
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

	// Band 0, part one: the probes respond to PF0 in both directions. Without the 0s
	// this says only "something is set", which is what litmus_collide_all can show and
	// why it cannot be used as a sensor.
	cal := [4]byte{at(0x88), at(0x89), at(0x8A), at(0x8B)}
	if cal != [4]byte{1, 0, 1, 0} {
		t.Fatalf("calibration = %v, want [1 0 1 0]: P0 and P1 must each read 1 over a lit "+
			"playfield and 0 over a blank one before anything below means anything", cal)
	}

	// Band 0, part two: which copy is P1 on? E = left lit / right blank, F = the reverse.
	eF := [2]byte{at(0x8C), at(0x8D)}
	switch eF {
	case [2]byte{0, 1}: // right copy — what this fixture needs
	case [2]byte{1, 0}:
		t.Fatal("E,F = 1,0: the probe is reading the LEFT copy, so the sweep below measures " +
			"the left boundary at cy ~27.7 while claiming to watch the right one")
	case [2]byte{1, 1}:
		t.Fatal("E,F = 1,1: the probe covers BOTH copies — check its width against the " +
			"160-clock wrap before trusting any column here")
	default:
		t.Fatalf("E,F = %v: the probe is on neither copy", eF)
	}

	// The sweep: exactly one step, and it is the last point, where the store lands
	// inside the copy and splits it.
	col := make([]byte, 7)
	for i := range col {
		col[i] = at(0xA0 + i)
	}
	steps, pos := at(0x92), at(0x93)
	if steps != 1 {
		t.Fatalf("column %v gives %d steps, want exactly 1: 0 means the sweep missed the "+
			"copy entirely (the blind gap between the copies is ~28 cycles wide), more than "+
			"1 means it is crossing something else as well", col, steps)
	}
	if pos != 6 {
		t.Fatalf("column %v: step at index %d, want 6. The right copy is drawn at cy ~70.7-75.7 "+
			"and the line ends at 76, so the only point that can show it is the last one",
			col, pos)
	}
	if col[0] != 0 || col[6] != 1 {
		t.Fatalf("column %v: must start blank (second write lands before the copy) and end lit "+
			"(second write lands inside it, leaving the leading half old)", col)
	}
}
