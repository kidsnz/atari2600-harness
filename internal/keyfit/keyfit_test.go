package keyfit

import (
	"math"
	"sort"
	"testing"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

const clk = audio.BaseClockNTSC

// The figures this package was written for, so the tests grade it against answers that
// were arrived at independently and then acted on.
var (
	bassline = []int{0, 5, 7, 8, 10, 12} // tonic, 4th, 5th, b6, b7, octave
	line     = []int{7, 8, 10, 12}       // the moving part alone, no pedal
)

func TestNearestPinsMeasuredGridPoints(t *testing.T) {
	cases := []struct {
		hz          float64
		waves       []int
		audc, audf  int
		cents       float64
	}{
		{87.31, []int{1}, 1, 23, -1.7},   // F2 on the saw voice
		{44.04, []int{6}, 6, 22, 0.0},    // the pedal, exactly on a grid point
		{130.81, []int{1}, 1, 15, 0.2},   // C3
		{207.65, []int{12}, 12, 24, 13.9}, // G#3 on the lead voice, 13.9 cents sharp
	}
	for _, c := range cases {
		gc, gf, ce := Nearest(c.hz, c.waves, clk)
		if gc != c.audc || gf != c.audf {
			t.Errorf("Nearest(%.2f) = AUDC %d AUDF %d, want AUDC %d AUDF %d", c.hz, gc, gf, c.audc, c.audf)
		}
		if math.Abs(ce-c.cents) > 1.0 {
			t.Errorf("Nearest(%.2f) cents %.1f, want about %.1f", c.hz, ce, c.cents)
		}
	}
}

// The finding the whole reproduction turned on: the source key is unusable and one
// semitone down is not. If this stops being true the analysis behind three ROMs is
// wrong and I want to be told.
func TestTheSourceKeyIsWorseThanASemitoneDown(t *testing.T) {
	fsharp := FitTonic(46.249, bassline, clk) // F#1, the record's key
	f := FitTonic(43.654, bassline, clk)      // F1
	if math.Abs(fsharp.Worst) < 35 {
		t.Errorf("F# minor's worst degree is %.1f cents; it was +43.5 when the key was chosen, "+
			"and the transposition only makes sense while it is bad", fsharp.Worst)
	}
	if math.Abs(f.Worst) >= math.Abs(fsharp.Worst) {
		t.Errorf("F minor (%.1f c) is not better than F# minor (%.1f c) — the transposition "+
			"in pattern-bassline-sub.asm rests on it being better", f.Worst, fsharp.Worst)
	}
	if f.WorstDeg != 5 {
		t.Errorf("F minor's worst degree is the %d-semitone one; it is supposed to be the 4th "+
			"(5), which is the note that build omits", f.WorstDeg)
	}
}

// D and E are the specific obstacle, and it holds in EVERY octave. This is the fact
// that ruled the source key out, so it is the one most worth locking down.
func TestDAndEAreUnplayableAnywhereInTheBass(t *testing.T) {
	waves := Playable()
	for _, n := range []struct {
		name string
		hz   float64
	}{{"D1", 36.71}, {"D2", 73.42}, {"D3", 146.83}, {"E1", 41.20}, {"E2", 82.41}} {
		_, _, cents := Nearest(n.hz, waves, clk)
		if math.Abs(cents) < 20 {
			t.Errorf("%s comes out %.1f cents off; it was more than 25 in every octave when "+
				"the key was chosen, and that is why the piece is transposed", n.name, cents)
		}
	}
}

// The negative control for the one above: C and F ARE playable, so "everything is out
// of tune" is not what this package reports.
func TestCAndFArePlayable(t *testing.T) {
	for _, n := range []struct {
		name string
		hz   float64
	}{{"C2", 65.41}, {"C3", 130.81}, {"F2", 87.31}, {"F3", 174.61}} {
		_, _, cents := Nearest(n.hz, Playable(), clk)
		if math.Abs(cents) > 5 {
			t.Errorf("%s is %.1f cents off; it should be within a couple", n.name, cents)
		}
	}
}

func TestSweepFindsTheThreeUsableRegistersForTheLine(t *testing.T) {
	// Sweeping four octaves from F1, only three tonics hold all four line notes in tune
	// on a SINGLE waveform. That count is what made the register a choice of three and
	// not a free parameter.
	got := 0
	for _, f := range Sweep(43.654, 4, line, clk) {
		if math.Abs(f.OneWorst) <= 16 {
			got++
		}
	}
	if got != 3 {
		t.Errorf("%d registers hold the line within 16 cents on one waveform; it was 3 "+
			"(F1 and C#2 on AUDC 1, C#3 on AUDC 12) when roll-hi was built", got)
	}
}

func TestOnlyThreePitchClassesRepeatAcrossOctaves(t *testing.T) {
	// The finding a piece was written from, stated precisely. Within 8 cents over
	// 95..800 Hz the TIA offers six pitch classes, but only THREE of them appear in
	// more than one octave: C, F and A#. The other three (A, B, G) exist at exactly one
	// pitch each and cannot carry a line that moves between registers.
	//
	// The loose version of this claim -- "the TIA has three pitch classes" -- is what
	// went into a ROM comment, and it is wrong. The test is written the strict way so
	// the comment cannot drift back.
	oct := map[string]map[int]bool{}
	for _, p := range InTune(95, 800, 8, clk) {
		cl, o := p.Note[:len(p.Note)-1], int(p.Note[len(p.Note)-1]-'0')
		if oct[cl] == nil {
			oct[cl] = map[int]bool{}
		}
		oct[cl][o] = true
	}
	var repeated []string
	for cl, os := range oct {
		if len(os) > 1 {
			repeated = append(repeated, cl)
		}
	}
	sort.Strings(repeated)
	want := []string{"A#", "C", "F"}
	if len(repeated) != len(want) {
		t.Fatalf("classes present in more than one octave: %v, want %v (all classes: %d)", repeated, want, len(oct))
	}
	for i := range want {
		if repeated[i] != want[i] {
			t.Errorf("classes present in more than one octave: %v, want %v", repeated, want)
			break
		}
	}
}

func TestInTuneTightensAsTheToleranceTightens(t *testing.T) {
	// A tolerance that changes nothing would mean the filter is not being applied.
	wide := len(InTune(95, 800, 30, clk))
	tight := len(InTune(95, 800, 3, clk))
	if tight >= wide {
		t.Fatalf("30 cents gives %d pitches and 3 cents gives %d; the tolerance is not doing anything", wide, tight)
	}
	if tight == 0 {
		t.Fatal("nothing at all is within 3 cents, which would make the tight end useless")
	}
}
