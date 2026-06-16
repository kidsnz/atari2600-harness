// Package refdiff extracts a small "layout fingerprint" from a ROM's rendered
// frame and diffs it against a reference ROM (the original game = the oracle).
// Unlike golden-frame regression (which compares a ROM to its OWN past), this
// catches "wrong vs the original" — structural mistakes (a wall inset from the
// edge, an undersized ball) that an eyeball certifies as fine and self-regression
// never sees. It compares specific *measured features*, not raw pixels, so it
// isn't swamped by the legitimate differences between a reproduction and the
// original. docs/testing-playbook.md (differential testing).
package refdiff

import (
	"fmt"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Fingerprint is the set of structural features compared between two ROMs.
type Fingerprint struct {
	LeftWallClock   int // start clock of the left side wall (should touch the edge = 0)
	RightWallEnd    int // end clock of the right side wall (should touch 159)
	BallWidthClocks int // ball width in color clocks (CTRLPF ball size)
	BallHeight      int // ball height in scanlines (rendered; -1 if not measurable)
}

// ballSizeClocks maps the TIA ball-size index (0..3) to color clocks.
var ballSizeClocks = [...]int{1, 2, 4, 8}

// Extract drives the ROM `warmup` frames then measures the fingerprint.
func Extract(romPath string, warmup int) (Fingerprint, error) {
	fp := Fingerprint{LeftWallClock: -1, RightWallEnd: -1, BallWidthClocks: -1, BallHeight: -1}
	// ball height (rendered): start the game and measure the ball in the open field.
	if _, h, err := MeasureBall(romPath); err == nil {
		fp.BallHeight = h
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return fp, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return fp, err
	}
	if err := e.RunFrames(warmup); err != nil {
		return fp, err
	}

	// Ball width: CTRLPF ball-size bits (persist at frame top from the kernel).
	regs := e.ReadTIARegisters()
	if int(regs.Ball.Size) < len(ballSizeClocks) {
		fp.BallWidthClocks = ballSizeClocks[regs.Ball.Size]
	}

	// Walls: find a clean lower-play scanline showing only the two side walls.
	for sl := 100; sl <= 215; sl++ {
		runs, w, err := e.ReadRow(sl)
		if err != nil {
			continue
		}
		if l, r, ok := wallsFromRow(runs, w); ok {
			fp.LeftWallClock, fp.RightWallEnd = l, r
			break
		}
	}
	return fp, nil
}

// MeasureBall starts the game, serves, and measures the ball's rendered width
// (clocks) and height (scanlines) once it is in the open field. It tries both
// input styles (joystick fire-to-serve, and paddle controller) since plugging a
// paddle disables the joystick on that port. Needs emu.SetPanel.
func MeasureBall(romPath string) (width, height int, err error) {
	if w, h, ok := tryMeasureBall(romPath, false); ok { // fire-to-serve games
		return w, h, nil
	}
	if w, h, ok := tryMeasureBall(romPath, true); ok { // paddle-controller games (the original)
		return w, h, nil
	}
	return 0, 0, fmt.Errorf("ball never appeared in the open field")
}

func tryMeasureBall(romPath string, usePaddle bool) (width, height int, ok bool) {
	e, err := emu.New("NTSC")
	if err != nil {
		return 0, 0, false
	}
	if err := e.LoadROM(romPath); err != nil {
		return 0, 0, false
	}
	e.RunFrames(4)
	_ = e.SetPanel("reset", true) // momentary GAME RESET (the original leaves attract)
	e.RunFrames(4)
	_ = e.SetPanel("reset", false)
	e.RunFrames(6)
	if usePaddle {
		_ = e.SetPaddle(0, 0.5)
	}
	for f := 0; f < 300; f++ {
		_ = e.SetInput(0, "fire", f%24 < 3) // periodic serve pulses (re-serve after a loss)
		if err := e.RunFrames(1); err != nil {
			return 0, 0, false
		}
		if w, h, found := findBall(e); found {
			return w, h, true
		}
	}
	return 0, 0, false
}

// findBall looks for the ball: an isolated narrow interior run (not a wall, not a
// full brick row), and measures its width (run length) and height (consecutive
// scanlines carrying that interior run at the same clock).
func findBall(e *emu.Emu) (width, height int, ok bool) {
	interior := func(sl int) (clk, ln int, found bool) {
		runs, w, err := e.ReadRow(sl)
		if err != nil {
			return 0, 0, false
		}
		bgIdx := 0
		for i, r := range runs {
			if r.Len > runs[bgIdx].Len {
				bgIdx = i
			}
		}
		bg := runs[bgIdx].Hex
		var hit []emu.RowRun
		for _, r := range runs {
			if r.Hex != bg && r.Clock >= 15 && r.Clock+r.Len <= w-15 && r.Len <= 10 {
				hit = append(hit, r)
			}
		}
		if len(hit) != 1 {
			return 0, 0, false
		}
		return hit[0].Clock, hit[0].Len, true
	}
	for sl := 80; sl <= 205; sl++ {
		clk, ln, f := interior(sl)
		if !f {
			continue
		}
		// extend downward while an interior run overlaps the same column.
		h := 1
		for d := sl + 1; d <= 215; d++ {
			c2, l2, f2 := interior(d)
			if !f2 || c2+l2 <= clk || c2 >= clk+ln {
				break
			}
			h++
		}
		return ln, h, true
	}
	return 0, 0, false
}

// wallsFromRow picks out the two narrow side walls from a scanline's run list.
// Background = the widest run; walls = exactly two non-background runs, narrow
// (<=12px) and hugging each edge.
func wallsFromRow(runs []emu.RowRun, width int) (left, right int, ok bool) {
	if len(runs) < 3 {
		return 0, 0, false
	}
	bgIdx := 0
	for i, r := range runs {
		if r.Len > runs[bgIdx].Len {
			bgIdx = i
		}
	}
	bg := runs[bgIdx].Hex
	var fg []emu.RowRun
	for _, r := range runs {
		if r.Hex != bg {
			fg = append(fg, r)
		}
	}
	if len(fg) != 2 {
		return 0, 0, false
	}
	l, r := fg[0], fg[1]
	if l.Len > 12 || r.Len > 12 {
		return 0, 0, false
	}
	if l.Clock > 20 || r.Clock+r.Len < width-20 {
		return 0, 0, false
	}
	return l.Clock, r.Clock + r.Len - 1, true
}

// Diff is one feature's comparison between two fingerprints.
type Diff struct {
	Feature string
	Got     int
	Want    int
	Match   bool
}

// Compare diffs `got` (the ROM under test) against `want` (the reference ROM).
func Compare(got, want Fingerprint) []Diff {
	return []Diff{
		{"left_wall_clock", got.LeftWallClock, want.LeftWallClock, got.LeftWallClock == want.LeftWallClock},
		{"right_wall_end", got.RightWallEnd, want.RightWallEnd, got.RightWallEnd == want.RightWallEnd},
		{"ball_width_clocks", got.BallWidthClocks, want.BallWidthClocks, got.BallWidthClocks == want.BallWidthClocks},
		{"ball_height", got.BallHeight, want.BallHeight, got.BallHeight == want.BallHeight},
	}
}

// AllMatch reports whether every feature matched.
func AllMatch(ds []Diff) bool {
	for _, d := range ds {
		if !d.Match {
			return false
		}
	}
	return true
}

func (f Fingerprint) String() string {
	return fmt.Sprintf("leftWall=%d rightWallEnd=%d ballWidth=%d ballHeight=%d", f.LeftWallClock, f.RightWallEnd, f.BallWidthClocks, f.BallHeight)
}
