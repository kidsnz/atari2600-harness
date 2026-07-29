package emu

import "testing"

func TestZZZRestoreInputProbe(t *testing.T) {
	e, err := New("NTSC")
	if err != nil { t.Fatal(err) }
	if err := e.LoadROM("/Users/shinji/Documents/2D/260609_atari2600-dev/sandbox/studies/outlaw/Outlaw.bin"); err != nil { t.Skip(err) }
	_ = e.SetPanel("reset", true)
	_ = e.RunFrames(8)
	_ = e.SetPanel("reset", false)
	_ = e.RunFrames(60)
	_ = e.SetInput(0, "right", true)
	_ = e.SetInput(0, "fire", true)
	_ = e.RunFrames(5)
	sw, _ := e.PeekRAM(0x0280)
	in4, _ := e.PeekRAM(0x000C)
	t.Logf("BEFORE save: SWCHA=%02X INPT4=%02X", sw, in4)
	st := e.SaveState()
	_ = e.RunFrames(3)
	if err := e.RestoreState(st); err != nil { t.Fatal(err) }
	sw2, _ := e.PeekRAM(0x0280)
	in42, _ := e.PeekRAM(0x000C)
	t.Logf("AFTER restore: SWCHA=%02X INPT4=%02X", sw2, in42)
	if sw != sw2 || in4 != in42 { t.Errorf("INPUT STATE NOT RESTORED: SWCHA %02X->%02X INPT4 %02X->%02X", sw, sw2, in4, in42) }

	// collision sampling at frame boundary vs during frame
	_ = e.RestoreState(st)
	for i := 0; i < 30; i++ {
		_, _ = e.StepFrame()
		c, _ := e.ReadCollisions()
		anyc := c.M0P1 || c.M0P0 || c.M1P0 || c.M1P1 || c.P0P1 || c.M0PF || c.M1PF || c.P0PF || c.P1PF
		if anyc { t.Logf("frame %d: collision latched at frame boundary %+v", i, c) }
	}
	t.Logf("done collision scan")
}
