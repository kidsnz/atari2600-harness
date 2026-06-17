// Command motion measures how smoothly a TIA object moves frame-to-frame —
// turning "does it judder / ブルブル" into numbers: velocity (1st difference),
// acceleration (2nd difference), and the RMS of the 2nd difference as a
// smoothness score (a constant-velocity glide scores 0, a stutter/snap scores
// high). Automates the hand frame-by-frame trace. docs/verified-coverage.md (VV-4).
//
//	go run ./cmd/motion -rom x.bin -object BL -frames 60 -ywin 115-190
//
// Prints JSON. Track the rendered top over a window where the object moves on a
// uniform background; the X axis (HmovedPixel) is always exact.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/motion"
)

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	object := flag.String("object", "BL", "object to track: P0 M0 P1 M1 BL")
	frames := flag.Int("frames", 60, "frames to track")
	warmup := flag.Int("warmup", 0, "frames to settle before tracking")
	ywin := flag.String("ywin", "0-260", "scanline search window lo-hi (grid-y) for the rendered top")
	flag.Parse()

	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: motion -rom x.bin [-object BL] [-frames 60] [-ywin 115-190] [-warmup N]")
		os.Exit(2)
	}
	yTop, yBot, err := parseWin(*ywin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR -ywin: %v\n", err)
		os.Exit(2)
	}

	e, err := emu.New("NTSC")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	if err := e.LoadROM(*rom); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR rom: %v\n", err)
		os.Exit(2)
	}
	if *warmup > 0 {
		if err := e.RunFrames(*warmup); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR warmup: %v\n", err)
			os.Exit(2)
		}
	}

	tr, err := motion.TrackObject(e, *object, *frames, yTop, yBot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	out, _ := json.MarshalIndent(tr, "", "  ")
	fmt.Println(string(out))
}

func parseWin(s string) (int, int, error) {
	p := strings.SplitN(s, "-", 2)
	if len(p) != 2 {
		return 0, 0, fmt.Errorf("want lo-hi (e.g. 115-190)")
	}
	lo, err := strconv.Atoi(strings.TrimSpace(p[0]))
	if err != nil {
		return 0, 0, err
	}
	hi, err := strconv.Atoi(strings.TrimSpace(p[1]))
	if err != nil {
		return 0, 0, err
	}
	if hi <= lo {
		return 0, 0, fmt.Errorf("hi must exceed lo")
	}
	return lo, hi, nil
}
