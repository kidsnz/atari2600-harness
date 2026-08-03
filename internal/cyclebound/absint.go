// Abstract-interpretation engine for the cycle-budget prover (VV-2 v2). It tracks,
// statically and SOUNDLY, the value ranges of the 6502 registers/flags and a few
// zero-page cells along the CFG, so the prover can: classify regions by the
// VSYNC/VBLANK display state (S1), decide whether an indexed read crosses a page
// (S3), bound counted/divide loops from a counter's range (S4), and prune
// provably unreachable branch paths (S5).
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

// andImm over-approximates A AND #m: AND can only CLEAR bits, so the result is
// <= each operand → [0, min(A.Hi, m)]. Sound (e.g. `and #$7F` → [0,127]).
func andImm(a ValueRange, m int) ValueRange {
	hi := m
	if !a.Top && a.Hi < hi {
		hi = a.Hi
	}
	return vRange(0, hi)
}

// oraImm over-approximates A ORA #m: ORA can only SET bits, so the result is
// >= each operand → [max(A.Lo, m), 255]. Sound.
func oraImm(a ValueRange, m int) ValueRange {
	lo := m
	if !a.Top && a.Lo > lo {
		lo = a.Lo
	}
	return vRange(lo, 255)
}

// romTableRange (3D) returns the value range of a ROM data table read as
// `lda base,x` over the proven index range idx. The table bytes are constant and
// in the binary, so this is exact: [min,max] over base+idx.Lo .. base+idx.Hi.
// Unknown index, unreadable address, or an over-wide span ⇒ Top (sound).
//
// bank routes the read: on a bank-switched cartridge the same address holds different
// bytes in different banks, so folding the wrong bank's table would produce a narrow,
// confident, wrong range that then bounds a loop.
//
// A footprint containing a bank-switch HOTSPOT is Top, and the reason is the
// hardware's: on a hotspot access the mapper switches FIRST and then returns the byte
// from the NEW bank (Gopher2600 mapper_atari.go Read: bankswitch(addr) followed by
// `return cart.banks[cart.state.bank][addr]`). Folding the current bank's byte there
// would be exactly SD-8c's failure mode — a value the hardware never hands out —
// moved from the RAM axis to the bank axis, and it feeds a loop trip count.
func romTableRange(romAt func(int, uint16) (byte, bool), bank int, base uint16, idx ValueRange, sw switchModel) ValueRange {
	if romAt == nil || idx.Top || idx.Hi-idx.Lo > 255 {
		return vTop()
	}
	lo, hi := 256, -1
	for i := idx.Lo; i <= idx.Hi; i++ {
		if sw.active() {
			if ma, ok := cartHotspotKey(base + uint16(i)); ok {
				if _, isHot := sw.hotspots[ma]; isHot {
					return vTop()
				}
			}
		}
		b, ok := romAt(bank, base+uint16(i))
		if !ok {
			return vTop()
		}
		if int(b) < lo {
			lo = int(b)
		}
		if int(b) > hi {
			hi = int(b)
		}
	}
	if hi < 0 {
		return vTop()
	}
	return vRange(lo, hi)
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
	// SP is the stack pointer's low byte. It is tracked because on the 2600 the
	// stack is not merely bookkeeping: page 1 mirrors the addresses the console
	// decodes, so a program that aims SP at a TIA register turns PHA/PHP into a
	// store to it. Leaving SP untracked made that write invisible to every
	// analysis here while the hardware performed it, and forced every push to be
	// treated as clobbering all of RAM.
	SP      ValueRange
	C, Z, N TriBool
	Mem     map[uint16]ValueRange
	VBlank  TriBool // VBLANK display-disable bit (bit1 of the last value stored to $01)
	VSync   TriBool // VSYNC bit (bit1 of the last value stored to $00)
	// ZPVal (3B): the join of EVERY value stored anywhere in zero page, seeded to
	// the ZP power-on/clear value. Reading via an index (`lda base,x`) returns this
	// — sound because it over-approximates any zero-page cell regardless of which
	// array/index is used (no per-array aliasing reasoning needed). Top = untracked
	// (no recognized init), so indexed ZP loads stay Top = unbounded, as before.
	ZPVal ValueRange
	// romAt (3D) reads a cartridge ROM byte (nil outside Prove), routed BY BANK
	// because the same address holds different bytes in different banks. Lets an
	// indexed load from a ROM data table (`lda table,x`) return the table's actual
	// value range over the proven index range — the data is constant and in the binary.
	// Not part of state equality (it is a constant capability, not a value).
	romAt func(bank int, addr uint16) (byte, bool)
	// preservesDisplay reports whether the subroutine at the given entry SITE
	// provably cannot write VSYNC/VBLANK, so those bits survive a JSR to it. Like
	// romAt it is a constant capability, not a value, and not part of equality.
	preservesDisplay func(site) bool
	// sw is the cross-bank switch oracle, carried so romTableRange can refuse to fold
	// a byte sitting on a bank-switch hotspot. Also a constant capability, not a value.
	sw    switchModel
	valid bool
}

func botState() State { return State{} } // bottom: valid=false (unreachable)
func topState() State {
	return State{A: vTop(), X: vTop(), Y: vTop(), SP: vTop(), Mem: map[uint16]ValueRange{}, ZPVal: vTop(), valid: true}
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

// displayOff reports that the beam is provably in VSYNC or VBLANK (not drawing a
// visible scanline). Unknown stays false (sound: treat as visible and check it).
func (s State) displayOff() bool {
	return s.VSync == triTrue || s.VBlank == triTrue
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
	r.SP = s.SP.join(o.SP)
	r.C, r.Z, r.N = s.C.join(o.C), s.Z.join(o.Z), s.N.join(o.N)
	r.VBlank, r.VSync = s.VBlank.join(o.VBlank), s.VSync.join(o.VSync)
	r.ZPVal = s.ZPVal.join(o.ZPVal)
	if r.romAt = s.romAt; r.romAt == nil { // constant capability — carry it through
		r.romAt = o.romAt
	}
	if r.preservesDisplay = s.preservesDisplay; r.preservesDisplay == nil {
		r.preservesDisplay = o.preservesDisplay
	}
	if r.sw = s.sw; !r.sw.active() {
		r.sw = o.sw
	}
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
	if !(s.A.eq(o.A) && s.X.eq(o.X) && s.Y.eq(o.Y) && s.SP.eq(o.SP) && s.C == o.C && s.Z == o.Z && s.N == o.N &&
		s.VBlank == o.VBlank && s.VSync == o.VSync && s.ZPVal.eq(o.ZPVal)) {
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
		// The sum wraps. The exact post-value is not expressible as one interval
		// (the range straddles the wrap), but the RESULT IS STILL A BYTE, so [0,255]
		// is a true and useful statement about it. Returning Top instead says "this
		// register could be anything", which is the same claim numerically and a much
		// worse one structurally: Top makes every consumer refuse, while a byte range
		// still bounds a divide loop's trip count.
		//
		// This mattered. `XCAL = -5` assembles to $FB, so the ordinary calibration
		// idiom `lda #80 / clc / adc #XCAL` computes 80 + 251 = 331 and the old code
		// answered Top. Nine divide folds across the technique corpus lost their entry
		// bound to it — and were then quietly rescued by the `lda #imm` address proxy,
		// which is how a heuristic SD-9 had already deleted from the other path stayed
		// load-bearing without anyone noticing.
		return vRange(0, 255)
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

// storeIndex returns the range of the index register a store uses, or Top when
// the mode does not index (in which case it is unused).
func (s State) storeIndex(in Instr) ValueRange {
	switch in.Def.AddressingMode {
	case instructions.AbsoluteX, instructions.PreIndexed:
		return s.X
	case instructions.AbsoluteY, instructions.PostIndexed:
		return s.Y
	}
	return vTop()
}

// killStoreTargets drops every tracked cell the store MIGHT have written.
//
// Without this the interpreter is unsound, not merely imprecise: `sta $80,X` or
// `sta ($90),Y` would leave earlier tracked cells standing, a later absolute load
// would read a value the machine no longer holds, and that too-narrow range feeds
// refineBranch, determineBound and pagePenalty — so a "proven" worst case can come
// out BELOW what the hardware does. A budget that certifies a kernel the hardware
// overruns is precisely the failure this package exists to prevent.
//
// idx is the index register's range at the store; a bounded index kills only the
// window it can reach, an unknown one kills all of RAM. A zero-page indexed store
// wraps inside the page, so a base below $100 with an unbounded index can reach
// anywhere and is treated as such.
func (n *State) killStoreTargets(in Instr, idx ValueRange) {
	killAll := func() {
		if len(n.Mem) > 0 {
			n.Mem = map[uint16]ValueRange{}
		}
	}
	switch in.Def.AddressingMode {
	case instructions.Absolute:
		return // exact target; the caller does a strong update
	case instructions.AbsoluteX, instructions.AbsoluteY:
		base := int(in.Operand)
		if idx.Top {
			killAll()
			return
		}
		zp := base < 0x100
		for a := range n.Mem {
			for i := idx.Lo; i <= idx.Hi; i++ {
				t := base + i
				if zp {
					t = (base + i) & 0xFF // zero-page indexing wraps within the page
				}
				if uint16(t) == a {
					delete(n.Mem, a)
					break
				}
			}
		}
	default:
		// Indirect, (ind,X), (ind),Y and any mode not modelled above: the target is
		// unknown, so nothing tracked can be trusted to have survived.
		killAll()
	}
}

// applyStore updates VSYNC/VBLANK tracking and a tracked zero-page cell for a
// store of value v to a statically known address.
func (n *State) applyStore(in Instr, v ValueRange, idx ValueRange) {
	n.killStoreTargets(in, idx)
	// 3B: any store landing in zero-page RAM ($80-$FF) contributes to the RAM value
	// range. Restricted to $80-$FF on purpose: $00-$7F is TIA/RIOT registers (not
	// RAM), which kernels write with computed values constantly — including those
	// would force ZPVal to Top in every kernel. Sound (join over all RAM writes).
	switch in.Def.AddressingMode {
	case instructions.Absolute, instructions.AbsoluteX, instructions.AbsoluteY:
		if in.Operand >= 0x80 && in.Operand < 0x100 {
			n.ZPVal = n.ZPVal.join(v)
		}
	}
	a, ok := storeAddr(in)
	if !ok {
		return
	}
	if a < 0x100 {
		n.Mem[a] = v
	}
	switch a {
	case 0x00:
		n.VSync = vblankBit(v)
	case 0x01:
		n.VBlank = vblankBit(v)
	}
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
		if d.AddressingMode == instructions.AbsoluteX || d.AddressingMode == instructions.AbsoluteY {
			// 3B: an indexed load from a zero-page RAM base ($80-$FF) reads some RAM
			// cell → the RAM value range (sound over-approx of any element/index).
			if in.Operand >= 0x80 && in.Operand < 0x100 {
				return s.ZPVal
			}
			// 3D: an indexed load from a ROM address is a data-table read → its actual
			// value range over the proven index range (X for abs,X; Y for abs,Y), read
			// from THIS instruction's own bank.
			idx := s.X
			if d.AddressingMode == instructions.AbsoluteY {
				idx = s.Y
			}
			return romTableRange(s.romAt, in.Bank, in.Operand, idx, s.sw)
		}
		return vTop() // indirect ((ind,X)/(ind),Y) source value unknown
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
		n.X = s.SP
		n.setNZ(n.X)
	case instructions.TXS:
		n.SP = s.X // no flags
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
		n.applyStore(in, s.A, s.storeIndex(in))
	case instructions.STX:
		n.applyStore(in, s.X, s.storeIndex(in))
	case instructions.STY:
		n.applyStore(in, s.Y, s.storeIndex(in))
	case instructions.AND:
		if imm {
			n.A = andImm(s.A, immv) // 3A: `and #$7F` → [0,127]
		} else {
			n.A = vTop()
		}
		n.setNZ(n.A)
	case instructions.ORA:
		if imm {
			n.A = oraImm(s.A, immv)
		} else {
			n.A = vTop()
		}
		n.setNZ(n.A)
	case instructions.EOR:
		n.A = vTop() // bit flips — not modeled
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
		n.SP = incWrap(s.SP)
	case instructions.PLP:
		n.C, n.Z, n.N = triUnknown, triUnknown, triUnknown
		n.SP = incWrap(s.SP)
	case instructions.PHA, instructions.PHP:
		// A push stores to $0100+SP, and page 1 mirrors this same RAM, so it is a
		// store like any other — with a target that is known exactly whenever SP is.
		// When SP is unknown the push could have landed anywhere in RAM, which is
		// why the whole tracked map used to be dropped on every push.
		if sp, ok := s.SP.konst(); ok {
			if t := uint16(0x0100 | (sp & 0xFF)); t >= 0x0180 {
				delete(n.Mem, t&0xFF) // page-1 mirror of RAM $80-$FF
			}
		} else {
			n.Mem = map[uint16]ValueRange{}
		}
		n.SP = decWrap(s.SP)
	case instructions.JSR:
		// pushes the return address: two bytes.
		if sp, ok := s.SP.konst(); ok {
			for i := 0; i < 2; i++ {
				if t := uint16(0x0100 | ((sp - i) & 0xFF)); t >= 0x0180 {
					delete(n.Mem, t&0xFF)
				}
			}
			n.SP = vConst((sp - 2) & 0xFF)
		} else {
			n.Mem = map[uint16]ValueRange{}
			n.SP = vTop()
		}
	case instructions.RTS, instructions.RTI:
		if sp, ok := s.SP.konst(); ok {
			n.SP = vConst((sp + 2) & 0xFF)
		} else {
			n.SP = vTop()
		}
	case instructions.JMP, instructions.BRK,
		instructions.BEQ, instructions.BNE, instructions.BCS, instructions.BCC,
		instructions.BMI, instructions.BPL, instructions.BVS, instructions.BVC:
		// control flow: registers/flags unchanged here (branch refinement is separate)
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

// absEdge is a CFG successor SITE paired with the abstract state on entry.
type absEdge struct {
	at    site
	state State
}

// absSuccessors lists the CFG successors of in with the abstract state flowing to
// each (branches refine the tested flag; a JSR's return point is reset to Top
// since the callee's effect is not modeled).
//
// CROSS-BANK EDGES ARE INCLUDED, and omitting them would not merely lose precision:
// on the merged program an unvisited landing site has NO entry in the state map, so
// `states[at]` is the zero State (valid=false), which determineBound reads as "a
// predecessor we know nothing about" and answers 0 — the region stays unbounded and
// the whole exercise buys nothing.
func absSuccessors(in Instr, st State, sw switchModel) []absEdge {
	d := in.Def
	switch d.Operator {
	case instructions.JMP:
		if d.AddressingMode == instructions.Indirect {
			return nil
		}
		return []absEdge{{site{in.Bank, in.Operand}, st}}
	case instructions.JSR:
		// The callee's effect is not modeled, so the return point starts from Top —
		// except for the display bits, which are kept when the callee provably
		// cannot write VSYNC/VBLANK.
		//
		// Dropping them unconditionally is not merely imprecise, it is
		// self-defeating on the ordinary two-sprite kernel: `jsr SetXPos` twice
		// leaves VBLANK unknown at the SECOND call site, that unknown flows into the
		// shared subroutine, joins with the known-on state from the first call, and
		// the routine's own entry state comes out unknown. The region inside it is
		// then classified `visible` although it runs entirely inside VBLANK, and a
		// blank-region overrun (frame-line drift, caught by `ntsc_frame_lines`) is
		// reported as a visible-line tear, which is a different and much louder
		// claim. Measured on a generated clone: one call site certified, the same
		// kernel with two did not.
		ret := topState()
		if st.preservesDisplay != nil && st.preservesDisplay(site{in.Bank, in.Operand}) {
			ret.VSync, ret.VBlank = st.VSync, st.VBlank
		}
		ret.preservesDisplay = st.preservesDisplay
		ret.romAt, ret.sw = st.romAt, st.sw
		return []absEdge{{site{in.Bank, in.Operand}, st}, {in.nextSite(), ret}}
	case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
		return nil
	}
	if d.IsBranch() {
		tk, nt := st.refineBranch(in)
		var es []absEdge
		if tk.valid {
			es = append(es, absEdge{in.branchSite(), tk})
		}
		if nt.valid {
			es = append(es, absEdge{in.nextSite(), nt})
		}
		return es
	}
	// A straight-line instruction can still leave its bank: an access reaching a
	// bank-switch hotspot continues at the same address in the target bank, and the
	// post-state flows along that edge exactly as it would along a fall-through. An
	// instruction whose switch is NOT modelled contributes no edge here, which is why
	// Prove widens every site to Top when any such instruction exists (see
	// anyUnmodelledSwitch) — an incomplete predecessor set would otherwise leave the
	// real landing site with a state narrower than the machine's.
	post := st.transfer(in)
	succ, refusal := successors(in, st, sw)
	if refusal != "" {
		return nil
	}
	es := make([]absEdge, 0, len(succ))
	for _, nx := range succ {
		es = append(es, absEdge{nx, post})
	}
	return es
}

// zpInitRange returns the value every zero-page cell is known to hold at start:
// vConst(0) when the program clears ZP with the canonical idiom (a backward loop
// containing `sta $00,x`+`dex/dey`, with a `lda #0` present), else vTop(). Used to
// seed State.ZPVal so indexed ZP loads are bounded ONLY when ZP is provably
// initialised — otherwise they stay Top (unbounded), which is sound.
// Scanning the MERGED map is correct and strictly better than scanning one bank: the
// RAM-clear sweep lives in bank 0 but the fact it establishes ("ZP is zero") is
// program-wide, so a worker bank's states now inherit it.
func zpInitRange(instrs map[site]Instr) ValueRange {
	hasLdaZero := false
	for _, in := range instrs {
		if in.Def.Operator == instructions.LDA && in.Def.AddressingMode == instructions.Immediate && (in.Operand&0xFF) == 0 {
			hasLdaZero = true
			break
		}
	}
	if !hasLdaZero {
		return vTop()
	}
	for _, latch := range instrs {
		if latch.Def.Operator != instructions.BNE && latch.Def.Operator != instructions.BPL {
			continue
		}
		hdr := latch.branchSite()
		if hdr.addr > latch.Addr {
			continue // forward branch, not a loop latch
		}
		hasClear, hasDec := false, false
		for a := hdr; ; {
			in, ok := instrs[a]
			if !ok {
				break
			}
			op, m := in.Def.Operator, in.Def.AddressingMode
			if op == instructions.STA && (m == instructions.AbsoluteX || m == instructions.AbsoluteY) && in.Operand < 0x02 {
				hasClear = true // `sta $00,x` / `sta $01,x` — clearing zero page
			}
			if op == instructions.DEX || op == instructions.DEY {
				hasDec = true
			}
			if in.Addr == latch.Addr {
				break
			}
			a = in.nextSite()
		}
		if hasClear && hasDec {
			return vConst(0)
		}
	}
	return vTop()
}

// computeStates runs a forward abstract interpretation (worklist fixpoint) from
// the entry points and returns the abstract state on entry to each instruction.
// The lattice is finite (8-bit ranges + tri-bools), so it converges; maxIter is a
// safety cap (on hit the partial result is still sound — callers treat an absent
// address as Top / display-unknown, i.e. checked conservatively).
// computeStates runs the forward fixpoint. The second return value reports
// whether it actually CONVERGED: the iteration cap is a backstop against a
// non-terminating ascending chain, and a run that hits it has stopped early with
// states that are UNDER-approximations. Consumers (bounds, page penalties, branch
// refinement) treat their inputs as sound, so an unconverged result must never be
// silently handed to them — a proof resting on it would be a proof of nothing.
// widen lists sites that must be seeded to Top IN ADDITION to the entry points,
// because an unmodelled bank switch can land on them: see Report.SwitchWidenedSites.
func computeStates(instrs map[site]Instr, entries []site, romAt func(int, uint16) (byte, bool),
	sw switchModel, widen []site) (map[site]State, bool) {
	st, ok := computeStatesWith(instrs, entries, romAt, sw, widen, nil)
	// Second pass: now that index ranges are known, a shared subroutine that picks
	// its object with `sta RESP0,X` can be shown not to touch the display, so the
	// display bits survive the call instead of being dropped at every return point.
	st2, ok2 := computeStatesWith(instrs, entries, romAt, sw, widen, st)
	if !ok2 {
		return st, ok
	}
	return st2, ok2
}

// iterCap scales the fixpoint's safety cap with the program.
//
// The cap is a backstop against a non-terminating ascending chain, not a budget, and
// a fixed one silently becomes a budget as the node set grows: a merged bank-switched
// program is several times the size of any single bank (measured, litmus_bank_f4 merges
// bank 0's 103 instructions with banks 1-7's 1362..1398 into 9873 sites, against
// ~1.4k per bank before), and the fixpoint runs twice. A capped run returns
// Converged=false and can certify nothing, so the cost of a cap that is too small is
// a refusal — but a refusal for an arithmetic reason nobody asked about is still a
// wrong answer to the question the caller posed.
func iterCap(n int) int {
	const base = 300000
	if c := 200 * n; c > base {
		return c
	}
	return base
}

func computeStatesWith(instrs map[site]Instr, entries []site, romAt func(int, uint16) (byte, bool),
	sw switchModel, widen []site, prev map[site]State) (map[site]State, bool) {
	entryState := map[site]State{}
	var work []site
	inWork := map[site]bool{}
	push := func(a site, s State) {
		old, ok := entryState[a]
		merged := s
		if ok {
			merged = old.joinState(s)
		}
		if !ok || !merged.eqState(old) {
			entryState[a] = merged
			if !inWork[a] {
				work = append(work, a)
				inWork[a] = true
			}
		}
	}
	zpInit := zpInitRange(instrs) // 3B: seed ZP value range (0 if cleared, else Top)
	preserves := displayPreserver(instrs, prev, sw)
	seedAt := func(e site) {
		seed := topState()
		seed.ZPVal = zpInit
		seed.romAt = romAt // 3D: ROM-table reads
		seed.preservesDisplay = preserves
		seed.sw = sw
		push(e, seed)
	}
	for _, e := range entries {
		seedAt(e)
	}
	// A possible landing of an unmodelled switch has an incomplete incoming-edge set, so
	// its state may not be narrower than Top.
	for _, a := range widen {
		seedAt(a)
	}
	maxIter := iterCap(len(instrs))
	it := 0
	for ; len(work) > 0 && it < maxIter; it++ {
		at := work[0]
		work = work[1:]
		inWork[at] = false
		in, ok := instrs[at]
		if !ok {
			continue
		}
		st := entryState[at]
		if !st.valid {
			continue
		}
		for _, e := range absSuccessors(in, st, sw) {
			push(e.at, e.state)
		}
	}
	return entryState, len(work) == 0 && it < maxIter
}

// baseCost is the instruction's cycle count WITHOUT the page-cross penalty.
func (in Instr) baseCost() int { return in.Def.Cycles }

// pagePenalty returns the worst-case +1 page-cross cycle for a page-sensitive
// READ (S3). For abs,X / abs,Y with a known index range we prove whether
// [base+lo, base+hi] crosses a 256-byte page; if it provably can't, the penalty
// is 0. An unknown index, or a pointer-based (ind),Y whose base we don't track,
// stays conservative (+1) — sound. Branches are costed on the CFG edges, not here.
func (in Instr) pagePenalty(s State) int {
	d := in.Def
	if !d.PageSensitive || d.IsBranch() {
		return 0
	}
	// A PAGE-ALIGNED BASE CANNOT BE CROSSED, and this needs no index analysis at
	// all: a 6502 index register holds 0..255, so `$NN00 + idx` is at most `$NNFF`
	// and stays in the base's page for every possible index. The check therefore
	// runs BEFORE the two conservative bail-outs below, which is the whole point —
	// the case that matters is precisely the one where the index range is NOT
	// provable, because that is when a table gets aligned in the first place.
	//
	// Only absolute,X / absolute,Y carry a base to reason about. For (ind),Y the
	// operand is a zero-page POINTER, not the base, and its low byte says nothing
	// about where the read lands.
	//
	// Measured 2026-08-01. Over the 135 ROMs in `roms/`, this branch fires ZERO
	// times — and that is a fact about the corpus, not about the case. 24 of the 31
	// technique kernels draw no playfield at all, so the corpus contains no
	// table-driven picture kernel, which is exactly the shape that aligns tables.
	// The first one written (`sandbox/practice/horizon`, an asymmetric-playfield
	// sunset) produced **8 wasted charges immediately**, 11 across the practice set:
	// eight `lda Tab,y` reads from eight page-aligned tables, each charged a cycle
	// the hardware cannot spend. Its proven worst was 74 against a budget of 76 and
	// a machine that takes 66 — the author is told there are 2 cycles of headroom
	// when there are 10. A bound that is merely safe sends an author trimming work
	// that was never over budget, which is the objection this file already records
	// against loose conditional bounds.
	if (d.AddressingMode == instructions.AbsoluteX || d.AddressingMode == instructions.AbsoluteY) &&
		int(in.Operand)&0xFF == 0 {
		return 0
	}
	if !s.valid {
		return 1 // no tracked state at this point -> conservative
	}
	var idx ValueRange
	switch d.AddressingMode {
	case instructions.AbsoluteX:
		idx = s.X
	case instructions.AbsoluteY:
		idx = s.Y
	default:
		return 1 // (ind),Y / other indexed: base unknown -> conservative
	}
	if idx.Top {
		return 1
	}
	base := int(in.Operand)
	// The 6502 charges the extra cycle when the indexed TARGET lands in a different
	// page from the BASE — not when the range of possible targets straddles a page
	// boundary. The two are different questions and the old code asked the second
	// one: it compared base+Lo against base+Hi, so a CONSTANT index (Lo == Hi)
	// always compared equal and the penalty was never charged, however far the
	// access reached.
	//
	// Measured on a fixture with `ldy #200` and four `lda Table,y` in one WSYNC
	// region: at Table $F0F8 the target is $F1C0, a page beyond the base, and the
	// prover reported the same worst case (35) as Table $F100 -> $F1C8, which stays
	// in page $F1. Four reads, so four cycles the hardware spends and the proof did
	// not — an UNDER-approximation, the one direction this package forbids.
	//
	// Crossing is monotonic in the index for a fixed base, so the worst case over
	// [Lo, Hi] is decided by Hi alone.
	if (base >> 8) != ((base + idx.Hi) >> 8) {
		return 1 // the indexed access reaches into the next page
	}
	return 0 // provably within the base's page
}
