package audioingest_test

import (
	"math"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Can the TIA play a pitch it has no register for?
//
// The ladder is fixed: freq = clock / divisor / (AUDF+1), and in the bass the rungs are
// far enough apart that a real record's key can be unplayable. Measured on "Bassline",
// D and E are more than 23 cents out in every bass octave on every waveform, and that
// is why the reproduction is in a different key from the record.
//
// roms/litmus/litmus_pitchdither.asm offers two ways round it, and this measures both
// on the machine rather than on paper. The arithmetic says alternating AUDF between
// adjacent rungs puts the mean PERIOD within 8 cents of E1; the arithmetic cannot say
// whether the result is a pitch or a 30 Hz buzz, and that is the whole question.

const tiaRate = 31440.0 // NTSC audio sample rate out of emu.AudioSamples

// play pokes the mode, discards the settling frames, and returns the mixed samples.
func play(t *testing.T, mode int, frames int) []float64 {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_pitchdither.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioCapture(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ { // let the ROM reach its loop before the mode is set
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Poke(0x80, uint8(mode)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ { // and let the tone settle before anything is kept
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}
	e.ResetAudioCapture()
	for i := 0; i < frames; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}
	ch0, ch1 := e.AudioSamples()
	if len(ch0) == 0 {
		t.Fatal("no audio captured")
	}
	x := make([]float64, len(ch0))
	for i := range ch0 {
		v := float64(ch0[i])
		if i < len(ch1) {
			v += float64(ch1[i])
		}
		x[i] = v/15 - 1 // 4-bit unsigned to roughly [-1,1], DC removed below
	}
	mean := 0.0
	for _, v := range x {
		mean += v
	}
	mean /= float64(len(x))
	for i := range x {
		x[i] -= mean
	}
	return x
}

func cents(got, want float64) float64 { return 1200 * math.Log2(got/want) }

// The controls. If the measurement cannot recover the two rungs the ROM holds steady,
// nothing it says about the alternation between them means anything.
func TestTheTwoRungsMeasureWhereTheArithmeticSaysTheyAre(t *testing.T) {
	for _, c := range []struct {
		mode int
		want float64
		name string
	}{
		{0, 40.568, "AUDF 24 held"},
		{1, 42.258, "AUDF 23 held"},
	} {
		x := play(t, c.mode, 90)
		hz, conf := audioingest.F0(x, tiaRate, 30, 60)
		if conf < 0.5 {
			t.Errorf("%s: confidence %.2f, too low to call a pitch at all", c.name, conf)
		}
		if e := math.Abs(cents(hz, c.want)); e > 15 {
			t.Errorf("%s measured %.3f Hz, want %.3f (%.1f cents out); the register maps to a "+
				"frequency by clock/31/(AUDF+1) and nothing here should move it", c.name, hz, c.want, e)
		}
	}
}

// THE FINDING, and it is not the obvious one. Alternating AUDF every frame FAILS: the
// tone lands at 40.2 Hz, 41.7 cents below E1 and further out than simply holding the
// flat rung. Alternating every TWO frames works, at +8.8 cents, which is the mean
// period the arithmetic predicts.
//
// The rule behind it: the alternation period must EXCEED the note's own period. E1's
// period is 24.2 ms and a frame is 16.7 ms, so a per-frame swap changes AUDF in the
// middle of nearly every cycle and neither value ever completes one; two frames is
// 33.4 ms and each value gets a whole cycle to itself. A slower swap on a higher note
// would fail the other way, by becoming an audible trill, so this is a window and not
// a direction.
func TestAlternatingEveryTwoFramesReachesTheInBetweenPitch(t *testing.T) {
	hz, conf := audioingest.F0(play(t, 3, 90), tiaRate, 30, 60)
	if conf < 0.7 {
		t.Fatalf("confidence %.2f: the result is not a pitch at all", conf)
	}
	if hz <= 40.568 || hz >= 42.258 {
		t.Fatalf("measured %.3f Hz, which is not between the rungs 40.568 and 42.258", hz)
	}
	if e := math.Abs(cents(hz, 41.396)); e > 10 {
		t.Errorf("measured %.3f Hz against the predicted mean period 41.396 (%.1f cents out)", hz, e)
	}
	if e := math.Abs(cents(hz, 41.203)); e > 15 {
		t.Errorf("%.1f cents from E1; the point of the mechanism is that a note the machine "+
			"has no register for becomes playable, and 15 cents is the budget", e)
	}
}

// The negative control, and the version that would have been built without measuring.
// Swapping every frame is the natural thing to write and it is worse than doing
// nothing: -41.7 cents against the flat rung's -26.9.
func TestAlternatingEveryFrameIsWorseThanNotAlternating(t *testing.T) {
	fast, _ := audioingest.F0(play(t, 2, 90), tiaRate, 30, 60)
	flat, _ := audioingest.F0(play(t, 0, 90), tiaRate, 30, 60)
	ef := math.Abs(cents(fast, 41.203))
	e0 := math.Abs(cents(flat, 41.203))
	t.Logf("every frame %.2f Hz (%.1f c from E1) vs the flat rung held %.2f Hz (%.1f c)", fast, ef, flat, e0)
	if ef <= e0 {
		t.Errorf("swapping every frame (%.1f c) is not worse than holding the flat rung (%.1f c); "+
			"if that changes, the two-frame rule below is not the reason this works", ef, e0)
	}
	if fast >= 40.568 {
		t.Errorf("every-frame alternation measured %.3f Hz, at or above the flat rung; it is "+
			"supposed to fall BELOW both rungs, which is what makes it useless", fast)
	}
}

// Detuning two channels does not fuse into one pitch: the spectrum keeps two separate
// peaks and the estimator locks to one of them. It also throws 3.5x as much energy
// outside the note as a steady tone. Recorded because it is the other obvious idea and
// it costs both channels, so it needs to be ruled out explicitly rather than forgotten.
func TestDetuningTwoChannelsDoesNotFuse(t *testing.T) {
	hz, _ := audioingest.F0(play(t, 4, 90), tiaRate, 30, 60)
	t.Logf("two channels detuned: %.3f Hz, %.1f cents from E1", hz, cents(hz, 41.203))
	if math.Abs(cents(hz, 41.396)) < 10 {
		t.Errorf("the detune measured %.3f Hz, within 10 cents of the mean period; if it DOES "+
			"fuse then it is a second mechanism and the piece has a choice to make", hz)
	}
}

// The part the arithmetic cannot answer: is it a PITCH or a buzz? An alternation
// modulates the tone, and a modulation puts energy either side of the note. This
// measures how much, against the steady rung as the control, so "it sounds rough"
// becomes a number.
//
// Measured: 0.063 steady, 0.066 every frame, 0.065 every two frames, 0.218 detuned.
// The working mechanism adds nothing audible; the one that fails and the one that
// costs two channels are the noisy ones.
func TestTheWorkingAlternationAddsNoRoughness(t *testing.T) {
	steady := sidebandRatio(t, play(t, 0, 90))
	two := sidebandRatio(t, play(t, 3, 90))
	det := sidebandRatio(t, play(t, 4, 90))
	t.Logf("energy outside the note -- steady %.3f | alternating every 2 frames %.3f | detuned %.3f",
		steady, two, det)
	if two > steady*1.5 {
		t.Errorf("the two-frame alternation puts %.3f of its energy outside the note against the "+
			"steady tone's %.3f; the mechanism is only usable while it stays quiet", two, steady)
	}
	if det <= steady*2 {
		t.Errorf("the detune measures %.3f against steady %.3f; it is supposed to be the noisy "+
			"option, and if it is not, ruling it out on that ground is wrong", det, steady)
	}
}

// sidebandRatio is the fraction of spectral energy in 5..35 Hz and 55..95 Hz -- either
// side of the note, where a clean tone has essentially none and a modulated one does.
func sidebandRatio(t *testing.T, x []float64) float64 {
	t.Helper()
	n := 1 << 15
	if len(x) < n {
		t.Fatalf("%d samples, need %d for the transform", len(x), n)
	}
	x = x[:n]
	re := make([]float64, n)
	for i, v := range x {
		re[i] = v * (0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	im := make([]float64, n)
	dft(re, im)
	band := func(lo, hi float64) float64 {
		s := 0.0
		for k := int(lo * float64(n) / tiaRate); k <= int(hi*float64(n)/tiaRate) && k < n/2; k++ {
			s += re[k]*re[k] + im[k]*im[k]
		}
		return s
	}
	side := band(5, 35) + band(55, 95)
	note := band(35, 55)
	if note <= 0 {
		return math.Inf(1)
	}
	return side / note
}

func dft(re, im []float64) {
	n := len(re)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		ang := -2 * math.Pi / float64(l)
		wr, wi := math.Cos(ang), math.Sin(ang)
		for i := 0; i < n; i += l {
			cr, ci := 1.0, 0.0
			for k := 0; k < l/2; k++ {
				ur, ui := re[i+k], im[i+k]
				vr := re[i+k+l/2]*cr - im[i+k+l/2]*ci
				vi := re[i+k+l/2]*ci + im[i+k+l/2]*cr
				re[i+k], im[i+k] = ur+vr, ui+vi
				re[i+k+l/2], im[i+k+l/2] = ur-vr, ui-vi
				cr, ci = cr*wr-ci*wi, cr*wi+ci*wr
			}
		}
	}
}
