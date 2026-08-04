package emu

import (
	"fmt"
	"strings"
	"testing"
)

// Grading for `roms/litmus/litmus_nusiz_shape.asm` — capability gap G9(a).
//
// The claim under test is not "a player can be made wide". It is that a per-scanline
// NUSIZ + HMOVE kernel produces an INTENDED silhouette, so the intended silhouette
// has to be stated somewhere that is not the harness's own output. It is stated
// twice: as a table of drawn runs in the ROM's header, and here as a generator built
// from the band table plus the two hardware rules below. Neither was obtained by
// running the ROM and writing down what came out.
//
// The two rules are facts about the TIA, and both are anchored on fixtures that
// existed before this one:
//
//	RULE 1  copies sit 16 / 32 / 64 clocks apart (close / medium / wide); double and
//	        quad width draw 16 and 32 clocks of solid ink.
//	RULE 2  double and quad width start ONE CLOCK LATER than the 1x modes — measured
//	        on litmus_nusiz_all, which moves nothing: modes 0-4 and 6 ink from clock
//	        24, modes 5 and 7 from clock 25. TestNusizWidthModesStartOneClockLate
//	        below re-measures it on that ROM every run, so this generator cannot
//	        quietly acquire a rule of its own invention.
//
// The anchor X0 = 42 is litmus_pos's delay unit 4 (TestCoarseAdjustSweepIsWhatTheDocSays
// pins HmovedPixel 42) and a plain player inks its first clock there.

const (
	shapeInk = "FFFFFE" // COLUP0 = $0E
	shapeX0  = 42       // ÷15 delay unit 4, first inked clock

	// Gopher2600's NTSC capture window is fixed (visibleTop 29, height 214) and
	// reports this ROM's VCS scanline L at L-4, so the ROM's first shape line (VCS
	// 42) lands at visibleTop+9. TestNusizShapeFrameGeometry fails by name if that
	// stops being true, rather than letting every row grade against the wrong line.
	shapeRow0 = 9
	shapeRows = 40 // per block
	blockStep = 48
)

type shapeBand struct {
	nusiz byte
	hm    int // clocks, positive = right; applied on the band's first line
}

// The ROM's design table, restated. Five scanlines per band, eight bands.
var shapeBands = []shapeBand{
	{0x01, 0},  // forked tail
	{0x01, +4}, //
	{0x07, +4}, // body
	{0x07, +4}, //
	{0x07, +4}, //
	{0x05, +8}, // taper
	{0x05, +4}, //
	{0x00, +8}, // snout
}

// nusizRuns is RULE 1 + RULE 2: the ink a player carrying $FF draws when its
// position counter sits at pos. Returned as half-open [start,end) clock spans.
func nusizRuns(nusiz byte, pos int) [][2]int {
	switch nusiz & 7 {
	case 0:
		return [][2]int{{pos, pos + 8}}
	case 1:
		return [][2]int{{pos, pos + 8}, {pos + 16, pos + 24}}
	case 2:
		return [][2]int{{pos, pos + 8}, {pos + 32, pos + 40}}
	case 3:
		return [][2]int{{pos, pos + 8}, {pos + 16, pos + 24}, {pos + 32, pos + 40}}
	case 4:
		return [][2]int{{pos, pos + 8}, {pos + 64, pos + 72}}
	case 5:
		return [][2]int{{pos + 1, pos + 17}} // double width, one clock late
	case 6:
		return [][2]int{{pos, pos + 8}, {pos + 32, pos + 40}, {pos + 64, pos + 72}}
	}
	return [][2]int{{pos + 1, pos + 33}} // quad width, one clock late
}

// wantBand is the intended outline of one band of one block, generated from the
// design table and the masks that block applies.
func wantBand(block, band int) [][2]int {
	nusizOn := block == 0 || block == 1
	hmoveOn := block == 0 || block == 2
	n := byte(0)
	if nusizOn {
		n = shapeBands[band].nusiz
	}
	pos := shapeX0
	if hmoveOn {
		for j := 0; j <= band; j++ {
			pos += shapeBands[j].hm
		}
	}
	return nusizRuns(n, pos)
}

func loadShape(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_nusiz_shape.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

// gotRuns reads one rendered scanline and returns the ink spans. Pixels, not
// registers: what the register HELD is a proxy, what the beam PAINTED is the claim.
func gotRuns(t *testing.T, e *Emu, row int) [][2]int {
	t.Helper()
	runs, _, err := e.ReadRow(row)
	if err != nil {
		t.Fatalf("read row %d: %v", row, err)
	}
	var out [][2]int
	for _, r := range runs {
		if r.Hex == shapeInk {
			out = append(out, [2]int{r.Clock, r.Clock + r.Len})
		}
	}
	return out
}

func fmtRuns(rs [][2]int) string {
	if len(rs) == 0 {
		return "(blank)"
	}
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = fmt.Sprintf("[%d,%d)", r[0], r[1])
	}
	return strings.Join(parts, " ")
}

func sameRuns(a, b [][2]int) bool {
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

func ink(rs [][2]int) int {
	n := 0
	for _, r := range rs {
		n += r[1] - r[0]
	}
	return n
}

func shapeRow(top, block, band, line int) int {
	return top + shapeRow0 + block*blockStep + band*5 + line
}

// TestNusizShapeDrawsTheIntendedOutline is the headline measurement: 40 scanlines of
// block 0 against the silhouette the ROM's header commits to, per row, in pixels.
func TestNusizShapeDrawsTheIntendedOutline(t *testing.T) {
	e, top := loadShape(t)
	matched, totalInk, wantInk := 0, 0, 0
	for band := 0; band < 8; band++ {
		want := wantBand(0, band)
		for line := 0; line < 5; line++ {
			row := shapeRow(top, 0, band, line)
			got := gotRuns(t, e, row)
			totalInk += ink(got)
			wantInk += ink(want)
			if sameRuns(got, want) {
				matched++
				continue
			}
			t.Errorf("block 0 band %d line %d (row %d): drew %s, intended outline is %s",
				band, line, row, fmtRuns(got), fmtRuns(want))
		}
	}
	if matched != shapeRows {
		t.Errorf("the outline matches at %d of %d scanlines (ink drawn %d px, intended %d px)",
			matched, shapeRows, totalInk, wantInk)
	}
	t.Logf("outline matches at %d of %d scanlines; %d px of ink drawn against %d intended",
		matched, shapeRows, totalInk, wantInk)
	// A silhouette that never exceeds 8 px is not the thing being claimed, so the
	// fixture must be measured to actually go wide — otherwise a degenerate ROM
	// could satisfy every row above by accident of a matching degenerate table.
	widest, span := 0, 0
	for band := 0; band < 8; band++ {
		got := gotRuns(t, e, shapeRow(top, 0, band, 2))
		if n := ink(got); n > widest {
			widest = n
		}
		if len(got) > 0 && got[len(got)-1][1]-got[0][0] > span {
			span = got[len(got)-1][1] - got[0][0]
		}
	}
	if widest < 16 || span < 24 {
		t.Errorf("widest band is %d px of ink over a %d-clock span — this fixture is supposed to "+
			"exceed the 8 px a single player gives you", widest, span)
	}
}

// TestNusizShapeControlBlocksMatchTheirIntent grades the other 120 shape rows. The
// controls are only worth something if they are themselves correct.
func TestNusizShapeControlBlocksMatchTheirIntent(t *testing.T) {
	e, top := loadShape(t)
	names := map[int]string{1: "NUSIZ only", 2: "HMOVE only", 3: "flat"}
	for block := 1; block <= 3; block++ {
		matched := 0
		for band := 0; band < 8; band++ {
			want := wantBand(block, band)
			for line := 0; line < 5; line++ {
				row := shapeRow(top, block, band, line)
				got := gotRuns(t, e, row)
				if sameRuns(got, want) {
					matched++
					continue
				}
				t.Errorf("block %d (%s) band %d line %d (row %d): drew %s, intended %s",
					block, names[block], band, line, row, fmtRuns(got), fmtRuns(want))
			}
		}
		if matched != shapeRows {
			t.Errorf("block %d (%s): %d of %d scanlines match", block, names[block], matched, shapeRows)
		}
		t.Logf("block %d (%s): %d of %d scanlines match their intent", block, names[block], matched, shapeRows)
	}
}

// TestNusizShapeIsTheProductOfItsTwoAxes is the metamorphic half. The shaped block
// must equal the NUSIZ-only block translated by the HMOVE-only block's displacement,
// for all eight bands. It uses no table at all, so it survives any error in the
// intent table and catches the ones the table cannot: a kernel where the two writes
// interfere (a NUSIZ write that also disturbs the position, an HMOVE that also
// disturbs the size) breaks this relation while each block on its own still looks
// like something.
func TestNusizShapeIsTheProductOfItsTwoAxes(t *testing.T) {
	e, top := loadShape(t)
	for band := 0; band < 8; band++ {
		shaped := gotRuns(t, e, shapeRow(top, 0, band, 2))
		widthOnly := gotRuns(t, e, shapeRow(top, 1, band, 2))
		moveOnly := gotRuns(t, e, shapeRow(top, 2, band, 2))
		if len(shaped) == 0 || len(widthOnly) == 0 || len(moveOnly) == 0 {
			t.Fatalf("band %d: a block drew nothing (shaped %s, width %s, move %s)",
				band, fmtRuns(shaped), fmtRuns(widthOnly), fmtRuns(moveOnly))
		}
		shift := moveOnly[0][0] - shapeX0
		translated := make([][2]int, len(widthOnly))
		for i, r := range widthOnly {
			translated[i] = [2]int{r[0] + shift, r[1] + shift}
		}
		if !sameRuns(shaped, translated) {
			t.Errorf("band %d: shaped block drew %s; NUSIZ-only %s translated by the HMOVE-only "+
				"displacement (%+d) predicts %s", band, fmtRuns(shaped), fmtRuns(widthOnly),
				shift, fmtRuns(translated))
		}
	}
}

// TestNusizShapeFlatBlockIsMotionlessAndUnlikeTheShape is the control that catches
// the failure mode no assertion about block 0 can: both registers going inert. It
// also pins that HMOVE with HM=$00, strobed 40 times, moves nothing — the defect
// that was actually present in this fixture while it was being built.
func TestNusizShapeFlatBlockIsMotionlessAndUnlikeTheShape(t *testing.T) {
	e, top := loadShape(t)
	first := gotRuns(t, e, shapeRow(top, 3, 0, 0))
	moved := 0
	for band := 0; band < 8; band++ {
		for line := 0; line < 5; line++ {
			got := gotRuns(t, e, shapeRow(top, 3, band, line))
			if !sameRuns(got, first) {
				moved++
				if moved <= 3 {
					t.Errorf("flat block band %d line %d: drew %s, but the first row drew %s — "+
						"40 zero-motion HMOVE strobes must not displace anything",
						band, line, fmtRuns(got), fmtRuns(first))
				}
			}
		}
	}
	if moved != 0 {
		t.Errorf("the flat control moved on %d of %d scanlines", moved, shapeRows)
	}
	differ := 0
	for band := 0; band < 8; band++ {
		for line := 0; line < 5; line++ {
			if !sameRuns(gotRuns(t, e, shapeRow(top, 0, band, line)),
				gotRuns(t, e, shapeRow(top, 3, band, line))) {
				differ++
			}
		}
	}
	if differ < 30 {
		t.Errorf("the shaped block differs from the flat block on only %d of %d scanlines; if both "+
			"registers went inert every other assertion here would still be satisfiable",
			differ, shapeRows)
	}
	t.Logf("flat control static on %d of %d scanlines; the shaped block differs from it on %d of %d",
		shapeRows-moved, shapeRows, differ, shapeRows)
}

// TestNusizShapeFrameGeometry pins the frame the rows are counted in. Without it a
// shifted capture window would silently regrade every band against a neighbour.
func TestNusizShapeFrameGeometry(t *testing.T) {
	e, top := loadShape(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262 (3 VSYNC + 37 VBLANK + 192 visible + 30 overscan)", n)
	}
	for r := top; r < top+shapeRow0; r++ {
		if got := gotRuns(t, e, r); len(got) != 0 {
			t.Errorf("row %d (visibleTop+%d) drew %s; the rows above the first band must be blank — "+
				"if this fails the capture window moved and every band is being graded one line off",
				r, r-top, fmtRuns(got))
		}
	}
	for block := 0; block < 4; block++ {
		for s := 0; s < 6; s++ {
			r := top + shapeRow0 + block*blockStep + shapeRows + s
			if got := gotRuns(t, e, r); len(got) != 0 {
				t.Errorf("block %d separator row %d (row %d) drew %s, want blank", block, s, r, fmtRuns(got))
			}
		}
	}
}

// TestNusizWidthModesStartOneClockLate re-measures RULE 2 on the fixture it came
// from, so this package's generator can never drift into asserting a hardware rule
// that only this package believes.
func TestNusizWidthModesStartOneClockLate(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_nusiz_all.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	base := -1
	for mode := 0; mode < 8; mode++ {
		got := gotRuns(t, e, top+mode*24+12)
		if len(got) == 0 {
			t.Fatalf("NUSIZ mode %d drew nothing", mode)
		}
		start := got[0][0]
		if mode == 0 {
			base = start
			continue
		}
		want := base
		if mode == 5 || mode == 7 {
			want = base + 1
		}
		if start != want {
			t.Errorf("NUSIZ mode %d inks from clock %d, want %d (mode 0 inks from %d; RULE 2 says "+
				"only double and quad width start a clock late)", mode, start, want, base)
		}
	}
}
