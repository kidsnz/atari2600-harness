// beamtrace — write→visible-pixel timeline (authoring aid). Runs a ROM and prints,
// per scanline, every TIA write with the beam clock it lands at and the visible-pixel
// span it governs (until the next write to the same register). Answers "where on the
// line does this `sta GRP0` actually paint?".
//
//	go run ./cmd/beamtrace -rom game.bin -scanline 96
//	go run ./cmd/beamtrace -rom game.bin            # every scanline that has writes
//	go run ./cmd/beamtrace -rom game.bin -json
//
// Clock convention: HBLANK = -68..-1, visible = 0..159 (Gopher2600).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	rom := flag.String("rom", "", "ROM .bin to trace (required)")
	spec := flag.String("spec", "NTSC", "TV spec (NTSC/PAL)")
	warmup := flag.Int("warmup", 1, "frames to run before tracing (let positions settle)")
	frames := flag.Int("frames", 1, "frames to trace")
	scanline := flag.Int("scanline", -1, "scanline to report (default: all with writes)")
	asJSON := flag.Bool("json", false, "emit JSON")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: beamtrace -rom game.bin [-scanline N] [-json]")
		os.Exit(2)
	}

	e, err := emu.New(*spec)
	if err != nil {
		fail(err)
	}
	if err := e.LoadROM(*rom); err != nil {
		fail(err)
	}
	if *warmup > 0 {
		if err := e.RunFrames(*warmup); err != nil {
			fail(err)
		}
	}
	ws, err := beamtrace.Trace(e, *frames)
	if err != nil {
		fail(err)
	}
	frs := beamtrace.Frames(ws)
	if len(frs) == 0 {
		fmt.Println("beamtrace: no TIA writes observed")
		return
	}
	frame := frs[0]

	var rows []beamtrace.Row
	if *scanline >= 0 {
		rows = append(rows, beamtrace.Timeline(ws, frame, *scanline))
	} else {
		for _, sl := range beamtrace.Scanlines(ws, frame) {
			rows = append(rows, beamtrace.Timeline(ws, frame, sl))
		}
	}

	if *asJSON {
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
		return
	}
	fmt.Printf("beamtrace: %s — frame %d\n", *rom, frame)
	for _, r := range rows {
		if len(r.Writes) == 0 {
			fmt.Printf("  scanline %3d: (no writes)\n", r.Scanline)
			continue
		}
		fmt.Printf("  scanline %3d:\n", r.Scanline)
		for _, w := range r.Writes {
			val := "    "
			if w.HasValue {
				val = fmt.Sprintf("$%02X ", w.Value)
			}
			fmt.Printf("    clk %+4d  %-6s %-8s %s pixels[%3d..%3d)  @$%04X\n",
				w.Clock, w.Name, "("+string(w.Kind)+")", val, w.VisFrom, w.VisTo, w.PC)
		}
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(2)
}
