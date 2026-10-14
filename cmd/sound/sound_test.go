package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/hold"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// soundROM is built here, so the test says what it needs: a sound that answers to the
// buttons. Channel 0 plays a square wave (AUDC 4) at volume 8, at AUDF 15, and at AUDF 7
// -- an octave up -- while fire is down. Channel 1 is the same wave at AUDF 3, silent
// unless the stick is pushed left.
const soundROM = `
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
AUDC0   = $15
AUDC1   = $16
AUDF0   = $17
AUDF1   = $18
AUDV0   = $19
AUDV1   = $1A
INPT4   = $3C
SWCHA   = $0280
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #4
        sta AUDC0
        sta AUDC1
        lda #3
        sta AUDF1
        lda #VOLUME
        sta AUDV0
        ldy #15
        bit INPT4
        bmi Up              ; bit 7 set = fire not pressed
        ldy #7
Up:     sty AUDF0
        ldy #0
        lda SWCHA
        and #%01000000      ; bit 6 clear = left pushed
        bne NoLeft
        ldy #6
NoLeft: sty AUDV1
        ldx #37
Vb:     sta WSYNC
        dex
        bne Vb
        lda #0
        sta VBLANK
        ldx #192
Pic:    sta WSYNC
        dex
        bne Pic
        lda #2
        sta VBLANK
        ldx #30
Os:     sta WSYNC
        dex
        bne Os
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`

func buildSoundROM(t *testing.T, volume string) string {
	t.Helper()
	dir := t.TempDir()
	asm, bin := filepath.Join(dir, "sound.asm"), filepath.Join(dir, "sound.bin")
	src := strings.Replace(soundROM, "#VOLUME", "#"+volume, 1)
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Skipf("dasm is not available, nothing can be said about the recording here: %v\n%s", err, out)
	}
	return bin
}

func mustRecord(t *testing.T, rom string, skip, frames int, spec string) *recording {
	t.Helper()
	holds, err := hold.Parse(spec)
	if err != nil {
		t.Fatal(err)
	}
	r, err := record(rom, skip, frames, holds)
	if err != nil {
		t.Fatalf("-hold %q: %v", spec, err)
	}
	return r
}

// risings counts the upward zero crossings of x: one a period for a square wave.
func risings(x []float64) int {
	n := 0
	for i := 1; i < len(x); i++ {
		if x[i-1] <= 0 && x[i] > 0 {
			n++
		}
	}
	return n
}

// The claim: the WAV is the sound the ROM makes. The file is read back by the decoder
// the measuring tools use, and the pitch counted in it is the one the registers ask for.
func TestTheWAVHoldsThePitchTheRegistersAskFor(t *testing.T) {
	rom := buildSoundROM(t, "8")
	const skip, frames = 5, 60
	r := mustRecord(t, rom, skip, frames, "")
	x, peak := r.pcm()
	got, gotRate, err := audioingest.DecodeWAV(audioingest.EncodeWAV(x, rate))
	if err != nil {
		t.Fatalf("the WAV does not read back: %v", err)
	}
	if gotRate != rate || len(got) != frames*524 {
		t.Fatalf("read back %d samples at %d Hz, want %d at %d (60 frames of 262 lines, two samples a line)",
			len(got), gotRate, frames*524, rate)
	}
	if peak != 8 {
		t.Errorf("peak level %d, want 8 (one channel at volume 8, the other silent)", peak)
	}
	period := audio.PeriodSamples(4, 15)
	want := len(got) / period
	if n := risings(got); n < want-2 || n > want+2 {
		t.Errorf("%d periods in the WAV, want %d (AUDC 4 AUDF 15 is %d samples a period)", n, want, period)
	}
	if s := r.frameLengths(); s != "524 samples a frame" {
		t.Errorf("frame lengths: %s", s)
	}
	if r.first != skip+1 || len(r.regs) != frames {
		t.Errorf("recorded frames %d.. (%d of them), want %d.. (%d)", r.first, len(r.regs), skip+1, frames)
	}
	for i, s := range r.regs {
		c := s.Channel0
		if c.Control != 4 || c.Freq != 15 || c.Volume != 8 || s.Channel1.Volume != 0 {
			t.Fatalf("frame %d: registers %+v, want channel 0 at 4/15/8 and channel 1 silent", r.first+i, s)
		}
	}
	if text := r.regsText(rom); !strings.Contains(text, "\n6 4 15 8 4 3 0\n") || strings.Count(text, "\n") != frames+2 {
		t.Errorf("the register file:\n%s", text)
	}
}

// The claim: a button named in -hold is down while the ROM runs, from the frame it says,
// and the recording is of the sound with it down. Each row is a control for the others.
func TestHoldChangesWhatIsRecordedFromTheFrameItNames(t *testing.T) {
	rom := buildSoundROM(t, "8")
	const frames = 40
	cases := []struct {
		spec      string
		freqFrom  int // the first frame channel 0 is at AUDF 7; 0 = never
		leftSound bool
	}{
		{"", 0, false},
		{"fire", 1, false},
		{"left", 0, true},
		{"fire@20", 20, false},
		{"fire@20,left", 20, true},
	}
	for _, c := range cases {
		r := mustRecord(t, rom, 0, frames, c.spec)
		for i, s := range r.regs {
			f := r.first + i
			if f < 3 {
				continue // the ROM has not reached its first write yet
			}
			wantFreq := uint8(15)
			if c.freqFrom != 0 && f >= c.freqFrom {
				wantFreq = 7
			}
			wantV1 := uint8(0)
			if c.leftSound {
				wantV1 = 6
			}
			if s.Channel0.Freq != wantFreq || s.Channel1.Volume != wantV1 {
				t.Errorf("-hold %q frame %d: AUDF0 %d (want %d), AUDV1 %d (want %d)",
					c.spec, f, s.Channel0.Freq, wantFreq, s.Channel1.Volume, wantV1)
				break
			}
		}
	}
	// and in the samples, not only in the registers: fire for the second half of the
	// recording is an octave up there, so that half holds twice the periods of the first
	r := mustRecord(t, rom, 0, frames, "fire@21")
	x, _ := r.pcm()
	half := len(x) / 2
	a, b := risings(x[:half]), risings(x[half:])
	if b < 2*a-8 || b > 2*a+8 {
		t.Errorf("fire from frame 21 of 40: %d periods in the first half and %d in the second, want twice as many", a, b)
	}
}

// The claim: a recording in which nothing sounds is told apart from one in which
// something does, whatever level the channels stand at.
func TestSilenceIsToldApart(t *testing.T) {
	if quiet, _ := mustRecord(t, buildSoundROM(t, "8"), 5, 20, "").silent(); quiet {
		t.Error("a ROM playing a square wave at volume 8 was called silent")
	}
	quiet, level := mustRecord(t, buildSoundROM(t, "0"), 5, 20, "").silent()
	if !quiet || level != 0 {
		t.Errorf("a ROM with both volumes at 0: silent %v at level %d, want true at 0", quiet, level)
	}
	// the same ROM is not silent once the stick is left: channel 1 sounds
	if quiet, _ := mustRecord(t, buildSoundROM(t, "0"), 5, 20, "left").silent(); quiet {
		t.Error("channel 1 at volume 6 was called silent")
	}
}

// envROM writes a volume that changes every frame (the frame counter's low four bits),
// after PAD two-cycle instructions that do nothing.
const envROM = `
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
AUDC0   = $15
AUDF0   = $17
AUDV0   = $19
Count   = $80
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        inc Count
        repeat PAD
        nop
        repend
        lda #4
        sta AUDC0
        lda #15
        sta AUDF0
        lda Count
        sta AUDV0
        ldx #37
Vb:     sta WSYNC
        dex
        bne Vb
        lda #0
        sta VBLANK
        ldx #192
Pic:    sta WSYNC
        dex
        bne Pic
        lda #2
        sta VBLANK
        ldx #30
Os:     sta WSYNC
        dex
        bne Os
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`

// The claim, and the reason the register file and the WAV are two outputs: code put in
// front of a sound routine moves WHEN in the frame the registers are written, which
// changes the samples -- and so a hash of the sound -- while every value written, frame
// by frame, is what it was. Ten idle instructions (20 cycles) stand in for the code.
func TestCodeInFrontOfTheSoundMovesTheSamplesAndNotTheRegisters(t *testing.T) {
	build := func(pad string) *recording {
		dir := t.TempDir()
		asm, bin := filepath.Join(dir, "env.asm"), filepath.Join(dir, "env.bin")
		if err := os.WriteFile(asm, []byte("PAD = "+pad+"\n"+envROM), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Skipf("dasm is not available: %v\n%s", err, out)
		}
		return mustRecord(t, bin, 5, 120, "")
	}
	a, b := build("0"), build("10")
	moved := 0
	for i := range a.regs {
		if a.regs[i] != b.regs[i] {
			t.Fatalf("frame %d: the registers differ, %+v against %+v", a.first+i, a.regs[i], b.regs[i])
		}
		if i > 0 && a.regs[i].Channel0.Volume != a.regs[i-1].Channel0.Volume {
			moved++
		}
	}
	if moved < 100 {
		t.Fatalf("the volume changed on %d of 119 frames; the test needs it to change on most", moved)
	}
	if len(a.ch0) != len(b.ch0) {
		t.Fatalf("%d samples against %d", len(a.ch0), len(b.ch0))
	}
	differ := 0
	for i := range a.ch0 {
		if a.ch0[i] != b.ch0[i] {
			differ++
		}
	}
	if differ == 0 {
		t.Error("20 cycles in front of the writes changed no sample")
	}
	t.Logf("registers equal on %d frames; %d of %d samples differ", len(a.regs), differ, len(a.ch0))
}
