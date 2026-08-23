package emu

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// framelinesExclusions lists ROMs allowed to change their frame length, with the reason.
// It is NOT a list of things that are broken: everything here exists in order to fail, and
// a ROM that merely happens to be red does not belong in it. The four that used to breathe
// for real (banked_game, exerciser, lint_bank_hazard, lint_bank_split) were repaired rather
// than listed.
var framelinesExclusions = map[string]string{
	"framelines_trap": "the witness for scenario check frame_lines_stable — it spends an extra line every 128th frame ON PURPOSE",
	"pf_wraps": "the third witness for the playfield-DEADLINE check, and it overruns its line ON " +
		"PURPOSE: forty nops at the head of the kernel push every playfield store onto the FOLLOWING " +
		"scanline, which is the whole point — colour clocks fold back every 228, so a write a line " +
		"late compares as comfortably early and the verdict goes greener the harder the kernel is " +
		"broken. A kernel that overruns cannot hold a frame length, and measured over 130 frames " +
		"this one holds none (4x1 6x1 ... 44x1 350x21). Its siblings pf_ontime and pf_late both fit " +
		"76 cycles and are swept normally, so this is one witness's shape and not an exemption for " +
		"the pair it belongs to",
	"cart_f4sc": "a bank-switch/superchip FINGERPRINT fixture, not a display ROM: every one of its " +
		"eight banks ends `lda $FFF4 / jmp .reset`, handing back to bank 0 and re-entering the reset " +
		"vector, so the machine ping-pongs between banks instead of driving frames. Measured over 130 " +
		"frames it never produces a 262 at all (1x1 2x1 3x1 4x4 ... 350x22). Its four siblings " +
		"(cart_3e, cart_3eplus, cart_dpc, cart_f6sc) DO carry a frame loop and are swept normally, so " +
		"this is one fixture's shape and not a blanket exemption for roms/carts/",
}

// TestNoRomBreathesAcrossFrames requires every ROM in the corpus to hold ONE frame length
// for the whole window. It is the corpus-wide sibling of the `frame_lines_stable` scenario
// check, and it exists because the scenarios cannot reach most of the corpus: measured at
// the time of writing, 36 of 164 ROMs carry that check and 128 carry nothing. Wiring 128
// scenarios by hand is more work and more to maintain than one sweep.
//
// THE INVARIANT IS "SINGLE-VALUED", NOT "262". 38 of the ROMs hold a deliberately different
// frame length — litmus fixtures with short or odd frames — and they are stable at it. A
// gate that demanded 262 would fail them all and teach everyone to ignore it.
//
// WHY 130 FRAMES. A stability gate covers only the frames it measures, and the real defect
// this exists for had a 120-frame period (banked_game switched level on `cmp #120` and ran
// 264 lines on that frame). A 60-frame window sails past it. 130 clears the longest period
// found in this corpus with room to spare; if a ROM ever arrives with a slower cycle, this
// number has to grow with it, and the failure mode of getting that wrong is a silent pass.
func TestNoRomBreathesAcrossFrames(t *testing.T) {
	const warmup, frames = 3, 130

	var roms []string
	err := filepath.WalkDir("../../roms", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".bin") {
			roms = append(roms, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk roms: %v", err)
	}
	sort.Strings(roms)
	if len(roms) < 100 {
		t.Fatalf("found only %d ROMs under roms/ — the corpus is not where this test thinks "+
			"it is, and a sweep over nothing passes silently", len(roms))
	}

	var swept, skipped int
	for _, rom := range roms {
		name := strings.TrimSuffix(filepath.Base(rom), ".bin")
		if why, ok := framelinesExclusions[name]; ok {
			t.Logf("SKIP %s — %s", name, why)
			skipped++
			continue
		}
		swept++
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, err := New("NTSC")
			if err != nil {
				t.Fatal(err)
			}
			if err := e.LoadROM(rom); err != nil {
				t.Fatal(err)
			}
			if err := e.RunFrames(warmup); err != nil {
				t.Fatal(err)
			}
			hist := map[int]int{}
			for i := 0; i < frames; i++ {
				n, err := e.StepFrame()
				if err != nil {
					t.Fatal(err)
				}
				hist[n]++
			}
			if len(hist) == 1 {
				return
			}
			counts := make([]int, 0, len(hist))
			for k := range hist {
				counts = append(counts, k)
			}
			sort.Ints(counts)
			var table strings.Builder
			for _, k := range counts {
				table.WriteString(" ")
				table.WriteString(itoa(k))
				table.WriteString("x")
				table.WriteString(itoa(hist[k]))
			}
			t.Errorf("frame length is not single-valued over %d frames:%s — a total that changes "+
				"between frames rolls the whole picture by that many lines on a CRT. Move the "+
				"variable-cost work INSIDE the region whose length is fixed, or make both paths "+
				"cost the same.", frames, table.String())
		})
	}
	t.Logf("swept %d ROMs (%d frames each after %d warmup), %d excluded by name",
		swept, frames, warmup, skipped)
}
