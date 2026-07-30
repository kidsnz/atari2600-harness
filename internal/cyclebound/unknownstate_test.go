package cyclebound

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

// TestAbsentAbstractStateIsTopNotZero locks the meaning of a MISSING abstract state
// at the one place every caller funnels through.
//
// Every call site of successors indexes a map — absStates[at], states[a] — and a Go
// map miss yields the zero State. Its ValueRanges are Top=false, Lo=0, Hi=0, i.e.
// EXACT ZERO, so accessOf reads "X is definitely 0" and "SP is definitely 0" out of
// a state that knows nothing. The footprint that comes back is narrow and confident
// and wrong, and switchEdges decides from it whether the instruction can reach a
// bank-switch hotspot. Missing a hotspot drops a cross-bank successor, which
// shortens the predecessor set determineBound maximises over — an
// under-approximation, the direction this package forbids.
//
// Measured over roms/techniques + roms/litmus with Prove + BeamIntervals + DefUse +
// Lint: 1,994,520 successors calls, 2,572 with no usable state, 212 of those on a
// bank-switched cartridge, 124 still producing a concrete address, and 6 whose
// footprint genuinely differs from the sound answer. No corpus ROM's OUTPUT changes
// (verified byte-identical on 113 flat and 6 banked images), so this is a hole
// closed before it bites, not a wrong number repaired — which is exactly why it
// needs a test of its own.
func TestAbsentAbstractStateIsTopNotZero(t *testing.T) {
	sw := switchModel{
		banked:   true,
		banks:    map[int]bool{0: true, 1: true},
		hotspots: map[uint16]string{0x1FF8: "BANK0", 0x1FF9: "BANK1"},
	}

	// `lda $1F00,X` in bank 0. With X unknown the access spans $1F00-$1FFF, which
	// CONTAINS the hotspots — the instruction can switch banks. With X read as an
	// exact 0 it touches $1F00 alone and looks perfectly harmless.
	in := Instr{
		Bank:    0,
		Addr:    0xF010,
		Op:      0xBD,
		Def:     instructions.Definitions[0xBD],
		Operand: 0x1F00,
	}
	if in.Def.AddressingMode != instructions.AbsoluteX || in.Def.Operator != instructions.LDA {
		t.Fatalf("fixture decoded as %v %v, expected LDA AbsoluteX — opcode table changed",
			in.Def.Operator, in.Def.AddressingMode)
	}

	// The zero State: what a map miss hands over.
	var absent State
	if absent.valid {
		t.Fatal("the zero State reports valid — this test's premise is gone")
	}
	if absent.X.Top || absent.X.Lo != 0 || absent.X.Hi != 0 {
		t.Fatalf("the zero State's X is %+v, expected an exact [0,0] — that exactness is the defect", absent.X)
	}

	// Read literally, it hides the switch: one address, no cross-bank successor.
	if acc, ok := accessOf(in, absent); !ok || acc.Unbounded || len(acc.Addrs) != 1 || acc.Addrs[0] != 0x1F00 {
		t.Fatalf("accessOf with the zero State gave %+v (ok=%v); the point of this test is that it "+
			"resolves to exactly $1F00", acc, ok)
	}
	rawEdges, _, rawRefusal := sw.switchEdges(in, absent)
	if rawRefusal != "" {
		t.Fatalf("switchEdges refused outright (%q) — the defect this test describes is silence, not refusal",
			rawRefusal)
	}
	if len(rawEdges) != 0 {
		t.Fatalf("switchEdges found %d cross-bank edges from the zero State; the defect is that it "+
			"finds NONE", len(rawEdges))
	}

	// Read soundly, the same instruction reaches both hotspots.
	topEdges, _, _ := sw.switchEdges(in, topState())
	if len(topEdges) != 2 {
		t.Fatalf("with an unknown X the access spans $1F00-$1FFF and must reach BANK0 and BANK1; "+
			"got %d edges", len(topEdges))
	}

	// successors is the funnel every caller goes through, so it must give the sound
	// answer for an absent state — the same one it gives for an explicit Top.
	gotAbsent, refAbsent := successors(in, absent, sw)
	gotTop, refTop := successors(in, topState(), sw)
	if refAbsent != refTop {
		t.Errorf("absent-state refusal %q differs from Top's %q", refAbsent, refTop)
	}
	if len(gotAbsent) != len(gotTop) {
		t.Errorf("successors(absent) returned %d successors, successors(Top) returned %d — an absent "+
			"state is still being read as exact zero", len(gotAbsent), len(gotTop))
	}
	if len(gotAbsent) != 3 {
		t.Errorf("expected 2 cross-bank landings plus the fall-through, got %d", len(gotAbsent))
	}

	// And the same instruction with a genuinely known X of 0 must still be allowed
	// through, or the fix would just be "assume the worst everywhere".
	known := topState()
	known.X = ValueRange{Lo: 0, Hi: 0}
	if got, _ := successors(in, known, sw); len(got) != 1 {
		t.Errorf("a PROVEN X=0 reaches no hotspot and must yield only the fall-through; got %d", len(got))
	}

	// The counter has to move, or a future refactor could satisfy the assertions
	// above by some other route while the substitution quietly stops happening.
	before := invalidStateAsTop
	successors(in, State{}, sw)
	if invalidStateAsTop == before {
		t.Error("invalidStateAsTop did not advance — the absent-state substitution was bypassed")
	}
}
