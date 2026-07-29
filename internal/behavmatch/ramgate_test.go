package behavmatch

import (
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// romAnim is a self-animating litmus ROM from THIS repo (the harness must not
// depend on any game repo — CLAUDE.md).
const romAnim = "../../roms/litmus/motion_glide.bin"

// synth builds a trace whose RAM is fully determined: byte $80+i on frame f is
// base[i] + f*delta[i]. Deterministic input to the gate, so a failure is the
// gate's fault and never the emulator's.
func synth(name string, frames int, base, delta [emu.RAMSize]uint8, sp uint8) *Trace {
	tr := &Trace{Scenario: name}
	for f := 0; f < frames; f++ {
		var s Sample
		for i := 0; i < emu.RAMSize; i++ {
			s.AllRAM[i] = base[i] + uint8(f)*delta[i]
		}
		s.SP = sp
		tr.Samples = append(tr.Samples, s)
	}
	return tr
}

func liveBases() (base, delta [emu.RAMSize]uint8) {
	for i := range base {
		base[i] = uint8(i)
	}
	// Only $80..$8F move; the rest are constant, so LiveMask should keep exactly
	// those (minus whatever the stack claims).
	for i := 0; i < 16; i++ {
		delta[i] = 1
	}
	return
}

// A trace compared against itself must never diverge. If this fails, every green
// the gate ever gives is worthless — false positives would be indistinguishable
// from real findings.
func TestGateSelfComparisonIsClean(t *testing.T) {
	base, delta := liveBases()
	tr := synth("self", 40, base, delta, 0xFF)
	g := GateRAM(tr, tr, FullMask())
	if !g.Pass() {
		t.Fatalf("self-comparison diverged: %v", g.First)
	}
	if g.FramesCompared != 40 {
		t.Errorf("FramesCompared=%d want 40", g.FramesCompared)
	}
	if len(g.Compared) != emu.RAMSize {
		t.Errorf("FullMask compared %d bytes, want %d", len(g.Compared), emu.RAMSize)
	}
}

// A planted discrepancy must be reported at exactly the frame and address where
// it was planted — the whole value of the gate is that it names the byte and the
// frame, not merely that something is wrong.
func TestGateFindsPlantedDivergenceExactly(t *testing.T) {
	base, delta := liveBases()
	a := synth("planted", 30, base, delta, 0xFF)
	b := synth("planted", 30, base, delta, 0xFF)
	b.Samples[17].AllRAM[0x8A-emu.RAMBase] ^= 0x40

	g := GateRAM(a, b, FullMask())
	if g.Pass() {
		t.Fatal("planted divergence not detected")
	}
	if g.First.Frame != 17 || g.First.Addr != 0x8A {
		t.Errorf("first divergence = frame %d $%02X, want frame 17 $8A", g.First.Frame, g.First.Addr)
	}
	want := a.Samples[17].AllRAM[0x8A-emu.RAMBase]
	if g.First.TargetVal != want || g.First.MineVal != want^0x40 {
		t.Errorf("values = target %02X mine %02X, want %02X / %02X",
			g.First.TargetVal, g.First.MineVal, want, want^0x40)
	}
}

// "First" must mean earliest frame, then lowest address — otherwise the reported
// address is an arbitrary one of several and points at the wrong rule to fix.
func TestGateFirstIsEarliestFrameThenLowestAddr(t *testing.T) {
	base, delta := liveBases()
	a := synth("order", 30, base, delta, 0xFF)
	b := synth("order", 30, base, delta, 0xFF)
	b.Samples[20].AllRAM[0x81-emu.RAMBase] ^= 0xFF // later frame, lower addr
	b.Samples[12].AllRAM[0x95-emu.RAMBase] ^= 0xFF // earlier frame, higher addr
	b.Samples[12].AllRAM[0x90-emu.RAMBase] ^= 0xFF // earlier frame, lowest of that frame

	g := GateRAM(a, b, FullMask())
	if g.Pass() {
		t.Fatal("no divergence found")
	}
	if g.First.Frame != 12 || g.First.Addr != 0x90 {
		t.Errorf("first = frame %d $%02X, want frame 12 $90", g.First.Frame, g.First.Addr)
	}
}

// A masked byte must really be excluded — and the verdict must SAY it was, with
// a reason. A silent mask turns a hidden bug into a green.
func TestGateMaskExcludesAndReportsWhy(t *testing.T) {
	base, delta := liveBases()
	a := synth("masked", 30, base, delta, 0xFF)
	b := synth("masked", 30, base, delta, 0xFF)
	b.Samples[5].AllRAM[0xC0-emu.RAMBase] ^= 0xFF

	full := GateRAM(a, b, FullMask())
	if full.Pass() {
		t.Fatal("premise broken: full mask should see the planted divergence")
	}

	m := MaskFromAddrs([]uint16{0x80, 0x81, 0x82})
	g := GateRAM(a, b, m)
	if !g.Pass() {
		t.Errorf("masked-out byte still reported: %v", g.First)
	}
	if len(g.Compared) != 3 {
		t.Errorf("compared %d bytes, want 3", len(g.Compared))
	}
	if r := g.ExcludeReason[0xC0]; r == "" {
		t.Error("excluded byte $C0 carries no reason")
	}
	// The verdict must show the exclusion. Addresses are collapsed into ranges so
	// 125 excluded bytes stay readable, so $C0 appears inside "$83-$FF".
	out := g.String()
	for _, want := range []string{"compared: $80-$82", "excluded: $83-$FF", "not in the requested address set"} {
		if !strings.Contains(out, want) {
			t.Errorf("verdict text is missing %q:\n%s", want, out)
		}
	}
}

// LiveMask must be derived from the measurement: keep what moved, drop what
// never moved, and drop what the stack reached.
func TestLiveMaskKeepsMovedDropsConstantAndStack(t *testing.T) {
	base, delta := liveBases()
	tr := synth("live", 30, base, delta, 0xF0) // stack observed down to $F0

	m := LiveMask(tr)
	for a := uint16(0x80); a <= 0x8F; a++ {
		if !m.Has(a) {
			t.Errorf("$%02X changed every frame but was excluded", a)
		}
	}
	for _, a := range []uint16{0x90, 0xA0, 0xEF} {
		if m.Has(a) {
			t.Errorf("$%02X never changed but was included", a)
		}
	}
	if low, ok := StackReach(tr); !ok || low != 0xF0 {
		t.Errorf("StackReach = $%02X ok=%v, want $F0 true", low, ok)
	}
	// A byte that both moved AND sits under the stack must be dropped, with the
	// stack cited as the reason rather than "constant".
	base2, delta2 := liveBases()
	delta2[0xF5-emu.RAMBase] = 3
	tr2 := synth("live2", 30, base2, delta2, 0xF0)
	m2 := LiveMask(tr2)
	if m2.Has(0xF5) {
		t.Error("$F5 is inside the stack's reach but was included")
	}
	if r := m2.Reason[0xF5]; !strings.Contains(r, "stack") {
		t.Errorf("$F5 exclusion reason = %q, want it to cite the stack", r)
	}
}

// A build that dies early must not pass by being short.
func TestGateReportsLengthMismatch(t *testing.T) {
	base, delta := liveBases()
	a := synth("len", 30, base, delta, 0xFF)
	b := synth("len", 10, base, delta, 0xFF)

	g := GateRAM(a, b, FullMask())
	if g.FramesCompared != 10 || g.TargetFrames != 30 || g.MineFrames != 10 {
		t.Errorf("frames: compared=%d target=%d mine=%d", g.FramesCompared, g.TargetFrames, g.MineFrames)
	}
	if !strings.Contains(g.String(), "LENGTH DIFF") {
		t.Errorf("length mismatch not surfaced:\n%s", g.String())
	}
}

// A pass over nothing must announce itself as vacuous, so an empty mask can
// never read as evidence.
func TestGateVacuousPassIsLabelled(t *testing.T) {
	base, delta := liveBases()
	a := synth("vac", 30, base, delta, 0xFF)
	g := GateRAM(a, a, NewMask())
	if !g.Pass() {
		t.Fatal("empty mask should not diverge")
	}
	if !strings.Contains(g.String(), "VACUOUS") {
		t.Errorf("empty-mask verdict is not labelled vacuous:\n%s", g.String())
	}
}

// End to end on a real ROM: recording the same ROM twice under the same scenario
// must produce byte-identical RAM on every one of the 128 addresses. This is the
// determinism the whole reproduction system rests on — if the emulator were not
// reproducible frame-for-frame, no divergence could ever be attributed to the build.
func TestRecordIsDeterministicOnRealROM(t *testing.T) {
	scn := Scenario{
		Name: "det", Frames: 25, Reset: true,
		At:      map[int][]InputChange{3: {{Action: "right", Player: 0, Press: true}}},
		Objects: []int{0},
	}
	a, err := Record(romAnim, "NTSC", scn, 0)
	if err != nil {
		t.Skipf("ROM unavailable (%s): %v", romAnim, err)
	}
	b, err := Record(romAnim, "NTSC", scn, 0)
	if err != nil {
		t.Fatal(err)
	}
	g := GateRAM(a, b, FullMask())
	if !g.Pass() {
		t.Errorf("same ROM recorded twice diverged: %v", g.First)
	}
	if g.FramesCompared != scn.Frames {
		t.Errorf("compared %d frames, want %d", g.FramesCompared, scn.Frames)
	}
}

// The input level must be reconstructed from the scenario's press/release edges,
// because a byte's transition function is fitted against what was HELD, not
// against the edge that started it.
func TestRecordTracksHeldInputLevel(t *testing.T) {
	scn := Scenario{
		Name: "held", Frames: 12, Reset: true,
		At: map[int][]InputChange{
			2: {{Action: "right", Player: 0, Press: true}},
			5: {{Action: "fire", Player: 0, Press: true}},
			8: {{Action: "right", Player: 0, Press: false}},
		},
		Objects: []int{0},
	}
	tr, err := Record(romAnim, "NTSC", scn, 0)
	if err != nil {
		t.Skipf("ROM unavailable (%s): %v", romAnim, err)
	}
	if got := tr.Samples[1].Inputs.Key(); got != "||" {
		t.Errorf("frame 1 inputs = %q, want empty", got)
	}
	if got := tr.Samples[3].Inputs.P0; len(got) != 1 || got[0] != "right" {
		t.Errorf("frame 3 P0 = %v, want [right]", got)
	}
	if got := tr.Samples[6].Inputs.P0; len(got) != 2 || got[0] != "fire" || got[1] != "right" {
		t.Errorf("frame 6 P0 = %v, want [fire right] (sorted, both held)", got)
	}
	if got := tr.Samples[9].Inputs.P0; len(got) != 1 || got[0] != "fire" {
		t.Errorf("frame 9 P0 = %v, want [fire] (right released at 8)", got)
	}
}
