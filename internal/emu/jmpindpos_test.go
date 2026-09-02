package emu

import "testing"

// Grading for `roms/litmus/litmus_jmpind_pos.asm` — indirect-jump object positioning.
//
// The claim, from the Stella list in July 2002 (Erik Mooney, 200207/msg00330 and
// msg00334): a kernel does not need a cycle-counting delay loop to place an object.
// Prepare N hardcoded positioning lines, compute a pointer off-screen, and `JMP (ptr)`
// into the right one. Each extra variant delays the RESPx strobe by 5 CPU cycles, so
// consecutive variants land the object 15 colour clocks apart. The fine nibble written
// to HMPx before the strobe, applied by a following HMOVE, fills in between.
//
// Manuel Polik's arithmetic in the same thread (msg00332), which is what makes the
// technique practical and which nobody had checked: "I'd only need 9 kernel parts, as
// there are only 128 horizontal positions." Nine variants span 8*15 = 120 clocks and
// HMOVE reaches -7..+8, sixteen values. The reachable set is contiguous ONLY because
// the fine range is at least the coarse step: intervals [15k-7, 15k+8] and
// [15k+8, 15k+23] meet exactly at their endpoint. One fewer fine value and the set has
// holes. TestJmpIndPosCoverageIsContiguous derives that from the measured step and the
// measured range rather than from these constants.
//
// The band table is a PERMUTATION. A kernel that positioned by band index instead of by
// the dispatched variant would draw a monotone ramp, so the ROM never draws one, and
// TestJmpIndPosCoarseStepIsFifteenClocks fails by name if the drawn sequence is sorted.

const (
	jipInk     = "FFFFFE" // COLUP0 = $0E, the dispatched one-pixel object
	jipCtrlInk = "EC3333" // COLUPF = $46, the ball: strobed once per frame, never moved

	// Band geometry, measured from the rendered frame and pinned by
	// TestJmpIndPosFrameGeometry. Each band is 5 scanlines: setup, dispatch+RESP0,
	// HMOVE, and two more. The first four rows of a band carry its final position;
	// the fifth is where the next band's strobe can suppress the draw.
	jipBand0Row  = 10
	jipBandLines = 5
	jipGradeRows = 4

	jipCoarseStep = 15 // 5 CPU cycles between variants x 3 colour clocks per cycle
)

// jipCoarseOrder is the order the coarse variants appear in, not the variants
// themselves. Deliberately unsorted.
var jipCoarseOrder = []int{4, 0, 5, 2, 8, 1, 6, 3, 7}

// jipFineNibbles is the HMPx high nibble each fine band writes, in band order.
var jipFineNibbles = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// hmoveClocks is the rightward displacement HMOVE applies for a motion nibble: the
// nibble is a 4-bit two's-complement value and the hardware moves the object the
// other way, so $10 is one clock LEFT and $80 is eight clocks RIGHT.
func hmoveClocks(nibble int) int {
	if nibble >= 8 {
		nibble -= 16
	}
	return -nibble
}

func loadJmpIndPos(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_jmpind_pos.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

// jipObjectX returns the clock of the one-pixel object of the given colour on a
// rendered scanline, or -1 when it is not on that line.
func jipObjectX(t *testing.T, e *Emu, row int, hex string) int {
	t.Helper()
	runs, _, err := e.ReadRow(row)
	if err != nil {
		t.Fatalf("read row %d: %v", row, err)
	}
	for _, r := range runs {
		if r.Hex == hex {
			if r.Len != 1 {
				t.Errorf("row %d: the %s object is %d px wide, want 1 — this fixture only "+
					"means anything while the object is a single pixel", row, hex, r.Len)
			}
			return r.Clock
		}
	}
	return -1
}

// jipBandX reads one band's position and fails if the band's own rows disagree, which
// would mean the position is being read off a transitional line rather than a settled
// one.
func jipBandX(t *testing.T, e *Emu, top, band int) int {
	t.Helper()
	first := top + jipBand0Row + band*jipBandLines
	x := jipObjectX(t, e, first, jipInk)
	if x < 0 {
		t.Fatalf("band %d: the object is not drawn on its first graded row (%d)", band, first)
	}
	for i := 1; i < jipGradeRows; i++ {
		got := jipObjectX(t, e, first+i, jipInk)
		if got != x {
			t.Fatalf("band %d: row %d puts the object at %d but row %d puts it at %d — the band "+
				"is not settled, so every position read here is off a moving target",
				band, first, x, first+i, got)
		}
	}
	return x
}

// TestJmpIndPosCoarseStepIsFifteenClocks is the headline measurement: nine bands, each
// dispatched by `JMP (ptr)` into a different hardcoded positioning line, land the
// object 15 colour clocks apart in the order the pointer table names — not in the order
// the bands are drawn.
func TestJmpIndPosCoarseStepIsFifteenClocks(t *testing.T) {
	e, top := loadJmpIndPos(t)

	// The anchor is the one measured input: variant 0's position. Everything else is
	// arithmetic checked against pixels.
	anchorBand := -1
	for i, k := range jipCoarseOrder {
		if k == 0 {
			anchorBand = i
		}
	}
	if anchorBand < 0 {
		t.Fatal("the coarse order does not contain variant 0, so there is no anchor")
	}
	x0 := jipBandX(t, e, top, anchorBand)

	drawn := make([]int, len(jipCoarseOrder))
	for i, k := range jipCoarseOrder {
		got := jipBandX(t, e, top, i)
		drawn[i] = got
		want := x0 + jipCoarseStep*k
		if got != want {
			t.Errorf("band %d dispatches variant %d: object at clock %d, the %d-clock step says %d "+
				"(anchor: variant 0 at %d)", i, k, got, jipCoarseStep, want, x0)
		}
	}

	// The control that no equation above can replace. Every want above is computed
	// from the variant index, so a kernel that positioned by BAND index would produce
	// a sorted ramp and could only pass if the table happened to be sorted too.
	sorted := true
	for i := 1; i < len(drawn); i++ {
		if drawn[i] < drawn[i-1] {
			sorted = false
			break
		}
	}
	if sorted {
		t.Errorf("the nine drawn positions %v are in ascending order — this fixture is only "+
			"evidence of dispatch while they are not, and the table order %v is not sorted",
			drawn, jipCoarseOrder)
	}

	span := jipCoarseStep * (len(jipCoarseOrder) - 1)
	t.Logf("9 variants, %d clocks apart, anchored at %d: %v (span %d clocks)",
		jipCoarseStep, x0, drawn, span)
}

// TestJmpIndPosFineNibbleIsItsSignedValue grades the other half of the technique: the
// HMPx nibble loaded before the dispatch and applied by the HMOVE after it.
func TestJmpIndPosFineNibbleIsItsSignedValue(t *testing.T) {
	e, top := loadJmpIndPos(t)
	base := -1
	for i, n := range jipFineNibbles {
		band := len(jipCoarseOrder) + i
		got := jipBandX(t, e, top, band)
		if n == 0 {
			base = got
			continue
		}
		if base < 0 {
			t.Fatal("the fine sweep does not start with nibble 0, so there is no unmoved anchor")
		}
		want := base + hmoveClocks(n)
		if got != want {
			t.Errorf("fine nibble $%X0: object at clock %d, want %d (%+d from the unmoved %d)",
				n, got, want, hmoveClocks(n), base)
		}
	}
	t.Logf("16 nibbles anchored on the unmoved position %d, reaching %d..%d",
		base, base+hmoveClocks(7), base+hmoveClocks(8))
}

// TestJmpIndPosCoverageIsContiguous is Manuel Polik's 2002 arithmetic, done on the
// measured step and the measured HMOVE range instead of on the constants above: nine
// hardcoded lines reach every one of 128 horizontal positions, and they do so only
// because the fine range is at least as wide as the coarse step.
func TestJmpIndPosCoverageIsContiguous(t *testing.T) {
	e, top := loadJmpIndPos(t)

	// Measure the coarse step rather than assuming it.
	var atVariant [9]int
	for i, k := range jipCoarseOrder {
		atVariant[k] = jipBandX(t, e, top, i)
	}
	step := atVariant[1] - atVariant[0]
	for k := 2; k < len(atVariant); k++ {
		if d := atVariant[k] - atVariant[k-1]; d != step {
			t.Fatalf("the coarse step is not constant: variant %d is %d clocks past variant %d, "+
				"but variant 1 is %d past variant 0", k, d, k-1, step)
		}
	}

	// Measure the fine range rather than assuming it.
	lo, hi := 0, 0
	base := -1
	for i, n := range jipFineNibbles {
		if n == 0 {
			base = jipBandX(t, e, top, len(jipCoarseOrder)+i)
		}
	}
	if base < 0 {
		t.Fatal("the fine sweep has no nibble-0 band, so there is no unmoved anchor to measure from")
	}
	for i := range jipFineNibbles {
		d := jipBandX(t, e, top, len(jipCoarseOrder)+i) - base
		if d < lo {
			lo = d
		}
		if d > hi {
			hi = d
		}
	}
	fineValues := hi - lo + 1

	if fineValues < step {
		t.Errorf("the fine range spans %d values (%+d..%+d) but the coarse step is %d clocks — "+
			"the reachable set has holes, and %d hardcoded lines do NOT cover a contiguous run",
			fineValues, lo, hi, step, len(atVariant))
	}
	reach := step*(len(atVariant)-1) + fineValues
	if reach < 128 {
		t.Errorf("nine variants reach %d contiguous positions, short of the 128 the 2002 thread "+
			"claims (step %d, fine range %d)", reach, step, fineValues)
	}
	t.Logf("step %d clocks x %d variants + fine range %d (%+d..%+d) = %d contiguous positions "+
		"(>= 128 claimed in 200207/msg00332)", step, len(atVariant), fineValues, lo, hi, reach)
}

// TestJmpIndPosStaticControlNeverMoves is the control an engine-wide nudge could not
// survive. The ball is strobed once per frame in VBLANK and carries HMBL = 0 through
// every HMOVE the picture does; it must hold one clock for the whole picture.
func TestJmpIndPosStaticControlNeverMoves(t *testing.T) {
	e, top := loadJmpIndPos(t)
	bands := len(jipCoarseOrder) + len(jipFineNibbles)
	x0 := jipObjectX(t, e, top+jipBand0Row, jipCtrlInk)
	if x0 < 0 {
		t.Fatalf("the static control is not drawn on row %d", top+jipBand0Row)
	}
	moved, firstBad := 0, -1
	rows := bands * jipBandLines
	for r := 0; r < rows; r++ {
		row := top + jipBand0Row + r
		got := jipObjectX(t, e, row, jipCtrlInk)
		if got < 0 {
			t.Fatalf("the static control is missing from row %d", row)
		}
		if got != x0 {
			moved++
			if firstBad < 0 {
				firstBad = row
			}
		}
	}
	if moved != 0 {
		t.Errorf("the never-restrobed control moved on %d of %d picture rows (first at row %d) — "+
			"%d HMOVE strobes with HMBL = $00 must displace nothing", moved, rows, firstBad, bands)
	}
	t.Logf("static control held clock %d on all %d picture rows", x0, rows)
}

// TestJmpIndPosFrameGeometry pins the window the bands are counted in. Everything above
// indexes rows off jipBand0Row, so if the capture window moves, every band is graded
// against the wrong variant and the equations would still be satisfiable by accident.
func TestJmpIndPosFrameGeometry(t *testing.T) {
	e, top := loadJmpIndPos(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262 (3 VSYNC + 37 VBLANK + 192 picture + 30 overscan)", n)
	}
	// Band 0 must not have started yet one row earlier, or the whole index is off.
	first := jipBandX(t, e, top, 0)
	if before := jipObjectX(t, e, top+jipBand0Row-1, jipInk); before == first {
		t.Errorf("row %d already shows band 0's position (clock %d) — the first graded row is "+
			"late and every band is being read one row into the next",
			top+jipBand0Row-1, before)
	}
	// And the last band must end where the count says it does.
	bands := len(jipCoarseOrder) + len(jipFineNibbles)
	last := jipBandX(t, e, top, bands-1)
	after := jipObjectX(t, e, top+jipBand0Row+bands*jipBandLines+2, jipInk)
	if after != last {
		t.Errorf("two rows past the last band the object is at %d, not the %d it was left at — "+
			"the picture is longer or shorter than %d bands of %d lines",
			after, last, bands, jipBandLines)
	}
	t.Logf("262-line frame, top = %d, %d bands of %d lines from row %d",
		top, bands, jipBandLines, jipBand0Row)
}
