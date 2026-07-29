// Package behavmatch does behavioral reproduction diffing: it drives a target
// ROM and your build through the SAME scripted input scenarios, records every
// object's per-frame trajectory (X/Y/height/present) and chosen RAM bytes, then
// compares them so a mechanic (movement speed, clamp, fire→freeze coupling,
// ricochet) that differs shows up as a numeric diff instead of a hand-played
// hunch. It is the behavioral sibling of package vismatch.
package behavmatch

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// InputChange is a controller/panel edit applied before a given frame steps.
// Exactly one of Panel or Action is set. Panel ∈ reset/select/…; Action ∈
// up/down/left/right/fire/center.
type InputChange struct {
	Panel  string
	Action string
	Player int
	Press  bool
}

// Scenario is a deterministic input script plus what to record. At frame f, any
// InputChanges in At[f] are applied, then one frame is stepped and sampled.
type Scenario struct {
	Name    string
	Frames  int                   // total frames to run & sample
	Reset   bool                  // press RESET (8f) then release before frame 0
	At      map[int][]InputChange // frame index → input edits applied before that frame
	Objects []int                 // object idxs to trace: 0=P0 1=M0 2=P1 3=M1 4=BL
	RAM     []uint16              // RAM addrs ($80-$FF) to trace
}

var objName = [...]string{"P0", "M0", "P1", "M1", "BL"}

// InputState is the controller/panel state in force when a frame was sampled —
// the *input half* of the (state, input) → state' transition the reproduction
// system is trying to recover. Recording it alongside RAM is what lets a byte's
// update function be fitted against what the player was doing, rather than
// against frame number alone. Slices are kept sorted so equal states compare and
// key equal.
type InputState struct {
	P0    []string `json:"p0"`    // actions held by player 0 (up/down/left/right/fire)
	P1    []string `json:"p1"`    // actions held by player 1
	Panel []string `json:"panel"` // console switches held (reset/select/…)
}

// Key is a canonical, comparable rendering of the input state, usable as a map
// key when grouping transitions by input.
func (s InputState) Key() string {
	return strings.Join(s.P0, "+") + "|" + strings.Join(s.P1, "+") + "|" + strings.Join(s.Panel, "+")
}

// Sample is one frame's observation.
type Sample struct {
	X, YTop, Height map[int]int
	Present         map[int]bool
	RAM             map[uint16]uint8 // the scenario's declared subset (reports)

	// AllRAM is every one of the 128 RAM bytes ($80–$FF), indexed addr−$80.
	// Recorded unconditionally: which bytes carry meaning is the thing being
	// measured, so it cannot be declared in advance.
	AllRAM [emu.RAMSize]uint8

	// Coll is every collision that OCCURRED during the frame, not merely what was
	// still latched at its end. Cross-frame hardware state is invisible to a
	// RAM-only sampler, so a byte driven by a collision would otherwise show an
	// unexplainable residual — and recording it is also what lets "this game does
	// not use collision detection" be *proven* rather than assumed. Reading CXxx
	// at the frame boundary would not do: games clear the latches every frame, so
	// the evidence is gone by the time the boundary arrives.
	Coll emu.Collisions

	// Inputs is what was held when this frame stepped.
	Inputs InputState

	// SPLow/SPHigh bound where the stack pointer travelled during the frame.
	// Sampling SP at the frame boundary instead gives $FF on essentially every
	// frame, which says nothing at all; and the LOW value alone cannot distinguish
	// "pinned low" from "descended from the top", which turned out to matter — see
	// SPRange in ramgate.go for what this can and cannot be used to conclude.
	SPLow, SPHigh uint8
}

// Trace is a scenario's full recording.
type Trace struct {
	Scenario string
	Samples  []Sample
}

// Record runs scn on romPath and returns the trace. reset from the scenario is
// honoured (originals usually need RESET to start). warmup runs that many frames
// with no scripted input BEFORE frame 0 of the scenario — essential for a game
// whose title screen auto-advances to gameplay (otherwise the scripted inputs
// land on the title, not the game). warmup is per-side (target vs mine differ).
func Record(romPath, spec string, scn Scenario, warmup int) (*Trace, error) {
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, fmt.Errorf("load %s: %w", romPath, err)
	}
	if scn.Reset {
		_ = e.SetPanel("reset", true)
		if err := e.RunFrames(8); err != nil {
			return nil, err
		}
		_ = e.SetPanel("reset", false)
		if err := e.RunFrames(1); err != nil {
			return nil, err
		}
	}
	if warmup > 0 {
		if err := e.RunFrames(warmup); err != nil {
			return nil, err
		}
	}
	tr := &Trace{Scenario: scn.Name}
	held := newHeldInputs()
	for f := 0; f < scn.Frames; f++ {
		for _, ic := range scn.At[f] {
			if ic.Panel != "" {
				if err := e.SetPanel(ic.Panel, ic.Press); err != nil {
					return nil, err
				}
				held.set("panel", 0, ic.Panel, ic.Press)
			} else if ic.Action != "" {
				if err := e.SetInput(ic.Player, ic.Action, ic.Press); err != nil {
					return nil, err
				}
				held.set("action", ic.Player, ic.Action, ic.Press)
			}
		}
		e.StartFrameWatch() // collisions + stack depth are only visible INSIDE the frame
		if _, err := e.StepFrame(); err != nil {
			return nil, err
		}
		s := Sample{X: map[int]int{}, YTop: map[int]int{}, Height: map[int]int{},
			Present: map[int]bool{}, RAM: map[uint16]uint8{}, Inputs: held.snapshot()}
		for _, idx := range scn.Objects {
			yTop, _, h, present := e.ObjectYExtent(idx)
			s.Present[idx] = present
			if present {
				s.YTop[idx] = yTop
				s.Height[idx] = h
			}
			s.X[idx] = e.Markers()[idx].Clock
		}
		ram, err := e.CurrentRAM()
		if err != nil {
			return nil, fmt.Errorf("frame %d: %w", f, err)
		}
		s.AllRAM = ram
		for _, a := range scn.RAM {
			if a >= emu.RAMBase && a < emu.RAMBase+emu.RAMSize {
				s.RAM[a] = ram[a-emu.RAMBase] // same read, no second peek
			} else if v, err := e.PeekRAM(a); err == nil {
				s.RAM[a] = v
			}
		}
		s.Coll, _ = e.FrameWatch()
		s.SPLow, s.SPHigh = e.FrameWatchSPRange()
		tr.Samples = append(tr.Samples, s)
	}
	return tr, nil
}

// heldInputs tracks which controller actions / panel switches are currently
// pressed, so each sample can carry the input that produced it. The scenario
// script only ever states *edges* (press/release), so the level has to be
// accumulated here.
type heldInputs struct {
	actions [2]map[string]bool
	panel   map[string]bool
}

func newHeldInputs() *heldInputs {
	return &heldInputs{
		actions: [2]map[string]bool{{}, {}},
		panel:   map[string]bool{},
	}
}

func (h *heldInputs) set(kind string, player int, name string, pressed bool) {
	var m map[string]bool
	switch {
	case kind == "panel":
		m = h.panel
	case player >= 0 && player < len(h.actions):
		m = h.actions[player]
	default:
		return
	}
	// "center" is a release of every direction, not a button of its own.
	if name == "center" {
		for _, d := range []string{"up", "down", "left", "right"} {
			delete(m, d)
		}
		return
	}
	if pressed {
		m[name] = true
	} else {
		delete(m, name)
	}
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (h *heldInputs) snapshot() InputState {
	return InputState{
		P0:    sortedKeys(h.actions[0]),
		P1:    sortedKeys(h.actions[1]),
		Panel: sortedKeys(h.panel),
	}
}

// --- metrics ---------------------------------------------------------------

// AxisMetric summarises one object's motion on one axis across a trace.
type AxisMetric struct {
	Object      int
	Axis        string  // "X" or "Y"
	Min, Max    int     // range reached
	NetDelta    int     // last - first
	StepPxPer2F float64 // median |Δ| per frame doubled (≈ px per 2 frames); 0 = frozen
	FramesMoved int
}

func medianAbsNonzero(deltas []int) float64 {
	var a []int
	for _, d := range deltas {
		if d != 0 {
			if d < 0 {
				d = -d
			}
			a = append(a, d)
		}
	}
	if len(a) == 0 {
		return 0
	}
	sort.Ints(a)
	return float64(a[len(a)/2])
}

// AxisMetrics derives per-object X and Y motion summaries from a trace.
func AxisMetrics(tr *Trace, objects []int) []AxisMetric {
	var out []AxisMetric
	for _, idx := range objects {
		for _, axis := range []string{"X", "Y"} {
			var series []int
			for _, s := range tr.Samples {
				switch axis {
				case "X":
					series = append(series, s.X[idx])
				case "Y":
					if s.Present[idx] {
						series = append(series, s.YTop[idx])
					} else if len(series) > 0 {
						series = append(series, series[len(series)-1])
					} else {
						series = append(series, 0)
					}
				}
			}
			if len(series) == 0 {
				continue
			}
			mn, mx := series[0], series[0]
			var deltas []int
			moved := 0
			for i, v := range series {
				if v < mn {
					mn = v
				}
				if v > mx {
					mx = v
				}
				if i > 0 {
					d := v - series[i-1]
					deltas = append(deltas, d)
					if d != 0 {
						moved++
					}
				}
			}
			out = append(out, AxisMetric{Object: idx, Axis: axis, Min: mn, Max: mx,
				NetDelta: series[len(series)-1] - series[0], StepPxPer2F: medianAbsNonzero(deltas), FramesMoved: moved})
		}
	}
	return out
}

// --- diff ------------------------------------------------------------------

// Diff is a behavioral comparison of a target vs a mine trace for one scenario.
type Diff struct {
	Scenario string
	Lines    []string // human-readable metric comparisons
	Match    bool
}

// CompareTraces compares two traces (same scenario) and reports per-object axis
// metric differences plus a frozen-frames (freeze mechanic) comparison. tol is
// the tolerance for "same" on speed/range (in px).
func CompareTraces(target, mine *Trace, objects []int, tol float64) *Diff {
	d := &Diff{Scenario: target.Scenario, Match: true}
	tm := indexMetrics(AxisMetrics(target, objects))
	mm := indexMetrics(AxisMetrics(mine, objects))
	for _, idx := range objects {
		for _, axis := range []string{"X", "Y"} {
			key := fmt.Sprintf("%d/%s", idx, axis)
			a, ok1 := tm[key]
			b, ok2 := mm[key]
			if !ok1 || !ok2 {
				continue
			}
			// Separate the MECHANIC (speed + travelled span) from CALIBRATION
			// (absolute rest position). A 3px start offset with identical speed
			// is a position note, not a mechanic divergence.
			spanA, spanB := a.Max-a.Min, b.Max-b.Min
			mechOK := math.Abs(a.StepPxPer2F-b.StepPxPer2F) <= tol && absInt(spanA-spanB) <= int(tol)+1
			posOff := b.Min - a.Min
			mark := "mechanic ok"
			if !mechOK {
				mark = "**MECHANIC DIFF**"
				d.Match = false
			}
			note := ""
			if absInt(posOff) > int(tol)+1 {
				note = fmt.Sprintf("  [pos offset %+d]", posOff)
			}
			d.Lines = append(d.Lines, fmt.Sprintf(
				"  %-2s %s: target[%d..%d span%d step≈%.1f]  mine[%d..%d span%d step≈%.1f]  %s%s",
				objName[idx], axis, a.Min, a.Max, spanA, a.StepPxPer2F,
				b.Min, b.Max, spanB, b.StepPxPer2F, mark, note))
		}
	}
	return d
}

func indexMetrics(ms []AxisMetric) map[string]AxisMetric {
	m := map[string]AxisMetric{}
	for _, x := range ms {
		m[fmt.Sprintf("%d/%s", x.Object, x.Axis)] = x
	}
	return m
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// FreezeReport measures, for a scenario where a direction is held while a bullet
// (missile idx) is fired, whether the shooter (player idx) is frozen while the
// bullet is present — i.e. the "no Getaway" freeze==bullet-lifetime coupling.
// Returns frames the bullet was present and frames the shooter's X was static
// while the bullet was present.
func FreezeReport(tr *Trace, player, missile int) (bulletFrames, frozenWhileBullet, movedWhileBullet int) {
	for i := 1; i < len(tr.Samples); i++ {
		if !tr.Samples[i].Present[missile] {
			continue
		}
		bulletFrames++
		if tr.Samples[i].X[player] == tr.Samples[i-1].X[player] {
			frozenWhileBullet++
		} else {
			movedWhileBullet++
		}
	}
	return
}

// FormatTraceX is a compact debug dump of one object's X series.
func FormatTraceX(tr *Trace, idx int) string {
	var b strings.Builder
	for i, s := range tr.Samples {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d", s.X[idx])
	}
	return b.String()
}
