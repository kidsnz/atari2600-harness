// Package beamtrace builds a write→visible-pixel timeline (authoring aid). It runs
// a ROM instruction-by-instruction, records every TIA write together with the beam
// position where it lands, and tabulates, per scanline, which visible pixels each
// write governs. It answers "this `sta GRP0` lands at beam clock X, so it paints
// pixels X..Y" — the causal map the runtime tools (trace_clocks/read_row) only show
// piecemeal. It states only what is sound: a write at clock C can affect a pixel
// only if that pixel is rendered at clock ≥ C, and a later write to the SAME
// register supersedes it from that point — so the governed span is [C, next-same-
// reg-write). Interpreting whether a write is *too late* is the beam-race detector's
// job (T3); here we just lay out the facts.
package beamtrace

import (
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Kind classifies a TIA write-register by what it controls (so the author can read
// the timeline without memorising the address map).
type Kind string

const (
	KindColor    Kind = "color"    // COLUP0/1, COLUPF, COLUBK
	KindGraphics Kind = "graphics" // GRP0/1, PF0/1/2, ENAM0/1, ENABL
	KindPosition Kind = "position" // RESP0/1, RESM0/1, RESBL (strobe = sets X here)
	KindMotion   Kind = "motion"   // HMP0/1, HMM0/1, HMBL, HMOVE, HMCLR
	KindControl  Kind = "control"  // NUSIZ, CTRLPF, REFP, VDEL, RESMP, VBLANK
	KindAudio    Kind = "audio"    // AUDC/AUDF/AUDV
	KindStrobe   Kind = "strobe"   // WSYNC, RSYNC, VSYNC, CXCLR
	KindOther    Kind = "other"
)

type regMeta struct {
	name string
	kind Kind
}

// regTable maps each canonical TIA write-register address ($00-$2C) to its name and
// kind. Source: Stella Programmer's Guide / docs/resources.md.
var regTable = map[uint16]regMeta{
	0x00: {"VSYNC", KindStrobe}, 0x01: {"VBLANK", KindControl}, 0x02: {"WSYNC", KindStrobe},
	0x03: {"RSYNC", KindStrobe}, 0x04: {"NUSIZ0", KindControl}, 0x05: {"NUSIZ1", KindControl},
	0x06: {"COLUP0", KindColor}, 0x07: {"COLUP1", KindColor}, 0x08: {"COLUPF", KindColor},
	0x09: {"COLUBK", KindColor}, 0x0A: {"CTRLPF", KindControl}, 0x0B: {"REFP0", KindControl},
	0x0C: {"REFP1", KindControl}, 0x0D: {"PF0", KindGraphics}, 0x0E: {"PF1", KindGraphics},
	0x0F: {"PF2", KindGraphics}, 0x10: {"RESP0", KindPosition}, 0x11: {"RESP1", KindPosition},
	0x12: {"RESM0", KindPosition}, 0x13: {"RESM1", KindPosition}, 0x14: {"RESBL", KindPosition},
	0x15: {"AUDC0", KindAudio}, 0x16: {"AUDC1", KindAudio}, 0x17: {"AUDF0", KindAudio},
	0x18: {"AUDF1", KindAudio}, 0x19: {"AUDV0", KindAudio}, 0x1A: {"AUDV1", KindAudio},
	0x1B: {"GRP0", KindGraphics}, 0x1C: {"GRP1", KindGraphics}, 0x1D: {"ENAM0", KindGraphics},
	0x1E: {"ENAM1", KindGraphics}, 0x1F: {"ENABL", KindGraphics}, 0x20: {"HMP0", KindMotion},
	0x21: {"HMP1", KindMotion}, 0x22: {"HMM0", KindMotion}, 0x23: {"HMM1", KindMotion},
	0x24: {"HMBL", KindMotion}, 0x25: {"VDELP0", KindControl}, 0x26: {"VDELP1", KindControl},
	0x27: {"VDELBL", KindControl}, 0x28: {"RESMP0", KindControl}, 0x29: {"RESMP1", KindControl},
	0x2A: {"HMOVE", KindMotion}, 0x2B: {"HMCLR", KindMotion}, 0x2C: {"CXCLR", KindStrobe},
}

// RegInfo returns the name and kind for a canonical TIA write-register address.
func RegInfo(reg uint16) (name string, kind Kind) {
	if m, ok := regTable[reg]; ok {
		return m.name, m.kind
	}
	return "?", KindOther
}

// valueIrrelevant reports whether a register is a pure strobe — any write triggers
// it and the byte written carries no meaning (so we don't report a value). VSYNC and
// VBLANK are NOT strobes (their bits matter), nor are HMxx (they hold a nibble).
func valueIrrelevant(reg uint16) bool {
	switch reg {
	case 0x02, 0x03, // WSYNC, RSYNC
		0x10, 0x11, 0x12, 0x13, 0x14, // RESP0/1, RESM0/1, RESBL
		0x2A, 0x2B, 0x2C: // HMOVE, HMCLR, CXCLR
		return true
	}
	return false
}

// Write is one TIA write located on the beam. Clock follows the Gopher2600
// convention (HBLANK = -68..-1, visible = 0..159) and is the beam position right
// after the write completes — the first pixel the new value can affect.
type Write struct {
	Frame    int    `json:"frame"`
	Scanline int    `json:"scanline"`
	Clock    int    `json:"clock"`
	PC       uint16 `json:"pc"`
	Reg      uint16 `json:"reg"`
	Name     string `json:"name"`
	Kind     Kind   `json:"kind"`
	Value    uint8  `json:"value"`
	HasValue bool   `json:"has_value"`
}

// Trace runs the loaded ROM forward `frames` whole frames from the current position
// and returns every TIA write in beam order. The emulator is left advanced.
func Trace(e *emu.Emu, frames int) ([]Write, error) {
	if frames < 1 {
		frames = 1
	}
	start := e.Coords().Frame
	var out []Write
	for e.Coords().Frame-start < frames {
		if err := e.StepInstruction(); err != nil {
			return out, err
		}
		tw, ok := e.LastTIAWrite()
		if !ok {
			continue
		}
		c := e.Coords()
		name, kind := RegInfo(tw.Reg)
		hasVal := tw.HasVal && !valueIrrelevant(tw.Reg)
		out = append(out, Write{
			Frame: c.Frame, Scanline: c.Scanline, Clock: c.Clock,
			PC: tw.PC, Reg: tw.Reg, Name: name, Kind: kind,
			Value: tw.Val, HasValue: hasVal,
		})
	}
	return out, nil
}

// SpanWrite is a Write annotated with the visible-pixel span it governs on its
// scanline: [VisFrom, VisTo) in visible pixels (0..160). VisTo is the clock of the
// next write to the SAME register on the line, or 160 (end of the visible line).
type SpanWrite struct {
	Write
	VisFrom int `json:"vis_from"`
	VisTo   int `json:"vis_to"`
}

// Row is the write timeline for one (frame, scanline).
type Row struct {
	Frame    int         `json:"frame"`
	Scanline int         `json:"scanline"`
	Writes   []SpanWrite `json:"writes"`
}

func clampVisible(clk int) int {
	if clk < 0 {
		return 0 // a write during HBLANK governs from the first visible pixel
	}
	if clk > 160 {
		return 160
	}
	return clk
}

// Timeline selects the writes on (frame, scanline), orders them by beam clock, and
// computes each write's governed visible span. Pure: testable with synthetic writes.
func Timeline(ws []Write, frame, scanline int) Row {
	var sel []Write
	for _, w := range ws {
		if w.Frame == frame && w.Scanline == scanline {
			sel = append(sel, w)
		}
	}
	sort.SliceStable(sel, func(i, j int) bool { return sel[i].Clock < sel[j].Clock })

	out := make([]SpanWrite, len(sel))
	for i, w := range sel {
		from := clampVisible(w.Clock)
		to := 160
		for j := i + 1; j < len(sel); j++ {
			if sel[j].Reg == w.Reg { // next write to the same register supersedes
				to = clampVisible(sel[j].Clock)
				break
			}
		}
		if to < from {
			to = from
		}
		out[i] = SpanWrite{Write: w, VisFrom: from, VisTo: to}
	}
	return Row{Frame: frame, Scanline: scanline, Writes: out}
}

// Scanlines returns the sorted distinct scanlines that have writes in the given
// frame (so the CLI can print a full per-line table).
func Scanlines(ws []Write, frame int) []int {
	seen := map[int]bool{}
	var out []int
	for _, w := range ws {
		if w.Frame == frame && !seen[w.Scanline] {
			seen[w.Scanline] = true
			out = append(out, w.Scanline)
		}
	}
	sort.Ints(out)
	return out
}

// Frames returns the sorted distinct frame numbers present in the trace.
func Frames(ws []Write) []int {
	seen := map[int]bool{}
	var out []int
	for _, w := range ws {
		if !seen[w.Frame] {
			seen[w.Frame] = true
			out = append(out, w.Frame)
		}
	}
	sort.Ints(out)
	return out
}
