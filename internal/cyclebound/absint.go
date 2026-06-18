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
func romTableRange(romAt func(uint16) (byte, bool), base uint16, idx ValueRange) ValueRange {
	if romAt == nil || idx.Top || idx.Hi-idx.Lo > 255 {
		return vTop()
	}
	lo, hi := 256, -1
	for i := idx.Lo; i <= idx.Hi; i++ {
		b, ok := romAt(base + uint16(i))
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
	// romAt (3D) reads a cartridge ROM byte (nil outside Prove). Lets an indexed
	// load from a ROM data table (`lda table,x`) return the table's actual value
	// range over the proven index range — the data is constant and in the binary.
	// Not part of state equality (it is a constant capability, not a value).
	romAt func(uint16) (byte, bool)
	valid bool
}

func botState() State { return State{} } // bottom: valid=false (unreachable)
func topState() State {
	return State{A: vTop(), X: vTop(), Y: vTop(), Mem: map[uint16]ValueRange{}, ZPVal: vTop(), valid: true}
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
	r.C, r.Z, r.N = s.C.join(o.C), s.Z.join(o.Z), s.N.join(o.N)
	r.VBlank, r.VSync = s.VBlank.join(o.VBlank), s.VSync.join(o.VSync)
	r.ZPVal = s.ZPVal.join(o.ZPVal)
	if r.romAt = s.romAt; r.romAt == nil { // constant capability — carry it through
		r.romAt = o.romAt
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
	if !(s.A.eq(o.A) && s.X.eq(o.X) && s.Y.eq(o.Y) && s.C == o.C && s.Z == o.Z && s.N == o.N &&
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

// applyStore updates VSYNC/VBLANK tracking and a tracked zero-page cell for a
// store of value v to a statically known address.
func (n *State) applyStore(in Instr, v ValueRange) {
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
			// value range over the proven index range (X for abs,X; Y for abs,Y).
			idx := s.X
			if d.AddressingMode == instructions.AbsoluteY {
				idx = s.Y
			}
			return romTableRange(s.romAt, in.Operand, idx)
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
		n.applyStore(in, s.A)
	case instructions.STX:
		n.applyStore(in, s.X)
	case instructions.STY:
		n.applyStore(in, s.Y)
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
	case instructions.PLP:
		n.C, n.Z, n.N = triUnknown, triUnknown, triUnknown
	case instructions.PHA, instructions.PHP:
		// no tracked change
	case instructions.JMP, instructions.JSR, instructions.RTS, instructions.RTI, instructions.BRK,
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

// absEdge is a CFG successor address paired with the abstract state on entry.
type absEdge struct {
	addr  uint16
	state State
}

// absSuccessors lists the CFG successors of in with the abstract state flowing to
// each (branches refine the tested flag; a JSR's return point is reset to Top
// since the callee's effect is not modeled).
func absSuccessors(in Instr, st State) []absEdge {
	d := in.Def
	switch d.Operator {
	case instructions.JMP:
		if d.AddressingMode == instructions.Indirect {
			return nil
		}
		return []absEdge{{in.Operand, st}}
	case instructions.JSR:
		return []absEdge{{in.Operand, st}, {in.next(), topState()}}
	case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
		return nil
	}
	if d.IsBranch() {
		tk, nt := st.refineBranch(in)
		var es []absEdge
		if tk.valid {
			es = append(es, absEdge{in.branchTarget(), tk})
		}
		if nt.valid {
			es = append(es, absEdge{in.next(), nt})
		}
		return es
	}
	return []absEdge{{in.next(), st.transfer(in)}}
}

// zpInitRange returns the value every zero-page cell is known to hold at start:
// vConst(0) when the program clears ZP with the canonical idiom (a backward loop
// containing `sta $00,x`+`dex/dey`, with a `lda #0` present), else vTop(). Used to
// seed State.ZPVal so indexed ZP loads are bounded ONLY when ZP is provably
// initialised — otherwise they stay Top (unbounded), which is sound.
func zpInitRange(instrs map[uint16]Instr) ValueRange {
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
		hdr := latch.branchTarget()
		if hdr > latch.Addr {
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
			a = in.next()
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
func computeStates(instrs map[uint16]Instr, entries []uint16, romAt func(uint16) (byte, bool)) map[uint16]State {
	entryState := map[uint16]State{}
	var work []uint16
	inWork := map[uint16]bool{}
	push := func(a uint16, s State) {
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
	for _, e := range entries {
		seed := topState()
		seed.ZPVal = zpInit
		seed.romAt = romAt // 3D: ROM-table reads
		push(e, seed)
	}
	const maxIter = 300000
	for it := 0; len(work) > 0 && it < maxIter; it++ {
		addr := work[0]
		work = work[1:]
		inWork[addr] = false
		in, ok := instrs[addr]
		if !ok {
			continue
		}
		st := entryState[addr]
		if !st.valid {
			continue
		}
		for _, e := range absSuccessors(in, st) {
			push(e.addr, e.state)
		}
	}
	return entryState
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
	if (base+idx.Lo)>>8 != (base+idx.Hi)>>8 {
		return 1 // the indexed access may cross a page
	}
	return 0 // provably within one page
}
