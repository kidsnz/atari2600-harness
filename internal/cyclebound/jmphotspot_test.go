package cyclebound

import (
	"strings"
	"testing"
)

// TestJmpIntoHotspotIsRefused is the only witness for one branch of switchEdges.
//
// SD-8b judged that branch unsound and fixed it: Gopher2600 classifies jmp/jsr as
// Subroutine/Flow rather than Read, so a check driven off an instruction's DATA
// access never looks at them, and `jmp $FFF9` — a control transfer whose landing
// address is itself a bank-switch hotspot — slipped past. The fix has been in the
// tree since. Measured 2026-07-30 by counting which branch of switchEdges each
// instruction takes across 123 ROMs: that branch ran ZERO times. The repair was
// never demonstrated to work, only written.
//
// roms/litmus/litmus_bank_jmphotspot.asm plants `jmp $FFF9` inside a visible kernel
// region of bank 0 of an F8 cartridge. Landing there means the instruction FETCH at
// $1FF9 selects another bank, which is a transition this analysis does not model, so
// the region must be refused BY NAME rather than costed.
//
// Refusing is the conservative direction, which is exactly why an outcome gate could
// not have caught this: a branch that never fires produces no wrong number, it
// produces a missing refusal.
func TestJmpIntoHotspotIsRefused(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bank_jmphotspot.asm"
	rep := mustProve(t, asm, 76)

	if rep.BankedDeclined != "" {
		t.Fatalf("the cartridge was declined outright (%s) — then nothing reaches switchEdges and this "+
			"test proves nothing", rep.BankedDeclined)
	}
	if rep.Banks != 2 {
		t.Fatalf("expected a 2-bank analysis, got %d", rep.Banks)
	}
	if rep.Certified {
		t.Error("certified a kernel that jumps into a bank-switch hotspot")
	}

	var found string
	for _, r := range rep.Unbounded {
		if strings.Contains(r.Reason, "transfers control to") {
			found = r.Reason
			break
		}
	}
	if found == "" {
		var reasons []string
		for _, r := range rep.Unbounded {
			reasons = append(reasons, r.Reason)
		}
		t.Fatalf("no region was refused for transferring control into a hotspot. The planted `jmp $FFF9` "+
			"is a bank switch this analysis does not model, so costing the bytes that happen to follow "+
			"in the current bank would be a number about a path the hardware does not take.\nreasons: %v",
			reasons)
	}
	// The refusal has to name what is wrong, or it is indistinguishable from any
	// other unbounded region to whoever reads the report.
	for _, want := range []string{"jmp", "BANK1", "$1FF9", "selects another bank"} {
		if !strings.Contains(found, want) {
			t.Errorf("the refusal does not mention %q: %s", want, found)
		}
	}
	t.Logf("refused: %s", found)
}
