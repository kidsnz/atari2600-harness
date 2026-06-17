// Abstract-interpretation engine for the cycle-budget prover (VV-2 v2). It tracks,
// statically and SOUNDLY, the value ranges of the 6502 registers/flags and a few
// zero-page cells along the CFG, so the prover can later: classify regions by the
// VBLANK display bit (S1), decide whether an indexed read crosses a page (S3),
// bound counted/divide loops from a counter's range (S4), and prune provably
// unreachable branch paths (S5).
//
// SOUNDNESS is the invariant: every abstract value OVER-approximates the real set
// of values. Anything not tracked precisely collapses to Top (unknown value) or
// triUnknown (unknown flag), so the prover never UNDER-estimates a cost — false
// positives are allowed, false negatives are not. (Cousot & Cousot, POPL 1977.)
package cyclebound

import "github.com/jetsetilly/gopher2600/hardware/cpu/instructions"

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ValueRange over-approximates an 8-bit value: the inclusive interval [Lo,Hi]
// (0..255), or Top (any value).
type ValueRange struct {
	Lo, Hi int
	Top    bool
}

func vConst(v int) ValueRange { return ValueRange{Lo: v, Hi: v} }
func vTop() ValueRange        { return ValueRange{Top: true} }
func vRange(lo, hi int) ValueRange {
	if lo > hi {
		lo, hi = hi, lo
	}
	return ValueRange{Lo: lo, Hi: hi}
}
func (r ValueRange) konst() (int, bool) {
	if !r.Top && r.Lo == r.Hi {
		return r.Lo, true
	}
	return 0, false
}
func (r ValueRange) join(o ValueRange) ValueRange {
	if r.Top || o.Top {
		return vTop()
	}
	return vRange(imin(r.Lo, o.Lo), imax(r.Hi, o.Hi))
}
func (r ValueRange) eq(o ValueRange) bool {
	if r.Top || o.Top {
		return r.Top == o.Top
	}
	return r.Lo == o.Lo && r.Hi == o.Hi
}

// incWrap / decWrap model ±1 in the 8-bit domain. A wrap (0xFF->0x00 / 0x00->0xFF)
// can't be expressed as one interval, so we fall back to Top (sound).
func incWrap(r ValueRange) ValueRange {
	if r.Top || r.Hi+1 > 255 {
		return vTop()
	}
	return vRange(r.Lo+1, r.Hi+1)
}
func decWrap(r ValueRange) ValueRange {
	if r.Top || r.Lo-1 < 0 {
		return vTop()
	}
	return vRange(r.Lo-1, r.Hi-1)
}

// TriBool is a three-valued flag: known true/false, or unknown.
type TriBool uint8

const (
	triUnknown TriBool = iota
	triFalse
	triTrue
)

func (t TriBool) join(o TriBool) TriBool {
	if t == o {
		return t
	}
	return triUnknown
}

// State is the abstract machine state at a program point. valid=false is bottom
// (unreachable). An absent Mem key means that cell is unknown (Top).
type State struct {
	A, X, Y ValueRange
	C, Z, N TriBool
	Mem     map[uint16]ValueRange
	VBlank  TriBool // VBLANK display-disable bit (bit1 of the last value stored to $01)
	valid   bool
}

func botState() State { return State{} } // bottom: valid=false (unreachable)
func topState() State {
	return State{A: vTop(), X: vTop(), Y: vTop(), Mem: map[uint16]ValueRange{}, valid: true}
}
func (s State) clone() State {
	m := make(map[uint16]ValueRange, len(s.Mem))
	for k, v := range s.Mem {
		m[k] = v
	}
	s.Mem = m
	return s
}
func (s State) memGet(a uint16) ValueRange {
	if v, ok := s.Mem[a]; ok {
		return v
	}
	return vTop()
}

// joinState is the least-upper-bound of two states (used at CFG merges/fixpoint).
func (s State) joinState(o State) State {
	if !s.valid {
		return o
	}
	if !o.valid {
		return s
	}
	r := topState()
	r.A, r.X, r.Y = s.A.join(o.A), s.X.join(o.X), s.Y.join(o.Y)
	r.C, r.Z, r.N = s.C.join(o.C), s.Z.join(o.Z), s.N.join(o.N)
	r.VBlank = s.VBlank.join(o.VBlank)
	r.Mem = map[uint16]ValueRange{}
	for k, v := range s.Mem {
		if v2, ok := o.Mem[k]; ok { // only cells known in BOTH survive; others -> Top
			r.Mem[k] = v.join(v2)
		}
	}
	return r
}
func (s State) eqState(o State) bool {
	if s.valid != o.valid {
		return false
	}
	if !s.valid {
		return true
	}
	if !(s.A.eq(o.A) && s.X.eq(o.X) && s.Y.eq(o.Y) && s.C == o.C && s.Z == o.Z && s.N == o.N && s.VBlank == o.VBlank) {
		return false
	}
	if len(s.Mem) != len(o.Mem) {
		return false
	}
	for k, v := range s.Mem {
		if v2, ok := o.Mem[k]; !ok || !v.eq(v2) {
			return false
		}
	}
	return true
}

func (s *State) setNZ(r ValueRange) {
	if v, ok := r.konst(); ok {
		if v == 0 {
			s.Z = triTrue
		} else {
			s.Z = triFalse
		}
	} else if !r.Top && r.Lo > 0 {
		s.Z = triFalse
	} else {
		s.Z = triUnknown
	}
	switch {
	case r.Top:
		s.N = triUnknown
	case r.Hi < 0x80:
		s.N = triFalse
	case r.Lo >= 0x80:
		s.N = triTrue
	default:
		s.N = triUnknown
	}
}

// setCmp models the flag effect of CMP/CPX/CPY (reg - m, unsigned).
func (s *State) setCmp(a, m ValueRange) {
	switch {
	case !a.Top && !m.Top && a.Lo >= m.Hi:
		s.C = triTrue
	case !a.Top && !m.Top && a.Hi < m.Lo:
		s.C = triFalse
	default:
		s.C = triUnknown
	}
	av, ac := a.konst()
	mv, mc := m.konst()
	switch {
	case ac && mc && av == mv:
		s.Z = triTrue
	case ac && mc && av != mv:
		s.Z = triFalse
	case !a.Top && !m.Top && (a.Hi < m.Lo || a.Lo > m.Hi):
		s.Z = triFalse
	default:
		s.Z = triUnknown
	}
	s.N = triUnknown // bit7 of (a-m) not modeled
}

// adcRange = a + m + carry-in (Top on overflow).
func adcRange(a, m ValueRange, c TriBool) ValueRange {
	if a.Top || m.Top {
		return vTop()
	}
	cl, ch := 0, 1
	switch c {
	case triTrue:
		cl, ch = 1, 1
	case triFalse:
		cl, ch = 0, 0
	}
	lo, hi := a.Lo+m.Lo+cl, a.Hi+m.Hi+ch
	if hi > 255 {
		return vTop()
	}
	return vRange(lo, hi)
}

// sbcRange = a - m - (1-carry); returns (result, carry-out). The carry-out is the
// "no borrow" flag that divide-by-15 loops branch on (S4). Top/unknown on wrap.
func sbcRange(a, m ValueRange, c TriBool) (ValueRange, TriBool) {
	if a.Top || m.Top {
		return vTop(), triUnknown
	}
	bl, bh := 0, 1 // borrow-in = 1 - C
	switch c {
	case triTrue:
		bl, bh = 0, 0
	case triFalse:
		bl, bh = 1, 1
	}
	lo, hi := a.Lo-m.Hi-bh, a.Hi-m.Lo-bl
	if lo < 0 || hi > 255 {
		return vTop(), triUnknown // borrow/underflow -> give up soundly
	}
	cout := triUnknown
	if a.Lo-m.Hi-bh >= 0 { // worst case still non-negative -> no borrow in any case
		cout = triTrue
	}
	return vRange(lo, hi), cout
}

func vblankBit(r ValueRange) TriBool {
	if v, ok := r.konst(); ok {
		if (v>>1)&1 == 1 {
			return triTrue
		}
		return triFalse
	}
	return triUnknown
}

func storeAddr(in Instr) (uint16, bool) {
	if in.Def.AddressingMode == instructions.Absolute {
		return in.Operand, true
	}
	return 0, false
}

// transfer returns the abstract state after executing in. Unmodeled effects are
// over-approximated (Top / triUnknown) so the result is always sound.
func (s State) transfer(in Instr) State {
	n := s.clone()
	d := in.Def
	imm := d.AddressingMode == instructions.Immediate
	immv := int(in.Operand & 0xFF)
	src := func() ValueRange {
		if imm {
			return vConst(immv)
		}
		if d.AddressingMode == instructions.Absolute {
			return s.memGet(in.Operand)
		}
		return vTop() // indexed/indirect source value unknown
	}
	switch d.Operator {
	case instructions.LDA:
		n.A = src()
		n.setNZ(n.A)
	case instructions.LDX:
		n.X = src()
		n.setNZ(n.X)
	case instructions.LDY:
		n.Y = src()
		n.setNZ(n.Y)
	case instructions.TAX:
		n.X = s.A
		n.setNZ(n.X)
	case instructions.TAY:
		n.Y = s.A
		n.setNZ(n.Y)
	case instructions.TXA:
		n.A = s.X
		n.setNZ(n.A)
	case instructions.TYA:
		n.A = s.Y
		n.setNZ(n.A)
	case instructions.TSX:
		n.X = vTop()
		n.setNZ(n.X)
	case instructions.TXS:
		// SP not tracked
	case instructions.INX:
		n.X = incWrap(s.X)
		n.setNZ(n.X)
	case instructions.INY:
		n.Y = incWrap(s.Y)
		n.setNZ(n.Y)
	case instructions.DEX:
		n.X = decWrap(s.X)
		n.setNZ(n.X)
	case instructions.DEY:
		n.Y = decWrap(s.Y)
		n.setNZ(n.Y)
	case instructions.CLC:
		n.C = triFalse
	case instructions.SEC:
		n.C = triTrue
	case instructions.CLV, instructions.CLD, instructions.SED, instructions.CLI, instructions.SEI, instructions.NOP:
		// no effect on tracked state
	case instructions.ADC:
		n.A = adcRange(s.A, src(), s.C)
		n.setNZ(n.A)
		n.C = triUnknown // carry-out not computed for ADC (conservative)
	case instructions.SBC:
		n.A, n.C = sbcRange(s.A, src(), s.C)
		n.setNZ(n.A)
	case instructions.CMP:
		n.setCmp(s.A, src())
	case instructions.CPX:
		n.setCmp(s.X, src())
	case instructions.CPY:
		n.setCmp(s.Y, src())
	case instructions.STA:
		if a, ok := storeAddr(in); ok {
			if a < 0x100 {
				n.Mem[a] = s.A
			}
			if a == 0x01 {
				n.VBlank = vblankBit(s.A)
			}
		}
	case instructions.STX:
		if a, ok := storeAddr(in); ok {
			if a < 0x100 {
				n.Mem[a] = s.X
			}
			if a == 0x01 {
				n.VBlank = vblankBit(s.X)
			}
		}
	case instructions.STY:
		if a, ok := storeAddr(in); ok {
			if a < 0x100 {
				n.Mem[a] = s.Y
			}
			if a == 0x01 {
				n.VBlank = vblankBit(s.Y)
			}
		}
	case instructions.AND, instructions.ORA, instructions.EOR:
		n.A = vTop()
		n.setNZ(n.A)
	case instructions.ASL, instructions.LSR, instructions.ROL, instructions.ROR:
		n.A = vTop() // may be accumulator-mode; lose A soundly
		n.C, n.Z, n.N = triUnknown, triUnknown, triUnknown
	case instructions.INC, instructions.DEC:
		if a, ok := storeAddr(in); ok && a < 0x100 {
			n.Mem[a] = vTop()
		}
		n.Z, n.N = triUnknown, triUnknown
	case instructions.BIT:
		n.Z, n.N = triUnknown, triUnknown
	case instructions.PLA:
		n.A = vTop()
		n.setNZ(n.A)
	case instructions.PLP:
		n.C, n.Z, n.N = triUnknown, triUnknown, triUnknown
	case instructions.PHA, instructions.PHP:
		// no tracked change
	default:
		// Unknown/illegal opcode: forget everything that could change (sound).
		n.A, n.X, n.Y = vTop(), vTop(), vTop()
		n.C, n.Z, n.N = triUnknown, triUnknown, triUnknown
	}
	return n
}

// refineBranch returns the entry states of the taken / not-taken successors of a
// conditional branch, tightening the flag the branch tests. A returned state with
// valid=false marks that edge as provably unreachable (used for S5 pruning). For
// untracked conditions (BVS/BVC) both edges stay reachable (sound).
func (s State) refineBranch(in Instr) (taken, notTaken State) {
	taken, notTaken = s.clone(), s.clone()
	prune := func(t, nt *State, f, takenWants TriBool) {
		if f == triUnknown {
			return
		}
		if f == takenWants {
			nt.valid = false
		} else {
			t.valid = false
		}
	}
	switch in.Def.Operator {
	case instructions.BEQ:
		taken.Z, notTaken.Z = triTrue, triFalse
		prune(&taken, &notTaken, s.Z, triTrue)
	case instructions.BNE:
		taken.Z, notTaken.Z = triFalse, triTrue
		prune(&taken, &notTaken, s.Z, triFalse)
	case instructions.BCS:
		taken.C, notTaken.C = triTrue, triFalse
		prune(&taken, &notTaken, s.C, triTrue)
	case instructions.BCC:
		taken.C, notTaken.C = triFalse, triTrue
		prune(&taken, &notTaken, s.C, triFalse)
	case instructions.BMI:
		taken.N, notTaken.N = triTrue, triFalse
		prune(&taken, &notTaken, s.N, triTrue)
	case instructions.BPL:
		taken.N, notTaken.N = triFalse, triTrue
		prune(&taken, &notTaken, s.N, triFalse)
	}
	return
}
