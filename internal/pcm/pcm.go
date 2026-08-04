// Package pcm grades a digitised-speech (4-bit DAC PCM) stream against the
// waveform its ROM meant to play.
//
// The technique (AtariAge topic/234209 "Doctor Who Berzerk w/speech", iesposta +
// spiceware; topic/184034 for the two-channel variant) is: park AUDC so the tone
// generator contributes nothing, then write the 4-bit VOLUME register AUDVx once per
// fixed time slot — one write per scanline or per two lines — so the amplitude
// envelope IS the waveform. Samples are the top nibble of an 8-bit source sample,
// packed two per byte; the thread warns that packer and player must agree on nibble
// order, and separately that the old Berzerk speech hack made the TV ROLL during
// playback because the playback loop ate the scanline budget.
//
// That second warning is what this package exists for. Comparing the VALUES that
// came out is not enough: a stream that plays every sample correctly but a line late
// each time is a recording at the wrong speed, and a value-only comparison passes it.
// So a stream is graded on two independent axes:
//
//   - VALUE, by write order: the k-th write is compared to the k-th intended sample.
//     A uniform time shift does not move this axis at all.
//   - TIMING, by absolute beam position: the k-th write must land on scanline
//     StartLine + k*LinesPerSample, and at the same intra-line beam clock as its
//     neighbours. A uniform shift, a dropped line and an accumulating drift all move
//     this axis, and none of them move the value axis.
//
// The anchor is DECLARED (Spec.StartLine), never fitted to the capture. Fitting an
// offset is how you lose the measurement: the raw audio mixer stream the harness
// already had (emu.EnableAudioCapture) reproduces all 144 samples of the litmus
// fixture exactly — but only after a search over 236 possible offsets, and a search
// for the best alignment is precisely a search that aligns the drift away.
package pcm

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// AUDV0 / AUDV1 are the canonical TIA write-register addresses of the two volume
// registers — the DAC ports this technique uses.
const (
	AUDV0 uint16 = 0x19
	AUDV1 uint16 = 0x1A
)

// Spec is the declared intent: what the ROM says it is going to play, and when.
// Everything in it comes from the ROM's source (its sample table and its frame
// layout), never from a capture.
type Spec struct {
	Reg            uint16  // which volume register carries the stream
	StartLine      int     // scanline the first sample must land on
	LinesPerSample int     // slot pitch, in scanlines (1 = one sample per line)
	Samples        []uint8 // intended 4-bit samples, in play order
}

// Slot is one intended sample and whatever was found for it.
type Slot struct {
	Index     int   `json:"index"`
	Want      uint8 `json:"want"`
	Got       uint8 `json:"got"`
	Present   bool  `json:"present"`    // a write was paired with this slot at all
	WantLine  int   `json:"want_line"`  // StartLine + Index*LinesPerSample
	GotLine   int   `json:"got_line"`   // scanline the paired write actually landed on
	LineError int   `json:"line_error"` // GotLine - WantLine; signed, in scanlines
	Clock     int   `json:"clock"`      // beam clock of the paired write (HBLANK = -68..-1)
}

// Report is the graded result for one frame. Every count carries the same
// denominator: len(Spec.Samples).
type Report struct {
	Frame    int `json:"frame"`
	Intended int `json:"intended"` // the denominator
	Captured int `json:"captured"` // writes to Spec.Reg found in the frame

	ValueExact  int    `json:"value_exact"` // slots whose value matched
	ValueErrors []Slot `json:"value_errors,omitempty"`

	OnSlot          int    `json:"on_slot"`    // slots with LineError == 0
	WithinOne       int    `json:"within_one"` // slots with |LineError| <= 1
	TimingErrors    []Slot `json:"timing_errors,omitempty"`
	MaxAbsLineError int    `json:"max_abs_line_error"`

	// MeanPitch is the measured mean gap, in scanlines, between consecutive
	// captured writes. It is the playback RATE: a kernel that has lost cycles
	// reports a pitch above DeclaredPitch even when every value is right.
	MeanPitch     float64 `json:"mean_pitch"`
	DeclaredPitch int     `json:"declared_pitch"` // Spec.LinesPerSample, echoed

	// ClockHistogram counts how many writes landed at each intra-line beam clock.
	// One bucket means the kernel reaches the DAC at the same moment on every line;
	// more than one means the write wanders inside the line, which is a jitter a
	// scanline-resolution check cannot see.
	ClockHistogram map[int]int `json:"clock_histogram"`
}

// OK reports whether the stream came out as declared: right count, right values,
// every sample in its slot, and a single intra-line clock.
func (r Report) OK() bool {
	return r.Intended > 0 &&
		r.Captured == r.Intended &&
		r.ValueExact == r.Intended &&
		r.OnSlot == r.Intended &&
		len(r.ClockHistogram) == 1
}

// String is the builder-facing verdict: numbers with their denominator, and the
// first few offenders by index and by how much.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "frame %d: %d/%d samples captured; %d/%d values exact; %d/%d land in their slot, %d/%d within one line",
		r.Frame, r.Captured, r.Intended, r.ValueExact, r.Intended, r.OnSlot, r.Intended, r.WithinOne, r.Intended)
	fmt.Fprintf(&b, "; mean pitch %.3f lines/sample (declared %d)", r.MeanPitch, r.DeclaredPitch)
	fmt.Fprintf(&b, "; %s", r.clockPhrase())
	if len(r.TimingErrors) > 0 {
		fmt.Fprintf(&b, "\n  late/early: %s", describe(r.TimingErrors, func(s Slot) string {
			if !s.Present {
				return fmt.Sprintf("slot %d wanted line %d, never written", s.Index, s.WantLine)
			}
			return fmt.Sprintf("slot %d wanted line %d, got %d (%+d)", s.Index, s.WantLine, s.GotLine, s.LineError)
		}))
	}
	if len(r.ValueErrors) > 0 {
		fmt.Fprintf(&b, "\n  wrong value: %s", describe(r.ValueErrors, func(s Slot) string {
			if !s.Present {
				return fmt.Sprintf("slot %d wanted %d, no write", s.Index, s.Want)
			}
			return fmt.Sprintf("slot %d wanted %d, got %d", s.Index, s.Want, s.Got)
		}))
	}
	return b.String()
}

func (r Report) clockPhrase() string {
	if len(r.ClockHistogram) == 0 {
		return "no writes, so no intra-line clock"
	}
	if len(r.ClockHistogram) == 1 {
		for c := range r.ClockHistogram {
			return fmt.Sprintf("all writes at beam clock %d", c)
		}
	}
	clocks := make([]int, 0, len(r.ClockHistogram))
	for c := range r.ClockHistogram {
		clocks = append(clocks, c)
	}
	sort.Ints(clocks)
	parts := make([]string, len(clocks))
	for i, c := range clocks {
		parts[i] = fmt.Sprintf("%d×%d", r.ClockHistogram[c], c)
	}
	return "writes spread over beam clocks " + strings.Join(parts, ", ")
}

func describe(sl []Slot, f func(Slot) string) string {
	const max = 4
	parts := make([]string, 0, max+1)
	for i, s := range sl {
		if i == max {
			parts = append(parts, fmt.Sprintf("… and %d more", len(sl)-max))
			break
		}
		parts = append(parts, f(s))
	}
	return strings.Join(parts, "; ")
}

// Grade compares a captured write stream against the spec. It is pure: give it
// synthetic writes and it grades those, which is how the negative controls plant
// a dropped sample or a one-line shift without touching the emulator.
//
// Pairing is by ORDER (k-th write ↔ k-th sample) on the value axis and by ABSOLUTE
// scanline on the timing axis. That is deliberate: it is what makes "right notes,
// wrong speed" show up as a timing failure with a clean value column, instead of
// smearing into a mess of value mismatches that hides which of the two went wrong.
func Grade(spec Spec, writes []beamtrace.Write, frame int) Report {
	pitch := spec.LinesPerSample
	if pitch < 1 {
		pitch = 1
	}
	r := Report{
		Frame:          frame,
		Intended:       len(spec.Samples),
		Captured:       len(writes),
		ClockHistogram: map[int]int{},
		DeclaredPitch:  pitch,
	}
	sorted := append([]beamtrace.Write(nil), writes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Scanline != sorted[j].Scanline {
			return sorted[i].Scanline < sorted[j].Scanline
		}
		return sorted[i].Clock < sorted[j].Clock
	})
	for _, w := range sorted {
		r.ClockHistogram[w.Clock]++
	}

	for i, want := range spec.Samples {
		s := Slot{Index: i, Want: want, WantLine: spec.StartLine + i*pitch}
		if i < len(sorted) {
			w := sorted[i]
			s.Present = true
			s.Got = w.Value
			s.GotLine = w.Scanline
			s.Clock = w.Clock
			s.LineError = w.Scanline - s.WantLine
		}
		if s.Present && s.Got == s.Want {
			r.ValueExact++
		} else {
			r.ValueErrors = append(r.ValueErrors, s)
		}
		if s.Present && s.LineError == 0 {
			r.OnSlot++
		} else {
			r.TimingErrors = append(r.TimingErrors, s)
		}
		if s.Present && abs(s.LineError) <= 1 {
			r.WithinOne++
		}
		if s.Present && abs(s.LineError) > r.MaxAbsLineError {
			r.MaxAbsLineError = abs(s.LineError)
		}
	}
	if len(sorted) >= 2 {
		r.MeanPitch = float64(sorted[len(sorted)-1].Scanline-sorted[0].Scanline) / float64(len(sorted)-1)
	}
	return r
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Capture runs the loaded ROM forward and returns, per frame, every write to
// spec.Reg with its beam position. The emulator is advanced; call it after the ROM
// has warmed up and is sitting on a frame boundary.
func Capture(e *emu.Emu, reg uint16, frames int) (map[int][]beamtrace.Write, error) {
	ws, err := beamtrace.Trace(e, frames)
	if err != nil {
		return nil, err
	}
	out := map[int][]beamtrace.Write{}
	for _, w := range ws {
		if w.Reg == reg {
			out[w.Frame] = append(out[w.Frame], w)
		}
	}
	return out, nil
}

// GradeROM is the whole capability in one call: load-and-run is the caller's job,
// this warms up, captures `frames` frames and grades each one.
func GradeROM(e *emu.Emu, spec Spec, frames int) ([]Report, error) {
	if _, err := e.StepFrame(); err != nil { // land on a frame boundary
		return nil, err
	}
	byFrame, err := Capture(e, spec.Reg, frames)
	if err != nil {
		return nil, err
	}
	nums := make([]int, 0, len(byFrame))
	for f := range byFrame {
		nums = append(nums, f)
	}
	sort.Ints(nums)
	var out []Report
	for _, f := range nums {
		out = append(out, Grade(spec, byFrame[f], f))
	}
	return out, nil
}

// --- the intended waveform, read from the ROM's own source ---

var byteLine = regexp.MustCompile(`(?i)^\s*(?:\.)?(?:byte|dc\.b)\s+(.*)$`)

// ParseTable extracts the packed sample bytes from a DASM source between the
// markers `; PCM_TABLE_BEGIN` and `; PCM_TABLE_END`. Reading the table out of the
// source — rather than restating it in Go — is what keeps the grader from grading
// against itself: the bytes the player reads and the bytes the grader expects are
// the same characters in the same file, so a typo in either cannot cancel out.
//
// The cost of that is worth stating, because it was found by a negative control that
// did NOT fire: editing a byte of the table moves the ROM and the intent TOGETHER, so
// the grade stays perfect (measured: 144/144 after flipping `$FF` to `$F1`). This
// package therefore does not check that a ROM plays the RIGHT waveform — it has no
// opinion on what the waveform should be — it checks that the ROM delivers the
// waveform it declares, in time. A value-axis defect is one where the PLAYER fails to
// reproduce its own table (measured: narrowing the low-nibble mask to `#$07` gives
// 107/144 values exact with 144/144 still in slot), and that is the only value defect
// there is anything here to catch.
func ParseTable(src string) ([]byte, error) {
	const begin, end = "PCM_TABLE_BEGIN", "PCM_TABLE_END"
	i := strings.Index(src, begin)
	if i < 0 {
		return nil, fmt.Errorf("no %s marker in source", begin)
	}
	j := strings.Index(src[i:], end)
	if j < 0 {
		return nil, fmt.Errorf("no %s marker after %s", end, begin)
	}
	body := src[i+len(begin) : i+j]

	var out []byte
	for _, line := range strings.Split(body, "\n") {
		if k := strings.Index(line, ";"); k >= 0 {
			line = line[:k]
		}
		m := byteLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, tok := range strings.Split(m[1], ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			v, err := parseByte(tok)
			if err != nil {
				return nil, fmt.Errorf("table token %q: %w", tok, err)
			}
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("table between the markers has no byte directives")
	}
	return out, nil
}

func parseByte(tok string) (byte, error) {
	switch {
	case strings.HasPrefix(tok, "$"):
		v, err := strconv.ParseUint(tok[1:], 16, 8)
		return byte(v), err
	case strings.HasPrefix(tok, "%"):
		v, err := strconv.ParseUint(tok[1:], 2, 8)
		return byte(v), err
	default:
		v, err := strconv.ParseUint(tok, 10, 8)
		return byte(v), err
	}
}

// Unpack splits nibble-packed bytes into 4-bit samples. highFirst selects the
// packing order the thread insists must match between packer and player: spiceware
// packs the high nibble first, iesposta the low nibble first, and getting it wrong
// degrades the voice without changing the sample COUNT — so it is a value-axis
// error the grader can name rather than a rate error.
func Unpack(packed []byte, highFirst bool) []uint8 {
	out := make([]uint8, 0, len(packed)*2)
	for _, b := range packed {
		hi, lo := b>>4, b&0x0F
		if highFirst {
			out = append(out, hi, lo)
		} else {
			out = append(out, lo, hi)
		}
	}
	return out
}
