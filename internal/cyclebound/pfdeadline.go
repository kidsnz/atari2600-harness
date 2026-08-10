package cyclebound

import (
	"fmt"
	"sort"
)

// A playfield write has a DEADLINE, and fitting inside 76 cycles does not meet it.
//
// This is the failure the whole file exists for. A cover kernel proved 75 cycles of 76
// and was CERTIFIED; its picture was still shifted two and a half columns to the right
// with the previous line's right edge wrapping in at the left, because three cycles of
// index arithmetic at the top of the line pushed PF0's store to cycle 26 against a
// deadline of 22.67. Prove and ProveDetail answer "what does this region COST". They do
// not answer "does each write land before the beam reaches the pixels it governs", and
// nothing in the repo gated on the second question even though BeamIntervals already
// computes the windows it needs.
//
// The deadlines below are the machine's, not a convention. A playfield column is four
// colour clocks wide and the left half runs clocks 0..79:
//
//	PF0  covers clocks   0..15   (columns 0-3)    so it must be written by clock 0
//	PF1  covers clocks  16..47   (columns 4-11)   by clock 16
//	PF2  covers clocks  48..79   (columns 12-19)  by clock 48
//
// and in REPEAT mode the right half repeats them at clocks 80/96/128. A kernel that
// rewrites the registers mid-line to get 40 distinct columns therefore has six
// deadlines, not three. COLUPF and COLUBK take effect where they are written, so their
// deadline is the start of the line for anything that must be the right colour from
// column 0.
//
// WHAT THIS DOES NOT DO. It checks the FIRST write to each register in a region against
// the deadline for the LEFT half, and a second write against the right half's. A kernel
// that writes a register three times in a line, or one that deliberately changes the
// playfield mid-column for a fine effect, is outside what this can judge and is reported
// as unchecked rather than passed.

// PFDeadline is one register's window against the clock it has to beat.
type PFDeadline struct {
	Region   string `json:"region"`    // where the region starts
	Reg      string `json:"reg"`       // PF0/PF1/PF2/COLUPF/COLUBK
	Nth      int    `json:"nth"`       // 1 = the left half's write, 2 = the right half's
	MaxClock int    `json:"max_clock"` // the latest this write can land, over all paths
	Deadline int    `json:"deadline"`  // the clock the beam reaches what it governs
	LateBy   int    `json:"late_by"`   // colour clocks past the deadline; <= 0 is fine
}

// pfDeadlines maps (register, nth write in the region) to the colour clock by which it
// must be in place. Clocks are this project's: HBLANK is -68..-1, visible 0..159.
func pfDeadlineFor(reg string, nth int) (int, bool) {
	// The bounds check comes FIRST. A kernel that writes PF0 three times in a line is
	// outside what this can judge, and indexing the table before saying so crashed --
	// caught by the table test, never by a real build, because nothing in the tree does
	// it yet.
	if nth < 1 || nth > 2 {
		return 0, false
	}
	switch reg {
	case "PF0":
		return []int{0, 80}[nth-1], true
	case "PF1":
		return []int{16, 96}[nth-1], true
	case "PF2":
		return []int{48, 128}[nth-1], true
	case "COLUPF", "COLUBK":
		// A colour written after the beam has passed a pixel recolours nothing. The
		// first write in a line governs from column 0.
		if nth == 1 {
			return 0, true
		}
		return 0, false
	}
	return 0, false
}

// PFDeadlineReport is the verdict for one program.
type PFDeadlineReport struct {
	Asm       string       `json:"asm"`
	Checked   int          `json:"checked"`             // writes that had a deadline to beat
	Unchecked int          `json:"unchecked"`           // writes past the second, or to registers with no rule
	Late      []PFDeadline `json:"late,omitempty"`      // the ones that miss, worst first
	Tightest  *PFDeadline  `json:"tightest,omitempty"`  // the closest write that still makes it
	Declined  string       `json:"declined,omitempty"`  // why nothing was checked
}

// CheckPFDeadlines proves, over ALL paths, that every playfield write lands before the
// beam reaches the pixels it governs.
func CheckPFDeadlines(asmPath string) (*PFDeadlineReport, error) {
	br, err := BeamIntervals(asmPath)
	if err != nil {
		return nil, err
	}
	rep := &PFDeadlineReport{Asm: asmPath}
	if br.BankedDeclined != "" {
		rep.Declined = br.BankedDeclined
		return rep, nil
	}
	if !br.Converged {
		rep.Declined = "the beam analysis did not converge, so no window is trustworthy"
		return rep, nil
	}
	best := 1 << 30
	for _, reg := range br.Regions {
		if reg.Kind != "visible" {
			continue // a blank line draws nothing; there is nothing to be late for
		}
		seen := map[string]int{}
		for _, w := range reg.Writes {
			seen[w.Reg]++
			dl, ok := pfDeadlineFor(w.Reg, seen[w.Reg])
			if !ok {
				rep.Unchecked++
				continue
			}
			rep.Checked++
			d := PFDeadline{Region: reg.StartLoc, Reg: w.Reg, Nth: seen[w.Reg],
				MaxClock: w.MaxClock, Deadline: dl, LateBy: w.MaxClock - dl}
			if d.LateBy > 0 {
				rep.Late = append(rep.Late, d)
			} else if -d.LateBy < best {
				best = -d.LateBy
				c := d
				rep.Tightest = &c
			}
		}
	}
	sort.Slice(rep.Late, func(i, j int) bool { return rep.Late[i].LateBy > rep.Late[j].LateBy })
	return rep, nil
}

// Summary is the one line a verdict prints.
func (r *PFDeadlineReport) Summary() string {
	if r.Declined != "" {
		return "playfield deadlines: not checked -- " + r.Declined
	}
	if r.Checked == 0 {
		return "playfield deadlines: nothing to check (no playfield writes in a visible region)"
	}
	if len(r.Late) == 0 {
		s := fmt.Sprintf("playfield deadlines: %d write(s) all land in time", r.Checked)
		if r.Tightest != nil {
			s += fmt.Sprintf("; tightest is %s at clock %d against a deadline of %d",
				r.Tightest.Reg, r.Tightest.MaxClock, r.Tightest.Deadline)
		}
		if r.Unchecked > 0 {
			s += fmt.Sprintf(" (%d write(s) had no rule and were NOT checked)", r.Unchecked)
		}
		return s
	}
	w := r.Late[0]
	return fmt.Sprintf("playfield deadlines: %d of %d write(s) LATE -- worst is %s in %s at clock %d, "+
		"%d colour clocks past its deadline of %d, which shifts the picture right by %d column(s)",
		len(r.Late), r.Checked, w.Reg, w.Region, w.MaxClock, w.LateBy, w.Deadline, (w.LateBy+3)/4)
}
