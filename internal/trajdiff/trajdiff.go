// Package trajdiff diffs the runtime BEHAVIOR of two ROMs (VV-8): it steps an
// "original" and an "authored/candidate" ROM in lockstep on the same input
// timeline and reports the first frame+field where their observable state
// diverges (or MATCH). This is the strongest oracle for a reproduction task —
// it compares behavior over time, not bytes, so a cosmetic/dead-byte difference
// is a MATCH while a real behavioral change is caught at the exact frame.
package trajdiff

import (
	"fmt"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/scenario"
)

// Divergence is the first point the two trajectories disagree.
type Divergence struct {
	Frame int    `json:"frame"`
	Field string `json:"field"`
	A     int64  `json:"a"` // value in romA
	B     int64  `json:"b"` // value in romB
}

// Result is the comparison outcome.
type Result struct {
	Match     bool        `json:"match"`
	Frames    int         `json:"frames"`    // frames actually compared (= full run on MATCH)
	Diverged  *Divergence `json:"diverged,omitempty"`
}

// Options configure a comparison.
type Options struct {
	Spec   string   // TV spec (default "NTSC")
	Frames int      // frames to compare (default 60)
	Warmup int      // frames to settle before comparing (applied to both)
	Fields []string // fields to compare each frame; empty = all 128 RAM bytes
	Held   []string // actions held on both ROMs every frame (cheap drive)
	Player int      // input port for Held
}

func load(spec, path string, warmup int) (*emu.Emu, error) {
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(path); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if warmup > 0 {
		if err := e.RunFrames(warmup); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// Compare steps romA and romB in lockstep and returns the first divergence.
func Compare(romA, romB string, o Options) (Result, error) {
	spec := o.Spec
	if spec == "" {
		spec = "NTSC"
	}
	frames := o.Frames
	if frames <= 0 {
		frames = 60
	}
	ea, err := load(spec, romA, o.Warmup)
	if err != nil {
		return Result{}, err
	}
	eb, err := load(spec, romB, o.Warmup)
	if err != nil {
		return Result{}, err
	}

	for f := 0; f < frames; f++ {
		for _, a := range o.Held {
			if err := ea.SetInput(o.Player, a, true); err != nil {
				return Result{}, err
			}
			if err := eb.SetInput(o.Player, a, true); err != nil {
				return Result{}, err
			}
		}
		if err := ea.RunFrames(1); err != nil {
			return Result{}, err
		}
		if err := eb.RunFrames(1); err != nil {
			return Result{}, err
		}
		if d, err := diffFrame(ea, eb, f, o.Fields); err != nil {
			return Result{}, err
		} else if d != nil {
			return Result{Match: false, Frames: f + 1, Diverged: d}, nil
		}
	}
	return Result{Match: true, Frames: frames}, nil
}

// diffFrame compares the chosen fields (default = the 128-byte RAM) at one frame.
func diffFrame(ea, eb *emu.Emu, f int, fields []string) (*Divergence, error) {
	if len(fields) == 0 {
		for addr := 0x80; addr <= 0xFF; addr++ {
			va, err := ea.PeekRAM(uint16(addr))
			if err != nil {
				return nil, err
			}
			vb, err := eb.PeekRAM(uint16(addr))
			if err != nil {
				return nil, err
			}
			if va != vb {
				return &Divergence{Frame: f, Field: fmt.Sprintf("ram.0x%02X", addr), A: int64(va), B: int64(vb)}, nil
			}
		}
		return nil, nil
	}
	for _, fld := range fields {
		va, err := scenario.ResolveField(ea, fld)
		if err != nil {
			return nil, err
		}
		vb, err := scenario.ResolveField(eb, fld)
		if err != nil {
			return nil, err
		}
		if va != vb {
			return &Divergence{Frame: f, Field: fld, A: va, B: vb}, nil
		}
	}
	return nil, nil
}
