package emu

import (
	"fmt"
	"strings"
	"testing"
)

// run is one contiguous stretch of an element on a scanline: where it starts and how wide it is.
type run struct{ clock, length int }

func (r run) String() string { return fmt.Sprintf("(%d,%d)", r.clock, r.length) }

// restrobeObjectBands is the contract with scripts/gen_litmus_restrobe_objects.py: same objects,
// same widths, same spacings, same order. `sentX`/`sentLen` describe the untouched line before each
// measurement, and `want` is what the machine actually draws on the measurement line.
//
// WHAT THE TABLE SAYS, and why it is worth 62 bands. sprite-placement.md rule 11 records that the
// BALL places exactly like a missile -- same x = 3c-61, same clamp at 2 -- which is true and is
// about PLACEMENT. Three throwaway probes in a private work then read it as covering re-strobe
// behaviour too, and each measured ONE state. Swept, the three objects disagree, and they disagree
// in three different ways:
//
//	BALL     a mid-line strobe ADDS a block: 1 + k of them, at every width and every spacing here.
//	MISSILE  a mid-line strobe adds nothing. Aimed INSIDE the block that is drawing it EXTENDS that
//	         block instead -- 10 px at +6 and 9 px at +9, both past the 8 px its size field allows.
//	PLAYER   with one copy, neither happens: rule 8 cancels the pending draw and the drawn block is
//	         untouched, mid-draw or not.
//
// So rule 11 holds for where an object lands and says nothing about what a second strobe does to
// it, and this table is the witness for that scope note. If a future change makes the ball and the
// missile agree, the ladder rows and TestRestrobeObjectsDisagree both fail.
var restrobeObjectBands = []struct {
	tag     string
	el      string
	sentX   int
	sentLen int
	k       int // part A: how many re-strobes. 0 marks a part B band (one strobe, mid-draw).
	want    []run
}{
	{"M0 w2 s3 k1", "M0", 17, 2, 1, []run{{17, 2}}},
	{"M0 w2 s3 k2", "M0", 17, 2, 2, []run{{17, 2}}},
	{"M0 w2 s8 k1", "M0", 17, 2, 1, []run{{17, 2}}},
	{"M0 w2 s8 k2", "M0", 17, 2, 2, []run{{17, 2}}},
	{"M0 w2 s16 k1", "M0", 17, 2, 1, []run{{17, 2}}},
	{"M0 w2 s16 k2", "M0", 17, 2, 2, []run{{17, 2}}},
	{"M0 w8 s3 k1", "M0", 17, 8, 1, []run{{17, 8}}},
	{"M0 w8 s3 k2", "M0", 17, 8, 2, []run{{17, 8}}},
	{"M0 w8 s8 k1", "M0", 17, 8, 1, []run{{17, 8}}},
	{"M0 w8 s8 k2", "M0", 17, 8, 2, []run{{17, 8}}},
	{"M0 w8 s16 k1", "M0", 17, 8, 1, []run{{17, 8}}},
	{"M0 w8 s16 k2", "M0", 17, 8, 2, []run{{17, 8}}},
	{"M0 w8 s8 k3", "M0", 17, 8, 3, []run{{17, 8}}},
	{"M1 w2 s3 k1", "M1", 17, 2, 1, []run{{17, 2}}},
	{"M1 w2 s3 k2", "M1", 17, 2, 2, []run{{17, 2}}},
	{"M1 w2 s8 k1", "M1", 17, 2, 1, []run{{17, 2}}},
	{"M1 w2 s8 k2", "M1", 17, 2, 2, []run{{17, 2}}},
	{"M1 w2 s16 k1", "M1", 17, 2, 1, []run{{17, 2}}},
	{"M1 w2 s16 k2", "M1", 17, 2, 2, []run{{17, 2}}},
	{"M1 w8 s3 k1", "M1", 17, 8, 1, []run{{17, 8}}},
	{"M1 w8 s3 k2", "M1", 17, 8, 2, []run{{17, 8}}},
	{"M1 w8 s8 k1", "M1", 17, 8, 1, []run{{17, 8}}},
	{"M1 w8 s8 k2", "M1", 17, 8, 2, []run{{17, 8}}},
	{"M1 w8 s16 k1", "M1", 17, 8, 1, []run{{17, 8}}},
	{"M1 w8 s16 k2", "M1", 17, 8, 2, []run{{17, 8}}},
	{"M1 w8 s8 k3", "M1", 17, 8, 3, []run{{17, 8}}},
	{"BL w2 s3 k1", "BL", 17, 2, 1, []run{{17, 2}, {41, 2}}},
	{"BL w2 s3 k2", "BL", 17, 2, 2, []run{{17, 2}, {41, 2}, {50, 2}}},
	{"BL w2 s8 k1", "BL", 17, 2, 1, []run{{17, 2}, {41, 2}}},
	{"BL w2 s8 k2", "BL", 17, 2, 2, []run{{17, 2}, {41, 2}, {65, 2}}},
	{"BL w2 s16 k1", "BL", 17, 2, 1, []run{{17, 2}, {41, 2}}},
	{"BL w2 s16 k2", "BL", 17, 2, 2, []run{{17, 2}, {41, 2}, {89, 2}}},
	{"BL w8 s3 k1", "BL", 17, 8, 1, []run{{17, 8}, {41, 8}}},
	{"BL w8 s3 k2", "BL", 17, 8, 2, []run{{17, 8}, {41, 5}, {50, 8}}},
	{"BL w8 s8 k1", "BL", 17, 8, 1, []run{{17, 8}, {41, 8}}},
	{"BL w8 s8 k2", "BL", 17, 8, 2, []run{{17, 8}, {41, 8}, {65, 8}}},
	{"BL w8 s16 k1", "BL", 17, 8, 1, []run{{17, 8}, {41, 8}}},
	{"BL w8 s16 k2", "BL", 17, 8, 2, []run{{17, 8}, {41, 8}, {89, 8}}},
	{"BL w8 s8 k3", "BL", 17, 8, 3, []run{{17, 8}, {41, 8}, {65, 8}, {89, 8}}},
	{"P0 s3 k1", "P0", 18, 8, 1, []run{{18, 8}}},
	{"P0 s3 k2", "P0", 18, 8, 2, []run{{18, 8}}},
	{"P0 s8 k1", "P0", 18, 8, 1, []run{{18, 8}}},
	{"P0 s8 k2", "P0", 18, 8, 2, []run{{18, 8}}},
	{"P0 s16 k1", "P0", 18, 8, 1, []run{{18, 8}}},
	{"P0 s16 k2", "P0", 18, 8, 2, []run{{18, 8}}},
	{"P0 s8 k3", "P0", 18, 8, 3, []run{{18, 8}}},
	{"M0 w8 mid+6", "M0", 50, 8, 0, []run{{50, 10}}},
	{"M0 w8 mid+9", "M0", 50, 8, 0, []run{{50, 9}}},
	{"M0 w8 mid+12", "M0", 50, 8, 0, []run{{50, 8}}},
	{"M0 w8 mid+15", "M0", 50, 8, 0, []run{{50, 8}}},
	{"M1 w8 mid+6", "M1", 50, 8, 0, []run{{50, 10}}},
	{"M1 w8 mid+9", "M1", 50, 8, 0, []run{{50, 9}}},
	{"M1 w8 mid+12", "M1", 50, 8, 0, []run{{50, 8}}},
	{"M1 w8 mid+15", "M1", 50, 8, 0, []run{{50, 8}}},
	{"BL w8 mid+6", "BL", 50, 8, 0, []run{{50, 2}, {56, 8}}},
	{"BL w8 mid+9", "BL", 50, 8, 0, []run{{50, 5}, {59, 8}}},
	{"BL w8 mid+12", "BL", 50, 8, 0, []run{{50, 8}, {62, 8}}},
	{"BL w8 mid+15", "BL", 50, 8, 0, []run{{50, 8}, {65, 8}}},
	{"P0 mid+6", "P0", 51, 8, 0, []run{{51, 8}}},
	{"P0 mid+9", "P0", 51, 8, 0, []run{{51, 8}}},
	{"P0 mid+12", "P0", 51, 8, 0, []run{{51, 8}}},
	{"P0 mid+15", "P0", 51, 8, 0, []run{{51, 8}}},
}

// TestRestrobeObjectsSweep grades roms/litmus/litmus_restrobe_objects.bin (built by
// scripts/gen_litmus_restrobe_objects.py) band by band.
//
// HOW A BAND IS FOUND. Not by scanline number. litmus_restrobe's first grader fixed bands to line
// numbers, graded the wrong rows, and the failure read like a hardware disagreement. Here every
// band declares its own sentinel -- element, x and width -- and the anchor is the single offset at
// which EVERY band's sentinel checks out, so a wrong anchor fails on the next band instead of
// quietly grading a neighbour.
func TestRestrobeObjectsSweep(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_restrobe_objects.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	runs := func(line int, el string) []run {
		rs, _, err := e.DecomposeRow(line)
		if err != nil {
			return nil
		}
		var out []run
		for _, r := range rs {
			if r.Element == el {
				out = append(out, run{r.Clock, r.Len})
			}
		}
		return out
	}
	show := func(rs []run) string {
		var b strings.Builder
		for _, r := range rs {
			fmt.Fprintf(&b, "(%d,%d)", r.clock, r.length)
		}
		if b.Len() == 0 {
			return "nothing"
		}
		return b.String()
	}
	same := func(a, b []run) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	const stride = 3
	anchor := -1
	for cand := 20; cand < 60 && anchor < 0; cand++ {
		ok := true
		for i, band := range restrobeObjectBands {
			if !same(runs(cand+stride*i, band.el), []run{{band.sentX, band.sentLen}}) {
				ok = false
				break
			}
		}
		if ok {
			anchor = cand
		}
	}
	if anchor < 0 {
		t.Fatal("no offset makes every band's sentinel land where its row says — the fixture did " +
			"not park, or its three-line stride has changed")
	}

	for i, band := range restrobeObjectBands {
		got := runs(anchor+stride*i+1, band.el)
		t.Logf("%-16s line %3d: %s", band.tag, anchor+stride*i+1, show(got))
		if !same(got, band.want) {
			t.Errorf("band %s draws %s, want %s", band.tag, show(got), show(band.want))
		}
	}
}

// TestRestrobeObjectsDisagree states, over the pinned table, the three laws the sweep is for. It
// guards the TABLE rather than the ROM: a future edit that quietly made the ball and the missile
// agree would keep TestRestrobeObjectsSweep green, because the sweep only checks that the ROM still
// matches whatever the table says. This one checks that the table still says something.
func TestRestrobeObjectsDisagree(t *testing.T) {
	seen := map[string]int{}
	for _, band := range restrobeObjectBands {
		seen[band.el]++
		switch {
		case band.k > 0 && band.el == "BL":
			// THE BALL RE-DRAWS. One block from the park, one from each strobe.
			if len(band.want) != 1+band.k {
				t.Errorf("%s: the ball draws %d blocks, want 1+k = %d — the ladder that separates "+
					"it from the missile has gone", band.tag, len(band.want), 1+band.k)
			}
		case band.k > 0:
			// A MISSILE OR A PLAYER DOES NOT. The strobe places for the next line and adds nothing
			// to this one, however many times it is struck and however far apart.
			if len(band.want) != 1 {
				t.Errorf("%s: %s draws %d blocks after %d re-strobe(s), want 1 — if this object "+
					"now re-draws, sprite-placement.md rule 11's scope note is wrong",
					band.tag, band.el, len(band.want), band.k)
			}
		case strings.HasPrefix(band.tag, "M") && (strings.HasSuffix(band.tag, "+6") ||
			strings.HasSuffix(band.tag, "+9")):
			// A MISSILE STRUCK WHILE ITS BLOCK IS DRAWING EXTENDS THE BLOCK, past the widest its
			// size field can ask for. Nothing else in the catalogue says a missile can exceed 8 px.
			if len(band.want) != 1 || band.want[0].length <= 8 {
				t.Errorf("%s: %s, want one block wider than the 8 px NUSIZ allows",
					band.tag, band.want)
			}
		case band.el == "BL" && (strings.HasSuffix(band.tag, "+6") ||
			strings.HasSuffix(band.tag, "+9")):
			// THE BALL RESTARTS INSTEAD, and the restart CUTS the block that was drawing.
			if len(band.want) != 2 || band.want[0].length >= 8 {
				t.Errorf("%s: %s, want two blocks with the first cut short", band.tag, band.want)
			}
		case band.el == "BL":
			// +12 is the first offset that leaves both whole: the grid is 3 px, so +8 -- what
			// butting two 8 px blocks together would need -- is not a position the ball can take.
			if len(band.want) != 2 || band.want[0].length != 8 || band.want[1].length != 8 {
				t.Errorf("%s: %s, want two whole 8 px blocks", band.tag, band.want)
			}
		case band.el == "P0":
			// A PLAYER WITH ONE COPY IS UNTOUCHED mid-draw: not extended, not restarted, not cut.
			if len(band.want) != 1 || band.want[0].length != 8 {
				t.Errorf("%s: %s, want the parked block unchanged", band.tag, band.want)
			}
		}
	}
	for _, el := range []string{"M0", "M1", "BL", "P0"} {
		if seen[el] == 0 {
			t.Errorf("no bands for %s: the sweep has lost an object and can no longer show that "+
				"the four disagree", el)
		}
	}
	// The contrast itself, stated once so that "they behave alike" cannot pass silently.
	find := func(tag string) []run {
		for _, b := range restrobeObjectBands {
			if b.tag == tag {
				return b.want
			}
		}
		t.Fatalf("band %s has gone from the table", tag)
		return nil
	}
	if len(find("BL w8 s8 k2")) == len(find("M0 w8 s8 k2")) {
		t.Error("the ball and the missile now draw the same number of blocks from the same " +
			"re-strobes — rule 11 would then cover re-strobing after all, and its scope note in " +
			"sprite-placement.md must be retracted rather than left standing")
	}
}
