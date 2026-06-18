package oracle

import "github.com/kidsnz/atari2600-harness/internal/emu"

// Gopher is the embedded Gopher2600 oracle — fully automated, no external
// dependency, the in-tree reference member of the vote.
type Gopher struct{ Spec string } // TV spec; "" = NTSC

func (g Gopher) Name() string {
	if g.Spec != "" {
		return "gopher2600/" + g.Spec
	}
	return "gopher2600"
}

// DumpRAM runs the ROM from power-on for `frames` frames and returns RAM $80-$FF.
func (g Gopher) DumpRAM(romPath string, frames int) (RAMDump, error) {
	spec := g.Spec
	if spec == "" {
		spec = "NTSC"
	}
	var d RAMDump
	e, err := emu.New(spec)
	if err != nil {
		return d, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return d, err
	}
	if err := e.RunFrames(frames); err != nil {
		return d, err
	}
	for i := 0; i < 128; i++ {
		v, err := e.PeekRAM(uint16(0x80 + i))
		if err != nil {
			return d, err
		}
		d[i] = v
	}
	return d, nil
}
