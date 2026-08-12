package audioingest_test

import (
	"github.com/jetsetilly/gopher2600/hardware/tia/audio/mix"
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

// note names a case for the parameterised litmus: a waveform, the FLAT rung of an
// adjacent pair, and what pitch that pair is trying to reach.
type note struct {
	name   string
	audc   int
	flat   int // the larger AUDF, i.e. the lower pitch
	target float64
}

// E1 is the case the mechanism was found on, in the bass. D2 is the register the melody
// of "Bassline" actually sits in, and it is a different regime: a frame is LONGER than
// D2's period, so the swap rate that works down at E1 is not obviously the one that
// works up here. That has to be measured, not assumed.
var (
	e1 = note{"E1 on AUDC 6", 6, 24, 41.203}
	d2 = note{"D2 on AUDC 1", 1, 28, 73.416}
	e2 = note{"E2 on AUDC 1", 1, 25, 82.407}
	f2 = note{"F#2 on AUDC 1", 1, 22, 92.499}
)

func rung(n note, audf int) float64 {
	div := map[int]float64{1: 15, 6: 31, 12: 6}[n.audc]
	return tiaRate / div / float64(audf+1)
}

// play pokes the case, discards the settling frames, and returns the mixed samples.
// swap is how many frames each rung is held (mode 2 only).
func play(t *testing.T, mode int, n note, swap int, frames int) []float64 {
	return playAt(t, mode, n, swap, 0, frames)
}

// playAt is play with the write POSITION inside the frame as a parameter. It exists
// because the fast swap's result moved when an unrelated WSYNC was added to this ROM,
// and a mechanism whose pitch depends on which scanline the store lands on is not one
// to build a piece out of without knowing that.
func playAt(t *testing.T, mode int, n note, swap, delay, frames int) []float64 {
	return playRaw(t, mode, n, swap, swap, delay, frames)
}

// playRaw is playAt with the SHARP rung's hold separate from the flat one's.
func playRaw(t *testing.T, mode int, n note, swap, duty, delay, frames int) []float64 {
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
	for _, w := range []struct{ addr, val int }{
		// $88 is the SHARP rung's hold. Equal to $84 it is the plain 1:1 swap; leaving
		// it at the ROM's power-on default while poking $84 silently makes a duty ratio,
		// which is how the first run of this after the ROM gained a duty parameter came
		// back with every note sharp.
		{0x80, mode}, {0x82, n.audc}, {0x83, n.flat}, {0x84, swap}, {0x85, 1}, {0x87, delay}, {0x88, duty},
	} {
		if err := e.Poke(uint16(w.addr), uint8(w.val)); err != nil {
			t.Fatal(err)
		}
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
	// Through the TIA's own output stage, not added. mix.Mono is what a speaker receives
	// and it is NOT a sum -- it indexes mono[c0+c1] into a hyperbolic curve, so a loud
	// second channel squashes the first (measured: to 48% of its contribution in silence).
	// The linear sum this used to do is precisely the "the two channels do not interfere"
	// assumption that AtariAge topic 272769 warns about, and having it in OUR code while
	// the engine models the real thing is the worse half of that mistake.
	//
	// It does not change this file's conclusions, and that was checked rather than assumed:
	// mix.Mono is monotonic in (c0+c1), so it cannot move a zero crossing or a period, and
	// every number here is a pitch. It is corrected because the next person to copy this
	// loop will not be measuring a pitch.
	x := make([]float64, len(ch0))
	for i := range ch0 {
		c1 := uint8(0)
		if i < len(ch1) {
			c1 = ch1[i]
		}
		x[i] = float64(mix.Mono(ch0[i], c1)) / 16384 // roughly [-1,1]; DC removed below
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
	for _, n := range []note{e1, d2} {
		for mode, want := range map[int]float64{0: rung(n, n.flat), 1: rung(n, n.flat-1)} {
			hz, conf := audioingest.F0(play(t, mode, n, 2, 90), tiaRate, want*0.75, want*1.35)
			if conf < 0.5 {
				t.Errorf("%s mode %d: confidence %.2f, too low to call a pitch at all", n.name, mode, conf)
			}
			if e := math.Abs(cents(hz, want)); e > 15 {
				t.Errorf("%s mode %d measured %.3f Hz, want %.3f (%.1f cents out)", n.name, mode, hz, want, e)
			}
		}
	}
}

// THE FINDING, and the swap rate is not a constant. The rule is that the swap period
// must EXCEED the note's own period, so it depends on the note:
//
//	E1  41.2 Hz, period 24.2 ms  ->  one frame (16.7 ms) is too FAST; two frames work
//	D2  73.4 Hz, period 13.6 ms  ->  one frame already exceeds it
//
// Measured below for both. The failing rate is measured too, because "swap every frame"
// is the natural thing to write and at E1 it is worse than not swapping at all.
func TestTheSwapRateThatWorksDependsOnTheNote(t *testing.T) {
	for _, c := range []struct {
		n    note
		swap int
	}{{e1, 2}, {d2, 1}, {e2, 1}, {f2, 1}} {
		lo, hi := rung(c.n, c.n.flat), rung(c.n, c.n.flat-1)
		hz, conf := audioingest.F0(play(t, 2, c.n, c.swap, 90), tiaRate, c.n.target*0.8, c.n.target*1.25)
		mean := 2 / (1/lo + 1/hi)
		t.Logf("%-14s swap every %d frame(s): %7.3f Hz (conf %.2f) | rungs %.2f / %.2f | "+
			"%+6.1f c from the note, best single rung is %+6.1f",
			c.n.name, c.swap, hz, conf, lo, hi, cents(hz, c.n.target), bestSingle(c.n))
		if conf < 0.6 {
			t.Errorf("%s: confidence %.2f -- the result is not a pitch", c.n.name, conf)
		}
		if hz <= lo || hz >= hi {
			t.Errorf("%s: measured %.3f Hz, which is not between the rungs %.3f and %.3f",
				c.n.name, hz, lo, hi)
		}
		if e := math.Abs(cents(hz, mean)); e > 12 {
			t.Errorf("%s: measured %.3f Hz against the predicted mean period %.3f (%.1f c out)",
				c.n.name, hz, mean, e)
		}
		if e, b := math.Abs(cents(hz, c.n.target)), math.Abs(bestSingle(c.n)); e >= b {
			t.Errorf("%s: the dither lands %.1f cents from the note and the best SINGLE rung "+
				"lands %.1f -- the mechanism has to beat doing nothing or it is not worth its state", c.n.name, e, b)
		}
	}
}

// bestSingle is how far the nearest single rung on this waveform falls from the note,
// which is the bar the dither has to clear.
func bestSingle(n note) float64 {
	best := 1e9
	for a := 0; a < 32; a++ {
		if c := cents(rung(n, a), n.target); math.Abs(c) < math.Abs(best) {
			best = c
		}
	}
	return best
}

// THE SWAP RATE IS NOT FREE, and this is the test that replaced a wrong one. A first
// version of this file asserted that swapping every frame FAILS and that two frames is
// required. That came from one ROM at one write position, where the per-frame swap
// measured 40.00 Hz; adding an unrelated `sta WSYNC` elsewhere in the same ROM's VBLANK
// moved it to 41.17. Sweeping the store across five scanlines settles it: the per-frame
// swap is the most STABLE of the three rates, and the two-frame swap is the fragile one
// (F#2 moved 14.3 cents across the same five positions).
//
// A mechanism measured at a single operating point is not measured.
func TestThePerFrameSwapIsTheStableOne(t *testing.T) {
	lines := []int{0, 5, 11, 17, 24}
	spread := func(n note, swap int) (float64, float64) {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, d := range lines {
			hz, _ := audioingest.F0(playAt(t, 2, n, swap, d, 90), tiaRate, n.target*0.8, n.target*1.25)
			c := cents(hz, n.target)
			lo, hi = math.Min(lo, c), math.Max(hi, c)
		}
		return hi - lo, (lo + hi) / 2
	}
	for _, n := range []note{e1, f2} {
		s1, m1 := spread(n, 1)
		s2, _ := spread(n, 2)
		t.Logf("%-14s every frame: %.1f c spread (centre %+.1f c) | every 2 frames: %.1f c spread",
			n.name, s1, m1, s2)
		// The absolute bound is loose on purpose: it moves a little whenever this
		// ROM's own instruction count changes, which is not the property being
		// asserted. The COMPARISON below is, and it is what the piece rests on.
		if s1 > 6 {
			t.Errorf("%s: the per-frame swap moves %.1f cents depending only on WHICH SCANLINE "+
				"the store lands on; a pitch that depends on that is not usable", n.name, s1)
		}
		if s1 > s2 {
			t.Errorf("%s: the per-frame swap (%.1f c spread) is less stable than the two-frame "+
				"one (%.1f c); the piece picks per-frame on the strength of this", n.name, s1, s2)
		}
	}
}

// Detuning two channels does not fuse into one pitch: the spectrum keeps two separate
// peaks and the estimator locks to one of them. Recorded because it is the other obvious
// idea and it costs BOTH channels, so it needs ruling out explicitly.
func TestDetuningTwoChannelsDoesNotFuse(t *testing.T) {
	lo, hi := rung(e1, e1.flat), rung(e1, e1.flat-1)
	mean := 2 / (1/lo + 1/hi)
	hz, _ := audioingest.F0(play(t, 3, e1, 2, 90), tiaRate, e1.target*0.8, e1.target*1.25)
	t.Logf("two channels detuned: %.3f Hz, %.1f cents from E1", hz, cents(hz, e1.target))
	if math.Abs(cents(hz, mean)) < 10 {
		t.Errorf("the detune measured %.3f Hz, within 10 cents of the mean period; if it DOES "+
			"fuse then it is a second mechanism and the piece has a choice to make", hz)
	}
}

// The part the arithmetic cannot answer: is it a PITCH or a buzz? A modulation puts
// energy either side of the note. This measures how much, against the SAME note held
// steady as the control -- the absolute figure depends on the waveform's own harmonics
// (AUDC 1 is a rich saw and reads 0.96 even when perfectly steady), so only the
// steady-vs-dithered comparison means anything.
func TestTheDitherAddsNoRoughness(t *testing.T) {
	for _, n := range []note{e1, d2, e2, f2} {
		steady := sidebandRatio(t, play(t, 0, n, 1, 90), n.target)
		dith := sidebandRatio(t, play(t, 2, n, 1, 90), n.target)
		t.Logf("%-14s energy outside the note -- steady %.3f | dithered %.3f", n.name, steady, dith)
		if dith > steady*1.3 {
			t.Errorf("%s: the dither puts %.3f of its energy outside the note against the steady "+
				"tone's %.3f; the mechanism is only usable while it stays quiet", n.name, dith, steady)
		}
	}
}

// sidebandRatio is the fraction of spectral energy outside a fifth either side of the
// note -- where a clean tone has essentially none and a modulated one does.
func sidebandRatio(t *testing.T, x []float64, hz float64) float64 {
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
	note := band(hz/1.5, hz*1.5)
	side := band(hz/4, hz/1.5) + band(hz*1.5, hz*2.3)
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

// A DUTY RATIO does not give a weighted mean, and it is the obvious next idea. A 1:1
// swap can only reach the MIDPOINT of a pair of rungs; where the target is not near
// that midpoint the natural move is to hold one rung longer and land a third of the way.
//
// Measured at D3 (146.8 Hz, AUDC 1, rungs 14/13), holding the sharp rung two frames to
// the flat one's one gives +30.6 cents -- the sharp rung itself, at +33.6 -- and the
// reverse gives the flat rung. At that pitch a frame holds two and a half cycles, so
// each frame is its own pitch and the longer one wins outright.
//
// The technique therefore offers ONE extra pitch per pair, not a continuum. This is a
// test so that stays written down.
func TestADutyRatioDoesNotGiveAWeightedMean(t *testing.T) {
	d3 := note{"D3 on AUDC 1", 1, 14, 146.832}
	lo, hi := rung(d3, d3.flat), rung(d3, d3.flat-1)
	mid := 2 / (1/lo + 1/hi)

	even, _ := audioingest.F0(playDuty(t, d3, 1, 1, 0, 90), tiaRate, d3.target*0.8, d3.target*1.25)
	sharp, _ := audioingest.F0(playDuty(t, d3, 1, 2, 0, 90), tiaRate, d3.target*0.8, d3.target*1.25)
	t.Logf("rungs %.2f / %.2f | 1:1 %.2f Hz (%+.1f c) | 2:1 %.2f Hz (%+.1f c) | midpoint %.2f",
		lo, hi, even, cents(even, d3.target), sharp, cents(sharp, d3.target), mid)

	if math.Abs(cents(even, mid)) > 12 {
		t.Errorf("the 1:1 swap measured %.2f Hz against a midpoint of %.2f; the whole technique "+
			"is that a 1:1 swap lands on the mean period", even, mid)
	}
	// The claim: 2:1 does NOT land between the midpoint and the sharp rung. It lands ON
	// the sharp rung. If that ever stops being true, a duty ratio IS a continuum and the
	// note in docs/techniques/pitch-dither.md is wrong.
	if math.Abs(cents(sharp, hi)) > 15 {
		t.Errorf("a 2:1 duty measured %.2f Hz, which is %.1f cents from the sharp rung %.2f -- "+
			"if it is landing somewhere in between then the technique has a continuum and "+
			"cover-fs-hi did not need to change key", sharp, cents(sharp, hi), hi)
	}
}

// playDuty is playAt with the two rungs held for different numbers of frames.
func playDuty(t *testing.T, n note, flatFrames, sharpFrames, delay, frames int) []float64 {
	t.Helper()
	return playRaw(t, 2, n, flatFrames, sharpFrames, delay, frames)
}
