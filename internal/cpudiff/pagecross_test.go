package cpudiff

import "testing"

// TestPageCrossPenaltyRules machine-locks the 6502 timing rules CLAUDE.md lists under
// "constants you must never get wrong": a STORE never pays the page-cross penalty
// (STA abs,X is always 5, STA (ind),Y always 6), a READ pays +1 when the indexed
// address crosses a page, and a branch costs 2 not-taken, 3 taken, 4 taken across a
// page.
//
// Those rules are not decoration. `cyclebound`'s whole per-scanline proof rests on
// "kernel store timing is deterministic", and its page-cross costing was where a real
// under-approximation was found on 2026-07-30 — the penalty was never charged for a
// CONSTANT index. The rules were cited from 6502.org and never re-measured here.
//
// Measured against the engine one instruction at a time, with the same harness the
// silicon differential uses, so no ROM or kernel timing gets in the way.
func TestPageCrossPenaltyRules(t *testing.T) {
	const at = InstructionAddr // 0xF800

	cases := []struct {
		name   string
		v      Vector
		cycles int
	}{
		// STA abs,X — the claim is 5 whether or not the index crosses a page.
		{"STA abs,X no cross", Vector{X: 0x01, Mem: map[uint16]byte{
			at: 0x9D, at + 1: 0x00, at + 2: 0x02}}, 5}, // $0200 + 1
		{"STA abs,X crossing", Vector{X: 0xFF, Mem: map[uint16]byte{
			at: 0x9D, at + 1: 0x01, at + 2: 0x02}}, 5}, // $0201 + $FF = $0300
		// STA (ind),Y — the claim is 6 either way.
		{"STA (ind),Y no cross", Vector{Y: 0x01, Mem: map[uint16]byte{
			at: 0x91, at + 1: 0x80, 0x0080: 0x00, 0x0081: 0x02}}, 6},
		{"STA (ind),Y crossing", Vector{Y: 0xFF, Mem: map[uint16]byte{
			at: 0x91, at + 1: 0x80, 0x0080: 0x01, 0x0081: 0x02}}, 6},
		// LDA abs,X — a READ does pay the penalty.
		{"LDA abs,X no cross", Vector{X: 0x01, Mem: map[uint16]byte{
			at: 0xBD, at + 1: 0x00, at + 2: 0x02}}, 4},
		{"LDA abs,X crossing", Vector{X: 0xFF, Mem: map[uint16]byte{
			at: 0xBD, at + 1: 0x01, at + 2: 0x02}}, 5},
		// LDA (ind),Y — same, 5 then 6.
		{"LDA (ind),Y no cross", Vector{Y: 0x01, Mem: map[uint16]byte{
			at: 0xB1, at + 1: 0x80, 0x0080: 0x00, 0x0081: 0x02}}, 5},
		{"LDA (ind),Y crossing", Vector{Y: 0xFF, Mem: map[uint16]byte{
			at: 0xB1, at + 1: 0x80, 0x0080: 0x01, 0x0081: 0x02}}, 6},
		// Branches: 2 not taken, 3 taken, 4 taken across a page.
		// BNE with Z set is not taken; with Z clear it is. P bit 1 is Z.
		{"BNE not taken", Vector{P: 0x02, Mem: map[uint16]byte{
			at: 0xD0, at + 1: 0x10}}, 2},
		{"BNE taken same page", Vector{P: 0x00, Mem: map[uint16]byte{
			at: 0xD0, at + 1: 0x10}}, 3}, // $F802 + $10 = $F812, same page
		// A FORWARD branch from $F802 can never cross: the largest positive offset,
		// $7F, reaches $F881, still page $F8. Crossing needs a backward one, and
		// writing the forward case first is how this test caught its own mistake.
		{"BNE taken across page (backward)", Vector{P: 0x00, Mem: map[uint16]byte{
			at: 0xD0, at + 1: 0xFD}}, 4}, // $F802 - 3 = $F7FF — page $F7, crossed
	}

	for _, c := range cases {
		got, err := RunGopher(c.v)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.Status != "ok" {
			t.Fatalf("%s: status %q", c.name, got.Status)
		}
		if got.Cycles != c.cycles {
			t.Errorf("%s: %d cycles, documented %d", c.name, got.Cycles, c.cycles)
		}
	}
}
