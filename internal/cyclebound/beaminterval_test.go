package cyclebound

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The claim: every TIA write lands somewhere inside its proven beam-clock
// window, on every path. The check: run the ROM and see where the writes
// actually land.
//
// This is the only way the claim can be falsified. An interval derived from the
// same cycle model that produced it would agree with itself no matter how wrong
// the model was; the emulator does not care what the analysis expected. It also
// checks the one constant that cannot be derived from the CFG — the beam clock
// at which the instruction after a WSYNC begins — because an offset error would
// otherwise shift every window by a fixed amount and still look self-consistent.
func TestBeamIntervalContainsObserved(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	for _, pat := range []string{"../../roms/litmus/*.asm", "../../roms/exerciser/*.asm"} {
		more, _ := filepath.Glob(pat)
		files = append(files, more...)
	}
	checked, inside := 0, 0
	var offenders []string

	for _, asm := range files {
		r, err := BeamIntervals(asm)
		if err != nil || !r.Converged {
			continue
		}
		// PC -> proven window, for the bounded regions only.
		want := map[[2]uint16][2]int{}
		for _, reg := range r.Regions {
			if !reg.Bounded {
				continue
			}
			rs := parseHexAddr(reg.Start)
			for _, w := range reg.Writes {
				pc := parseHexAddr(w.PC)
				k := [2]uint16{rs, pc}
				// Compare in UNFOLDED colour clocks since the region start. Folding a
				// window back into one line turns a straddling one into nonsense (min
				// above max), which is how the first version manufactured 112 violations
				// out of writes that were inside their window.
				// A PC can appear in more than one region (shared code); widen to the
				// union so the check never fails for a reason the analysis did not claim.
				if old, ok := want[k]; ok {
					want[k] = [2]int{imin(old[0], w.MinAbs), imax(old[1], w.MaxAbs)}
				} else {
					want[k] = [2]int{w.MinAbs, w.MaxAbs}
				}
			}
		}
		if len(want) == 0 {
			continue
		}

		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Logf("assemble %s: %s", asm, out)
			continue
		}
		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		if err := e.RunFrames(2); err != nil {
			continue
		}
		obs, err := tracedWritesByRegion(e, 2)
		if err != nil {
			continue
		}
		for _, o := range obs {
			win, known := want[[2]uint16{o.region, o.pc}]
			if !known {
				continue
			}
			w := o
			checked++
			// The observed clock is folded into one line, the window is not, so the
			// observation is inside if it matches at ANY line offset the window spans.
			hit := false
			// abs 0 is the WSYNC boundary, which is clock -68, so a folded clock c
			// sits at abs (c+68) plus a whole number of scanlines.
			for k := 0; k*228 <= win[1]+228; k++ {
				abs := (w.Clock + 68) + 228*k
				if abs >= win[0] && abs <= win[1] {
					hit = true
					break
				}
			}
			if hit {
				inside++
				continue
			}
			if len(offenders) < 6 {
				offenders = append(offenders, filepath.Base(asm)+" PC "+hexAddr(w.pc)+
					" "+w.Name+" landed at clk "+itoa(w.Clock)+
					" outside proven abs["+itoa(win[0])+".."+itoa(win[1])+"]")
			}
		}
	}

	if checked == 0 {
		t.Fatal("no writes were checked against a proven window — the test proves nothing")
	}
	t.Logf("beam containment: %d/%d observed TIA writes inside their proven window", inside, checked)
	if inside != checked {
		t.Errorf("%d observed writes fell OUTSIDE their proven beam window:\n  %v",
			checked-inside, offenders)
	}
}

// A proven window is only worth having if it is often narrow. A report of
// "somewhere on this line" is technically true and useless, so the width is
// measured and stated rather than assumed to be small.
func TestBeamIntervalWidthIsMeasured(t *testing.T) {
	files, _ := filepath.Glob("../../roms/techniques/*.asm")
	total, exact, widths := 0, 0, 0
	for _, asm := range files {
		r, err := BeamIntervals(asm)
		if err != nil || !r.Converged {
			continue
		}
		for _, reg := range r.Regions {
			if !reg.Bounded {
				continue
			}
			for _, w := range reg.Writes {
				total++
				if w.Exact {
					exact++
				}
				widths += w.MaxClock - w.MinClock
			}
		}
	}
	if total == 0 {
		t.Skip("no bounded writes")
	}
	t.Logf("beam windows: %d writes, %d exact (%.0f%%), mean width %.1f colour clocks",
		total, exact, 100*float64(exact)/float64(total), float64(widths)/float64(total))
}

func parseHexAddr(s string) uint16 {
	var v uint16
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | uint16(c-'0')
		case c >= 'A' && c <= 'F':
			v = v<<4 | uint16(c-'A'+10)
		case c >= 'a' && c <= 'f':
			v = v<<4 | uint16(c-'a'+10)
		}
	}
	return v
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// obsWrite is one observed TIA write tagged with the WSYNC that started the
// region it happened in.
type obsWrite struct {
	region uint16
	pc     uint16
	Clock  int
	Name   string
}

// tracedWritesByRegion records every TIA write together with the WSYNC store
// that most recently executed. Without that tag a write cannot be matched to the
// region whose window claims it: the same instruction is often reachable from
// more than one WSYNC, and comparing it against the union of whichever regions
// happened to be bounded is not a check of anything the analysis said.
func tracedWritesByRegion(e *emu.Emu, frames int) ([]obsWrite, error) {
	var out []obsWrite
	var region uint16
	start := e.Coords().Frame
	for i := 0; i < 4_000_000 && e.Coords().Frame-start < frames; i++ {
		before := e.Coords()
		if err := e.StepInstruction(); err != nil {
			return nil, err
		}
		w, ok := e.LastTIAWrite()
		if !ok {
			continue
		}
		name, _ := beamtrace.RegInfo(w.Reg)
		if name == "WSYNC" || name == "RSYNC" {
			region = w.PC
			continue
		}
		if region == 0 {
			continue // no region entered yet
		}
		_ = before
		out = append(out, obsWrite{region: region, pc: w.PC, Clock: e.Coords().Clock, Name: name})
	}
	return out, nil
}
