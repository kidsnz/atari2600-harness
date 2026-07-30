package oracle

import (
	"os"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// framePhaseROM assembles the phase fixture so the test does not depend on a .bin
// that happens to be lying around.
func framePhaseROM(t *testing.T) string {
	t.Helper()
	const asm = "../../roms/litmus/litmus_framephase.asm"
	if _, err := os.Stat(asm); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	bin := build.BinPathFor(asm)
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("assemble %s:\n%s", asm, out)
	}
	return bin
}

// TestOracleSamplingPhaseIsMeasured is G6: "run N frames and dump RAM" does not name
// a moment, and two oracles pick different ones.
//
// litmus_framephase bumps a different counter at three points in the frame — $80 just
// after VSYNC, $81 at the midpoint of the visible field, $82 as the last instruction
// before the next VSYNC. Whatever an oracle reports for those three bytes says where
// in the frame it read them.
//
// MEASURED at frames=10: Gopher2600 gives $80=10 $81=9 $82=9 (it stops at the
// program's own frame boundary, just after VSYNC) and MAME gives $80=10 $81=10 $82=9
// (its frame notifier fires after the midpoint of the visible field). So on a ROM
// where nothing is wrong, the two disagree about $81, every time. Any game that
// updates a byte between those two moments -- a frame counter bumped in the kernel,
// say -- produces the same false dissent.
func TestOracleSamplingPhaseIsMeasured(t *testing.T) {
	rom := framePhaseROM(t)

	g10, err := Gopher{}.DumpRAM(rom, 10)
	if err != nil {
		t.Fatal(err)
	}
	g11, err := Gopher{}.DumpRAM(rom, 11)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture has to actually move, or everything below is vacuous.
	if g10[0] == g10[1] && g10[1] == g10[2] {
		t.Fatalf("all three counters read the same at frame 10 (%d) — the fixture is not "+
			"distinguishing points in the frame", g10[0])
	}
	// Gopher's own phase: sampled after POINT A and before POINT B.
	if g10[0] != g10[1]+1 {
		t.Errorf("gopher2600 $80=%d $81=%d: expected the frame-start counter to be one ahead of the "+
			"mid-visible one, i.e. a sample taken just after VSYNC", g10[0], g10[1])
	}
	if g11[0] != g10[0]+1 {
		t.Errorf("one more frame did not advance the frame-start counter: %d -> %d", g10[0], g11[0])
	}

	if !MameAvailable() {
		t.Skip("mame not installed — the cross-oracle half of this test needs it")
	}
	m10, err := Mame{}.DumpRAM(rom, 10)
	if err != nil {
		t.Fatal(err)
	}

	raw := Diff(g10, m10)
	if len(raw) == 0 {
		t.Fatal("gopher2600 and MAME agree on every byte of the phase fixture — either they now sample " +
			"at the same moment (good news, but this test's premise is gone) or the fixture stopped working")
	}
	real, phase := ClassifyDiff(g10, g11, m10)
	if len(real) != 0 {
		t.Errorf("offsets %v differ in a way a one-frame sampling shift cannot explain — that is a real "+
			"disagreement between the two emulators, not a phase artefact", real)
	}
	if len(phase) == 0 {
		t.Error("the raw difference is not explained by the sampling phase either — ClassifyDiff is not " +
			"classifying anything")
	}
	t.Logf("frames=10 gopher $80=%d $81=%d $82=%d | mame $80=%d $81=%d $82=%d | raw diff %v -> real %v, phase %v",
		g10[0], g10[1], g10[2], m10[0], m10[1], m10[2], raw, real, phase)
}

// TestClassifyDiffSeparatesRealFromPhase drives the classifier directly, so it is
// tested without depending on an installed MAME.
func TestClassifyDiffSeparatesRealFromPhase(t *testing.T) {
	var refN, refNext, other RAMDump
	refN[0], refNext[0], other[0] = 5, 5, 5   // agrees: neither
	refN[1], refNext[1], other[1] = 9, 10, 10 // one frame ahead: phase
	refN[2], refNext[2], other[2] = 3, 4, 77  // matches neither: real
	refN[3], refNext[3], other[3] = 8, 8, 9   // ref did not move, other did: real

	real, phase := ClassifyDiff(refN, refNext, other)
	if len(phase) != 1 || phase[0] != 1 {
		t.Errorf("phase = %v, want [1]", phase)
	}
	if len(real) != 2 || real[0] != 2 || real[1] != 3 {
		t.Errorf("real = %v, want [2 3]", real)
	}
	// Offset 3 is the one that matters: a byte the reference did NOT change cannot be
	// excused by a sampling shift, and calling it phase would silence a real bug.
}
