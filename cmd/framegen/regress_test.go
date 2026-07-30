package main

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// framegen had NO test file at all, and CI never invoked it. That is exactly how
// litmus_nusiz_all went from 2 mismatched cells to 1868 unnoticed: the number was
// recorded in prose, a conclusion was built on it ("per-line NUSIZ replay works"),
// and nothing asserted it. The regression sweep that should have caught it globs
// roms/techniques/*.asm — and this litmus lives in roms/litmus/, so the ROM added
// BECAUSE it lights axes nothing else does fell straight out of the regression set.
//
// The cause was a clamp: P1's target reset X of 4 needs a position input of -3, and
// clamping the input to 0 stalled the calibration at X 7 rather than wrapping to the
// equivalent 157. The clone then reported 1074 of P1's cells as "a cause this tool
// has not measured".
//
// So the numbers live here now, where a change that moves them fails the build.
func TestGeneratedCloneCellCounts(t *testing.T) {
	cases := []struct {
		rom      string
		maxCells int    // mismatched cells; the clone must not be worse than this
		want     string // substring the RESULT line must contain
	}{
		// The axes nothing else in the corpus exercises: all eight NUSIZ modes on both
		// players. 2 cells is a stated hardware limit, not slack — the target clears
		// GRP1 part-way along one scanline, leaving a 10-pixel run that a once-per-line
		// HBLANK write can draw as 8 or 12 and never as 10.
		{"../../roms/litmus/litmus_nusiz_all.bin", 2, "differences remain"},
		// Every missile and ball width, and VDELBL. Measured, not assumed: M0 and M1
		// match exactly (728/728, 720/720) but the ball is NOT reproduced and the
		// missiles are drawn on MORE lines than the target draws them (clone 1544 vs
		// target 728), so this ROM is a partial reproduction. Recorded at its measured
		// value so an improvement is visible as a change rather than hidden by a
		// hopeful expectation — I first wrote "pixel-exact" here from assumption and
		// the test caught me.
		{"../../roms/litmus/litmus_objsizes.bin", 2568, "partial reproduction"},
		// A sprite repositioned per zone (RL-8b).
		{"../../roms/techniques/zone_multiplex.bin", 0, "pixel-exact"},
		// Missiles and ball on every line, positioned once (RL-8a).
		{"../../roms/techniques/shared_setxpos.bin", 16, "differences remain"},
	}

	cellRe := regexp.MustCompile(`(\d+) of \d+ visible cells`)
	lineRe := regexp.MustCompile(`frame length: (\d+) scanlines`)

	for _, c := range cases {
		out, err := exec.Command("go", "run", ".", "-rom", c.rom, "-frames", "28",
			"-out", t.TempDir()+"/clone.asm").CombinedOutput()
		// framegen exits 1 when the reproduction is incomplete, which is a verdict and
		// not a failure to run, so the exit status alone is not the check.
		if err != nil && !strings.Contains(string(out), "RESULT:") {
			t.Fatalf("%s: framegen did not run: %v\n%s", c.rom, err, out)
		}
		s := string(out)

		if !strings.Contains(s, c.want) {
			t.Errorf("%s: RESULT does not contain %q", c.rom, c.want)
		}
		cells := 0
		if m := cellRe.FindStringSubmatch(s); m != nil {
			cells, _ = strconv.Atoi(m[1])
		}
		if cells > c.maxCells {
			t.Errorf("%s: %d mismatched cells, was %d — the reproduction got WORSE. "+
				"litmus_nusiz_all silently went 2 -> 1868 once already, from an input clamp that "+
				"stopped the calibration reaching a left-edge position.", c.rom, cells, c.maxCells)
		}
		// Frame length is invisible to a picture comparison and has been wrong before
		// (every clone emitted 267 scanlines at one point), so it is asserted here too.
		if m := lineRe.FindStringSubmatch(s); m == nil {
			t.Errorf("%s: no frame length reported", c.rom)
		} else if n, _ := strconv.Atoi(m[1]); n != 262 {
			t.Errorf("%s: frame is %d scanlines, want 262", c.rom, n)
		}
	}
}
