package emu

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// deliberateStackROMs are the ROMs in this tree whose subject IS the stack: they push on purpose,
// sweep it, or measure how far it reaches. They are excluded from the budget below because a bound
// on them would be a bound on the thing they exist to demonstrate.
var deliberateStackROMs = map[string]bool{
	"cb_pushdisplay.bin":     true, // writes the display through PHA
	"cb_romtable.bin":        true,
	"cb_arrloop.bin":         true,
	"litmus_jsr_stack.bin":   true, // JSR depth is the measurement
	"litmus_stack_trick.bin": true,
	"probe01-a.bin":          true,
	"probe01-b.bin":          true,
	"probe01-c.bin":          true,
	"probe01-a-b58.bin":      true,
}

// TestStackFitsInATinyCornerOfRAM puts a price on a 2004 policy.
//
// Christopher Tumber, stella-list `200401/msg00013`: *"I pretty much try to avoid using JSR
// completely, and only do so when absolutely needed (really large subroutines that get re-used a
// lot). **RAM management is really one of the keys**"*. That is a policy with no number attached,
// and this repository had no design rule about the stack's cost at all — only the power-on trap
// (`known-traps.md`, "post-reset SP / RAM / flags undefined").
//
// Measured 2026-09-07 over every `.bin` in this tree plus the works, after a ten-frame warmup (SP
// is undefined until TXS runs). The window was 30 frames when first measured and is 8 here: a
// kernel's stack use repeats every frame, and the package was over CI's ten-minute budget.
//
//	0 bytes    281 ROMs      <- most kernels never touch it
//	1-8 bytes   89 ROMs      <- 4 is by far the commonest: two levels of JSR
//	9-16 bytes   1 ROM       <- rts_dispatch, the technique that dispatches THROUGH the stack
//	17-128       0 ROMs
//	129+         9 ROMs      <- the deliberate set above
//
// **So the policy costs less than it sounds.** The stack lives at $01FF downward, which on the 2600
// is the same 128 bytes of RAM mirrored — every byte it takes is a byte of game state gone. The
// measurement says the realistic charge for using subroutines freely is **4 bytes**, and for the
// most stack-hungry technique in the catalogue **10**, out of 128. Tumber's instinct was right about
// the direction and wrong about the size: avoiding JSR to save RAM buys back single-digit bytes.
//
// ★**The warmup is not optional and the first version of this measurement did not have it.** SP is
// undefined until `TXS` runs, and reading it before then reported **255 bytes used for all 380
// ROMs** — a saturated number that looks like a finding and is an instrument reading its own
// power-on state. The give-away was that nothing used zero.
//
// Found by the mailing-list distillation (helper-1).
func TestStackFitsInATinyCornerOfRAM(t *testing.T) {
	const (
		warmupFrames  = 10 // SP is undefined until TXS runs; see the note above
		measureFrames = 8
		budget        = 16 // above rts_dispatch's 10, far below anything that would crowd game state
	)

	var files []string
	for _, g := range []string{"../../roms/techniques/*.bin", "../../roms/litmus/*.bin"} {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	if len(files) < 50 {
		t.Fatalf("only %d ROMs found — the .bin fixtures are built by the test setup, so an empty "+
			"walk is a broken path, not a corpus with nothing in it", len(files))
	}

	type row struct {
		name string
		used int
	}
	var measured []row
	for _, f := range files {
		e, err := New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(f); err != nil {
			continue
		}
		ok := true
		for i := 0; i < warmupFrames; i++ {
			if _, err := e.StepFrame(); err != nil {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		deepest := 0
		for i := 0; i < measureFrames; i++ {
			e.StartFrameWatch()
			if _, err := e.StepFrame(); err != nil {
				break
			}
			if _, lo := e.FrameWatch(); 0xFF-int(lo) > deepest {
				deepest = 0xFF - int(lo)
			}
		}
		measured = append(measured, row{filepath.Base(f), deepest})
	}
	if len(measured) < 50 {
		t.Fatalf("only %d of %d ROMs ran", len(measured), len(files))
	}

	var over []string
	zero, deliberate := 0, 0
	for _, r := range measured {
		if deliberateStackROMs[r.name] {
			deliberate++
			if r.used < 100 {
				over = append(over, r.name+" is listed as a deliberate stack ROM but used only "+
					strconv.Itoa(r.used)+" bytes — either it stopped exercising the stack or the list is stale")
			}
			continue
		}
		if r.used == 0 {
			zero++
		}
		if r.used > budget {
			over = append(over, r.name+" used "+strconv.Itoa(r.used)+" bytes of stack")
		}
	}
	sort.Strings(over)
	if len(over) > 0 {
		t.Errorf("the stack budget is %d bytes of the 128 and these do not fit:\n  %s\n"+
			"Either the ROM genuinely nests deeper — in which case say so in a design note, because "+
			"every stack byte is game state gone — or it belongs in deliberateStackROMs.",
			budget, strings.Join(over, "\n  "))
	}

	// Two-sided: the corpus must still contain both kinds, or this test is measuring nothing.
	if deliberate == 0 {
		t.Error("no deliberate stack ROM ran — the exclusion list names ROMs that are not there, so " +
			"the budget above was never tested against anything that would break it")
	}
	if zero == 0 {
		t.Error("no ROM used zero stack bytes. That was the exact signature of the broken first " +
			"measurement (SP read before TXS reports 255 for everything); check the warmup before " +
			"believing any number here")
	}
}
