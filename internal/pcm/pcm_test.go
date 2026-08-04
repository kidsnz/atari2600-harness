package pcm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const fixtureASM = "../../roms/litmus/litmus_pcm.asm"

// litmusSpec is the fixture's DECLARED contract, every field justified by the .asm:
//
//	Reg            AUDV0 — the ROM's only audio store target in the kernel.
//	StartLine      37 — the ROM emits 3 VSYNC lines then 37 VBLANK lines, and
//	               Gopher2600 numbers scanline 0 as the first line after VSYNC ends,
//	               so the first PCM line is line 37.
//	LinesPerSample 1 — the loop body is two `sta WSYNC` and two `sta AUDV0`.
//	Samples        parsed out of the .asm's own table, not restated here.
func litmusSpec(t *testing.T) Spec {
	t.Helper()
	src, err := os.ReadFile(fixtureASM)
	if err != nil {
		t.Skipf("fixture source unavailable (%s): %v", fixtureASM, err)
	}
	packed, err := ParseTable(string(src))
	if err != nil {
		t.Fatalf("parsing the intended waveform out of %s: %v", fixtureASM, err)
	}
	if len(packed) != 72 {
		t.Fatalf("intended table is %d bytes, want 72 (144 nibble-packed samples)", len(packed))
	}
	return Spec{Reg: AUDV0, StartLine: 37, LinesPerSample: 1, Samples: Unpack(packed, true)}
}

// loadFixture assembles the fixture if the .bin is absent (they are gitignored and
// CI assembles them in a separate step, so a bare checkout has only the .asm) and
// runs it up to a warm frame boundary.
func loadFixture(t *testing.T, asmPath string) *emu.Emu {
	t.Helper()
	bin := build.BinPathFor(asmPath)
	if _, err := os.Stat(bin); err != nil {
		out, aerr := build.Assemble(asmPath, bin)
		if aerr != nil {
			t.Skipf("dasm unavailable or fixture will not assemble: %v\n%s", aerr, out)
		}
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Skipf("emulator unavailable: %v", err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Skipf("ROM unavailable (%s): %v", bin, err)
	}
	for i := 0; i < 3; i++ { // warm up past power-on
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// TestLitmusPCMStreamIsExactAndOnTime is the positive measurement: the fixture's
// 144 samples per frame, graded against the table in its own source, over 3 frames.
func TestLitmusPCMStreamIsExactAndOnTime(t *testing.T) {
	spec := litmusSpec(t)
	e := loadFixture(t, fixtureASM)

	reports, err := GradeROM(e, spec, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 3 {
		t.Fatalf("graded %d frames, want 3 — the capture is not seeing whole frames", len(reports))
	}
	for _, r := range reports {
		t.Log(r.String())
		if r.Intended != 144 {
			t.Fatalf("denominator is %d, want 144", r.Intended)
		}
		if r.Captured != 144 {
			t.Errorf("frame %d: captured %d of %d AUDV0 writes", r.Frame, r.Captured, r.Intended)
		}
		if r.ValueExact != r.Intended {
			t.Errorf("frame %d: %d/%d values exact — %s", r.Frame, r.ValueExact, r.Intended, r)
		}
		if r.OnSlot != r.Intended {
			t.Errorf("frame %d: %d/%d samples land in their slot — %s", r.Frame, r.OnSlot, r.Intended, r)
		}
		if got := len(r.ClockHistogram); got != 1 {
			t.Errorf("frame %d: writes landed at %d distinct beam clocks (%v), want 1 — "+
				"the fixture pads both line types to the same cycle count", r.Frame, got, r.ClockHistogram)
		}
		if r.MeanPitch != 1.0 {
			t.Errorf("frame %d: mean pitch %.4f lines/sample, want exactly 1", r.Frame, r.MeanPitch)
		}
		if !r.OK() {
			t.Errorf("frame %d: OK() false on a stream that should be perfect: %s", r.Frame, r)
		}
	}
}

// --- negative controls on the METRIC (pure, synthetic) ---

// perfectWrites builds the capture a flawless run produces, so each control below
// can damage exactly one property and watch exactly one number move.
func perfectWrites(spec Spec) []beamtrace.Write {
	out := make([]beamtrace.Write, len(spec.Samples))
	for i, v := range spec.Samples {
		out[i] = beamtrace.Write{
			Frame: 7, Scanline: spec.StartLine + i*spec.LinesPerSample, Clock: -23,
			Reg: spec.Reg, Name: "AUDV0", Value: v, HasValue: true,
		}
	}
	return out
}

func TestPerfectStreamGradesClean(t *testing.T) {
	spec := litmusSpec(t)
	r := Grade(spec, perfectWrites(spec), 7)
	if !r.OK() {
		t.Fatalf("a synthetic perfect stream did not grade clean: %s", r)
	}
	t.Log(r.String())
}

// TestOneLineShiftFailsTimingAndPassesValues is the control this whole package
// exists for. Every sample is right; the whole stream is one scanline late. A
// value-only check — which is what "compare the AUDV sequence" means — reports a
// perfect 144/144 and says nothing.
func TestOneLineShiftFailsTimingAndPassesValues(t *testing.T) {
	spec := litmusSpec(t)
	ws := perfectWrites(spec)
	for i := range ws {
		ws[i].Scanline++
	}
	r := Grade(spec, ws, 7)
	t.Log(r.String())

	if r.ValueExact != r.Intended {
		t.Fatalf("the value axis moved under a pure time shift (%d/%d) — the two axes are "+
			"not independent and the control proves nothing", r.ValueExact, r.Intended)
	}
	if r.OnSlot != 0 {
		t.Errorf("%d/%d samples still counted as in-slot after shifting every one of them",
			r.OnSlot, r.Intended)
	}
	if r.WithinOne != r.Intended {
		t.Errorf("within-one = %d/%d; a one-line shift is by definition within one", r.WithinOne, r.Intended)
	}
	if r.MaxAbsLineError != 1 {
		t.Errorf("max |line error| = %d, want 1", r.MaxAbsLineError)
	}
	if r.MeanPitch != 1.0 {
		t.Errorf("mean pitch %.4f — a uniform shift does not change the RATE, only the phase", r.MeanPitch)
	}
	if r.OK() {
		t.Fatal("OK() true for a stream that plays a scanline late")
	}
}

// TestDroppedSampleIsCaught: one sample never reaches the DAC. The count drops, and
// everything after the hole is one slot early — the shape a kernel that skipped an
// iteration produces.
func TestDroppedSampleIsCaught(t *testing.T) {
	spec := litmusSpec(t)
	ws := perfectWrites(spec)
	const drop = 63
	ws = append(ws[:drop], ws[drop+1:]...)

	r := Grade(spec, ws, 7)
	t.Log(r.String())

	if r.Captured != r.Intended-1 {
		t.Errorf("captured %d, want %d after dropping one", r.Captured, r.Intended-1)
	}
	if r.OnSlot != drop {
		t.Errorf("%d slots in place, want %d (everything before the hole and nothing after)",
			r.OnSlot, drop)
	}
	if r.ValueExact >= r.Intended {
		t.Errorf("value axis reported %d/%d exact even though the stream lost a sample",
			r.ValueExact, r.Intended)
	}
	if r.OK() {
		t.Fatal("OK() true for a stream missing a sample")
	}
}

// TestOneWrongValueIsCaughtWithoutMovingTiming is the mirror of the shift control:
// one value corrupted, timing untouched.
func TestOneWrongValueIsCaughtWithoutMovingTiming(t *testing.T) {
	spec := litmusSpec(t)
	ws := perfectWrites(spec)
	const bad = 100
	ws[bad].Value ^= 0x0F

	r := Grade(spec, ws, 7)
	t.Log(r.String())

	if r.ValueExact != r.Intended-1 {
		t.Errorf("%d/%d values exact, want %d", r.ValueExact, r.Intended, r.Intended-1)
	}
	if r.OnSlot != r.Intended {
		t.Errorf("timing moved (%d/%d in slot) under a pure value corruption", r.OnSlot, r.Intended)
	}
	if len(r.ValueErrors) != 1 || r.ValueErrors[0].Index != bad {
		t.Errorf("value errors = %v, want exactly slot %d", r.ValueErrors, bad)
	}
	if r.OK() {
		t.Fatal("OK() true for a stream with a corrupted sample")
	}
}

// TestAccumulatingDriftIsCaught: the kernel loses one line every 32 samples. No
// single sample is far off, so a "within one line" tolerance would wave most of it
// through; the absolute slot grid does not.
func TestAccumulatingDriftIsCaught(t *testing.T) {
	spec := litmusSpec(t)
	ws := perfectWrites(spec)
	for i := range ws {
		ws[i].Scanline += i / 32
	}
	r := Grade(spec, ws, 7)
	t.Log(r.String())

	if r.ValueExact != r.Intended {
		t.Fatalf("value axis moved under pure drift (%d/%d)", r.ValueExact, r.Intended)
	}
	if r.OnSlot != 32 {
		t.Errorf("%d/%d in slot, want 32 (only the first block is still on time)", r.OnSlot, r.Intended)
	}
	if r.MaxAbsLineError != 4 {
		t.Errorf("max |line error| = %d, want 4", r.MaxAbsLineError)
	}
	if r.MeanPitch <= 1.0 {
		t.Errorf("mean pitch %.4f — drift must show as a rate above the declared 1", r.MeanPitch)
	}
	if r.OK() {
		t.Fatal("OK() true for a drifting stream")
	}
}

// TestIntraLineJitterIsCaught: every sample in its slot and correct, but the write
// wanders inside the line. At scanline resolution this is invisible; the clock
// histogram is the axis that sees it.
func TestIntraLineJitterIsCaught(t *testing.T) {
	spec := litmusSpec(t)
	ws := perfectWrites(spec)
	for i := range ws {
		if i%2 == 1 {
			ws[i].Clock = -14
		}
	}
	r := Grade(spec, ws, 7)
	t.Log(r.String())

	if r.OnSlot != r.Intended || r.ValueExact != r.Intended {
		t.Fatalf("jitter moved the slot or value axes (%d, %d of %d) — it should not",
			r.OnSlot, r.ValueExact, r.Intended)
	}
	if len(r.ClockHistogram) != 2 {
		t.Errorf("clock histogram has %d buckets (%v), want 2", len(r.ClockHistogram), r.ClockHistogram)
	}
	if r.OK() {
		t.Fatal("OK() true for a stream whose write wanders inside the scanline")
	}
}

// TestNibbleOrderMismatchIsAValueError pins the thread's own warning (iesposta packs
// low-first, spiceware high-first): the wrong order plays the right NUMBER of
// samples at the right TIME, so only the value axis can catch it.
func TestNibbleOrderMismatchIsAValueError(t *testing.T) {
	spec := litmusSpec(t)
	src, err := os.ReadFile(fixtureASM)
	if err != nil {
		t.Skipf("fixture source unavailable: %v", err)
	}
	packed, err := ParseTable(string(src))
	if err != nil {
		t.Fatal(err)
	}
	wrong := Spec{Reg: spec.Reg, StartLine: spec.StartLine, LinesPerSample: 1,
		Samples: Unpack(packed, false)} // low nibble first

	ws := perfectWrites(spec) // a player that packed high-first
	r := Grade(wrong, ws, 7)
	t.Log(r.String())

	if r.OnSlot != r.Intended {
		t.Errorf("timing moved (%d/%d) — nibble order is a value property", r.OnSlot, r.Intended)
	}
	if r.ValueExact == r.Intended {
		t.Fatal("swapping the nibble order graded as a perfect match; the value axis is not looking")
	}
	t.Logf("nibble-order mismatch: %d/%d values differ", r.Intended-r.ValueExact, r.Intended)
}

// --- negative control at the ROM level: mutate the kernel, not the capture ---

// mutateAndGrade rewrites the fixture source, assembles the mutant into the test's
// own temp dir and grades it against the UNCHANGED intent.
func mutateAndGrade(t *testing.T, spec Spec, mutate func(string) string, label string) Report {
	t.Helper()
	src, err := os.ReadFile(fixtureASM)
	if err != nil {
		t.Skipf("fixture source unavailable: %v", err)
	}
	mutSrc := mutate(string(src))
	if mutSrc == string(src) {
		t.Fatalf("%s: the mutation changed nothing, so this control tests nothing", label)
	}
	dir := t.TempDir()
	asm := filepath.Join(dir, "mutant.asm")
	if err := os.WriteFile(asm, []byte(mutSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := build.BinPathFor(asm)
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Skipf("dasm unavailable, cannot build the %s mutant: %v\n%s", label, err, out)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Skipf("emulator unavailable: %v", err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}
	reports, err := GradeROM(e, spec, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) == 0 {
		t.Fatalf("%s mutant produced no graded frame", label)
	}
	t.Logf("%s mutant: %s", label, reports[0])
	return reports[0]
}

// TestSlowKernelMutantDrifts is the end-to-end control: a real ROM whose playback
// loop takes an extra scanline per sample. Nothing about the sample data changes, so
// the values still march out in the right order — and the stream ends up 143
// scanlines late by the last slot.
func TestSlowKernelMutantDrifts(t *testing.T) {
	spec := litmusSpec(t)
	r := mutateAndGrade(t, spec, func(s string) string {
		// Insert one more line boundary per loop iteration.
		return strings.Replace(s,
			"        sta WSYNC          ; low-nibble line",
			"        sta WSYNC\n        sta WSYNC          ; low-nibble line", 1)
	}, "slow-kernel")

	if r.ValueExact < r.Intended-2 {
		t.Errorf("values fell apart (%d/%d) — this control is supposed to isolate TIMING",
			r.ValueExact, r.Intended)
	}
	if r.OnSlot > 1 {
		t.Errorf("%d/%d samples still counted as in-slot on a kernel that lost a line per sample",
			r.OnSlot, r.Intended)
	}
	if r.MeanPitch < 1.4 {
		t.Errorf("mean pitch %.3f — a kernel taking an extra line per sample must read near 1.5+",
			r.MeanPitch)
	}
	if r.OK() {
		t.Fatal("OK() true for a kernel that plays at the wrong speed")
	}
}

// TestShortTableMutantLosesSamples: delete one packed byte from the ROM's table but
// leave the loop counter alone, so the player reads past the table. The intent still
// says 144 samples; the capture says what actually came out.
func TestShortTableMutantLosesSamples(t *testing.T) {
	spec := litmusSpec(t)
	r := mutateAndGrade(t, spec, func(s string) string {
		return strings.Replace(s, "PACKED  = 72", "PACKED  = 71", 1)
	}, "short-table")

	if r.Captured != 142 {
		t.Errorf("captured %d writes, want 142 (71 bytes × 2 nibbles)", r.Captured)
	}
	if r.ValueExact >= r.Intended {
		t.Error("a stream two samples short graded as fully exact")
	}
	if r.OK() {
		t.Fatal("OK() true for a stream that lost its last two samples")
	}
}

// --- the honest baseline: what the pre-existing audio path could and could not do ---

// TestRawAudioCaptureCarriesValuesButNotTiming measures the capability that already
// existed. emu.EnableAudioCapture records the mixer output at ~2 samples per
// scanline, and every one of the fixture's 144 samples IS in there — but only
// findable by searching for the offset that fits best, and that search is exactly
// what makes it blind to a shift. Measured here: the same search fits a stream
// shifted by a whole scanline just as perfectly, at a different offset.
func TestRawAudioCaptureCarriesValuesButNotTiming(t *testing.T) {
	spec := litmusSpec(t)
	e := loadFixture(t, fixtureASM)
	if err := e.EnableAudioCapture(); err != nil {
		t.Skipf("audio capture unavailable: %v", err)
	}
	e.ResetAudioCapture()
	if _, err := e.StepFrame(); err != nil {
		t.Fatal(err)
	}
	ch0, _ := e.AudioSamples()
	if len(ch0) == 0 {
		t.Fatal("no raw audio samples captured")
	}

	best := func(want []uint8) (hits, off int) {
		for o := 0; o+2*len(want) <= len(ch0); o++ {
			for phase := 0; phase < 2; phase++ {
				n := 0
				for i := range want {
					if ch0[o+2*i+phase] == want[i] {
						n++
					}
				}
				if n > hits {
					hits, off = n, o
				}
			}
		}
		return
	}
	hits, off := best(spec.Samples)
	t.Logf("raw mixer stream: %d samples/frame; best fit to the intended 144 = %d/%d at offset %d",
		len(ch0), hits, len(spec.Samples), off)
	if hits != len(spec.Samples) {
		t.Errorf("the raw stream fits %d/%d — this baseline claim needs re-measuring",
			hits, len(spec.Samples))
	}
	// The point: that fit is a SEARCH. Shift the intent by one slot's worth of raw
	// samples and the search finds an equally perfect fit somewhere else, so the raw
	// path cannot distinguish on-time from one line late.
	if off < 2 {
		t.Fatalf("best offset %d leaves no room to demonstrate the shift", off)
	}
	shifted := ch0[2:]
	n := 0
	for o := 0; o+2*len(spec.Samples) <= len(shifted); o++ {
		for phase := 0; phase < 2; phase++ {
			m := 0
			for i := range spec.Samples {
				if shifted[o+2*i+phase] == spec.Samples[i] {
					m++
				}
			}
			if m > n {
				n = m
			}
		}
	}
	if n != len(spec.Samples) {
		t.Errorf("a one-line-shifted raw stream fit %d/%d; expected an equally perfect %d — "+
			"if it does not, the raw path is more timing-aware than this baseline claims",
			n, len(spec.Samples), len(spec.Samples))
	}
	t.Logf("the same search fits a one-line-shifted stream %d/%d — the raw path grades values, "+
		"not time", n, len(spec.Samples))
}

// TestReadAudioSeesOneOf144 pins the other half of the baseline: the register-level
// tool reads the CURRENT value, so a whole frame of a 144-sample stream reduces to
// one number.
func TestReadAudioSeesOneOf144(t *testing.T) {
	spec := litmusSpec(t)
	e := loadFixture(t, fixtureASM)
	if _, err := e.StepFrame(); err != nil {
		t.Fatal(err)
	}
	st := e.ReadAudio()
	if st.Channel0.Control != 0 {
		t.Errorf("AUDC0 = %d, want 0 — without it AUDV is a tone volume, not an amplitude",
			st.Channel0.Control)
	}
	last := spec.Samples[len(spec.Samples)-1]
	if st.Channel0.Volume != last {
		t.Errorf("read_audio at the frame boundary reports AUDV0=%d; the kernel leaves the last "+
			"sample (%d) standing, so this is the one reading it can make", st.Channel0.Volume, last)
	}
	t.Logf("read_audio yields 1 reading per call out of %d samples in the frame (%.2f%%)",
		len(spec.Samples), 100/float64(len(spec.Samples)))
}

// TestParseTableRejectsGarbage keeps the source-of-truth path honest: silently
// returning an empty intent would make every grade vacuously perfect.
func TestParseTableRejectsGarbage(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"no begin marker", "byte $12,$34\n"},
		{"no end marker", "; PCM_TABLE_BEGIN\n byte $12\n"},
		{"empty block", "; PCM_TABLE_BEGIN\n lda #0\n; PCM_TABLE_END\n"},
		{"bad token", "; PCM_TABLE_BEGIN\n byte $ZZ\n; PCM_TABLE_END\n"},
	} {
		if _, err := ParseTable(c.src); err == nil {
			t.Errorf("%s: ParseTable accepted it", c.name)
		}
	}
	got, err := ParseTable("; PCM_TABLE_BEGIN\nT:\n        byte $01,$23 ; note\n        byte 255\n; PCM_TABLE_END\n")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[1 35 255]" {
		t.Errorf("ParseTable = %v, want [1 35 255]", got)
	}
}
