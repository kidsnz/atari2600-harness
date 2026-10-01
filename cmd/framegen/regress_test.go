package main

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
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
		// Every missile and ball width, and VDELBL. This number was 2568 and is now 160,
		// and the drop is the whole point of recording it: the missiles used to be drawn
		// on 1544 and 1536 cells against targets of 728 and 720, because the per-line
		// NUSIZ table carried only `SizeAndCopies` — NUSIZ's low three bits, the player's
		// copy mode — and the MISSILE width lives in bits 4-5. All 214 lines therefore
		// read nz0 = 0, the table looked constant, and no NUSIZ replay block was emitted.
		// Measuring the full NUSIZ byte leaves M1 exact (720/720), M0 seven cells out,
		// and the 146 the ball still needs. Recorded at the measured value so the next
		// change has to say which way it moved.
		{"../../roms/litmus/litmus_objsizes.bin", 160, "partial reproduction"},
		// A sprite repositioned per zone (RL-8b).
		{"../../roms/techniques/zone_multiplex.bin", 0, "pixel-exact"},
		// Missiles and ball on every line, positioned once (RL-8a).
		{"../../roms/techniques/shared_setxpos.bin", 16, "differences remain"},
	}

	for _, c := range cases {
		s, cells := runClone(t, c.rom)
		if !strings.Contains(s, c.want) {
			t.Errorf("%s: RESULT does not contain %q", c.rom, c.want)
		}
		if cells > c.maxCells {
			t.Errorf("%s: %d mismatched cells, was %d — the reproduction got WORSE. "+
				"litmus_nusiz_all silently went 2 -> 1868 once already, from an input clamp that "+
				"stopped the calibration reaching a left-edge position.", c.rom, cells, c.maxCells)
		}
	}
}

// runClone runs framegen on rom and returns its output and the number of visible cells
// the clone draws with a different element than the target. It also fails the test when
// the frame is not 262 scanlines.
func runClone(t *testing.T, rom string) (string, int) {
	t.Helper()
	// THE OLD REGEX READ A PHRASE THAT ONLY ONE VERDICT PRINTS. "differences remain"
	// says "N of 34240 visible cells are drawn by a different element"; "partial
	// reproduction" says no such thing, so `cells` stayed 0 for those cases and the
	// maxCells comparison could never fail. litmus_objsizes was pinned at 2568 and that
	// number was never once read — it dropped to 160 and the test noticed nothing.
	//
	// Counting from the per-element table instead works for every verdict: the mismatch
	// is the visible area minus everything matched, and each element line carries its
	// own matched count.
	matchedRe := regexp.MustCompile(`(?m)^\s+\w+ target\s+\d+\s+matched\s+(\d+)`)
	areaRe := regexp.MustCompile(`visible (\d+) lines`)
	lineRe := regexp.MustCompile(`frame length: (\d+) scanlines`)

	out, err := exec.Command("go", "run", ".", "-rom", rom, "-frames", "28",
		"-out", t.TempDir()+"/clone.asm").CombinedOutput()
	// framegen exits 1 when the reproduction is incomplete, which is a verdict and
	// not a failure to run, so the exit status alone is not the check.
	if err != nil && !strings.Contains(string(out), "RESULT:") {
		t.Fatalf("%s: framegen did not run: %v\n%s", rom, err, out)
	}
	s := string(out)

	cells := 0
	if am := areaRe.FindStringSubmatch(s); am != nil {
		h, _ := strconv.Atoi(am[1])
		total := 0
		for _, m := range matchedRe.FindAllStringSubmatch(s, -1) {
			n, _ := strconv.Atoi(m[1])
			total += n
		}
		cells = h*160 - total
	} else {
		t.Errorf("%s: no \"visible N lines\" in the output — cannot compute the mismatch, "+
			"and a check that cannot compute its own number is a check that passes on nothing", rom)
	}
	// Frame length is invisible to a picture comparison and has been wrong before
	// (every clone emitted 267 scanlines at one point), so it is asserted here too.
	if m := lineRe.FindStringSubmatch(s); m == nil {
		t.Errorf("%s: no frame length reported", rom)
	} else if n, _ := strconv.Atoi(m[1]); n != 262 {
		t.Errorf("%s: frame is %d scanlines, want 262", rom, n)
	}
	return s, cells
}

// TestAnUnfollowedPlayerKeepsItsPlaceInEveryZone pins the case where a zone plan exists
// (one player moves down the frame) but the OTHER player sits at one X for the whole
// frame, so the plan does not follow it.
//
// Zone 0 is placed by the prologue's div-15 SetXPos and every later zone by the inline
// nop-chain block, and the two actuators map an input to different pixels. The
// calibration used to copy zone 0's input into every later zone for an unfollowed
// object, so the nop chain was handed a div-15 input. Measured on this ROM: P1 read 64
// in zone 0 at input 58, zones 1-5 were handed the same 58 -- which the nop chain
// places at 7 (zoneReadFor), not 64 -- and 190 of P1's 228 cells were wrong. The
// unmodified zone_multiplex follows both players, so the pixel-exact case above never
// reaches this path.
//
// The target is zone_multiplex with P1 frozen: all six zone X values are 60 and the
// per-frame update subtracts 0 instead of 1, which keeps the update loop's cycle count
// and therefore the target's frame layout unchanged.
func TestAnUnfollowedPlayerKeepsItsPlaceInEveryZone(t *testing.T) {
	src, err := os.ReadFile("../../roms/techniques/zone_multiplex.asm")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, r := range [][2]string{
		{"        byte 100, 70, 30, 60, 95, 120\n", "        byte 60, 60, 60, 60, 60, 60\n"},
		{"        sbc #1\n", "        sbc #0\n"},
	} {
		// Each edit must hit exactly once, or this test would quietly measure the
		// unmodified ROM -- the one that never reaches the path under test.
		if n := strings.Count(s, r[0]); n != 1 {
			t.Fatalf("zone_multiplex.asm: %q occurs %d times, want 1 -- the source changed "+
				"and this test no longer freezes P1", r[0], n)
		}
		s = strings.Replace(s, r[0], r[1], 1)
	}
	dir := t.TempDir()
	asm, bin := dir+"/zone_multiplex_p1_still.asm", dir+"/zone_multiplex_p1_still.bin"
	if err := os.WriteFile(asm, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("assemble: %v\n%s", err, out)
	}

	out, cells := runClone(t, bin)
	// The preconditions: a zone plan exists, and P1 is NOT in it. Without both, a pass
	// says nothing about the unfollowed path.
	if !strings.Contains(out, "zone kernel: 6 zones") {
		t.Fatalf("no 6-zone plan in the output -- the path under test was not reached\n%s", out)
	}
	if !strings.Contains(out, "measured P1: drawn on 42 lines") ||
		!strings.Contains(out, "1 distinct reset X [64(42 lines)]") {
		t.Fatalf("P1 is not at one reset X on all 42 of its lines -- it would be followed, "+
			"and the path under test was not reached\n%s", out)
	}
	if cells != 0 || !strings.Contains(out, "pixel-exact") {
		t.Errorf("%d mismatched cells, want 0 (pixel-exact) -- an unfollowed object must land "+
			"in every zone where zone 0 put it\n%s", cells, out)
	}
}

// TestHeldRunGeneralisesTheBlankRule pins what a positioning block may occupy.
//
// The planner used to demand BACKGROUND-ONLY lines. On this corpus every zone failure
// reported "have=0", and that number was not merely conservative, it was wrong about the
// target: at Fishing Derby's line-27 boundary the old rule saw 0 usable lines where 7
// exist. Real kernels draw something on every line, so "blank" was almost never true.
//
// What a positioning block actually needs is lines the HELD registers reproduce. The
// replay loop is stopped during the block, so GRP0/GRP1/PF keep whatever the last
// replayed line left, and the target matches exactly when those lines repeat it. That
// generalises the old rule instead of replacing it: an all-background run IS a run of
// identical lines, so everything the blank test accepted is still accepted.
func TestHeldRunGeneralisesTheBlankRule(t *testing.T) {
	row := func(s string) []string {
		out := make([]string, len(s))
		for i, c := range s {
			out[i] = map[rune]string{'B': "BG", 'P': "PF", '0': "P0"}[c]
		}
		return out
	}
	fd := &frameData{tgtElem: [][]string{
		row("BBBB"), // 0 blank
		row("BBBB"), // 1 blank
		row("PPBB"), // 2 a picture row...
		row("PPBB"), // 3 ...repeated
		row("PPBB"), // 4 ...and again
		row("P0BB"), // 5 different
	}}
	for _, c := range []struct {
		y    int
		want int
		why  string
	}{
		{1, 2, "two all-background lines — the case the old rule handled"},
		{4, 3, "three IDENTICAL picture rows — the case it could not see, and where Fishing Derby's 7 live"},
		{2, 1, "a picture row whose predecessor differs: the run is itself only"},
		{5, 1, "a unique row holds nothing"},
		{-1, 0, "out of range"},
	} {
		if got := fd.heldRun(c.y); got != c.want {
			t.Errorf("heldRun(%d) = %d, want %d — %s", c.y, got, c.want, c.why)
		}
	}
}
