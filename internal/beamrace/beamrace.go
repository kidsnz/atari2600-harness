// Package beamrace interprets, soundly, whether an object's graphics reached the
// beam in time. It is the intent-aware sibling of the beamtrace timeline: a write
// to an object's pixel-data register (GRP0/GRP1/ENAM0/ENAM1/ENABL) that lands at
// beam clock C while the object sits at X is "in time" iff C ≤ X — otherwise the
// object's pixels on that line drew with the PREVIOUS value (a one-line lag).
//
// IMPORTANT — why there is no fully-automatic verdict here. Whether a late write is
// a *bug* depends on author intent: the very same late `sta GRP0` is correct when
// it pre-loads the NEXT line, and wrong when it was meant for THIS line. The tool
// cannot read that intent. So beamrace ships two sound things and NOT a blanket
// detector:
//   - Events / Report: a FACTUAL advisory (per object, write clock vs object X,
//     in-time or late). No verdict ⇒ it cannot false-positive.
//   - Check: a verdict the author OPTS INTO — "object O must be updated before the
//     beam on scanlines A..B" — which is sound because the deadline (intent) is
//     supplied. It generalises the hardware-fixed VV-10 HMOVE-hazard gate.
//
// A fully-automatic heuristic detector (guess the intent, accept rare false
// positives) was deliberately deferred, not closed — see docs/capability-gap-audit
// (AT-3). Revisit if a concrete need appears.
package beamrace

import (
	"fmt"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// regToObject maps a pixel-data write-register to the object it draws.
var regToObject = map[uint16]string{
	0x1B: "P0", // GRP0
	0x1C: "P1", // GRP1
	0x1D: "M0", // ENAM0
	0x1E: "M1", // ENAM1
	0x1F: "BL", // ENABL
}

// Event is one pixel-data write located against its object's position.
type Event struct {
	Frame      int    `json:"frame"`
	Scanline   int    `json:"scanline"`
	Clock      int    `json:"clock"`    // beam clock the write lands at (-68..159)
	Object     string `json:"object"`   // P0/P1/M0/M1/BL
	Reg        string `json:"reg"`      // GRP0/GRP1/ENAM0/ENAM1/ENABL
	ObjectX    int    `json:"object_x"` // object's HmovedPixel at the write (0..159)
	Value      uint8  `json:"value"`
	HasValue   bool   `json:"has_value"`
	PC         uint16 `json:"pc"`
	BeforeBeam bool   `json:"before_beam"` // Clock <= ObjectX: the write reached the object in time
}

var regName = map[uint16]string{0x1B: "GRP0", 0x1C: "GRP1", 0x1D: "ENAM0", 0x1E: "ENAM1", 0x1F: "ENABL"}

// Trace runs the loaded ROM forward `frames` whole frames and records, for every
// write to a pixel-data register, the controlled object's position at that moment
// and whether the write reached the beam in time. The emulator is left advanced.
func Trace(e *emu.Emu, frames int) ([]Event, error) {
	if frames < 1 {
		frames = 1
	}
	start := e.Coords().Frame
	var out []Event
	for e.Coords().Frame-start < frames {
		if err := e.StepInstruction(); err != nil {
			return out, err
		}
		tw, ok := e.LastTIAWrite()
		if !ok {
			continue
		}
		obj, ok := regToObject[tw.Reg]
		if !ok {
			continue
		}
		c := e.Coords()
		x, _ := e.ObjectX(obj)
		out = append(out, Event{
			Frame: c.Frame, Scanline: c.Scanline, Clock: c.Clock,
			Object: obj, Reg: regName[tw.Reg], ObjectX: x,
			Value: tw.Val, HasValue: tw.HasVal, PC: tw.PC,
			BeforeBeam: c.Clock <= x, // HBLANK (clock<0) is always before the visible object
		})
	}
	return out, nil
}

// Violation is a write that missed its object's beam deadline under a Check.
type Violation struct {
	Event
	Detail string `json:"detail"`
}

// Check is the author-supplied deadline: object O's pixel data must be written
// before the beam reaches it on every scanline in [LineFrom, LineTo]. It returns
// the writes that landed late (Clock > ObjectX). Sound because the intent (which
// object, which lines must be updated in time) is supplied, not guessed.
type Check struct {
	Object   string
	LineFrom int
	LineTo   int
}

// Eval returns the late writes among events that fall under the check. Pure.
func (c Check) Eval(events []Event) []Violation {
	var v []Violation
	for _, e := range events {
		if e.Object != c.Object || e.Scanline < c.LineFrom || e.Scanline > c.LineTo {
			continue
		}
		if !e.BeforeBeam {
			v = append(v, Violation{Event: e, Detail: fmt.Sprintf(
				"%s written at clk %d but %s is at X=%d (write is %d clk too late; this line draws the previous value)",
				e.Reg, e.Clock, e.Object, e.ObjectX, e.Clock-e.ObjectX)})
		}
	}
	return v
}

// ByObject groups events by object (sorted P0,P1,M0,M1,BL) for the advisory report.
func ByObject(events []Event) map[string][]Event {
	m := map[string][]Event{}
	for _, e := range events {
		m[e.Object] = append(m[e.Object], e)
	}
	for k := range m {
		sort.SliceStable(m[k], func(i, j int) bool {
			if m[k][i].Scanline != m[k][j].Scanline {
				return m[k][i].Scanline < m[k][j].Scanline
			}
			return m[k][i].Clock < m[k][j].Clock
		})
	}
	return m
}
