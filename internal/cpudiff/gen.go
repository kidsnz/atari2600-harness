package cpudiff

import "math/rand"

// GenVectors produces n deterministic random test vectors drawn from the given
// opcode set. Each vector gets random A/X/Y/S/P, two random operand bytes at
// 0xF801/0xF802, and a scatter of random zero-page + scratch data — all of which
// both engines read from the identical image, so any divergence is a genuine
// CPU disagreement, not a setup artifact.
func GenVectors(seed int64, n int, opcodes []byte) []Vector {
	rng := rand.New(rand.NewSource(seed))
	out := make([]Vector, n)
	for i := range out {
		op := opcodes[rng.Intn(len(opcodes))]
		// The trailing code bytes must not equal the opcode: for a 1- or 2-byte
		// instruction the next opcode is fetched from F801/F802, and the
		// single-instruction boundary is detected by IR leaving the test opcode.
		// A repeat of the opcode there would hide the boundary. Costs one operand
		// value out of 256 — negligible coverage.
		ne := func() byte {
			b := byte(rng.Intn(256))
			if b == op {
				b++
			}
			return b
		}
		m := map[uint16]byte{
			InstructionAddr:     op,
			InstructionAddr + 1: ne(),
			InstructionAddr + 2: ne(),
		}
		// seed a little zero-page and scratch data for memory-touching modes
		for j := 0; j < 8; j++ {
			m[uint16(rng.Intn(0x100))] = byte(rng.Intn(256))   // zero page
			m[uint16(0x0200+rng.Intn(0x100))] = byte(rng.Intn(256)) // scratch page
		}
		out[i] = Vector{
			A:   byte(rng.Intn(256)),
			X:   byte(rng.Intn(256)),
			Y:   byte(rng.Intn(256)),
			S:   byte(rng.Intn(256)),
			P:   byte(rng.Intn(256)),
			Mem: m,
		}
	}
	return out
}

// DocumentedSmoke is a curated set of legal 6502 opcodes spanning every
// addressing mode and operation class, used for the strict zero-divergence
// validation. Not exhaustive — the generative run (AllOpcodes) covers the rest.
var DocumentedSmoke = []byte{
	0xA9, 0xA5, 0xB5, 0xAD, 0xBD, 0xB9, 0xA1, 0xB1, // LDA all modes
	0xA2, 0xA6, 0xB6, 0xAE, 0xBE, // LDX
	0xA0, 0xA4, 0xB4, 0xAC, 0xBC, // LDY
	0x85, 0x95, 0x8D, 0x9D, 0x99, 0x81, 0x91, // STA
	0x86, 0x96, 0x8E, 0x84, 0x94, 0x8C, // STX/STY
	0x69, 0x65, 0x75, 0x6D, 0x7D, 0x79, 0x61, 0x71, // ADC
	0xE9, 0xE5, 0xF5, 0xED, 0xFD, 0xF9, 0xE1, 0xF1, // SBC
	0x29, 0x09, 0x49, 0x2C, 0x24, // AND/ORA/EOR/BIT
	0xC9, 0xE0, 0xC0, 0xC5, 0xE4, 0xC4, // CMP/CPX/CPY
	0x0A, 0x4A, 0x2A, 0x6A, // ASL/LSR/ROL/ROR (accumulator)
	0x06, 0x16, 0x0E, 0x1E, // ASL mem
	0xE6, 0xF6, 0xEE, 0xFE, 0xC6, 0xD6, 0xCE, 0xDE, // INC/DEC mem
	0xE8, 0xC8, 0xCA, 0x88, // INX/INY/DEX/DEY
	0xAA, 0xA8, 0x8A, 0x98, 0xBA, 0x9A, // TAX/TAY/TXA/TYA/TSX/TXS
	0x48, 0x68, 0x08, 0x28, // PHA/PLA/PHP/PLP
	0x18, 0x38, 0x58, 0x78, 0xB8, 0xD8, 0xF8, // flag clears/sets
	0x4C, 0x6C, 0x20, 0x60, 0x40, // JMP/JMP()/JSR/RTS/RTI
	0xD0, 0xF0, 0x10, 0x30, 0x90, 0xB0, 0x50, 0x70, // branches
	0xEA, // NOP
}

// AllOpcodes is 0x00..0xFF.
func AllOpcodes() []byte {
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	return all
}

// jamOpcodes halt the CPU (KIL/JAM). perfect6502 stops fetching (reported as
// "overrun"); Gopher2600 sets its Jammed flag ("jam"). Both mean "halt".
var jamOpcodes = map[byte]bool{
	0x02: true, 0x12: true, 0x22: true, 0x32: true, 0x42: true, 0x52: true,
	0x62: true, 0x72: true, 0x92: true, 0xB2: true, 0xD2: true, 0xF2: true,
}

// IsHalt reports whether op halts the CPU.
func IsHalt(op byte) bool { return jamOpcodes[op] }

// ExpectedDivergence reports whether a silicon-vs-Gopher disagreement on this
// opcode is an EXPECTED, classified divergence (returning the class name) rather
// than a bug to investigate. Populated empirically — see TestSiliconAgreesDocumented
// for the zero-divergence guarantee on legal opcodes.
func ExpectedDivergence(op byte) (string, bool) {
	if c, ok := expectedDivergence[op]; ok {
		return c, true
	}
	return "", false
}

// expectedDivergence lists the ONLY opcodes on which the silicon netlist and the
// embedded Gopher2600 CPU are permitted to disagree. Established empirically by
// sweeping all 256 opcodes across many seeds (cmd/cpucheck): every divergence is
// an illegal/undocumented opcode, never a documented one. These fall in two
// classes, both of which the Tom Harte corpus (VV-1) likewise excludes:
//
//   - "unstable": magic-constant / analog opcodes whose result is not even
//     deterministic on real hardware (ANE, LXA, and the SH* high-byte stores).
//   - "undocumented": illegal opcodes with model-dependent flag/result behavior
//     (ANC, ALR, ARR, LAS).
//
// Any divergence OUTSIDE this set is treated as unexpected — a Gopher bug to
// investigate or a harness artifact to fix — and fails the check.
var expectedDivergence = map[byte]string{
	0x8B: "unstable", // ANE / XAA
	0xAB: "unstable", // LXA / LAX #imm
	0x93: "unstable", // SHA (zp),Y
	0x9F: "unstable", // SHA abs,Y
	0x9E: "unstable", // SHX abs,Y
	0x9C: "unstable", // SHY abs,X
	0x9B: "unstable", // TAS / SHS abs,Y
	0x0B: "undocumented", // ANC #imm
	0x2B: "undocumented", // ANC #imm
	0x4B: "undocumented", // ALR / ASR #imm
	0x6B: "undocumented", // ARR #imm
	0xBB: "undocumented", // LAS abs,Y
}

// HaltEquivalent reports whether two statuses both denote a halted CPU.
func HaltEquivalent(a, b string) bool {
	halt := func(s string) bool { return s == "jam" || s == "overrun" }
	return halt(a) && halt(b)
}
