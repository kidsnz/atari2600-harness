package emu

import (
	"testing"
)

// TestTextLadderIsCountNotWidth measures the axis `docs/techniques/text12.md` calls a "width
// ladder" and finds it is a **count** ladder — the character width never changes.
//
// The page says: *"width ladder is 12 (flicker-free) → 24 (column flicker) → 28 (Jentzsch) →
// 32 (interleaved)"*. Those are numbers of characters. How wide one character is appears nowhere,
// and it is the number an author drawing letters actually needs.
//
// Measured 2026-09-07, reading the rendered picture rather than the source:
//
//	text12   one band, clock  87..134   = 48 px   12 characters, no flicker
//	text24   two bands, clock 39..86 and 87..134  = 48 px each, alternating frames
//
// **48 px is one 6-store sprite block, and every rung uses the same 4x5 font: 48 / 12 = 4 px per
// character.** Going up the ladder does not make characters narrower — it adds a second block at a
// second X position and pays for it in flicker. What the rungs buy is how much of the 160-px line
// carries text; what they cost is how often each block is drawn.
//
// ★**The two bands only appear if both frame phases are sampled.** Reading one frame shows one
// 48-px band and invites the conclusion that 24 characters are squeezed into the same span at 2 px
// each. They are not. This is the same trap `phase_probe.py` exists for, met here by hand.
//
// Consequence for artwork: **a glyph wider than 4 px is not a character in any of these kernels.**
// A 10-px letter needs the 48-px block used as a picture (`bitmap48`), where it is one of roughly
// four shapes on the line, not one of twelve.
//
// Found by the mailing-list distillation (helper-2), from a 2003 thread in which three people took
// apart David Crane's routine and disagreed about whether letters could be 7 px or 8 px wide
// 〔`200309/msg00212`, `msg00216`, `msg00218`〕 — a width axis that this repository never recorded.
func TestTextLadderIsCountNotWidth(t *testing.T) {
	// band returns the leftmost and rightmost lit clock across the whole frame.
	band := func(e *Emu) (lo, hi, rows int) {
		lo, hi = 999, -1
		for ln := 0; ln < 262; ln++ {
			runs, _, err := e.ReadRow(ln)
			if err != nil || len(runs) < 3 {
				continue
			}
			rows++
			bg := runs[0].Hex
			for _, r := range runs {
				if r.Hex == bg {
					continue
				}
				if r.Clock < lo {
					lo = r.Clock
				}
				if r.Clock+r.Len-1 > hi {
					hi = r.Clock + r.Len - 1
				}
			}
		}
		return
	}

	load := func(rom string, frames int) *Emu {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/techniques/" + rom + ".bin"); err != nil {
			t.Fatalf("%s: %v", rom, err)
		}
		for i := 0; i < frames; i++ {
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
		}
		return e
	}

	// text12: a single block, 12 characters of a 4x5 font.
	lo, hi, rows := band(load("text12", 8))
	if rows == 0 {
		t.Fatal("text12 drew nothing — the reading below would be about an empty screen")
	}
	if lo != 87 || hi < 133 || hi > 134 {
		t.Errorf("text12's text band is clock %d..%d, was 87..133 on 2026-09-07. The 48-px "+
			"6-store block is where the 4-px character width comes from; if the span moved, the "+
			"width did too", lo, hi)
	}

	// text24: the SAME 48-px block twice, at two X positions, on alternate frames. Sampling one
	// frame shows one band and makes 24 characters look like 2 px each.
	loA, hiA, _ := band(load("text24", 8))
	loB, hiB, _ := band(load("text24", 9))
	if loA == loB {
		t.Fatalf("both frame phases of text24 gave the same band (clock %d..%d). Either the "+
			"alternation stopped or the sampling is landing on one phase twice — and one band is "+
			"exactly the reading that makes this ladder look like a width ladder", loA, hiA)
	}
	left, right := [2]int{loA, hiA}, [2]int{loB, hiB}
	if loB < loA {
		left, right = right, left
	}
	for _, b := range [][2]int{left, right} {
		if w := b[1] - b[0] + 1; w < 47 || w > 48 {
			t.Errorf("a text24 block spans %d px (clock %d..%d), want 48 — the block is one "+
				"6-store sprite and 48/12 is the character width this whole note is about",
				w, b[0], b[1])
		}
	}
	if left[1]+1 != right[0] {
		t.Errorf("the two blocks are at clock %d..%d and %d..%d — they are supposed to be "+
			"adjacent, giving 96 px of text on a 160-px line", left[0], left[1], right[0], right[1])
	}

	// The measurement, stated the way an author would use it.
	const charWidth = 4
	if got := (right[1] - left[0] + 1) / 24; got != charWidth {
		t.Errorf("24 characters across clock %d..%d works out at %d px each, not %d. Every rung of "+
			"the ladder uses the same 4x5 font, so this number should not move without the font "+
			"moving", left[0], right[1], got, charWidth)
	}
}
