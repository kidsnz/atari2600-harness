// Command pcmcheck grades a digitised-speech (4-bit DAC PCM) stream against the
// waveform its ROM meant to play, and says by how much and where it differs.
//
//	go run ./cmd/pcmcheck -rom roms/litmus/litmus_pcm.bin \
//	    -asm roms/litmus/litmus_pcm.asm -start 37 -pitch 1
//
// The intended waveform comes from the ROM's OWN source (`-asm`, the block between
// the `; PCM_TABLE_BEGIN` / `; PCM_TABLE_END` markers), never from what the emulator
// happened to produce, and the slot grid is declared (`-start`, `-pitch`), never
// fitted — a fitted anchor is an anchor that has already absorbed the drift the
// check exists to find. Exits 1 if any graded frame is not clean, so it is usable as
// a gate. See docs/techniques/tia-pcm.md and G3 in docs/capability-gap-audit.md.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/pcm"
)

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	asm := flag.String("asm", "", "source carrying the intended sample table between the PCM_TABLE markers (required)")
	start := flag.Int("start", 0, "scanline the first sample must land on (declared, from the ROM's frame layout)")
	pitch := flag.Int("pitch", 1, "slot pitch in scanlines (1 = one sample per line)")
	reg := flag.String("reg", "AUDV0", "volume register carrying the stream: AUDV0 or AUDV1")
	lowFirst := flag.Bool("low-first", false, "table is packed low nibble first (iesposta) instead of high first (spiceware)")
	frames := flag.Int("frames", 2, "frames to grade")
	warmup := flag.Int("warmup", 3, "frames to run before grading")
	asJSON := flag.Bool("json", false, "emit the reports as JSON")
	flag.Parse()

	if *rom == "" || *asm == "" {
		fmt.Fprintln(os.Stderr, "usage: pcmcheck -rom x.bin -asm x.asm -start N [-pitch 1] [-reg AUDV0] [-frames 2]")
		os.Exit(2)
	}
	var r uint16
	switch *reg {
	case "AUDV0":
		r = pcm.AUDV0
	case "AUDV1":
		r = pcm.AUDV1
	default:
		fmt.Fprintf(os.Stderr, "ERROR -reg: %q is not AUDV0 or AUDV1\n", *reg)
		os.Exit(2)
	}

	src, err := os.ReadFile(*asm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR -asm: %v\n", err)
		os.Exit(2)
	}
	packed, err := pcm.ParseTable(string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR -asm: %v\n", err)
		os.Exit(2)
	}
	spec := pcm.Spec{
		Reg: r, StartLine: *start, LinesPerSample: *pitch,
		Samples: pcm.Unpack(packed, !*lowFirst),
	}

	e, err := emu.New("NTSC")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if err := e.LoadROM(*rom); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR -rom: %v\n", err)
		os.Exit(1)
	}
	for i := 0; i < *warmup; i++ {
		if _, err := e.StepFrame(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	}
	reports, err := pcm.GradeROM(e, spec, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if len(reports) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: no %s write was captured in %d frames — either the ROM does not "+
			"play a stream on that register, or it is not running\n", *reg, *frames)
		os.Exit(1)
	}

	bad := 0
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(reports); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		for _, rep := range reports {
			if !rep.OK() {
				bad++
			}
		}
	} else {
		fmt.Printf("intended: %d samples on %s, first at scanline %d, one every %d line(s)\n",
			len(spec.Samples), *reg, *start, *pitch)
		for _, rep := range reports {
			fmt.Println(rep)
			if !rep.OK() {
				bad++
			}
		}
		fmt.Printf("%d/%d graded frames clean\n", len(reports)-bad, len(reports))
	}
	if bad > 0 {
		os.Exit(1)
	}
}
