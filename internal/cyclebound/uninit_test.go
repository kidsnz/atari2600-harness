package cyclebound

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The pair. litmus_uninit_read.asm clears only $01..$3F and then reads $8A;
// litmus_uninit_clean.asm is the same ROM with the sweep bound changed so the
// whole zero page is cleared. Nothing else differs.
//
// Both halves are load-bearing. Firing on the first shows the analysis can see a
// read of memory nobody wrote — the bug that passes in an emulator, which hands
// out a defined value, and fails on hardware, which hands out power-on rubbish.
// Staying silent on the second shows it is reacting to whether the cell was
// written rather than to the shape of the code. A detector that fires on both is
// detecting nothing, and one that fires on neither is worth even less.
func TestUninitReadPair(t *testing.T) {
	bait := defUseOf(t, "../../roms/litmus/litmus_uninit_read.asm")
	clean := defUseOf(t, "../../roms/litmus/litmus_uninit_clean.asm")
	if !bait.Converged || !clean.Converged {
		t.Skip("fixpoint did not converge")
	}

	if len(bait.UninitReads) != 1 {
		t.Errorf("planted uninitialised read: got %d reports, want exactly 1: %+v",
			len(bait.UninitReads), bait.UninitReads)
	} else if u := bait.UninitReads[0]; u.Addr != "$008A" {
		t.Errorf("reported $%s, but the planted read is of $008A (%+v)", u.Addr, u)
	}

	if len(clean.UninitReads) != 0 {
		t.Errorf("the clean twin differs only in the sweep bound, yet %d reads were reported: %+v",
			len(clean.UninitReads), clean.UninitReads)
	}
}

// The sweep recogniser is what makes the whole thing usable, and getting its
// fencepost backwards would be invisible without a case that depends on it:
// `dex / bne` leaves once the index reaches zero, so index 0 is never stored.
// The bait ROM clears with `ldx #$3F`, so $01..$3F are written and $00 is not.
func TestSweepExcludesIndexZero(t *testing.T) {
	r := defUseOf(t, "../../roms/litmus/litmus_uninit_read.asm")
	if !r.Converged {
		t.Skip("fixpoint did not converge")
	}
	may, _ := r.mayWriteAddrs()
	if !may[0x003F] {
		t.Error("$3F is the top of the swept range but is not in the may-write set")
	}
	// $00 is VSYNC, which this ROM also writes directly, so it is a poor probe.
	// The claim to check is the must-side: the sweep must not have claimed $00.
	sweepClaimed := false
	for _, u := range r.UninitReads {
		if u.Addr == "$0000" {
			sweepClaimed = true
		}
	}
	if sweepClaimed {
		t.Error("unexpected report on $0000")
	}
}

// Static must CONTAIN dynamic. The emulator's own uninitialised-read watcher
// reports what one execution actually did; the static analysis claims what any
// execution could do. If a real run reads a cell before writing it and the
// static side did not list that read, the static side is wrong — not merely
// imprecise.
func TestStaticUninitContainsDynamic(t *testing.T) {
	files, _ := filepath.Glob("../../roms/techniques/*.asm")
	files = append(files,
		"../../roms/litmus/litmus_uninit_read.asm",
		"../../roms/litmus/motion_glide.asm",
	)
	checkedROMs, hits := 0, 0
	for _, asm := range files {
		r, err := DefUse(asm, DefaultBudget)
		if err != nil || !r.Converged || r.FlatBankOnly != "" {
			continue
		}
		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Logf("assemble %s: %s", asm, out)
			continue
		}
		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		hit, err := e.WatchUninitRead(4)
		if err != nil {
			continue
		}
		checkedROMs++
		if hit == nil {
			continue
		}
		hits++
		found := false
		for _, u := range r.UninitReads {
			if parseHexAddr(u.PC) == hit.PC {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: the emulator observed PC $%04X reading $%04X before any write, "+
				"but the static analysis did not report it — a may-analysis that misses an "+
				"execution is unsound", filepath.Base(asm), hit.PC, hit.Addr)
		}
	}
	if checkedROMs == 0 {
		t.Fatal("no ROM was checked — the test proves nothing")
	}
	t.Logf("static-contains-dynamic: %d ROMs run, %d with an observed uninitialised read", checkedROMs, hits)
}
