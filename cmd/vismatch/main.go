// vismatch — palette-independent visual reproduction diff (Tool A core).
//
// Renders a TARGET frame and YOUR build's frame, compares WHICH TIA object drew
// each pixel (not RGB — palettes differ between ROMs), and reports every
// element-level difference plus a per-element "band diff" that names the exact
// scanline range and lit clock-spans where the playfield/sprite shapes disagree.
// This surfaces, in one pass, the 1-2px band-boundary errors that manual
// sparse-sampling misses.
//
//	go run ./cmd/vismatch -target Outlaw.bin -target-reset -mine build/outlaw.asm \
//	    -target-frames 28 -mine-frames 12 -elem PF -diff overlay.png
//
// Exit 1 when any compared element cell differs (a reproduction gate); 0 on match.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/vismatch"
)

// resolve turns a path into a .bin: if it's a .asm, assemble it first.
func resolve(path string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".asm") {
		return path, nil
	}
	bin := build.BinPathFor(path)
	out, err := build.Assemble(path, bin)
	if err != nil {
		return "", fmt.Errorf("assemble %s failed:\n%s", path, out)
	}
	return bin, nil
}

func main() {
	target := flag.String("target", "", "reference frame: ROM .bin or .asm (required)")
	mine := flag.String("mine", "", "your build: ROM .bin or .asm (required)")
	tFrames := flag.Int("target-frames", 28, "frames to run the target before capture")
	mFrames := flag.Int("mine-frames", 12, "frames to run your build before capture")
	tReset := flag.Bool("target-reset", false, "press console RESET to start the target (many originals need it)")
	mReset := flag.Bool("mine-reset", false, "press console RESET to start your build")
	spec := flag.String("spec", "NTSC", "TV spec")
	elemFilter := flag.String("elem", "", "only report band diffs for this element (PF/P0/P1/M0/M1/BL); empty = all")
	diffOut := flag.String("diff", "", "write an object-attribution overlay PNG here (green=match, red=target-only, blue=mine-only)")
	scale := flag.Int("scale", 4, "overlay upscale factor")
	genpf := flag.Bool("genpf", false, "GENERATE mode: measure the target's cactus/PF bands and emit paste-ready CacLTbl/CacRTbl + CACTOP/CACBOT (no -mine needed)")
	region := flag.String("region", "122-185", "genpf: target scanline range of the PF element, LO-HI")
	arenaTop := flag.Int("arena-top", 74, "genpf: absolute scanline of arena line 0 (scanline→arena-line offset)")
	tableLen := flag.Int("table-len", 112, "genpf: CacLTbl/CacRTbl length in arena lines")
	scenName := flag.String("scenario", "", "drive BOTH ROMs through this behavmatch scenario's input script, then compare the picture — a single-frame diff is exact for a static playfield and misleading for anything that moves, because two ROMs at different points in their game read as a difference in the object")
	tWarmup := flag.Int("target-warmup", 0, "matched state: frames to run the TARGET with no input before the scenario (skip a title screen). Deliberately separate from -target-frames: reusing the capture-frame counts here gave the two sides different warmups and reported a ROM as differing from itself")
	mWarmup := flag.Int("mine-warmup", 0, "matched state: frames to run YOUR build with no input before the scenario")
	scenFile := flag.String("scenarios", "", "load the scenario from a .json file or directory instead of the built-in library")
	untilPlay := flag.Bool("target-until-gameplay", false, "report the first frame at which the TARGET's playfield settles after its opening screen, so -target-frames need not be hand-tuned")
	untilFrames := flag.Int("until-gameplay-frames", 240, "how many frames to search for the settling point")
	untilStable := flag.Int("until-gameplay-stable", 8, "frames the new picture must hold still before it counts as settled")
	maxShift := flag.Int("max-shift", 24, "how far up/down to search for a global vertical offset between the two frames")
	pfFormat := flag.String("format", "arena", "genpf output: 'arena' = Outlaw-style centre CacLTbl/CacRTbl; 'pf012' = full-width PF0tab/PF1tab/PF2tab for a playfield that spans the line")
	flag.Parse()
	if *target == "" {
		fmt.Fprintln(os.Stderr, "usage: vismatch -target ref.bin -mine build.asm [-target-reset] [-elem PF] [-diff out.png]")
		fmt.Fprintln(os.Stderr, "   or: vismatch -target ref.bin -genpf -region 122-185 -arena-top 74")
		os.Exit(2)
	}

	tBin, err := resolve(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// --- GENERATE mode: emit the correct PF tables from the target, one-shot. ---
	if *genpf {
		tg, err := vismatch.ExtractROM(tBin, *spec, *tFrames, *tReset)
		if err != nil {
			fmt.Fprintln(os.Stderr, "target:", err)
			os.Exit(2)
		}
		var lo, hi int
		if _, err := fmt.Sscanf(*region, "%d-%d", &lo, &hi); err != nil {
			fmt.Fprintln(os.Stderr, "bad -region (want LO-HI):", err)
			os.Exit(2)
		}
		if *pfFormat == "pf012" {
			// Full-width playfield: the measurement below already computes every PF
			// byte; the Outlaw path just throws four of the six away.
			bands, mode := vismatch.MeasurePF(tg, lo, hi)
			fmt.Printf("measured %d playfield bands (scanline %d-%d); right half is %s of the left\n",
				len(bands), lo, hi, mode)
			fmt.Printf("  %-11s %-5s %-5s %-5s %s\n", "scanline", "PF0", "PF1", "PF2", "PF clk-spans")
			for _, b := range bands {
				fmt.Printf("  %4d-%-6d $%02X   $%02X   $%02X   %s\n",
					b.ScanlineLo, b.ScanlineHi, b.PF0, b.PF1, b.PF2, b.Spans)
			}
			fmt.Println()
			fmt.Print(vismatch.EmitPFTables(bands, mode, lo, hi))
			os.Exit(0)
		}
		bands := vismatch.MeasureCactus(tg, lo, hi, *arenaTop)
		fmt.Printf("measured %d cactus bands (scanline %d-%d, arena-top %d):\n", len(bands), lo, hi, *arenaTop)
		fmt.Printf("  %-11s %-9s %-5s %-5s %s\n", "scanline", "arena", "CacL", "CacR", "PF clk-spans")
		for _, b := range bands {
			fmt.Printf("  %4d-%-6d %3d-%-5d $%02X   $%02X   %s\n",
				b.ScanlineLo, b.ScanlineHi, b.ArenaLo, b.ArenaHi, b.CacL, b.CacR, b.Spans)
		}
		fmt.Println()
		fmt.Print(vismatch.EmitCactusTables(bands, *tableLen))
		os.Exit(0)
	}

	if *untilPlay {
		gs, err := vismatch.FindGameplayStart(tBin, *spec, *untilFrames, *untilStable)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if !gs.Found {
			fmt.Printf("no settling point in %d frames (playfield changed: %v) — this cartridge may have "+
				"no title screen, or it needs input to start\n", gs.Searched, gs.Changed)
			os.Exit(1)
		}
		fmt.Printf("target's picture settles at frame %d (held still for %d frames) — use -target-frames %d\n",
			gs.Frame, gs.StableFor, gs.Frame)
		os.Exit(0)
	}

	if *mine == "" {
		fmt.Fprintln(os.Stderr, "usage: vismatch -target ref.bin -mine build.asm [-target-reset] [-elem PF] [-diff out.png]")
		os.Exit(2)
	}
	mBin, err := resolve(*mine)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var tg, mg *vismatch.Grid

	// Matched state: drive both ROMs through the SAME input script and compare
	// the picture there. Without it, two ROMs at different points in their game
	// report a moving object as a difference — the difference being when they
	// were looked at, not where the object is.
	if *scenName != "" {
		lib := behavmatch.Library
		if *scenFile != "" {
			l, _, err := behavmatch.LoadScenarios(*scenFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, "scenarios:", err)
				os.Exit(2)
			}
			lib = l
		}
		scn, ok := lib[*scenName]
		if !ok {
			fmt.Fprintln(os.Stderr, "unknown scenario:", *scenName)
			os.Exit(2)
		}
		te, err := behavmatch.RunScenario(tBin, *spec, scn, *tWarmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, "target:", err)
			os.Exit(2)
		}
		me, err := behavmatch.RunScenario(mBin, *spec, scn, *mWarmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mine:", err)
			os.Exit(2)
		}
		fmt.Printf("matched state: both ROMs driven through scenario %q (%d frames)\n", scn.Name, scn.Frames)
		if *tWarmup != *mWarmup {
			// An asymmetry here is legitimate (one ROM has a title screen the other
			// does not) but it is also exactly what makes a ROM differ from itself,
			// so it is stated rather than left in the flags.
			fmt.Printf("  NOTE: warmups differ (target %d, mine %d) — the two sides start from different points\n",
				*tWarmup, *mWarmup)
		}
		tg, mg = vismatch.GridFrom(te), vismatch.GridFrom(me)
	}

	if tg == nil {
		var err error
		tg, err = vismatch.ExtractROM(tBin, *spec, *tFrames, *tReset)
		if err != nil {
			fmt.Fprintln(os.Stderr, "target:", err)
			os.Exit(2)
		}
		mg, err = vismatch.ExtractROM(mBin, *spec, *mFrames, *mReset)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mine:", err)
			os.Exit(2)
		}
	}

	rep := vismatch.Diff(tg, mg)

	fmt.Printf("compared scanlines %d..%d (w=%d)\n", rep.ScanlineLo, rep.ScanlineHi, rep.Width)
	fmt.Printf("per-element mismatched cells (target-only / mine-only):\n")
	for _, el := range []string{"PF", "P0", "P1", "M0", "M1", "BL"} {
		miss, extra := rep.Missing[el], rep.Extra[el]
		if miss == 0 && extra == 0 {
			continue
		}
		fmt.Printf("  %-3s  missing=%-5d extra=%-5d\n", el, miss, extra)
	}

	// Band diffs (the money view): exact scanline ranges + clock-spans.
	bands := rep.Bands
	if *elemFilter != "" {
		var f []vismatch.BandDiff
		for _, b := range bands {
			if b.Element == strings.ToUpper(*elemFilter) {
				f = append(f, b)
			}
		}
		bands = f
	}
	sort.SliceStable(bands, func(i, j int) bool { return bands[i].ScanlineLo < bands[j].ScanlineLo })
	if len(bands) == 0 {
		fmt.Println("band diffs: none")
	} else {
		fmt.Printf("band diffs (%d):\n", len(bands))
		fmt.Printf("  %-3s %-11s %-4s | %-22s | %-22s\n", "el", "scanlines", "h", "TARGET clk-spans", "MINE clk-spans")
		for _, b := range bands {
			fmt.Printf("  %-3s %4d-%-6d %-4d | %-22s | %-22s\n",
				b.Element, b.ScanlineLo, b.ScanlineHi, b.Height, b.TargetSpan, b.MineSpan)
		}
	}

	// One line for the thing a band diff makes a person derive by hand: when every
	// band is off by the same amount, N band rows are N ways of saying one number.
	if !rep.Match {
		vs := vismatch.FindVerticalShift(tg, mg, *maxShift)
		fmt.Println(vs.Describe())
	}

	if *diffOut != "" {
		f, err := os.Create(*diffOut)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := png.Encode(f, rep.Overlay(tg, mg, *scale)); err != nil {
			f.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		f.Close()
		fmt.Printf("overlay written: %s\n", *diffOut)
	}

	if rep.Match {
		fmt.Println("RESULT: pixel-exact (all compared element cells agree)")
		os.Exit(0)
	}
	fmt.Println("RESULT: differences found")
	os.Exit(1)
}
