package emu

import (
	"testing"
)

// TestAFreeRunningCounterCarriesNoEntropyOfItsOwn measures where the randomness in a 1997 idiom
// actually lives, and finds that the obvious way to check for it does not work.
//
// Eckhard Stolberg, telling someone their generator repeated too visibly: *"Having a **counter run
// from 0 to 6, that increments in every frame**, gave enough ramdomness for my Tetris version"*
// 〔`199706/msg00005`; `ramdomness` is his spelling〕. Seven values is exactly the number of
// tetrominoes, so there is not even a modulo. Manuel Polik names the other half of the problem:
// *"Total randomness won't work, since you've to **REPEAT** what you're doing every frame"*.
//
// So there are two needs and `roadmap.md` carried only the expensive one — *"#7 LFSR pseudo-random —
// cheap, **repeatable** randomness"*. A starfield or a terrain must be **reproducible**, and that is
// what a fixed-seed LFSR is for. A tetromino must only be **unpredictable**, and a counter is enough,
// because the entropy is not in the counter at all.
//
// `roms/litmus/litmus_counter_entropy.asm` runs 0..6 once per frame and tallies the value on each
// frame the fire button GOES DOWN. Driven three ways over 140 frames:
//
//	presses every 7 frames    [0 0 0 0 0 20 0]   every sample is the same value
//	presses every 3 frames    [7 7 6 7 7 7 6]    all seven, evenly, and entirely predictable
//	irregular gaps            [2 4 4 4 3 1 3]    spread
//
// ★**The counter contributes nothing by itself** — synchronise the presses with it and the output is
// a constant. Everything that looks like randomness came from **when a person pressed**.
//
// ★★**And the middle row is the one worth remembering: it is MORE uniform than the irregular case
// and completely deterministic.** A histogram cannot tell entropy from a well-chosen period. So
// "the distribution looks flat" is not evidence that a generator is unpredictable, and testing this
// idiom by counting outcomes would have passed the period-3 driver — which an auto-fire button is.
//
// Found by the mailing-list distillation (helper-2).
func TestAFreeRunningCounterCarriesNoEntropyOfItsOwn(t *testing.T) {
	tally := func(press func(frame int) bool) (t7 [7]int, total int) {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_counter_entropy.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 140; f++ {
			if err := e.SetInput(0, "fire", press(f)); err != nil {
				t.Fatal(err)
			}
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
		}
		for i := range t7 {
			v, _ := e.PeekRAM(uint16(0x90 + i))
			t7[i] = int(v)
			total += int(v)
		}
		return
	}

	distinct := func(t7 [7]int) int {
		n := 0
		for _, v := range t7 {
			if v > 0 {
				n++
			}
		}
		return n
	}

	// Synchronised with the counter: the sampled value can never change.
	sync7, n7 := tally(func(f int) bool { return f%7 == 0 })
	if n7 == 0 {
		t.Fatal("no press was recorded at period 7 — the litmus is not sampling and nothing below " +
			"means anything")
	}
	if d := distinct(sync7); d != 1 {
		t.Errorf("presses every 7 frames produced %d distinct values %v, want exactly 1. A counter "+
			"sampled at its own period is a constant; if it is not, the counter is not free-running "+
			"or the sampling is not on the press edge", d, sync7)
	}

	// Period 3: every value, evenly, and utterly predictable.
	sync3, n3 := tally(func(f int) bool { return f%3 == 0 })
	if n3 == 0 {
		t.Fatal("no press was recorded at period 3")
	}
	if d := distinct(sync3); d != 7 {
		t.Errorf("presses every 3 frames reached %d of the 7 values %v, want all 7 — 3 and 7 are "+
			"coprime so the sample walks the whole cycle", d, sync3)
	}

	// Irregular gaps.
	gaps := []int{5, 8, 3, 11, 6, 4, 9, 7, 13, 5, 2, 10}
	next, gi := 0, 0
	irregular, ni := tally(func(f int) bool {
		if f == next {
			next = f + gaps[gi%len(gaps)]
			gi++
			return true
		}
		return false
	})
	if ni == 0 {
		t.Fatal("no press was recorded on the irregular driver")
	}

	// The lesson, as a check: the DETERMINISTIC driver is at least as flat as the irregular one.
	// If this ever stops being true the note above — that a histogram cannot detect entropy — has
	// to be rewritten, because then it could.
	spread := func(t7 [7]int, total int) float64 {
		var worst float64
		mean := float64(total) / 7
		for _, v := range t7 {
			d := float64(v) - mean
			if d < 0 {
				d = -d
			}
			if r := d / mean; r > worst {
				worst = r
			}
		}
		return worst
	}
	flat3, flatIrr := spread(sync3, n3), spread(irregular, ni)
	if flat3 > flatIrr {
		t.Errorf("the period-3 driver (worst deviation %.2f) is less even than the irregular one "+
			"(%.2f). The point of this test is that a flat histogram proves nothing about "+
			"unpredictability, and it rests on the deterministic driver being the flatter of the two",
			flat3, flatIrr)
	}
}
