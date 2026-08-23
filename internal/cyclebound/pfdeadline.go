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
// isPlayfieldReg says whether a deadline WOULD govern this register at some repeat count. It is
// the difference between "not our business" and "our business, and we could not judge it".
// clocksPerLine is one scanline in TIA colour clocks, measured from a WSYNC.
const clocksPerLine = 228

func isPlayfieldReg(reg string) bool {
	switch reg {
	case "PF0", "PF1", "PF2", "COLUPF", "COLUBK":
		return true
	}
	return false
}

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
	Asm     string `json:"asm"`
	Checked int    `json:"checked"` // writes that had a deadline to beat
	// TWO NUMBERS, NOT ONE. These used to be summed into `Unchecked`, and the sum is unreadable:
	// NotOurs is "this rule does not apply", Unjudged is "this rule applies and we cannot say" —
	// opposite meanings to whoever reads the verdict. A reader who saw the combined 1 on a real
	// build could not tell whether the line was fully judged or not, and read it as a coverage
	// hole in the playfield check, which cost an afternoon and a retraction on 2026-08-23.
	// NotOurs > 0 is ordinary. Unjudged > 0 means `pf_deadlines: true` went green over a
	// playfield write nobody checked.
	NotOurs  int          `json:"not_ours"`           // registers no playfield deadline governs
	Unjudged int          `json:"unjudged"`           // PF0/1/2 past the second, COLUPF/COLUBK past the first
	Late     []PFDeadline `json:"late,omitempty"`     // the ones that miss, worst first
	Tightest *PFDeadline  `json:"tightest,omitempty"` // the closest write that still makes it
	Declined string       `json:"declined,omitempty"` // why nothing was checked
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
				if isPlayfieldReg(w.Reg) {
					rep.Unjudged++
				} else {
					rep.NotOurs++
				}
				continue
			}
			// A WINDOW THAT STRADDLES A SCANLINE BOUNDARY IS NOT A LATE WRITE, AND NOT AN ON-TIME
			// ONE EITHER. Colour clocks wrap every 228, so a write pushed a full line back reappears
			// as a small clock in the NEXT line's HBLANK and compares as comfortably early.
			// Measured 2026-08-23 by the other session, adding nops at the head of a play region:
			//
			//     +10 nops (96 > 76 cycles)  -> 6 of 23 LATE
			//     +26 nops (128 > 76)        -> 3 of 23 LATE
			//     +40 nops (156 > 76)        -> ok, all 23 land in time
			//
			// Breaking the kernel HARDER made the verdict greener, and the worst one was green.
			// That is why cmd/cyclebound refuses to run this check on an uncertified kernel — a
			// blunt rule with a real defect behind it. The rule can be replaced by the fact it was
			// standing in for: BeamIntervals already marks these writes, and this file had never
			// read the flag, which is precisely what its own opening paragraph says happened to the
			// deadline question itself ("BeamIntervals already computes the windows it needs").
			// Cutting on the measured fact rather than on "is the budget red" also keeps the
			// answer to the OTHER question — a certified kernel with one wrapping write still gets
			// its other writes judged.
			//
			// THE PREDICATE IS MaxAbs, NOT CrossesLine. CrossesLine is `minAbs/228 != maxAbs/228`,
			// which is false whenever the window is EXACT — and the measured failure above is a run
			// of nops, entirely exact, so a CrossesLine test would have caught none of it. The
			// question is not "is the landing line uncertain" but "is it this line at all": MinAbs
			// and MaxAbs count colour clocks from the region's own WSYNC, while MaxClock is folded
			// back modulo 228 by clockAt, so a write at 468 clocks compares as a comfortable 12.
			// Anything from 228 on is being measured against the deadlines of a line it is not on,
			// whether it got there by a defect or because the region legitimately spans two lines —
			// the table describes ONE line either way, so the comparison is not available.
			// MaxAbs >= 228 subsumes CrossesLine, since a straddling window has maxAbs >= 228.
			if w.MaxAbs >= clocksPerLine {
				rep.Unjudged++
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
		if r.NotOurs > 0 {
			s += fmt.Sprintf(" (%d write(s) to registers no playfield deadline governs)", r.NotOurs)
		}
		if r.Unjudged > 0 {
			// The third clause says "lands on another line" rather than "which line is not
			// decided", because MaxAbs >= 228 covers two shapes and only one of them is uncertain.
			// A run of nops is EXACT: which line it lands on is perfectly decided, just not the one
			// these deadlines describe. Saying "not decided" about it would be false — and mixing
			// two meanings into one sentence is exactly what this whole message was rewritten to
			// stop doing an hour earlier.
			s += fmt.Sprintf(" [%d PLAYFIELD write(s) NOT JUDGED: they repeat past the two per "+
				"line this models, or they land on a scanline other than the one these deadlines "+
				"describe (a window that straddles the boundary is the uncertain case of that); "+
				"this verdict is silent about them]", r.Unjudged)
		}
		return s
	}
	w := r.Late[0]
	return fmt.Sprintf("playfield deadlines: %d of %d write(s) LATE -- worst is %s in %s at clock %d, "+
		"%d colour clocks past its deadline of %d, which shifts the picture right by %d column(s)",
		len(r.Late), r.Checked, w.Reg, w.Region, w.MaxClock, w.LateBy, w.Deadline, (w.LateBy+3)/4)
}
