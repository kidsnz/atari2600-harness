// Command sound runs a ROM and writes down what it plays: a WAV to listen to, and the
// sound registers frame by frame for a script to hold against a score.
//
// It is the sound's counterpart of cmd/still. Until it existed the only way to the
// registers over time was the MCP tool read_audio_trace, which a script cannot call, and
// there was no way at all to HEAR a ROM short of opening an emulator -- so a work wrote
// its own (and its first WAV files had a broken header).
//
//	sound [-frames N] [-skip N] [-hold fire,left@60] [-regs FILE] <rom.bin> <out.wav>
//	sound [-frames N] [-skip N] [-hold fire,left@60] -regs FILE <rom.bin>
//
// THE WAV is what the two channels put out, summed: the engine hands over two samples a
// scanline (31,399.5 Hz on NTSC; the header says 31400), each channel's sample being its
// 4-bit level, so the sum runs 0..30. The standing level is taken out by a one-pole
// high-pass at about 2.5 Hz, as a speaker does. The gain is FIXED (a sum of 32 is full
// scale), never normalised, so that two ROMs' files compare in loudness as the ROMs do.
// It is 16-bit mono PCM, the one format internal/audioingest.DecodeWAV reads, so the
// file can be handed straight to the tools that measure a recording.
//
// A WAV IN WHICH NOTHING SOUNDS IS REFUSED, not written: both channels holding one
// level for the whole recording is silence whatever the level is, and a silent file is
// indistinguishable, on the disk, from one that was never listened to. This is cmd/still's
// blank-frame guard for the ear.
//
// THE REGISTER FILE (-regs FILE, or - for standard output) has one line a frame:
//
//	frame AUDC0 AUDF0 AUDV0 AUDC1 AUDF1 AUDV1
//
// as the registers stand at the END of that frame; lines beginning with # are comment.
// A register written twice in a frame shows its last value only, so this file describes
// a music player and not digitised sound (for that: cmd/pcmcheck). The frame number
// counts from 1 at power-on, as cmd/still's -frame does, and -hold counts the same way:
// `fire@60` is down for frame 60 and every frame after it.
//
// The line it prints says how long each frame was in samples. Two samples a scanline
// makes 524 for a 262-line frame, so a ROM whose frames are not all one length shows
// here without being asked.
//
// Answers: Write what a ROM plays as a WAV to listen to and as the sound registers frame by frame, with buttons held down if asked.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/hold"
)

const (
	rate      = 31400 // samples a second written in the header; the TIA's is 31,399.5
	fullScale = 32.0  // the two channels' sum that is written as full scale
	highPass  = 0.9995
)

// A recording is what one run of a ROM played.
type recording struct {
	ch0, ch1 []uint8          // each channel's 4-bit level, two samples a scanline
	regs     []emu.AudioState // the registers at the end of each recorded frame
	first    int              // the frame regs[0] belongs to
	lens     []int            // samples in each recorded frame
}

// record runs the ROM for skip+frames frames with the buttons held and keeps the last
// `frames` of them.
func record(rom string, skip, frames int, holds []hold.Hold) (*recording, error) {
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(rom); err != nil {
		return nil, err
	}
	if err := e.EnableAudioCapture(); err != nil {
		return nil, err
	}
	r := &recording{first: skip + 1}
	for f := 1; f <= skip+frames; f++ {
		if err := hold.Press(e, holds, f); err != nil {
			return nil, err
		}
		if f == skip+1 {
			e.ResetAudioCapture()
		}
		before, _ := e.AudioSamples()
		n := len(before)
		if _, err := e.StepFrame(); err != nil {
			return nil, fmt.Errorf("%s: frame %d: %w", rom, f, err)
		}
		if f <= skip {
			continue
		}
		after, _ := e.AudioSamples()
		r.lens = append(r.lens, len(after)-n)
		r.regs = append(r.regs, e.ReadAudio())
	}
	ch0, ch1 := e.AudioSamples()
	r.ch0 = append([]uint8(nil), ch0...)
	r.ch1 = append([]uint8(nil), ch1...)
	return r, nil
}

// silent reports whether both channels held one level throughout, and that level's sum.
func (r *recording) silent() (bool, int) {
	if len(r.ch0) == 0 {
		return true, 0
	}
	for i := range r.ch0 {
		if r.ch0[i] != r.ch0[0] || r.ch1[i] != r.ch1[0] {
			return false, 0
		}
	}
	return true, int(r.ch0[0]) + int(r.ch1[0])
}

// pcm is the two channels summed, the standing level taken out, in -1..1, and the
// highest sum of the two levels that occurred.
func (r *recording) pcm() (x []float64, peak int) {
	x = make([]float64, len(r.ch0))
	var prevIn, prevOut float64
	if len(r.ch0) > 0 {
		prevIn = float64(r.ch0[0]) + float64(r.ch1[0]) // no thump at the first sample
	}
	for i := range r.ch0 {
		level := int(r.ch0[i]) + int(r.ch1[i])
		if level > peak {
			peak = level
		}
		in := float64(level)
		out := highPass * (prevOut + in - prevIn)
		prevIn, prevOut = in, out
		x[i] = out / fullScale
	}
	return x, peak
}

// regsText is the register file.
func (r *recording) regsText(rom string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: the sound registers at the end of each frame\n", rom)
	b.WriteString("# frame AUDC0 AUDF0 AUDV0 AUDC1 AUDF1 AUDV1\n")
	for i, s := range r.regs {
		fmt.Fprintf(&b, "%d %d %d %d %d %d %d\n", r.first+i,
			s.Channel0.Control, s.Channel0.Freq, s.Channel0.Volume,
			s.Channel1.Control, s.Channel1.Freq, s.Channel1.Volume)
	}
	return b.String()
}

// sounding counts the recorded frames that end with a channel's volume above 0.
func (r *recording) sounding() (ch0, ch1 int) {
	for _, s := range r.regs {
		if s.Channel0.Volume > 0 {
			ch0++
		}
		if s.Channel1.Volume > 0 {
			ch1++
		}
	}
	return
}

// frameLengths says how long the frames were in samples: "524 samples a frame", or the
// range when they were not all alike.
func (r *recording) frameLengths() string {
	lo, hi, odd := r.lens[0], r.lens[0], 0
	for _, n := range r.lens {
		if n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	if lo == hi {
		return fmt.Sprintf("%d samples a frame", lo)
	}
	for _, n := range r.lens {
		if n != r.lens[0] {
			odd++
		}
	}
	return fmt.Sprintf("FRAMES OF %d TO %d SAMPLES (%d of %d differ from the first; two samples a scanline)", lo, hi, odd, len(r.lens))
}

func fail(code int, a ...any) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(code)
}

func main() {
	frames := flag.Int("frames", 600, "frames to record (60 a second)")
	skip := flag.Int("skip", 0, "frames to run before the recording starts")
	regs := flag.String("regs", "", "write the sound registers at the end of every frame to this file (- = standard output)")
	holdSpec := flag.String("hold", "", "buttons of the left joystick to keep down: fire,left@60 (a name alone = from power-on; @N = from frame N on)")
	flag.Parse()
	holds, err := hold.Parse(*holdSpec)
	if err != nil {
		fail(2, err)
	}
	if flag.NArg() < 1 || flag.NArg() > 2 || (flag.NArg() == 1 && *regs == "") || *frames < 1 || *skip < 0 {
		fmt.Fprintln(os.Stderr, "usage: sound [-frames N] [-skip N] [-hold fire,left@60] [-regs FILE] <rom.bin> <out.wav>")
		fmt.Fprintln(os.Stderr, "       sound [-frames N] [-skip N] [-hold fire,left@60] -regs FILE <rom.bin>")
		os.Exit(2)
	}
	rom := flag.Arg(0)
	r, err := record(rom, *skip, *frames, holds)
	if err != nil {
		fail(1, err)
	}
	if *regs != "" {
		text := r.regsText(rom)
		if *regs == "-" {
			fmt.Print(text)
		} else if err := os.WriteFile(*regs, []byte(text), 0o644); err != nil {
			fail(1, err)
		}
	}
	c0, c1 := r.sounding()
	span := fmt.Sprintf("frames %d..%d  %s  volume above 0 at the end of %d (channel 0) and %d (channel 1) of %d frames",
		r.first, r.first+len(r.regs)-1, r.frameLengths(), c0, c1, len(r.regs))
	if flag.NArg() == 1 {
		if *regs != "-" {
			fmt.Printf("%s  %s%s\n", *regs, span, hold.Note(*holdSpec))
		}
		return
	}
	if quiet, level := r.silent(); quiet {
		fail(1, fmt.Sprintf("%s: NOTHING SOUNDS in frames %d..%d -- both channels hold one level (their sum is %d of 30) "+
			"for all %d samples, which is silence whatever the level. No WAV written. Record later frames (-skip), "+
			"or hold the button that starts the sound (-hold)", rom, r.first, r.first+len(r.regs)-1, level, len(r.ch0)))
	}
	x, peak := r.pcm()
	out := flag.Arg(1)
	if err := os.WriteFile(out, audioingest.EncodeWAV(x, rate), 0o644); err != nil {
		fail(1, err)
	}
	line := fmt.Sprintf("%s  %.1f s  peak level %d of 30  %s%s", out, float64(len(x))/rate, peak, span, hold.Note(*holdSpec))
	if *regs == "-" {
		fmt.Fprintln(os.Stderr, line) // standard output is the register file
	} else {
		fmt.Println(line)
	}
}
