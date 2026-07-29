package behavmatch

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The RAM-equivalence gate.
//
// A trajectory diff answers "does it move the same?". This answers the stronger
// question "is it in the same STATE?" — the first frame and address at which the
// build's RAM stops agreeing with the target's. That first divergence is a
// debugging address: it names the byte whose update rule is wrong and the frame
// on which it first went wrong, instead of a downstream symptom many frames later.
//
// Two honest limits are built in rather than papered over:
//
//   - The gate compares a MASK, not all 128 bytes. Two correct implementations
//     legitimately differ in scratch, in never-read leftovers, and anywhere the
//     stack has been. An all-bytes gate would either fire on those or force the
//     build to replay dead writes — which would be reproducing a recording, not
//     reproducing the logic.
//   - Because a mask can hide a real divergence, every verdict carries the
//     excluded set with the reason each byte was dropped. A mask that is never
//     shown is a mask that silently inflates a pass.

// Mask selects which RAM addresses take part in the gate.
type Mask struct {
	In     [emu.RAMSize]bool // participating addresses, indexed addr−$80
	Reason map[uint16]string // why an excluded address was dropped
}

// NewMask returns a mask with nothing selected.
func NewMask() Mask { return Mask{Reason: map[uint16]string{}} }

// FullMask selects all 128 bytes — the strictest, noisiest gate.
func FullMask() Mask {
	m := NewMask()
	for i := range m.In {
		m.In[i] = true
	}
	return m
}

// Has reports whether addr participates.
func (m Mask) Has(addr uint16) bool {
	if addr < emu.RAMBase || addr >= emu.RAMBase+emu.RAMSize {
		return false
	}
	return m.In[addr-emu.RAMBase]
}

// Addrs lists the participating addresses, ascending.
func (m Mask) Addrs() []uint16 {
	var out []uint16
	for i, in := range m.In {
		if in {
			out = append(out, uint16(emu.RAMBase+i))
		}
	}
	return out
}

// Excluded lists the non-participating addresses, ascending.
func (m Mask) Excluded() []uint16 {
	var out []uint16
	for i, in := range m.In {
		if !in {
			out = append(out, uint16(emu.RAMBase+i))
		}
	}
	return out
}

// drop removes addr from the mask and records why.
func (m *Mask) drop(addr uint16, reason string) {
	if addr < emu.RAMBase || addr >= emu.RAMBase+emu.RAMSize {
		return
	}
	m.In[addr-emu.RAMBase] = false
	if m.Reason == nil {
		m.Reason = map[uint16]string{}
	}
	m.Reason[addr] = reason
}

// ChangedAddrs returns every address that took more than one distinct value
// across the given traces — i.e. the bytes the scenarios actually exercised.
// A byte that never changes is either dead or simply never reached; the two are
// distinguished by whether some scenario ever reads it, which needs the
// dependency tooling, so this reports the union and leaves that call to the caller.
func ChangedAddrs(traces ...*Trace) []uint16 {
	var seen [emu.RAMSize]map[uint8]bool
	for i := range seen {
		seen[i] = map[uint8]bool{}
	}
	for _, tr := range traces {
		if tr == nil {
			continue
		}
		for _, s := range tr.Samples {
			for i := 0; i < emu.RAMSize; i++ {
				seen[i][s.AllRAM[i]] = true
			}
		}
	}
	var out []uint16
	for i := range seen {
		if len(seen[i]) > 1 {
			out = append(out, uint16(emu.RAMBase+i))
		}
	}
	return out
}

// SPRange returns the range the stack pointer moved through across the traces.
//
// It is reported, never acted on. A first version of this gate excluded every
// address "at or above the lowest SP", on the reasoning that the 6502 stack lives
// in page 1, which mirrors this RAM, so anything the stack reached is scratch.
// Measuring the real target killed that rule outright: its SP sweeps $FF down to
// $1C on every single frame, which would have excluded all 128 bytes and turned
// the gate into a guaranteed pass. Descending past an address is a TXS, not a
// write — the pointer moving over RAM disturbs nothing.
//
// Deciding which bytes a push actually wrote needs write attribution (which
// instruction stored where), not the pointer's position. Until that exists, no
// byte is excluded on stack grounds and this range is printed as evidence.
func SPRange(traces ...*Trace) (low, high uint8, ok bool) {
	low, high = 0xFF, 0x00
	for _, tr := range traces {
		if tr == nil {
			continue
		}
		for _, s := range tr.Samples {
			ok = true
			if s.SPLow < low {
				low = s.SPLow
			}
			if s.SPHigh > high {
				high = s.SPHigh
			}
		}
	}
	return low, high, ok
}

// LiveMask builds the default gate mask from measurement: the bytes the target
// actually exercised. A byte that never took a second value across the whole
// recorded suite is either dead or unreached, and comparing it proves nothing —
// but which of those two it is cannot be settled from values alone, so the
// exclusion is stated as the honest disjunction rather than as "dead".
func LiveMask(targetTraces ...*Trace) Mask {
	m := NewMask()
	for _, a := range ChangedAddrs(targetTraces...) {
		m.In[a-emu.RAMBase] = true
	}
	for i := range m.In {
		if !m.In[i] {
			m.Reason[uint16(emu.RAMBase+i)] = "constant across the recorded traces (dead or unexercised)"
		}
	}
	return m
}

// MaskFromAddrs builds a mask selecting exactly the given addresses.
func MaskFromAddrs(addrs []uint16) Mask {
	m := NewMask()
	for _, a := range addrs {
		if a >= emu.RAMBase && a < emu.RAMBase+emu.RAMSize {
			m.In[a-emu.RAMBase] = true
		}
	}
	for _, a := range m.Excluded() {
		m.Reason[a] = "not in the requested address set"
	}
	return m
}

// RAMDivergence is the earliest disagreement: lowest frame, then lowest address.
type RAMDivergence struct {
	Frame     int    `json:"frame"`
	Addr      uint16 `json:"addr"`
	TargetVal uint8  `json:"target_val"`
	MineVal   uint8  `json:"mine_val"`
}

func (d RAMDivergence) String() string {
	return fmt.Sprintf("frame %d $%02X: target=%02X mine=%02X", d.Frame, d.Addr, d.TargetVal, d.MineVal)
}

// RAMGate is a full verdict: the first divergence (nil when none) together with
// everything needed to judge how much the verdict is worth — how many frames were
// actually compared, which addresses participated, and which were excluded and why.
type RAMGate struct {
	Scenario       string            `json:"scenario"`
	TargetFrames   int               `json:"target_frames"`
	MineFrames     int               `json:"mine_frames"`
	FramesCompared int               `json:"frames_compared"`
	Compared       []uint16          `json:"compared"`
	Excluded       []uint16          `json:"excluded"`
	ExcludeReason  map[uint16]string `json:"exclude_reason,omitempty"`
	First          *RAMDivergence    `json:"first_divergence"` // nil = pass
}

// Pass reports whether no divergence was found. Note that a pass over zero
// compared addresses or zero compared frames is vacuous — String() says so.
func (g *RAMGate) Pass() bool { return g != nil && g.First == nil }

// GateRAM compares two traces byte-for-byte over the mask and returns the first
// divergence. Traces of unequal length are compared over their common prefix and
// both lengths are reported, so a build that dies early cannot pass by being short.
func GateRAM(target, mine *Trace, m Mask) *RAMGate {
	g := &RAMGate{
		Compared:      m.Addrs(),
		Excluded:      m.Excluded(),
		ExcludeReason: m.Reason,
	}
	if target != nil {
		g.Scenario = target.Scenario
		g.TargetFrames = len(target.Samples)
	}
	if mine != nil {
		g.MineFrames = len(mine.Samples)
	}
	n := g.TargetFrames
	if g.MineFrames < n {
		n = g.MineFrames
	}
	g.FramesCompared = n
	for f := 0; f < n; f++ {
		ts, ms := target.Samples[f], mine.Samples[f]
		for i := 0; i < emu.RAMSize; i++ {
			if !m.In[i] {
				continue
			}
			if ts.AllRAM[i] != ms.AllRAM[i] {
				g.First = &RAMDivergence{
					Frame: f, Addr: uint16(emu.RAMBase + i),
					TargetVal: ts.AllRAM[i], MineVal: ms.AllRAM[i],
				}
				return g
			}
		}
	}
	return g
}

// String renders the verdict with its exclusions — never the verdict alone.
func (g *RAMGate) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "  RAM gate [%s]: %d/128 bytes compared over %d frames",
		g.Scenario, len(g.Compared), g.FramesCompared)
	if g.TargetFrames != g.MineFrames {
		fmt.Fprintf(&b, "  **LENGTH DIFF** target=%d mine=%d", g.TargetFrames, g.MineFrames)
	}
	b.WriteByte('\n')
	if len(g.Compared) == 0 || g.FramesCompared == 0 {
		b.WriteString("    VACUOUS: nothing was compared — this verdict proves nothing\n")
	}
	if g.First == nil {
		fmt.Fprintf(&b, "    first_divergence: none\n")
	} else {
		fmt.Fprintf(&b, "    **FIRST DIVERGENCE** %s\n", g.First)
	}
	fmt.Fprintf(&b, "    compared: %s\n", formatAddrs(g.Compared))
	fmt.Fprintf(&b, "    excluded: %s\n", formatAddrs(g.Excluded))
	// Group the exclusion reasons so the list stays readable at 128 bytes.
	if len(g.ExcludeReason) > 0 {
		byReason := map[string][]uint16{}
		for _, a := range g.Excluded {
			if r, ok := g.ExcludeReason[a]; ok {
				byReason[r] = append(byReason[r], a)
			}
		}
		reasons := make([]string, 0, len(byReason))
		for r := range byReason {
			reasons = append(reasons, r)
		}
		sort.Strings(reasons)
		for _, r := range reasons {
			fmt.Fprintf(&b, "      - %s: %d bytes\n", r, len(byReason[r]))
		}
	}
	return b.String()
}

// formatAddrs renders an ascending address list as compact $XX ranges.
func formatAddrs(a []uint16) string {
	if len(a) == 0 {
		return "(none)"
	}
	var parts []string
	start, prev := a[0], a[0]
	flush := func() {
		if start == prev {
			parts = append(parts, fmt.Sprintf("$%02X", start))
		} else {
			parts = append(parts, fmt.Sprintf("$%02X-$%02X", start, prev))
		}
	}
	for _, x := range a[1:] {
		if x == prev+1 {
			prev = x
			continue
		}
		flush()
		start, prev = x, x
	}
	flush()
	return strings.Join(parts, " ")
}
