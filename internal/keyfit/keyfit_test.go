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
		hz         float64
		waves      []int
		audc, audf int
		cents      float64
	}{
		{87.31, []int{1}, 1, 23, -1.7},    // F2 on the saw voice
		{44.04, []int{6}, 6, 22, 0.0},     // the pedal, exactly on a grid point
		{130.81, []int{1}, 1, 15, 0.2},    // C3
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

// The figure this project actually reproduced: F# minor, tonic / 4th / 5th / b6 / b7 /
// octave, measured off the record. Using the real one matters — a contrived scale would
// let a tuning result be arranged.
var fSharpMinorBass = []int{0, 5, 7, 8, 10, 12}

const fSharp1 = 46.249

// THE WITNESS for SweepDetuned. A piece does not have to start on a semitone, and on a
// ladder this coarse the difference is not a refinement.
func TestLettingTheTonicOffTheSemitoneGridIsWorthAQuarterTone(t *testing.T) {
	semi := Best(Sweep(fSharp1/2, 2, fSharpMinorBass, audio.BaseClockNTSC), true)
	cont := Best(SweepDetuned(fSharp1, 1200, 1, fSharpMinorBass, audio.BaseClockNTSC), true)

	t.Logf("best semitone tonic: %s, one voice AUDC %d, worst degree %+.1f cents",
		semi.TonicName, semi.OneVoice, semi.OneWorst)
	t.Logf("best detuned  tonic: %+.0f cents from F#1, one voice AUDC %d, worst degree %+.1f cents",
		cont.Detune, cont.OneVoice, cont.OneWorst)

	gain := math.Abs(semi.OneWorst) - math.Abs(cont.OneWorst)
	if gain <= 5 {
		t.Errorf("searching off the semitone grid bought %.1f cents (%.1f -> %.1f). If it really "+
			"buys nothing, SweepDetuned is ceremony and should go; this test is what says it is not",
			gain, math.Abs(semi.OneWorst), math.Abs(cont.OneWorst))
	}
	if math.Abs(cont.OneWorst) > 25 {
		t.Errorf("the best detuned tonic is still %.1f cents out, past a quarter of a semitone",
			math.Abs(cont.OneWorst))
	}
}

// The negative control that keeps the witness honest: a detuned sweep INCLUDES the
// semitone tonics (0, ±100, ±200 …), so it can never do worse. A result where it did
// would mean the search or the scoring is broken, not that semitones are better.
func TestTheDetunedSweepCanNeverLoseToTheSemitoneOne(t *testing.T) {
	for _, degs := range [][]int{fSharpMinorBass, {0, 3, 5, 7, 10}, {0, 4, 7, 12}, {0, 7}} {
		semi := Best(Sweep(fSharp1/2, 2, degs, audio.BaseClockNTSC), true)
		cont := Best(SweepDetuned(fSharp1, 1200, 1, degs, audio.BaseClockNTSC), true)
		if math.Abs(cont.OneWorst) > math.Abs(semi.OneWorst)+0.01 {
			t.Errorf("degrees %v: the detuned sweep found %.2f where the semitone sweep found %.2f. "+
				"The detuned range contains every semitone, so this is a bug in the search",
				degs, math.Abs(cont.OneWorst), math.Abs(semi.OneWorst))
		}
	}
}

// JUST INTONATION IS REFUTED FOR THIS MACHINE, and it is recorded as a test so it is not
// proposed again. The ear prefers just ratios, but the TIA cannot act on the preference:
// the largest 12-TET-to-just difference is 17.6 cents and its rungs are 53 to 182 cents
// apart in the bass, so moving the target never moves the register that gets chosen.
func TestAimingAtJustIntonationPicksTheSameRegistersAs12TET(t *testing.T) {
	just := map[int][2]float64{0: {1, 1}, 5: {4, 3}, 7: {3, 2}, 8: {8, 5}, 10: {9, 5}, 12: {2, 1}}
	waves := []int{6}
	differed := 0
	for _, d := range fSharpMinorBass {
		r := just[d]
		t12 := fSharp1 * math.Pow(2, float64(d)/12)
		tj := fSharp1 * r[0] / r[1]
		c12, f12, _ := Nearest(t12, waves, audio.BaseClockNTSC)
		cj, fj, _ := Nearest(tj, waves, audio.BaseClockNTSC)
		gap := 1200 * math.Log2(tj/t12)
		t.Logf("degree %2d: 12-TET picks (%d,%d), just (%+.1f c away) picks (%d,%d)",
			d, c12, f12, gap, cj, fj)
		if c12 != cj || f12 != fj {
			differed++
		}
	}
	if differed > 0 {
		t.Errorf("%d of %d degrees chose a different register under just intonation. That would "+
			"make just tuning actionable here, which the rung spacing says it is not — re-measure "+
			"before acting on it", differed, len(fSharpMinorBass))
	}
}

// The measurement the refutation rests on, asserted rather than left in a comment.
func TestTheRungsAreCoarserThanTheDifferenceBetweenTunings(t *testing.T) {
	const largestTuningGap = 17.6 // cents, the minor seventh: 9/5 against 12-TET's 1000
	worst := math.Inf(1)
	for f := 8; f < 31; f++ {
		lo := audio.Freq(6, f, audio.BaseClockNTSC)
		hi := audio.Freq(6, f+1, audio.BaseClockNTSC)
		gap := math.Abs(1200 * math.Log2(lo/hi))
		if gap < worst {
			worst = gap
		}
	}
	t.Logf("AUDC 6, AUDF 8..31: the CLOSEST two rungs are %.1f cents apart", worst)
	if worst <= largestTuningGap {
		t.Errorf("the tightest rung spacing is %.1f cents, no wider than the %.1f-cent gap between "+
			"12-TET and just intonation — on this evidence a just target COULD change the chosen "+
			"register, and TestAimingAtJustIntonation... should be re-derived", worst, largestTuningGap)
	}
}

// THE TRAP the OneVoiceFundamental field exists for. A tuning search optimises tuning, and
// on this machine the most in-tune waveform for a bass figure is one with no bass in it.
// Recorded as a test because the answer looks right: "AUDC 1, worst degree 16.7 cents" is
// a good-looking result that would produce a thin, pitchless line.
func TestTheMostInTuneVoiceForABassLineHasNoFundamental(t *testing.T) {
	best := Best(SweepDetuned(fSharp1, 1200, 1, fSharpMinorBass, audio.BaseClockNTSC), true)
	t.Logf("most accurate single voice: AUDC %d (%s), worst degree %+.1f cents, "+
		"fundamental %.3f of its first eight harmonics",
		best.OneVoice, audio.Name(best.OneVoice), best.OneWorst, best.OneVoiceFundamental)

	if best.OneVoiceFundamental == 0 {
		t.Fatalf("AUDC %d reports no fundamental strength at all; the table in "+
			"audio.FundamentalStrength has a hole", best.OneVoice)
	}
	// AUDC 6 is what the reproduction actually used, and it is the comparison that matters.
	if best.OneVoiceFundamental >= audio.FundamentalStrength(6) {
		t.Errorf("the most in-tune voice (AUDC %d, fundamental %.3f) is no weaker than AUDC 6 "+
			"(%.3f). If tuning and fundamental strength now agree, this trap has closed and the "+
			"field can go -- but check the spectra first, because they did not agree when it "+
			"was written", best.OneVoice, best.OneVoiceFundamental, audio.FundamentalStrength(6))
	}
}
