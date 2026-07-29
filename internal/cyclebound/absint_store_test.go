package cyclebound

import "testing"

// A store must invalidate every cell it MIGHT have written, not only the one cell
// it definitely wrote.
//
// The abstract interpreter's contract is that it over-approximates: a tracked
// value may be less precise than the truth, never wrong about it. An indexed or
// indirect store breaks that contract if it leaves previously-tracked cells
// standing, because a later load then reads a value the machine no longer holds.
// A too-narrow range is not a harmless imprecision here — it flows into branch
// refinement, loop bounding and the page-cross penalty, so the "proven" worst
// case can come out BELOW what the hardware actually does. A cycle budget that
// certifies a kernel the hardware would overrun is the exact failure this whole
// package exists to prevent.
//
// Opcodes: A9 lda# / 85 sta zp / 95 sta zp,X / 9D sta abs,X / 99 sta abs,Y /
// 91 sta (zp),Y / A2 ldx# / AD lda abs / 48 pha.

func trackedAt(s State, addr uint16) (int, bool) {
	v, ok := s.Mem[addr]
	if !ok {
		return 0, false
	}
	return v.konst()
}

// sta zp,X may land anywhere in the zero page, so it must not leave a cell that
// happens to be in range still claiming its old value.
func TestIndexedStoreKillsOverlappingCells(t *testing.T) {
	s := topState()
	s = s.transfer(mkInstr(0xA9, 0x11)) // lda #$11
	s = s.transfer(mkInstr(0x85, 0x8A)) // sta $8A   -> $8A tracked as $11
	if v, ok := trackedAt(s, 0x8A); !ok || v != 0x11 {
		t.Fatalf("premise broken: $8A should be tracked as $11, got %v ok=%v", v, ok)
	}

	s = s.transfer(mkInstr(0xA9, 0x22)) // lda #$22
	s = s.transfer(mkInstr(0x95, 0x80)) // sta $80,X  -- X unknown: may hit $8A

	if v, ok := trackedAt(s, 0x8A); ok {
		t.Errorf("after `sta $80,X` with an unknown X, $8A is still tracked as $%02X — "+
			"a later `lda $8A` would read a value the machine may no longer hold, and that "+
			"range feeds refineBranch/determineBound/pagePenalty", v)
	}
}

// The same hole via the other common form: an indexed absolute store whose base
// is in RAM.
func TestAbsoluteIndexedStoreKillsRAM(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   byte
	}{{"sta abs,X", 0x9D}, {"sta abs,Y", 0x99}} {
		t.Run(tc.name, func(t *testing.T) {
			s := topState()
			s = s.transfer(mkInstr(0xA9, 0x33)) // lda #$33
			s = s.transfer(mkInstr(0x85, 0xC0)) // sta $C0
			if _, ok := trackedAt(s, 0xC0); !ok {
				t.Fatal("premise broken: $C0 should be tracked")
			}
			s = s.transfer(mkInstr(0xA9, 0x44))
			s = s.transfer(mkInstr(tc.op, 0x00B0)) // base $B0, unknown index

			if v, ok := trackedAt(s, 0xC0); ok {
				t.Errorf("%s over an unknown index left $C0 tracked as $%02X", tc.name, v)
			}
		})
	}
}

// An indirect store's target is not known at all, so every RAM cell must be
// dropped. This is the form real kernels use for sprite pointers.
func TestIndirectStoreKillsAllRAM(t *testing.T) {
	s := topState()
	s = s.transfer(mkInstr(0xA9, 0x55))
	s = s.transfer(mkInstr(0x85, 0x8A))
	s = s.transfer(mkInstr(0x85, 0xF0))
	s = s.transfer(mkInstr(0x91, 0x90)) // sta ($90),Y — target unknown

	for _, a := range []uint16{0x8A, 0xF0} {
		if v, ok := trackedAt(s, a); ok {
			t.Errorf("after `sta ($90),Y` the cell $%02X is still tracked as $%02X", a, v)
		}
	}
}

// On the 2600 the stack lives in page 1, which mirrors the same 128 bytes of RAM,
// so a push is a store into RAM. A tracked cell the stack can reach must not
// survive one. (Measured on a real cartridge this session: its stack pointer
// sweeps $FF down to $1C on every frame.)
func TestPushKillsStackReachableRAM(t *testing.T) {
	s := topState()
	s = s.transfer(mkInstr(0xA9, 0x66))
	s = s.transfer(mkInstr(0x85, 0xFE)) // sta $FE — high RAM, where the stack lives
	if _, ok := trackedAt(s, 0xFE); !ok {
		t.Fatal("premise broken: $FE should be tracked")
	}
	s = s.transfer(mkInstr(0x48, 0)) // pha

	if v, ok := trackedAt(s, 0xFE); ok {
		t.Errorf("after PHA the cell $FE is still tracked as $%02X — a push writes into "+
			"the same 128 bytes of RAM", v)
	}
}

// The repair must not cost precision it does not have to: a plain absolute store
// to a different address leaves other cells alone.
func TestAbsoluteStoreKeepsOtherCells(t *testing.T) {
	s := topState()
	s = s.transfer(mkInstr(0xA9, 0x77))
	s = s.transfer(mkInstr(0x85, 0x8A))
	s = s.transfer(mkInstr(0xA9, 0x88))
	s = s.transfer(mkInstr(0x85, 0x91)) // a different cell

	if v, ok := trackedAt(s, 0x8A); !ok || v != 0x77 {
		t.Errorf("$8A = %v ok=%v after storing to $91 — an absolute store to a different "+
			"address must not lose an unrelated cell", v, ok)
	}
	if v, ok := trackedAt(s, 0x91); !ok || v != 0x88 {
		t.Errorf("$91 = %v ok=%v", v, ok)
	}
}

// A fixpoint that stopped at its iteration cap has left under-approximated
// states behind, and every consumer downstream treats its input as sound. So the
// analysis has to say whether it finished; a report that cannot say so must not
// certify anything. Anything less is a proof resting on values the analysis never
// finished computing.
func TestComputeStatesReportsConvergence(t *testing.T) {
	instrs := map[uint16]Instr{
		0xF000: mkAt(0xF000, 0xA9, 0x05), // lda #5
		0xF002: mkAt(0xF002, 0x85, 0x8A), // sta $8A
	}
	_, converged := computeStates(instrs, []uint16{0xF000}, nil)
	if !converged {
		t.Error("a two-instruction straight line did not converge")
	}
}

func mkAt(addr uint16, op byte, operand uint16) Instr {
	in := mkInstr(op, operand)
	in.Addr = addr
	return in
}
