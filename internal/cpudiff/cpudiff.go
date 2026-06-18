// Package cpudiff is a hardware-grade differential oracle for the 6502 CPU
// (VV-7, docs/capability-gap-audit.md). It runs one instruction at a time on
// BOTH the embedded Gopher2600 CPU core and the perfect6502 transistor-level
// netlist (via the external bin/p6502step harness) and reports any divergence
// in the resulting registers, cycle count, or memory writes.
//
// perfect6502 is a CPU-only model (no TIA/RIOT), so this is NOT a member of the
// full-system RAM vote (cmd/oraclevote). It cross-checks at the instruction
// layer — the layer where Gopher2600 and MAME (both software) could share a CPU
// bug that no software-vs-software vote would ever catch. It is also generative
// (random states, undocumented opcodes, decimal-mode edges) where the vendored
// Tom Harte corpus (VV-1) is a fixed set that explicitly excludes the unstable
// undocumented opcodes.
//
// Symmetric execution. Rather than inject CPU state directly (perfect6502
// exposes no register writers), both engines run an identical 64K image from
// reset: the reset vector points at a short prologue that injects S/P/A/X/Y and
// JMPs to the instruction under test at 0xF800. Because both run the same
// prologue, they reach identical pre-instruction state by construction — see
// buildImage, which mirrors p6502step.c's setup_memory byte-for-byte.
package cpudiff

import (
	"fmt"

	"github.com/jetsetilly/gopher2600/hardware/cpu"
)

// Memory layout shared with internal/cpudiff/p6502step/p6502step.c. Keep in sync.
const (
	SetupAddr       = 0xF400 // register-injection prologue
	InstructionAddr = 0xF800 // instruction under test
	brkVector       = 0xFC00
)

// Vector is one test case: the register immediates injected by the prologue and
// a memory map (instruction bytes at InstructionAddr plus any data the
// instruction reads). It deliberately mirrors what p6502step.c consumes.
type Vector struct {
	A, X, Y, S, P byte
	Mem           map[uint16]byte
}

// Opcode is the instruction under test.
func (v Vector) Opcode() byte { return v.Mem[InstructionAddr] }

// sentinel is the fill byte for unspecified memory; chosen distinct from the
// test opcode so the next opcode fetch reliably changes IR in p6502step.
func (v Vector) sentinel() byte {
	if v.Opcode() == 0xEA {
		return 0x00
	}
	return 0xEA
}

// Result is the post-instruction state, as observed by either engine.
type Result struct {
	Status string // "ok", "jam", "badfetch", "overrun", or an error string
	A, X, Y, S, P byte
	PC     uint16
	Cycles int
	Writes map[uint16]byte // net memory changes caused by the instruction
}

// buildImage constructs the full 64K image both engines run. MUST match
// p6502step.c's setup_memory + poke order exactly.
func buildImage(v Vector) [0x10000]byte {
	var m [0x10000]byte
	s := v.sentinel()
	for i := range m {
		m[i] = s
	}
	// reset vector -> prologue
	m[0xFFFC] = SetupAddr & 0xFF
	m[0xFFFD] = SetupAddr >> 8
	a := SetupAddr
	put := func(b byte) { m[a] = b; a++ }
	put(0xA2)
	put(v.S) // LDX #S
	put(0x9A) // TXS
	put(0xA9)
	put(v.P) // LDA #P
	put(0x48) // PHA
	put(0xA9)
	put(v.A) // LDA #A
	put(0xA2)
	put(v.X) // LDX #X
	put(0xA0)
	put(v.Y) // LDY #Y
	put(0x28) // PLP
	put(0x4C)
	put(InstructionAddr & 0xFF)
	put(InstructionAddr >> 8) // JMP InstructionAddr
	// BRK/IRQ vector landing pad: use the sentinel (not 0x00) so a BRK under
	// test lands on a non-BRK byte and the single-instruction boundary resolves.
	m[0xFFFE] = brkVector & 0xFF
	m[0xFFFF] = brkVector >> 8
	m[brkVector] = s
	// instruction + data pokes (override)
	for addr, val := range v.Mem {
		m[addr] = val
	}
	return m
}

// flatMem is a 64K memory implementing cpu.Memory for isolated CPU execution.
// Same shape as Gopher2600's own thomharte test memory.
type flatMem struct{ b [0x10000]byte }

func (mem *flatMem) Read(address uint16) (uint8, error)  { return mem.b[address], nil }
func (mem *flatMem) Write(address uint16, data uint8) error { mem.b[address] = data; return nil }

// RunGopher executes one test instruction on the embedded Gopher2600 CPU core,
// following the same image+prologue as p6502step.
func RunGopher(v Vector) (Result, error) {
	mem := &flatMem{b: buildImage(v)}
	mc := cpu.NewCPU(mem)
	if err := mc.Reset(nil); err != nil { // nil Random => deterministic zero regs, PC<-[FFFC]
		return Result{}, err
	}
	noop := func() error { return nil }
	// run the prologue until we arrive at the instruction under test
	for guard := 0; mc.PC.Address() != InstructionAddr; guard++ {
		if guard > 32 {
			return Result{Status: "badfetch"}, nil
		}
		if err := mc.ExecuteInstruction(noop); err != nil {
			return Result{Status: "prologue:" + err.Error()}, nil
		}
	}
	// snapshot memory so we can diff just the test instruction's writes
	pre := mem.b
	if err := mc.ExecuteInstruction(noop); err != nil {
		return Result{Status: "exec:" + err.Error()}, nil
	}
	r := Result{
		Status: "ok",
		A:      mc.A.Value(),
		X:      mc.X.Value(),
		Y:      mc.Y.Value(),
		S:      byte(mc.SP.Address()),
		P:      mc.Status.Value(),
		PC:     mc.PC.Address(),
		Cycles: mc.LastResult.Cycles,
		Writes: map[uint16]byte{},
	}
	if mc.Jammed {
		r.Status = "jam"
	}
	for i := range mem.b {
		if mem.b[i] != pre[i] {
			r.Writes[uint16(i)] = mem.b[i]
		}
	}
	return r, nil
}

// Field is one diverging field between two results.
type Field struct {
	Name   string
	Gopher string
	Silicon string
}

// pMask clears bits 4 (B) and 5 (unused) of the status register: these are not
// real register flip-flops and the two models represent them by different
// conventions, so they are excluded from comparison (as measure.c / Harte do).
const pMask = 0xCF

// Compare returns the fields on which the two results disagree. g is the
// Gopher2600 result, s the perfect6502 (silicon) result.
func Compare(g, s Result) []Field {
	var d []Field
	add := func(name string, gv, sv interface{}) {
		d = append(d, Field{name, fmt.Sprintf("%v", gv), fmt.Sprintf("%v", sv)})
	}
	if g.Status != s.Status {
		add("status", g.Status, s.Status)
	}
	// On a JAM, register/cycle/PC semantics differ between a full-instruction
	// model and a halted netlist; status divergence above is the signal.
	if g.Status == "ok" && s.Status == "ok" {
		if g.A != s.A {
			add("A", fmt.Sprintf("%02X", g.A), fmt.Sprintf("%02X", s.A))
		}
		if g.X != s.X {
			add("X", fmt.Sprintf("%02X", g.X), fmt.Sprintf("%02X", s.X))
		}
		if g.Y != s.Y {
			add("Y", fmt.Sprintf("%02X", g.Y), fmt.Sprintf("%02X", s.Y))
		}
		if g.S != s.S {
			add("S", fmt.Sprintf("%02X", g.S), fmt.Sprintf("%02X", s.S))
		}
		if g.P&pMask != s.P&pMask {
			add("P", fmt.Sprintf("%02X", g.P&pMask), fmt.Sprintf("%02X", s.P&pMask))
		}
		if g.PC != s.PC {
			add("PC", fmt.Sprintf("%04X", g.PC), fmt.Sprintf("%04X", s.PC))
		}
		if g.Cycles != s.Cycles {
			add("cycles", g.Cycles, s.Cycles)
		}
		if !sameWrites(g.Writes, s.Writes) {
			add("writes", fmtWrites(g.Writes), fmtWrites(s.Writes))
		}
	}
	return d
}

func sameWrites(a, b map[uint16]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func fmtWrites(w map[uint16]byte) string {
	s := ""
	for k, v := range w {
		s += fmt.Sprintf("%04X=%02X ", k, v)
	}
	if s == "" {
		return "(none)"
	}
	return s
}
