package emu

import "testing"

// Grading for `roms/litmus/litmus_respx_phase.asm` — the reset-strobe phase.
//
// Closes `docs/fundamentals-audit.md:43`, which harness marked 📖. Its own legend
// defines that as "stated by a document, NOT measured by our litmus ROMs", and the
// statement was: RESPx pipeline (Towers) — counter reset, then the first visible copy
// appears 5px right of the reset.
//
// Measured 2026-09-03: layer (2) techniques+roms and layer (3) internal+cmd+pkg+
// Gopher2600/hardware both returned 0 for that phrase, so nothing graded it, while
// `litmus_jmpind_pos` and `plan_sprite_placement` each measured their own x0 empirically
// because the constant was never pinned.
//
// The offset is measured against the strobe's OWN colour clock rather than derived from
// cycle arithmetic. TraceClocks reports each instruction's beam anatomy in visible
// coordinates (-68..159), so the strobe's end clock and the pixel it produces are two
// readings of the same frame and the difference needs no assumption about which cycle
// of a three-cycle write latches the reset.
//
// Result: the player lands 5 clocks right of its strobe — the documented number, now
// measured — and the missile and the ball land 4. The one-clock difference between an
// 8-clock object and a 1-clock object is a number the document never carried.

const (
	respxPlayerOffset = 5 // RESP0: the documented value
	respxOneClockOff  = 4 // RESM1 / RESBL: measured here, absent from the document

	respxPlayerInk  = "FFFFFE" // COLUP0 = $0E
	respxMissileInk = "2FA076" // COLUP1 = $B6
	respxBallInk    = "EC3333" // COLUPF = $46

	respxStaZp = 0x85 // sta zeropage — the only opcode the fixture strobes with
)

// respxStrobes are the three reset addresses the fixture sweeps, with the ink each
// object draws in and the offset each is expected to produce.
var respxStrobes = map[uint8]struct {
	name string
	ink  string
	want int
}{
	0x10: {"RESP0", respxPlayerInk, respxPlayerOffset},
	0x13: {"RESM1", respxMissileInk, respxOneClockOff},
	0x14: {"RESBL", respxBallInk, respxOneClockOff},
}

type respxHit struct {
	name       string
	ink        string
	want       int
	line       int
	endClock   int
	startClock int
}

// collectRespxStrobes walks instructions and records every reset strobe together with
// the beam position the strobe itself finished at.
func collectRespxStrobes(t *testing.T, e *Emu, maxInstr int) []respxHit {
	t.Helper()
	var out []respxHit
	for i := 0; i < maxInstr; i++ {
		pc := e.PC()
		op, err := e.VCS.Mem.Read(pc)
		if err != nil {
			t.Fatalf("read opcode at $%04X: %v", pc, err)
		}
		operand, err := e.VCS.Mem.Read(pc + 1)
		if err != nil {
			t.Fatalf("read operand at $%04X: %v", pc+1, err)
		}
		tr, err := e.TraceClocks(1)
		if err != nil {
			t.Fatalf("trace: %v", err)
		}
		if len(tr) == 0 {
			break
		}
		if op != respxStaZp {
			continue
		}
		s, ok := respxStrobes[operand]
		if !ok {
			continue
		}
		out = append(out, respxHit{s.name, s.ink, s.want, tr[0].EndLine, tr[0].EndClock, tr[0].StartClock})
	}
	return out
}

func loadRespxPhase(t *testing.T) *Emu {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_respx_phase.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}
	return e
}

// respxDrawnAt returns the clock of the one-pixel object of the given ink on a
// rendered scanline, or -1.
func respxDrawnAt(t *testing.T, e *Emu, line int, ink string) int {
	t.Helper()
	runs, _, err := e.ReadRow(line)
	if err != nil {
		return -1
	}
	for _, r := range runs {
		if r.Hex == ink {
			if r.Len != 1 {
				t.Errorf("line %d: the %s object is %d px wide, want 1", line, ink, r.Len)
			}
			return r.Clock
		}
	}
	return -1
}

// TestRespxPhaseIsFiveForThePlayerAndFourForOneClockObjects is the headline
// measurement, and it is the one that closes the 📖.
func TestRespxPhaseIsFiveForThePlayerAndFourForOneClockObjects(t *testing.T) {
	e := loadRespxPhase(t)
	hits := collectRespxStrobes(t, e, 6000)
	if len(hits) < 40 {
		t.Fatalf("only %d reset strobes traced, want the fixture's 48 — the sweep is not "+
			"being reached, so nothing below measures the phase", len(hits))
	}
	per := map[string]int{}
	bad := map[string]int{}
	for _, h := range hits {
		// The strobe's own line still carries the pre-strobe position for part of its
		// width, so read the line after it, which holds the new position untouched.
		x := respxDrawnAt(t, e, h.line+1, h.ink)
		if x < 0 {
			continue
		}
		per[h.name]++
		if got := x - h.endClock; got != h.want {
			bad[h.name]++
			if bad[h.name] <= 3 {
				t.Errorf("%s strobed to end at clock %d draws at %d = %+d, want %+d",
					h.name, h.endClock, x, got, h.want)
			}
		}
	}
	for _, s := range respxStrobes {
		if per[s.name] < 8 {
			t.Errorf("%s: only %d bands could be read, too few to call the offset measured",
				s.name, per[s.name])
		}
		if bad[s.name] == 0 && per[s.name] >= 8 {
			t.Logf("%s: %+d clocks past the strobe, on all %d readable bands",
				s.name, s.want, per[s.name])
		}
	}
}

// TestRespxPhaseSlopeIsThreeClocksPerCycle is the control that keeps the offset above
// from being satisfiable by a fixture that never moved. One extra CPU cycle before the
// strobe must move the object exactly three colour clocks — the CPU-to-TIA ratio.
func TestRespxPhaseSlopeIsThreeClocksPerCycle(t *testing.T) {
	e := loadRespxPhase(t)
	hits := collectRespxStrobes(t, e, 6000)
	byName := map[string][]int{}
	for _, h := range hits {
		if x := respxDrawnAt(t, e, h.line+1, h.ink); x >= 0 {
			byName[h.name] = append(byName[h.name], x)
		}
	}
	// The trace covers several frames, so each object's sixteen-band sweep repeats, and
	// it does not necessarily start on a sweep boundary. Rather than guess where the
	// first full sweep begins, measure the longest run of consecutive +3 steps: that is
	// the claim ("one extra CPU cycle moves the object three colour clocks") stated
	// without any assumption about where the trace happened to start.
	const wantRun = 12
	for name, xs := range byName {
		if len(xs) < wantRun {
			t.Errorf("%s: only %d bands readable, fewer than the %d a run needs", name, len(xs), wantRun)
			continue
		}
		best, run := 1, 1
		badStep, badAt := 0, -1
		for i := 1; i < len(xs); i++ {
			d := xs[i] - xs[i-1]
			switch {
			case d == 3:
				run++
				if run > best {
					best = run
				}
			case d < 0:
				run = 1 // the sweep wrapped to its start; not a fault
			default:
				run = 1
				if badAt < 0 {
					badStep, badAt = d, i
				}
			}
		}
		if badAt >= 0 {
			t.Errorf("%s: band %d is %+d clocks past the one before it — every step inside a "+
				"sweep must be exactly +3 (one CPU cycle = three colour clocks)", name, badAt, badStep)
		}
		if best < wantRun {
			t.Errorf("%s: the longest run of +3 steps is %d, want at least %d — the sweep is "+
				"flat or folded, so the offset above could be satisfied without motion",
				name, best, wantRun)
			continue
		}
		t.Logf("%s: %d bands read, longest unbroken +3 run %d, range %d..%d",
			name, len(xs), best, minInt(xs), maxInt(xs))
	}
}

func minInt(v []int) int {
	m := v[0]
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}

func maxInt(v []int) int {
	m := v[0]
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

// TestRespxPhaseFrameGeometry pins the frame, because every reading above is taken from
// a rendered line and a rolling frame would move all of them together.
func TestRespxPhaseFrameGeometry(t *testing.T) {
	e := loadRespxPhase(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262 (3 VSYNC + 37 VBLANK + 192 picture + 30 "+
			"overscan; the bands fill the picture exactly, so there is no filler loop)", n)
	}
}
