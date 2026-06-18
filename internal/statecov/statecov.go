// Package statecov builds a STATE-coverage matrix for a ROM run (VV-11,
// docs/capability-gap-audit.md): a coverage axis orthogonal to PC/branch
// coverage (VV-3). Instead of "which instructions ran", it answers "which TIA
// MODES did the test actually exercise" — e.g. did it ever drive a triple-copy
// NUSIZ, turn on a vertical-delay, use playfield reflect/score/priority, or
// switch banks. An axis the test never moves off its reset value is a blind
// spot: the kernel may handle that mode, but nothing here proves it.
//
// It samples the write-only TIA registers (emu.ReadTIARegisters) plus the active
// bank once per scanline across a multi-frame run, accumulating the set of
// distinct values seen per axis.
package statecov

import (
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Axes is the ordered list of state-coverage axes. Fixed-domain axes have a
// known cardinality (for a coverage fraction); "bank" is cart-dependent and
// reported as a raw count.
var Axes = []string{
	"nusiz0_copies", "nusiz1_copies",
	"missile0_size", "missile1_size", "ball_size",
	"vdelp0", "vdelp1", "vdelbl",
	"pf_reflect", "pf_priority", "pf_score",
	"bank",
}

// axisDomain is the number of distinct values a fixed-domain axis can take.
var axisDomain = map[string]int{
	"nusiz0_copies": 8, "nusiz1_copies": 8,
	"missile0_size": 4, "missile1_size": 4, "ball_size": 4,
	"vdelp0": 2, "vdelp1": 2, "vdelbl": 2,
	"pf_reflect": 2, "pf_priority": 2, "pf_score": 2,
}

// Matrix accumulates the distinct values seen per axis.
type Matrix struct {
	seen     map[string]map[int]bool
	Samples  int
}

// NewMatrix returns an empty matrix.
func NewMatrix() *Matrix { return &Matrix{seen: map[string]map[int]bool{}} }

func (m *Matrix) mark(axis string, v int) {
	s := m.seen[axis]
	if s == nil {
		s = map[int]bool{}
		m.seen[axis] = s
	}
	s[v] = true
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Sample records e's current TIA state and active bank into the matrix.
func (m *Matrix) Sample(e *emu.Emu) {
	t := e.ReadTIARegisters()
	m.mark("nusiz0_copies", int(t.Player0.SizeAndCopies))
	m.mark("nusiz1_copies", int(t.Player1.SizeAndCopies))
	m.mark("missile0_size", int(t.Missile0.Size))
	m.mark("missile1_size", int(t.Missile1.Size))
	m.mark("ball_size", int(t.Ball.Size))
	m.mark("vdelp0", b2i(t.Player0.VerticalDelay))
	m.mark("vdelp1", b2i(t.Player1.VerticalDelay))
	m.mark("vdelbl", b2i(t.Ball.VerticalDelay))
	m.mark("pf_reflect", b2i(t.Playfield.Reflected))
	m.mark("pf_priority", b2i(t.Playfield.Priority))
	m.mark("pf_score", b2i(t.Playfield.Scoremode))
	bank, _ := e.Bank()
	m.mark("bank", bank)
	m.Samples++
}

// Count is the number of distinct values seen on an axis.
func (m *Matrix) Count(axis string) int { return len(m.seen[axis]) }

// Values returns the distinct values seen on an axis, sorted.
func (m *Matrix) Values(axis string) []int {
	s := m.seen[axis]
	out := make([]int, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// Domain returns the axis cardinality and whether it is a fixed domain.
func Domain(axis string) (int, bool) {
	d, ok := axisDomain[axis]
	return d, ok
}

// Run loads romPath and samples TIA state once per scanline over the given
// number of frames. spec "" defaults to NTSC.
func Run(romPath, spec string, frames int) (*Matrix, error) {
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, err
	}
	m := NewMatrix()
	const scanlinesPerFrame = 262
	for f := 0; f < frames; f++ {
		for sl := 0; sl < scanlinesPerFrame; sl++ {
			if err := e.StepScanline(); err != nil {
				return m, err
			}
			m.Sample(e)
		}
	}
	return m, nil
}
