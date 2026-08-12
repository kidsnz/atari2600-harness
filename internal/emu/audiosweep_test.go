package emu

import (
	"fmt"
	"sort"
	"testing"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// THE WHOLE PITCH TABLE, MEASURED AGAINST THE MACHINE.
//
// pkg/audio computes every note this project plays from f = base/(AUDF+1)/D, and that
// formula stood on FOUR measured operating points -- (4,14) (4,30) (12,14) (6,9) -- out
// of the 16x32 = 512 the hardware has. Four is enough to catch a wrong constant and
// nowhere near enough to catch a divisor that is right for most of a waveform's range
// and wrong at one end, which is the exact shape of error this project keeps producing:
// the pitch-dither rule was derived from one ROM at one write position and was backwards.
//
// Worse, the four were not a random four. Three are square-like (AUDC 4 and 12), and the
// fourth is AUDC 6 -- which happens to be the one polynomial waveform whose transition
// count coincides with its period. So they were, by luck, exactly the cases the old
// measurement could handle. Running the sweep with it reported 145 of 338 pairs off by
// up to 7200 cents, and those "failures" were clean factors of 4, 8 and 64: not the
// formula being wrong, the measurement being unable to see past a waveform's shape. The
// factor is exactly (runs per cycle)/2, confirmed once the shapes were measured -- see
// TestAUDFScalesTheWaveformAndNeverChangesIt and audio.MeasurePeriod's comment.
//
// TWO-SIDED PROOF, because either half alone is weak:
//
//	IsPeriodic     the samples repeat EXACTLY at the formula's period, sample for
//	               sample, with no tolerance at all. On its own this cannot fail a
//	               formula that returns a MULTIPLE of the true period -- everything
//	               repeats at twice its period too.
//	MeasureFundamental  nothing SHORTER than the formula's period correlates better.
//	               On its own this is a similarity score and not an equality.
//
// Together they pin the period from both ends.
//
// THE FIRST CYCLE IS DISCARDED, and that is a measurement and not a fudge. Run without
// it, 14 of 338 pairs fail exactness -- all AUDC 6/10 (div 31) at long periods -- and
// the temptation is to loosen the tolerance. The run-length histogram says what is
// actually happening: AUDC=6 AUDF=31 emits runs of 416 and 576 samples, summing to
// exactly the formula's 992, plus ONE run of 490 at the start of the capture. The
// capture begins part-way through a cycle, so the opening run is truncated. Skipping one
// period makes all 14 exact for the ENTIRE remainder of the stream (13 to 21 further
// repetitions, the whole capture), which is what a start-phase artefact looks like and
// not what a wrong divisor looks like.
func TestEveryPitchTheHardwareHasMatchesTheFormula(t *testing.T) {
	if testing.Short() {
		t.Skip("sweeps 512 register pairs against the emulator")
	}
	const (
		frames    = 30
		minCycles = 8 // whole cycles the capture must hold for a pair to be gradable
	)
	// A frame is 262 scanlines x 2 audio samples.
	capacity := 262 * 2 * frames

	type miss struct {
		audc, audf int
		want, got  int
		corr       float64
		why        string
	}
	var misses []miss
	measured, skippedPitchless, skippedTooLong := 0, 0, 0

	for audc := 0; audc < 16; audc++ {
		// Pitchless by the table's own definition: DC (silence) and the 511-bit poly.
		if audio.Divisor(audc) == 0 || audio.Canonical(audc) == 8 {
			skippedPitchless += 32
			continue
		}
		for audf := 0; audf < 32; audf++ {
			want := audio.PeriodSamples(audc, audf)
			if want*(minCycles+1) > capacity {
				skippedTooLong++
				continue
			}
			e, err := New("NTSC")
			if err != nil {
				t.Fatal(err)
			}
			if err := e.LoadROM(buildAudioROM(t, uint8(audc), uint8(audf), 0x0A)); err != nil {
				t.Fatal(err)
			}
			if err := e.EnableAudioCapture(); err != nil {
				t.Fatal(err)
			}
			warmupStable(t, e)
			e.ResetAudioCapture()
			if err := e.RunFrames(frames); err != nil {
				t.Fatal(err)
			}
			ch0, _ := e.AudioSamples()
			measured++
			if len(ch0) <= want {
				t.Fatalf("AUDC=%d AUDF=%d: capture is %d samples, shorter than one %d-sample period",
					audc, audf, len(ch0), want)
			}
			ch0 = ch0[want:] // discard the truncated opening cycle -- see above

			if !audio.IsPeriodic(ch0, want, minCycles-2) {
				misses = append(misses, miss{audc, audf, want, 0, 0,
					"the samples do not repeat exactly at the formula's period"})
				continue
			}
			// Search from a quarter of the formula's period: enough room to catch the
			// formula returning 2x or 4x the truth, which is the failure IsPeriodic
			// cannot see.
			lo := want / 4
			if lo < 1 {
				lo = 1
			}
			got, corr := audio.MeasureFundamental(ch0, lo, want)
			if got != want {
				misses = append(misses, miss{audc, audf, want, got, corr,
					"a shorter period correlates better, so the formula is a multiple of the real one"})
			}
		}
	}

	t.Logf("MEASURED %d of 512 (AUDC,AUDF) pairs against the emulator; skipped %d pitchless "+
		"(DC and noise) and %d whose period will not fit %d whole cycles in %d frames",
		measured, skippedPitchless, skippedTooLong, minCycles, frames)
	if measured < 200 {
		t.Fatalf("only %d pairs were actually measured; a sweep that skips most of the table "+
			"proves less than the four spot checks it replaces", measured)
	}
	if len(misses) > 0 {
		sort.Slice(misses, func(i, j int) bool { return misses[i].audc < misses[j].audc })
		var b string
		for i, m := range misses {
			if i == 12 {
				b += fmt.Sprintf("\n  ... and %d more", len(misses)-12)
				break
			}
			b += fmt.Sprintf("\n  AUDC=%2d (%-8s div %3d) AUDF=%2d: formula %d samples, measured %d (r=%.3f) -- %s",
				m.audc, audio.Name(m.audc), audio.Divisor(m.audc), m.audf, m.want, m.got, m.corr, m.why)
		}
		t.Errorf("%d of %d measured pairs do not match f = base/(AUDF+1)/D:%s", len(misses), measured, b)
	}
}
