package emu

import (
	"testing"
	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
)

func TestZZZCXUse(t *testing.T) {
	e, err := New("NTSC")
	if err != nil { t.Fatal(err) }
	if err := e.LoadROM("/Users/shinji/Documents/2D/260609_atari2600-dev/sandbox/studies/outlaw/Outlaw.bin"); err != nil { t.Skip(err) }
	_ = e.SetPanel("reset", true); _ = e.RunFrames(8); _ = e.SetPanel("reset", false)
	_ = e.RunFrames(60)
	_ = e.SetInput(0, "fire", true)
	cxclr, cxread := 0, 0
	modifyRAM := map[uint16]int{}
	start := e.Coords().Frame
	for e.Coords().Frame-start < 20 {
		if err := e.StepInstruction(); err != nil { t.Fatal(err) }
		lr := e.VCS.CPU.LastResult
		if lr.Defn == nil { continue }
		if lr.Defn.Effect == instructions.Write {
			canon, area := memorymap.MapAddress(lr.InstructionData, false)
			if area == memorymap.TIA && canon == 0x2C { cxclr++ }
		}
		if lr.Defn.Effect == instructions.Read {
			canon, area := memorymap.MapAddress(lr.InstructionData, true)
			if area == memorymap.TIA && canon >= 0x30 && canon <= 0x37 { cxread++ }
		}
		if lr.Defn.Effect == instructions.Modify {
			a, ok := e.effectiveAddr()
			if ok {
				canon, area := memorymap.MapAddress(a, false)
				if area == memorymap.RAM { modifyRAM[canon]++ }
			}
		}
	}
	t.Logf("over 20 frames: CXCLR strobes=%d, CX register reads=%d", cxclr, cxread)
	t.Logf("RMW (INC/DEC/ASL/...) writes to RAM, by addr: %v", modifyRAM)
}
