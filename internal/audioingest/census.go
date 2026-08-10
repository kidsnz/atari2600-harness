package audioingest

import (
	"fmt"
	"math"
)

// A part census answers a question that comes BEFORE "what notes does it play":
// does this part exist in this record at all, and where.
//
// The question is not rhetorical. Reproducing "Bassline" put a hi-hat on the offbeat
// eighths because a house record has one, and the source comment had to admit the hat
// was an addition: in the opening section the 3-7 kHz and 7-14 kHz attacks land on
// sixteenths 0/4/8/12, which are the kick's own transient, and there is no separate
// hat there. That was worked out by hand, once, for one band, over one section. A
// record is six minutes long and a part that is absent at 0:20 can arrive at 1:30.
//
// So this measures, for every section of the track and every sixteenth of the bar,
// how much energy a chosen band carries. Reading the resulting grid tells you three
// different things:
//
//   - a part that fires only on 0/4/8/12 is on the beat and is probably the kick, or
//     something masked by it;
//   - a part that fires on 2/6/10/14 as strongly is a genuine offbeat part;
//   - a row of sections where a slot is dark and then lights up is an ARRANGEMENT, and
//     it tells you which minute of the record to reproduce.
//
// Absence is a result here, not a failure to find something. A census that comes back
// flat is the finding "there is no separate part in this band", and it is reported
// with the same numbers as a census that comes back with a pattern, so the two cannot
// be confused.

// Slots is the sixteenth-note grid of one bar. Four-four only, which is what this
// analysis has ever been pointed at; anything else would need the meter as an input.
const Slots = 16

// Section is one block of bars and the per-slot energy measured across it.
type Section struct {
	Bar0    int             // first bar of the section, counting from the analysis start
	Bars    int             // bars that contributed
	StartSec float64        // when the section starts, for pointing an ear at it
	Slot    [Slots]float64  // mean band energy at each sixteenth, 0..1 against the loudest slot in the WHOLE census
	Peak    float64         // the section's own loudest slot, on the same scale
}

// Census is the whole grid plus what it was measured with.
type Census struct {
	Band      [2]float64
	BeatSec   float64
	PhaseSec  float64
	BarsPer   int
	Sections  []Section
	RawPeak   float64 // the raw energy that maps to 1.0, kept so two censuses can be compared
}

// bandLimit returns the signal with everything outside [lo,hi] attenuated, by
// subtracting a low-pass at lo from a low-pass at hi. Two one-pole passes run forwards
// and backwards each, so the band edges are gentle -- this separates a hat from a kick,
// it does not resolve neighbouring partials.
func bandLimit(x []float64, lo, hi, rate float64) []float64 {
	high := lowpass(x, hi, rate)
	if lo <= 0 {
		return high
	}
	low := lowpass(x, lo, rate)
	out := make([]float64, len(x))
	for i := range out {
		out[i] = high[i] - low[i]
	}
	return out
}

// SlotCensus measures band energy at every sixteenth of every bar and averages it into
// sections of barsPer bars.
//
// The energy at a slot is the RMS of a window that STARTS at the slot and runs for a
// third of a sixteenth. Starting at the slot rather than centring on it is deliberate:
// centring lets the tail of the previous sixteenth leak in, and on a four-to-the-floor
// record that tail is the kick, which is precisely the thing an offbeat part has to be
// distinguished from.
func SlotCensus(samples []float64, rate int, beatSec, phaseSec float64, band [2]float64, barsPer int) (*Census, error) {
	if beatSec <= 0 {
		return nil, fmt.Errorf("audioingest: beat length %.4f s is not usable", beatSec)
	}
	if barsPer < 1 {
		return nil, fmt.Errorf("audioingest: %d bars per section", barsPer)
	}
	if band[0] >= band[1] {
		return nil, fmt.Errorf("audioingest: band %.0f-%.0f Hz is empty", band[0], band[1])
	}
	sixteenth := beatSec / 4
	barSec := beatSec * 4
	bars := int((float64(len(samples))/float64(rate) - phaseSec) / barSec)
	if bars < barsPer {
		return nil, fmt.Errorf("audioingest: %d bars of material, need at least %d", bars, barsPer)
	}

	x := bandLimit(samples, band[0], band[1], float64(rate))
	win := int(sixteenth * float64(rate) / 3)
	if win < 8 {
		return nil, fmt.Errorf("audioingest: a third of a sixteenth is %d samples, too short to measure", win)
	}

	c := &Census{Band: band, BeatSec: beatSec, PhaseSec: phaseSec, BarsPer: barsPer}
	for b0 := 0; b0+barsPer <= bars; b0 += barsPer {
		s := Section{Bar0: b0, Bars: barsPer, StartSec: phaseSec + float64(b0)*barSec}
		for b := b0; b < b0+barsPer; b++ {
			for k := 0; k < Slots; k++ {
				t := phaseSec + float64(b)*barSec + float64(k)*sixteenth
				i0 := int(t * float64(rate))
				if i0 < 0 || i0+win > len(x) {
					continue
				}
				sum := 0.0
				for i := i0; i < i0+win; i++ {
					sum += x[i] * x[i]
				}
				s.Slot[k] += math.Sqrt(sum / float64(win))
			}
		}
		for k := range s.Slot {
			s.Slot[k] /= float64(barsPer)
			if s.Slot[k] > c.RawPeak {
				c.RawPeak = s.Slot[k]
			}
		}
		c.Sections = append(c.Sections, s)
	}
	if c.RawPeak <= 0 {
		return nil, fmt.Errorf("audioingest: the %.0f-%.0f Hz band is silent across the whole file", band[0], band[1])
	}
	for i := range c.Sections {
		for k := range c.Sections[i].Slot {
			c.Sections[i].Slot[k] /= c.RawPeak
			if c.Sections[i].Slot[k] > c.Sections[i].Peak {
				c.Sections[i].Peak = c.Sections[i].Slot[k]
			}
		}
	}
	return c, nil
}

// Offbeat is the ratio of the offbeat-eighth slots (2, 6, 10, 14) to the downbeat slots
// (0, 4, 8, 12), averaged over a section. It is the number that answers "is there a
// separate offbeat part here":
//
//	near 0    the band only fires on the beat -- whatever is here belongs to the kick
//	near 1    the offbeats are as loud as the beats -- a real offbeat part
//
// Anything in between is a part that is present but quieter than the kick's leakage
// into this band, and the raw slot values are what to read then.
func (s Section) Offbeat() float64 {
	on, off := 0.0, 0.0
	for _, k := range []int{0, 4, 8, 12} {
		on += s.Slot[k]
	}
	for _, k := range []int{2, 6, 10, 14} {
		off += s.Slot[k]
	}
	if on <= 0 {
		return 0
	}
	return off / on
}

// EighthLift is how far the offbeat EIGHTHS (2, 6, 10, 14) stand above the other
// offbeat sixteenths (1, 3, 5, 7, 9, 11, 13, 15).
//
//	near 1    the band is a flat sixteenth texture -- whatever is here is not an
//	          eighth-note part, however loud it is
//	above ~1.15  something fires on the offbeat eighths specifically
//
// This is the metric to read on a modern dance record, and Offbeat() is not. Offbeat()
// divides by the DOWNBEATS, which assumes the downbeats are where the drum is; a house
// mix is sidechained, so the downbeat is the QUIETEST part of the bar in every band
// except the kick's own, and the ratio inverts. Measured on "Bassline": the 6-14 kHz
// downbeat slots read 0.09 against 0.41 everywhere else, which makes Offbeat() report
// 4.44 for a section that has no offbeat part in it at all.
//
// Comparing the offbeat eighths against their NEIGHBOURS has no such assumption. It
// asks the only question that matters -- is there something on the "and" that is not
// on the "e" and the "a" -- and the ducking cancels out because every slot it compares
// sits the same distance from the kick.
func (s Section) EighthLift() float64 {
	eighth, other := 0.0, 0.0
	for _, k := range []int{2, 6, 10, 14} {
		eighth += s.Slot[k]
	}
	for _, k := range []int{1, 3, 5, 7, 9, 11, 13, 15} {
		other += s.Slot[k]
	}
	if other <= 0 {
		return 0
	}
	return (eighth / 4) / (other / 8)
}

// KickSlot returns which sixteenth OF THE BEAT (0..3) carries the most energy,
// averaged over every audible section, together with how far it stands above an
// average slot. Run it on the kick's own band (say 30-60 Hz): it must come back 0. If
// it comes back 2, the grid is hung two sixteenths early and every other reading in
// this census is rotated by two.
//
// Of the beat and not of the bar, because a four-to-the-floor kick CANNOT identify the
// bar. Slots 0, 4, 8 and 12 carry the same drum, so asking which of the sixteen is the
// downbeat has no answer and whichever wins is floating-point noise -- the first
// version asked exactly that and answered 8 for a kick that was equally on all four.
// Aligning a sixteenth grid only needs the phase modulo the beat, which is what this
// returns and all a kick can support.
//
// This exists because the phase WAS wrong, on the first real run, by two sixteenths.
// The 6-14 kHz grid then read as "loud everywhere except the offbeat eighths", which
// is a coherent and completely false musical claim -- the truth was "quiet on the
// downbeats because the mix ducks". A grid is only as good as the beat it hangs on,
// and the cheapest way to check that is to point the same tool at the drum defining it.
func (c *Census) KickSlot() (slot int, confidence float64) {
	var sum [4]float64
	n := 0
	for _, s := range c.Sections {
		if s.Peak < AudibleFloor {
			continue
		}
		for k := 0; k < Slots; k++ {
			sum[k%4] += s.Slot[k]
		}
		n++
	}
	if n == 0 {
		return -1, 0
	}
	best, total := 0, 0.0
	for k := range sum {
		total += sum[k]
		if sum[k] > sum[best] {
			best = k
		}
	}
	if total <= 0 {
		return -1, 0
	}
	// 1.0 means the winner is exactly average (no peak at all); 4.0 means it holds all
	// of the beat's energy.
	return best, sum[best] / (total / 4)
}

// Onsets returns, per sixteenth, how much the band's energy RISES against the previous
// sixteenth, averaged over every audible section and normalised so the largest rise is 1.
//
// A note is a rise, not a level. Reading the level grid answers "where is there energy",
// and a note that sustains for four sixteenths lights all four of them; reading the rise
// answers "where does something START", which is the question when you are counting how
// many notes a figure has. Measured on "Bassline" this is the difference between six
// bright slots and four note onsets.
//
// Negative changes are clipped: a note ending is not an event this is looking for.
func (c *Census) Onsets() [Slots]float64 {
	var sum [Slots]float64
	n := 0
	for _, s := range c.Sections {
		if s.Peak < AudibleFloor {
			continue
		}
		for k := 0; k < Slots; k++ {
			prev := s.Slot[(k+Slots-1)%Slots]
			if d := s.Slot[k] - prev; d > 0 {
				sum[k] += d
			}
		}
		n++
	}
	if n == 0 {
		return sum
	}
	peak := 0.0
	for k := range sum {
		sum[k] /= float64(n)
		if sum[k] > peak {
			peak = sum[k]
		}
	}
	if peak > 0 {
		for k := range sum {
			sum[k] /= peak
		}
	}
	return sum
}

// AudibleFloor is the fraction of the census peak a section must reach before its
// offbeat RATIO means anything.
//
// This number exists because the first run on a real record picked a breakdown as its
// best section and called it a 109% offbeat part. Every slot in that section read
// between 0.01 and 0.05: the music had dropped out, and a ratio of two nearly-zero
// numbers is noise divided by noise. A ratio is only a measurement where there is
// something to measure.
const AudibleFloor = 0.15

// Loudest returns the section with the highest offbeat ratio AMONG the sections that
// are actually playing, which is where to listen if a part exists anywhere at all.
// Silence cannot win.
func (c *Census) Loudest() (Section, bool) {
	var best Section
	found := false
	for _, s := range c.Sections {
		if s.Peak < AudibleFloor {
			continue
		}
		if !found || s.Offbeat() > best.Offbeat() {
			best, found = s, true
		}
	}
	return best, found
}

// Silent counts the sections excluded for being below the audible floor, so a run that
// threw most of the record away says so instead of quietly answering from a fragment.
func (c *Census) Silent() int {
	n := 0
	for _, s := range c.Sections {
		if s.Peak < AudibleFloor {
			n++
		}
	}
	return n
}

// Verdict states what the census found, in the terms the question was asked in. It
// says "no separate part" out loud rather than leaving a reader to infer it from small
// numbers, because the whole point of running this is that absence is an answer.
//
// The judgement is on EighthLift, not on Offbeat -- see EighthLift for why the ratio
// against the downbeats is the wrong question on a sidechained mix.
func (c *Census) Verdict() string {
	best, ok := c.LoudestLift()
	if !ok {
		return fmt.Sprintf("nothing to judge: all %d section(s) are below the audible floor in %.0f-%.0f Hz",
			len(c.Sections), c.Band[0], c.Band[1])
	}
	quiet := ""
	if n := c.Silent(); n > 0 {
		quiet = fmt.Sprintf(" (%d of %d section(s) were too quiet to judge and were skipped)", n, len(c.Sections))
	}
	r := best.EighthLift()
	switch {
	case r < 1.15:
		return fmt.Sprintf("NO offbeat-eighth part in %.0f-%.0f Hz anywhere in this file: at its best "+
			"(bar %d, %.1f s) the offbeat eighths carry %.2fx what the neighbouring sixteenths carry, "+
			"which is a flat sixteenth texture and not a part%s",
			c.Band[0], c.Band[1], best.Bar0, best.StartSec, r, quiet)
	case r < 1.5:
		return fmt.Sprintf("a WEAK offbeat-eighth part in %.0f-%.0f Hz: best section is bar %d (%.1f s), "+
			"offbeat eighths at %.2fx the neighbouring sixteenths%s",
			c.Band[0], c.Band[1], best.Bar0, best.StartSec, r, quiet)
	default:
		return fmt.Sprintf("a CLEAR offbeat-eighth part in %.0f-%.0f Hz: best section is bar %d (%.1f s), "+
			"offbeat eighths at %.2fx the neighbouring sixteenths%s",
			c.Band[0], c.Band[1], best.Bar0, best.StartSec, r, quiet)
	}
}

// LoudestLift is Loudest, ranked by EighthLift. Silence still cannot win.
func (c *Census) LoudestLift() (Section, bool) {
	var best Section
	found := false
	for _, s := range c.Sections {
		if s.Peak < AudibleFloor {
			continue
		}
		if !found || s.EighthLift() > best.EighthLift() {
			best, found = s, true
		}
	}
	return best, found
}

// Enters returns the first audible section whose EighthLift crosses lift, which is
// where an arrangement brings the part in. Absent parts return false, and so do parts
// that were there from the first bar -- both are findings, and neither is an entry.
func (c *Census) Enters(lift float64) (Section, bool) {
	for _, s := range c.Sections {
		if s.Peak < AudibleFloor {
			continue
		}
		if s.EighthLift() >= lift {
			return s, true
		}
	}
	return Section{}, false
}
