package keyfit

import (
	"math"
	"testing"
)

// ntscAudioClock is the TIA audio base clock, 3.579545 MHz / 114 — the same constant `cmd/keyfit`
// defaults to.
const ntscAudioClock = 3579545.0 / 114

// bestWorst returns the smallest |worst-degree error| any tonic achieves for a figure, and the tonic
// that achieves it, sweeping three octaves up from 55 Hz as the command does.
func bestWorst(degrees []int) (cents float64, tonic string) {
	cents = math.Inf(1)
	for _, f := range Sweep(55.0, 3, degrees, ntscAudioClock) {
		if w := math.Abs(f.Worst); w < cents {
			cents, tonic = w, f.TonicName
		}
	}
	return
}

// TestHowWellATuneFitsIsAboutHOWMANYPitchesNotWHICH answers a 2003 question that could only be
// answered by listening, and finds the answer is about the size of the pitch set.
//
// Manuel Polik had SID-to-TIA conversion working in 2003 — *"No. manual. tweaking."* — and stated its
// limit himself: *"Basically it does any SID tune. They will just sound **more or less horrible** ;-)
// **Hubbard doesn't do to well**"* 〔`200308/msg00134`〕. The workflow was convert, listen, judge.
// `cmd/keyfit` can answer it before any conversion: give it the figure as semitones above a tonic and
// it reports, per tonic, how far each degree lands from where it should.
//
// Measured 2026-09-07, best tonic in three octaves from 55 Hz, worst degree in cents:
//
//	3 pitches   0,4,7            F2   2.7c      0,1,2   A2  4.7c     0,6,11  B2  4.7c
//	5 pitches   0,2,4,7,9        C#2 13.9c      0,1,6,7,11  F2 13.4c
//	7 pitches   0,2,4,5,7,9,11   C#2 13.9c
//	12 pitches  0..11            E3  19.3c
//
// ★**Within a size the intervals barely matter; across sizes they matter a lot.** Three pitches land
// within 2.7–4.7 cents whatever they are; five land at 13.4–13.9 whatever they are. **So the count of
// distinct pitches predicts the fit and the choice of pitches does not.**
//
// ★★That is a testable explanation for a subjective 23-year-old remark: **Hubbard's tunes use more
// distinct pitches**, so they land worse — nothing about the style, just the size of the set. And it
// is answerable from a score, before a single byte is converted.
//
// ★★★For the author's own rule — *a cover version may not be out of tune; transpose if you have to* —
// this is the number that decides. A three-note figure is free to sit anywhere; a chromatic one is
// 19 cents out at its best tonic, and no key rescues it.
func TestHowWellATuneFitsIsAboutHOWMANYPitchesNotWHICH(t *testing.T) {
	type figure struct {
		name    string
		degrees []int
	}
	sizes := map[int][]figure{
		3: {
			{"major triad", []int{0, 4, 7}},
			{"three adjacent semitones", []int{0, 1, 2}},
			{"tritone and seventh", []int{0, 6, 11}},
		},
		5: {
			{"major pentatonic", []int{0, 2, 4, 7, 9}},
			{"a deliberately awkward five", []int{0, 1, 6, 7, 11}},
		},
		12: {
			{"chromatic", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		},
	}

	worstBySize := map[int]float64{}
	for size, figs := range sizes {
		lo, hi := math.Inf(1), 0.0
		for _, f := range figs {
			c, tonic := bestWorst(f.degrees)
			if c == 0 {
				t.Fatalf("%s fits perfectly at %s, which no TIA figure does — the sweep is not "+
					"measuring anything", f.name, tonic)
			}
			if c < lo {
				lo = c
			}
			if c > hi {
				hi = c
			}
		}
		worstBySize[size] = hi
		// Within a size, the spread between different interval sets must stay small: that is the
		// claim that WHICH pitches barely matters.
		if hi-lo > 3.0 {
			t.Errorf("at %d pitches the best-tonic error ranges %.1f–%.1f cents across interval "+
				"sets — a spread of %.1f. The finding is that the content hardly matters at a given "+
				"size; a wide spread here would contradict it", size, lo, hi, hi-lo)
		}
	}

	// Across sizes it must matter, and monotonically: more pitches, worse best case.
	if !(worstBySize[3] < worstBySize[5] && worstBySize[5] < worstBySize[12]) {
		t.Errorf("best-case error by pitch count is %.1f (3), %.1f (5), %.1f (12) — it should rise "+
			"with the number of distinct pitches, which is the whole relation",
			worstBySize[3], worstBySize[5], worstBySize[12])
	}
	// And the gap must be large enough to be a finding rather than noise.
	if worstBySize[12]-worstBySize[3] < 10 {
		t.Errorf("twelve pitches are only %.1f cents worse than three. The point is that the "+
			"difference is audible and the interval choice's is not", worstBySize[12]-worstBySize[3])
	}
}
