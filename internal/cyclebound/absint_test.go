package cyclebound

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

// mkInstr builds an Instr from an opcode (+ operand) for unit-testing transfer.
func mkInstr(op byte, operand uint16) Instr {
	return Instr{Addr: 0xF000, Op: op, Def: instructions.Definitions[op], Operand: operand}
}

// opcodes used below: A9 lda# / 18 clc / 69 adc# / 38 sec / E9 sbc# / C9 cmp# /
// 85 sta zp / F0 beq.

func TestAbsIntArith(t *testing.T) {
	s := topState()
	s = s.transfer(mkInstr(0xA9, 5)) // lda #5
	if v, ok := s.A.konst(); !ok || v != 5 {
		t.Fatalf("lda #5 -> A=%v want const 5", s.A)
	}
	if s.Z != triFalse || s.N != triFalse {
		t.Fatalf("lda #5 -> Z=%v N=%v want false,false", s.Z, s.N)
	}
	s = s.transfer(mkInstr(0x18, 0)) // clc
	s = s.transfer(mkInstr(0x69, 3)) // adc #3
	if v, ok := s.A.konst(); !ok || v != 8 {
		t.Fatalf("clc; adc #3 -> A=%v want const 8", s.A)
	}
	s = s.transfer(mkInstr(0x38, 0)) // sec
	s = s.transfer(mkInstr(0xE9, 2)) // sbc #2
	if v, ok := s.A.konst(); !ok || v != 6 {
		t.Fatalf("sec; sbc #2 -> A=%v want const 6", s.A)
	}
	if s.C != triTrue { // 8-2 had no borrow
		t.Fatalf("sbc no-borrow -> C=%v want true", s.C)
	}
}

func TestAbsIntCmp(t *testing.T) {
	base := topState().transfer(mkInstr(0xA9, 10)) // lda #10
	if c := base.transfer(mkInstr(0xC9, 5)); c.C != triTrue {
		t.Fatalf("cmp #5 (10>=5) -> C=%v want true", c.C)
	}
	if c := base.transfer(mkInstr(0xC9, 20)); c.C != triFalse {
		t.Fatalf("cmp #20 (10<20) -> C=%v want false", c.C)
	}
	if c := base.transfer(mkInstr(0xC9, 10)); c.Z != triTrue || c.C != triTrue {
		t.Fatalf("cmp #10 (equal) -> Z=%v C=%v want true,true", c.Z, c.C)
	}
}

func TestAbsIntVBlank(t *testing.T) {
	s := topState()
	s = s.transfer(mkInstr(0xA9, 2))    // lda #2 (bit1 set)
	s = s.transfer(mkInstr(0x85, 0x01)) // sta VBLANK
	if s.VBlank != triTrue {
		t.Fatalf("sta VBLANK #2 -> VBlank=%v want true (blanked)", s.VBlank)
	}
	s = s.transfer(mkInstr(0xA9, 0))    // lda #0
	s = s.transfer(mkInstr(0x85, 0x01)) // sta VBLANK
	if s.VBlank != triFalse {
		t.Fatalf("sta VBLANK #0 -> VBlank=%v want false (visible)", s.VBlank)
	}
}

func TestAbsIntJoinSound(t *testing.T) {
	a := topState()
	a.A = vConst(5)
	b := topState()
	b.A = vConst(9)
	j := a.joinState(b)
	if j.A.Top || j.A.Lo > 5 || j.A.Hi < 9 {
		t.Fatalf("join({5},{9}).A = %v must contain both 5 and 9", j.A)
	}
	// joining bottom is identity
	if got := botState().joinState(a); !got.A.eq(a.A) {
		t.Fatalf("join(bottom,a) must equal a")
	}
}

func TestAbsIntWrapSound(t *testing.T) {
	if r := decWrap(vRange(0, 3)); !r.Top { // 0->0xFF wrap can't be an interval
		t.Fatalf("dec [0,3] = %v want Top (wraps at 0)", r)
	}
	if r := decWrap(vRange(1, 3)); r.Top || r.Lo != 0 || r.Hi != 2 {
		t.Fatalf("dec [1,3] = %v want [0,2]", r)
	}
	if r := incWrap(vRange(253, 255)); !r.Top {
		t.Fatalf("inc [253,255] = %v want Top (wraps at 255)", r)
	}
}

// TestAbsIntRefinePrunes is the seed of S5: a known flag makes one branch edge
// provably unreachable.
func TestAbsIntRefinePrunes(t *testing.T) {
	beq := mkInstr(0xF0, 0)
	zTrue := topState().transfer(mkInstr(0xA9, 0)) // lda #0 -> Z=true
	tk, nt := zTrue.refineBranch(beq)
	if !tk.valid || nt.valid {
		t.Fatalf("Z=true beq: taken.valid=%v not-taken.valid=%v want true,false", tk.valid, nt.valid)
	}
	zFalse := topState().transfer(mkInstr(0xA9, 7)) // lda #7 -> Z=false
	tk2, nt2 := zFalse.refineBranch(beq)
	if tk2.valid || !nt2.valid {
		t.Fatalf("Z=false beq: taken.valid=%v not-taken.valid=%v want false,true", tk2.valid, nt2.valid)
	}
	// unknown flag: both edges stay reachable (sound)
	tk3, nt3 := topState().refineBranch(beq)
	if !tk3.valid || !nt3.valid {
		t.Fatal("unknown Z: both edges must stay reachable")
	}
}
