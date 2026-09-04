package keyfit

import (
	"math"
	"testing"
)

// TestSpreadRanksDifferentlyFromWorst pins the reason `Fit` reports `Spread` and `Mean` alongside
// `Worst`: the three numbers order candidate keys differently, and `Worst` alone prefers the
// worse-sounding one whenever the better key is uniformly displaced.
//
// Glenn Saunders, stella-list 1998: *"It would be wiser to pick … an entire key that ranges from +5
// to +10 out of tune (since it will be **in tune relative to itself** to within 5 cents accuracy)."*
// A listener hears intervals, not absolute pitch — unless the piece is played against an external
// reference, which is why both numbers are reported and neither is applied automatically.
func TestSpreadRanksDifferentlyFromWorst(t *testing.T) {
	// A real fit from this repository's own notes: pitch-dither.md's F# minor bass figure.
	f := Fit{Choices: []Choice{
		{Cents: -0.9}, {Cents: -6.0}, {Cents: 14.7},
	}}
	// recompute the way FitTonic does
	lo, hi, sum := f.Choices[0].Cents, f.Choices[0].Cents, 0.0
	for _, c := range f.Choices {
		if c.Cents < lo {
			lo = c.Cents
		}
		if c.Cents > hi {
			hi = c.Cents
		}
		sum += c.Cents
		if math.Abs(c.Cents) > math.Abs(f.Worst) {
			f.Worst = c.Cents
		}
	}
	f.Spread, f.Mean = hi-lo, sum/3

	if math.Abs(f.Worst-14.7) > 0.05 {
		t.Errorf("Worst = %.1f, want 14.7", f.Worst)
	}
	if math.Abs(f.Spread-20.7) > 0.05 {
		t.Errorf("Spread = %.1f, want 20.7 (from -6.0 to +14.7)", f.Spread)
	}
	if math.Abs(f.Mean-2.6) > 0.05 {
		t.Errorf("Mean = %.1f, want 2.6", f.Mean)
	}

	// The ordering claim, made concrete. Key A sits uniformly +10 sharp: it is further out in
	// absolute terms and perfectly in tune with itself. Key B straddles zero: closer in absolute
	// terms and audibly out of tune. Worst prefers B; Spread prefers A.
	a := Fit{Choices: []Choice{{Cents: 10}, {Cents: 10}, {Cents: 10}}}
	b := Fit{Choices: []Choice{{Cents: -6}, {Cents: 2}, {Cents: 9}}}
	score := func(f *Fit) {
		lo, hi, sum := f.Choices[0].Cents, f.Choices[0].Cents, 0.0
		for _, c := range f.Choices {
			if c.Cents < lo {
				lo = c.Cents
			}
			if c.Cents > hi {
				hi = c.Cents
			}
			sum += c.Cents
			if math.Abs(c.Cents) > math.Abs(f.Worst) {
				f.Worst = c.Cents
			}
		}
		f.Spread, f.Mean = hi-lo, sum/float64(len(f.Choices))
	}
	score(&a)
	score(&b)
	if math.Abs(a.Worst) <= math.Abs(b.Worst) {
		t.Errorf("the uniformly-sharp key should look WORSE by Worst (%.1f vs %.1f), or the example "+
			"no longer demonstrates the disagreement", a.Worst, b.Worst)
	}
	if a.Spread >= b.Spread {
		t.Errorf("the uniformly-sharp key should look BETTER by Spread (%.1f vs %.1f)", a.Spread, b.Spread)
	}
	if a.Spread != 0 {
		t.Errorf("a key displaced uniformly has Spread 0, got %.1f", a.Spread)
	}
}

// TestFitTonicFillsSpread checks the real code path, not a hand-built struct.
func TestFitTonicFillsSpread(t *testing.T) {
	f := FitTonic(92.5, []int{0, 3, 5, 7}, 0) // F#2-ish, a minor-ish set; baseClock 0 = default
	if len(f.Choices) == 0 {
		t.Skip("no playable choices")
	}
	lo, hi := f.Choices[0].Cents, f.Choices[0].Cents
	for _, c := range f.Choices {
		if c.Cents < lo {
			lo = c.Cents
		}
		if c.Cents > hi {
			hi = c.Cents
		}
	}
	if math.Abs(f.Spread-(hi-lo)) > 1e-9 {
		t.Errorf("FitTonic reported Spread %.4f, recomputed %.4f", f.Spread, hi-lo)
	}
	if f.Spread < 0 {
		t.Errorf("Spread must not be negative: %.4f", f.Spread)
	}
	// Mean must lie inside the range it averages.
	if f.Mean < lo-1e-9 || f.Mean > hi+1e-9 {
		t.Errorf("Mean %.4f lies outside [%.4f, %.4f]", f.Mean, lo, hi)
	}
}
