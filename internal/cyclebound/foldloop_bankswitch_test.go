package cyclebound

import (
	"strings"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
)

// TestFoldLoopsRefusesABankSwitchInsideTheBody drives foldLoops' bank-switch refusal
// DIRECTLY, because no ROM reaches it.
//
// The branch is a soundness refusal added for bank support: a counted loop whose body
// switches banks cannot be folded as body-cost x trip-count, since the second
// iteration does not execute the same bytes. Measured 2026-07-30 across 123 ROMs, it
// ran ZERO times, and two attempts to build a ROM that reaches it were each blocked by
// a DIFFERENT coarser guard firing first — "multiple back-edges" when the planted loop
// shared a region with the kernel's own, then "no WSYNC reached from region start"
// when it was isolated, because the cross-bank edge sends the walk into a bank with no
// WSYNC.
//
// So this test answers the half that is answerable: IF the branch is reached, is it
// right? It builds the loop foldLoops expects — `lda $FFF9` / `dex` / `bne` back — and
// asserts the refusal. It deliberately does NOT claim the branch is reachable from a
// real cartridge; that remains open, and the audit records it as plausibly dead.
func TestFoldLoopsRefusesABankSwitchInsideTheBody(t *testing.T) {
	const base = 0xF000
	mk := func(off uint16, op byte, operand uint16) Instr {
		return Instr{Bank: 0, Addr: base + off, Op: op, Def: instructions.Definitions[op], Operand: operand}
	}
	// $F000 lda $FFF9 (3 bytes) | $F003 dex (1) | $F004 bne -> $F000 (2)
	lda := mk(0, 0xAD, 0xFFF9)
	dex := mk(3, 0xCA, 0)
	bne := mk(4, 0xD0, 0xFA) // relative, target $F000

	s := &solver{
		nodes:     map[site]Instr{},
		sinks:     map[site]bool{},
		folds:     map[site]loopInfo{},
		memo:      map[lkey]result{},
		state:     map[lkey]int{},
		absStates: map[site]State{},
		banked:    true,
		sw: switchModel{
			banked:   true,
			banks:    map[int]bool{0: true, 1: true},
			hotspots: map[uint16]string{memorymap.OriginCart | 0x0FF8: "BANK0", memorymap.OriginCart | 0x0FF9: "BANK1"},
		},
	}
	for _, in := range []Instr{lda, dex, bne} {
		s.nodes[in.site()] = in
		s.absStates[in.site()] = topState()
	}

	// Premise checks, or the test could pass for the wrong reason.
	if bne.branchSite() != lda.site() {
		t.Fatalf("the fixture's branch targets $%04X, not the loop header $%04X — the shape foldLoops "+
			"looks for is not present", bne.branchTarget(), lda.Addr)
	}
	if edges, _, refusal := s.sw.switchEdges(lda, topState()); len(edges) == 0 && refusal == "" {
		t.Fatal("the planted `lda $FFF9` reaches no hotspot, so the body does not switch banks and this " +
			"test would be checking nothing")
	}

	got := s.foldLoops()
	if !strings.Contains(got, "bank switch inside loop body") {
		t.Errorf("foldLoops returned %q; a counted loop whose body switches banks must be refused — "+
			"folding it would multiply a body cost by a trip count when the second iteration executes "+
			"different bytes", got)
	}

	// And the same loop WITHOUT the switch must fold, or the refusal above proves only
	// that foldLoops rejects everything.
	s2 := &solver{
		nodes: map[site]Instr{}, sinks: map[site]bool{}, folds: map[site]loopInfo{},
		memo: map[lkey]result{}, state: map[lkey]int{}, absStates: map[site]State{},
		banked: true, sw: s.sw,
	}
	nop := mk(0, 0xEA, 0) // same shape, no memory access
	nop2 := Instr{Bank: 0, Addr: base + 1, Op: 0xEA, Def: instructions.Definitions[0xEA]}
	nop3 := Instr{Bank: 0, Addr: base + 2, Op: 0xEA, Def: instructions.Definitions[0xEA]}
	for _, in := range []Instr{nop, nop2, nop3, dex, bne} {
		s2.nodes[in.site()] = in
		s2.absStates[in.site()] = topState()
	}
	if got := s2.foldLoops(); strings.Contains(got, "bank switch") {
		t.Errorf("a loop with no bank-switching access was still refused for one: %q", got)
	}
}
