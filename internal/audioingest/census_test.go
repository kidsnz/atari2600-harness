package audioingest

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

const crate = 22050 // Hz; low, so the synthetic fixtures stay fast

// samps converts seconds to samples. It is a function and not an inline expression
// because `int(0.03 * crate)` is a CONSTANT conversion of 661.5 and Go refuses it.
func samps(sec float64) int { return int(sec * crate) }

// track builds a four-to-the-floor bar grid. kickAt and hatAt are the sixteenth slots
// each part fires on, and hatFrom is the bar the hat enters (so an arrangement can be
// synthesised and then recovered).
func track(bars int, beat float64, kickAt, hatAt []int, hatFrom int, hatAmp float64) []float64 {
	n := int(float64(bars) * beat * 4 * crate)
	x := make([]float64, n+crate)
	for b := 0; b < bars; b++ {
		for _, k := range kickAt {
			t := (float64(b)*4 + float64(k)/4) * beat
			at := int(t * crate)
			// body plus a broadband CLICK. Without the click a synthetic kick has
			// almost no energy above 4 kHz, and then the high-band tests pass by
			// measuring nothing -- which is how the first breakdown fixture came out
			// with its quiet half louder than its loud half.
			ck := rand.New(rand.NewSource(int64(b*7 + k)))
			for i := 0; i < samps(0.12) && at+i < len(x); i++ {
				s := float64(i) / crate
				x[at+i] += math.Exp(-s/0.04) * math.Sin(2*math.Pi*55*s)
				x[at+i] += 0.5 * math.Exp(-s/0.004) * ck.NormFloat64()
			}
		}
		if b < hatFrom {
			continue
		}
		for _, k := range hatAt {
			t := (float64(b)*4 + float64(k)/4) * beat
			at := int(t * crate)
			rng := rand.New(rand.NewSource(int64(b*100 + k)))
			for i := 0; i < samps(0.03) && at+i < len(x); i++ {
				s := float64(i) / crate
				x[at+i] += hatAmp * math.Exp(-s/0.008) * rng.NormFloat64()
			}
		}
	}
	return x
}

// The witness. A hat that IS there, on the offbeat eighths, must be found in the band
// it lives in.
func TestARealOffbeatPartIsFound(t *testing.T) {
	beat := 0.5
	x := track(32, beat, []int{0, 4, 8, 12}, []int{2, 6, 10, 14}, 0, 0.8)
	c, err := SlotCensus(x, crate, beat, 0, [2]float64{4000, 10000}, 4)
	if err != nil {
		t.Fatal(err)
	}
	best, _ := c.Loudest()
	if r := best.Offbeat(); r < 0.75 {
		t.Errorf("offbeat ratio %.2f for a hat that is unmistakably present; the census cannot "+
			"see a part it was built to see", r)
	}
	if !strings.Contains(c.Verdict(), "CLEAR offbeat-eighth part") {
		t.Errorf("verdict does not report the part: %q", c.Verdict())
	}
	// and the slots it names must be the ones the hat is actually on
	for _, k := range []int{2, 6, 10, 14} {
		if best.Slot[k] < 0.5 {
			t.Errorf("slot %d reads %.2f; the hat fires there", k, best.Slot[k])
		}
	}
}

// The negative control, and the case that actually occurred. A record with NO separate
// hat -- only a kick whose transient reaches the high band -- must come back saying so.
// If this fails, the tool would have confirmed the invented hat instead of exposing it.
func TestAKickWithNoHatReportsNoOffbeatPart(t *testing.T) {
	beat := 0.5
	x := track(32, beat, []int{0, 4, 8, 12}, nil, 0, 0)
	c, err := SlotCensus(x, crate, beat, 0, [2]float64{4000, 10000}, 4)
	if err != nil {
		t.Fatal(err)
	}
	best, _ := c.Loudest()
	if r := best.Offbeat(); r >= 0.35 {
		t.Errorf("offbeat ratio %.2f on a track whose only drum is on the beat; this tool would "+
			"invent a hat out of the kick's own transient", r)
	}
	if !strings.Contains(c.Verdict(), "NO offbeat-eighth part") {
		t.Errorf("verdict does not state the absence: %q", c.Verdict())
	}
}

// Absence at the start and presence later is the ARRANGEMENT, and it is the reason this
// runs over the whole file instead of one section.
func TestAPartThatEntersLaterIsLocated(t *testing.T) {
	beat := 0.5
	x := track(32, beat, []int{0, 4, 8, 12}, []int{2, 6, 10, 14}, 16, 0.8)
	c, err := SlotCensus(x, crate, beat, 0, [2]float64{4000, 10000}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Sections) < 8 {
		t.Fatalf("%d sections of 4 bars from 32 bars", len(c.Sections))
	}
	early, late := c.Sections[0].Offbeat(), c.Sections[len(c.Sections)-1].Offbeat()
	if early >= 0.35 {
		t.Errorf("the first section reads %.2f, but the hat has not entered yet", early)
	}
	if late < 0.75 {
		t.Errorf("the last section reads %.2f, but the hat is playing there", late)
	}
	best, _ := c.Loudest()
	if best.Bar0 < 16 {
		t.Errorf("the loudest section starts at bar %d; the hat enters at bar 16", best.Bar0)
	}
}

// A band is a CHOICE, and the wrong one hides the part. The same hat must be invisible
// in the bass band -- otherwise the census is reporting broadband energy and the band
// argument is decoration.
func TestTheBandActuallySelects(t *testing.T) {
	beat := 0.5
	x := track(32, beat, []int{0, 4, 8, 12}, []int{2, 6, 10, 14}, 0, 0.8)
	low, err := SlotCensus(x, crate, beat, 0, [2]float64{30, 120}, 4)
	if err != nil {
		t.Fatal(err)
	}
	lb, _ := low.Loudest()
	if r := lb.Offbeat(); r >= 0.35 {
		t.Errorf("the hat shows at %.2f in the 30-120 Hz band, where it has no energy; the band "+
			"filter is not doing anything", r)
	}
}

func TestCensusRefusesInputItCannotJudge(t *testing.T) {
	x := track(8, 0.5, []int{0, 4, 8, 12}, nil, 0, 0)
	if _, err := SlotCensus(x, crate, 0, 0, [2]float64{4000, 10000}, 4); err == nil {
		t.Error("a zero beat length was accepted")
	}
	if _, err := SlotCensus(x, crate, 0.5, 0, [2]float64{10000, 4000}, 4); err == nil {
		t.Error("an inverted band was accepted")
	}
	if _, err := SlotCensus(x, crate, 0.5, 0, [2]float64{4000, 10000}, 64); err == nil {
		t.Error("64-bar sections were accepted from 8 bars of material")
	}
	// A band with nothing in it must fail rather than normalise silence to 1.0 and
	// report a confident pattern made of rounding error.
	if _, err := SlotCensus(make([]float64, 40*crate), crate, 0.5, 0, [2]float64{4000, 10000}, 4); err == nil {
		t.Error("a silent band was accepted")
	}
}

// The defect this floor exists for, reproduced. A breakdown -- every part out, only
// the room left -- produced an offbeat ratio of 1.09 on a real record and the verdict
// named it as the best section in the file. Two nearly-zero numbers divided by each
// other are not a measurement, and note that room noise scores a HIGH ratio precisely
// because it is featureless: the offbeats and the downbeats are equally nothing.
func TestABreakdownCannotWinTheCensus(t *testing.T) {
	beat := 0.5
	loud := track(16, beat, []int{0, 4, 8, 12}, []int{2, 6, 10, 14}, 0, 0.8)
	quiet := make([]float64, len(loud))
	rng := rand.New(rand.NewSource(11))
	for i := range quiet {
		quiet[i] = rng.NormFloat64() * 0.002
	}
	x := append(append([]float64{}, loud...), quiet...)

	c, err := SlotCensus(x, crate, beat, 0, [2]float64{4000, 10000}, 4)
	if err != nil {
		t.Fatal(err)
	}
	best, ok := c.Loudest()
	if !ok {
		t.Fatal("no section was audible, but half the fixture is a full groove")
	}
	if best.StartSec >= 16*4*beat {
		t.Errorf("the best section starts at %.1f s, inside the breakdown; a ratio of two "+
			"near-zero numbers was allowed to win", best.StartSec)
	}
	if c.Silent() == 0 {
		t.Error("no section was counted as too quiet to judge, but half the fixture is room tone")
	}
	if !strings.Contains(c.Verdict(), "too quiet to judge") {
		t.Errorf("the verdict does not say how much of the file it skipped: %q", c.Verdict())
	}
}

// And the floor must not swallow a real answer: a track that is quiet everywhere but
// genuinely has a hat should still be judged, not dismissed.
func TestTheFloorIsRelativeNotAbsolute(t *testing.T) {
	beat := 0.5
	x := track(32, beat, []int{0, 4, 8, 12}, []int{2, 6, 10, 14}, 0, 0.8)
	for i := range x {
		x[i] *= 0.001 // the whole thing mastered 60 dB down
	}
	c, err := SlotCensus(x, crate, beat, 0, [2]float64{4000, 10000}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Loudest(); !ok {
		t.Fatal("a quiet but complete track was thrown away; the floor is normalised to the " +
			"census peak and must not depend on absolute level")
	}
	if !strings.Contains(c.Verdict(), "CLEAR offbeat-eighth part") {
		t.Errorf("verdict: %q", c.Verdict())
	}
}

// duck applies sidechain ducking: everything drops on the beat and recovers, which is
// what a house mix does and what inverted the first metric.
func duck(x []float64, beat float64, depth, recover float64) []float64 {
	y := make([]float64, len(x))
	for i := range x {
		t := float64(i) / crate
		phase := math.Mod(t, beat)
		g := 1 - depth*math.Exp(-phase/recover)
		y[i] = x[i] * g
	}
	return y
}

// The metric this file had to change, with the record's own shape. A flat sixteenth
// texture under sidechain ducking has NO offbeat-eighth part in it, and the old
// downbeat ratio calls it a strong one because the downbeat is the quietest slot in
// the bar. Measured on "Bassline": Offbeat() read 4.44 for exactly this.
func TestSidechainFoolsTheDownbeatRatioAndNotTheLift(t *testing.T) {
	beat := 0.5
	// a continuous sixteenth texture, no eighth-note part anywhere
	sixteenth := beat / 4
	var six []int
	for k := 0; k < Slots; k++ {
		six = append(six, k)
	}
	x := track(32, beat, nil, six, 0, 0.8)
	x = duck(x, beat, 0.85, 0.06)
	_ = sixteenth

	c, err := SlotCensus(x, crate, beat, 0, [2]float64{4000, 10000}, 4)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := c.LoudestLift()
	if !ok {
		t.Fatal("nothing audible in a fixture that is a continuous hat texture")
	}
	if got := s.Offbeat(); got < 1.5 {
		t.Fatalf("the old downbeat ratio reads %.2f here; this test only means something while "+
			"that metric is fooled by ducking, which is why the lift exists", got)
	}
	if got := s.EighthLift(); got > 1.15 {
		t.Errorf("EighthLift %.2f on a FLAT sixteenth texture; the lift is supposed to be blind "+
			"to ducking and see only whether the 'and' stands above the 'e' and the 'a'", got)
	}
	if !strings.Contains(c.Verdict(), "NO offbeat-eighth part") {
		t.Errorf("verdict: %q", c.Verdict())
	}
}

// The near-miss that made KickSlot exist. The grid is only as good as the phase it is
// hung on, and the first real run was two sixteenths out -- which produced a coherent
// and completely false reading of the high band.
func TestKickSlotCatchesAWrongPhase(t *testing.T) {
	beat := 0.5
	x := track(32, beat, []int{0, 4, 8, 12}, nil, 0, 0)

	right, err := SlotCensus(x, crate, beat, 0, [2]float64{30, 120}, 4)
	if err != nil {
		t.Fatal(err)
	}
	slot, conf := right.KickSlot()
	if slot != 0 {
		t.Errorf("with the correct phase the kick lands on sixteenth %d of the beat, want 0", slot)
	}
	if conf < 2 {
		t.Errorf("kick confidence %.2f; a four-to-the-floor kick should hold well above an "+
			"average slot's energy, and a low number here means the check cannot assert a phase", conf)
	}

	// Now hang the same grid two sixteenths early, the exact error that occurred.
	off := 2 * beat / 4
	wrong, err := SlotCensus(x, crate, beat, off, [2]float64{30, 120}, 4)
	if err != nil {
		t.Fatal(err)
	}
	ws, _ := wrong.KickSlot()
	if ws == 0 {
		t.Fatal("a grid hung two sixteenths off still reports the kick on slot 0, so this check " +
			"would not have caught the error it was written for")
	}
	if ws != 2 {
		t.Errorf("kick reported on sixteenth %d of the beat for a grid hung two sixteenths "+
			"early; want 2, which is the correction to apply", ws)
	}
}
