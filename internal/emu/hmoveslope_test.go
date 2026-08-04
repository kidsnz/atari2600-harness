package emu

import "testing"

// Grading for `roms/litmus/litmus_hmove_slope.asm` — capability gap G9(b).
//
// HMOVE moves an object by a whole colour clock, so the only slopes a per-line strobe
// can draw by itself are 0 and +-1 px per scanline. The claim under test is that an
// 8-bit error accumulator, emitting a move only on the lines that carry, draws an
// ARBITRARY angle — which means the angle has to be stated as an equation and the
// drawn pixel measured against it, line by line, in pixels.
//
// The equation is fixed by the kernel's shape and not by its output: the accumulator
// is cleared before the band, line n's carry is consumed by line n+1's HMOVE, so
//
//	x(n) = x(0) + sign * floor(n * num / 256)
//
// x(0) is the only measured input — one anchor for 160 predicted positions per
// object. Everything else is arithmetic done here, in Go, against pixels read out of
// the rendered frame.

const (
	slopeBallInk    = "FFFFFE" // COLUPF = $0E, the ball
	slopeM0Ink      = "EC3333" // COLUP0 = $46, the static control
	slopeM1Ink      = "2FA076" // COLUP1 = $B6, the leftward missile
	slopeNumBall    = 0x60     // 96/256 = 3/8 px per line, rightward
	slopeNumMissile = 0x55     // 85/256 px per line, leftward, deliberately non-dyadic
	slopeLines      = 160

	// The band's first line. Gopher2600's NTSC capture window reports this ROM's VCS
	// scanline L at L-4 and the band starts at VCS 45; TestHmoveSlopeFrameGeometry
	// fails by name if that stops holding.
	slopeRow0 = 12
)

func loadSlope(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove_slope.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

// objectX returns the clock of the 1-pixel object of the given colour on one
// rendered scanline, or -1 if it is not on that line. Read from the frame: the
// register value is a proxy, the pixel is the claim.
func objectX(t *testing.T, e *Emu, row int, hex string) int {
	t.Helper()
	runs, _, err := e.ReadRow(row)
	if err != nil {
		t.Fatalf("read row %d: %v", row, err)
	}
	for _, r := range runs {
		if r.Hex == hex {
			if r.Len != 1 {
				t.Errorf("row %d: the %s object is %d px wide, want 1 — this fixture only means "+
					"anything if the object is a single pixel", row, hex, r.Len)
			}
			return r.Clock
		}
	}
	return -1
}

// TestHmoveSlopeFollowsTheLineEquation is the headline measurement: the drawn x of
// each 1-pixel object on each of 160 scanlines against floor(n*num/256), reported as
// a maximum error in pixels.
func TestHmoveSlopeFollowsTheLineEquation(t *testing.T) {
	e, top := loadSlope(t)
	cases := []struct {
		name string
		hex  string
		num  int
		sign int
	}{
		{"ball, 96/256 = 3/8 px per line, rightward", slopeBallInk, slopeNumBall, +1},
		{"missile 1, 85/256 px per line, leftward", slopeM1Ink, slopeNumMissile, -1},
	}
	for _, c := range cases {
		x0 := objectX(t, e, top+slopeRow0, c.hex)
		if x0 < 0 {
			t.Fatalf("%s: not drawn on the band's first line (row %d)", c.name, top+slopeRow0)
		}
		maxErr, exact, worstN, worstGot, worstWant := 0, 0, -1, 0, 0
		for n := 0; n < slopeLines; n++ {
			row := top + slopeRow0 + n
			got := objectX(t, e, row, c.hex)
			if got < 0 {
				t.Fatalf("%s: missing from row %d (line %d of the band)", c.name, row, n)
			}
			want := x0 + c.sign*(n*c.num)/256
			d := got - want
			if d < 0 {
				d = -d
			}
			if d == 0 {
				exact++
			}
			if d > maxErr {
				maxErr, worstN, worstGot, worstWant = d, n, got, want
			}
		}
		if maxErr != 0 {
			t.Errorf("%s: max error %d px over %d scanlines (%d of %d exact); worst at line %d, "+
				"drawn at %d, the line equation says %d",
				c.name, maxErr, slopeLines, exact, slopeLines, worstN, worstGot, worstWant)
		}
		travel := c.sign * ((slopeLines - 1) * c.num) / 256
		endX := objectX(t, e, top+slopeRow0+slopeLines-1, c.hex)
		if endX-x0 != travel {
			t.Errorf("%s: travelled %+d px over the band, the slope says %+d", c.name, endX-x0, travel)
		}
		// A slope test on an object that barely moves is not evidence of a slope.
		if travel < 8 && travel > -8 {
			t.Errorf("%s: total travel is only %+d px — too flat for this fixture to mean anything",
				c.name, travel)
		}
		t.Logf("%s: %d of %d scanlines exact, max error %d px, x %d -> %d (%+d)",
			c.name, exact, slopeLines, maxErr, x0, endX, endX-x0)
	}
}

// TestHmoveSlopeStaticControlNeverMoves is the control that no slope assertion can
// replace. Both slopes are graded relative to their OWN first line, so an engine that
// nudged every object by a clock on every HMOVE would satisfy them exactly and only
// shift what "x(0)" means. M0 sits on the same 160 strobes with HMM0 = $00 and must
// not move at all.
func TestHmoveSlopeStaticControlNeverMoves(t *testing.T) {
	e, top := loadSlope(t)
	x0 := objectX(t, e, top+slopeRow0, slopeM0Ink)
	if x0 < 0 {
		t.Fatalf("the static control is not drawn on the band's first line (row %d)", top+slopeRow0)
	}
	moved, firstBad := 0, -1
	for n := 0; n < slopeLines; n++ {
		got := objectX(t, e, top+slopeRow0+n, slopeM0Ink)
		if got < 0 {
			t.Fatalf("the static control is missing from line %d of the band", n)
		}
		if got != x0 {
			moved++
			if firstBad < 0 {
				firstBad = n
			}
		}
	}
	if moved != 0 {
		t.Errorf("the zero-motion control moved on %d of %d scanlines (first at line %d) — 160 HMOVE "+
			"strobes with HM=$00 must displace nothing", moved, slopeLines, firstBad)
	}
	t.Logf("static control held clock %d on %d of %d scanlines", x0, slopeLines-moved, slopeLines)
}

// TestHmoveSlopeObjectsStaySeparable guards the measurement rather than the
// technique: the three paths are laid out not to cross, because TIA priority would
// let one object hide another and a missing colour reads as "it vanished".
func TestHmoveSlopeObjectsStaySeparable(t *testing.T) {
	e, top := loadSlope(t)
	minGap := 999
	for n := 0; n < slopeLines; n++ {
		row := top + slopeRow0 + n
		bl := objectX(t, e, row, slopeBallInk)
		m0 := objectX(t, e, row, slopeM0Ink)
		m1 := objectX(t, e, row, slopeM1Ink)
		if bl < 0 || m0 < 0 || m1 < 0 {
			t.Fatalf("line %d (row %d): an object is missing (BL=%d M0=%d M1=%d)", n, row, bl, m0, m1)
		}
		if !(bl < m0 && m0 < m1) {
			t.Fatalf("line %d: the objects crossed (BL=%d M0=%d M1=%d); a hidden object would be "+
				"read as a missing one", n, bl, m0, m1)
		}
		if g := m0 - bl; g < minGap {
			minGap = g
		}
		if g := m1 - m0; g < minGap {
			minGap = g
		}
	}
	t.Logf("closest approach between any two paths over %d lines: %d px", slopeLines, minGap)
}

// TestHmoveSlopeFrameGeometry pins the frame the band is counted in, and that the
// fixture is a legal 262-line NTSC frame rather than a rolling one.
func TestHmoveSlopeFrameGeometry(t *testing.T) {
	e, top := loadSlope(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262 (3 VSYNC + 37 VBLANK + 192 visible + 30 overscan)", n)
	}
	// One row above the band belongs to the arming line, and one row below the band
	// is already blanked. Both bound the 160 rows the slopes are graded on.
	if x := objectX(t, e, top+slopeRow0+slopeLines, slopeBallInk); x >= 0 {
		t.Errorf("the ball is still drawn one row past the band (row %d, clock %d) — the band is "+
			"longer than the %d lines being graded", top+slopeRow0+slopeLines, x, slopeLines)
	}
	if x := objectX(t, e, top+slopeRow0-2, slopeBallInk); x >= 0 {
		t.Errorf("the ball is drawn two rows before the band (row %d, clock %d) — the capture window "+
			"moved and every line is being graded against the wrong equation index",
			top+slopeRow0-2, x)
	}
}
