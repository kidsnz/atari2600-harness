package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// romPath resolves a litmus ROM from this package's directory. The harness's tests
// reference only the harness's own ROMs, so this never leaves the repository.
func romPath(name string) string {
	return filepath.Join("..", "..", "roms", "litmus", name)
}

// THE WITNESS. -frame exists so a still picture -- a ROM with no moving state, where
// -trigger has nothing to watch -- can be rendered at all, and the moment you name a
// frame outright you can name frame 1, which is the trap this whole command was
// written about. The guard has to refuse it.
func TestFrameOneIsRefusedAsBlank(t *testing.T) {
	_, err := grabFrame(romPath("litmus_pf_allcols.bin"), 1)
	if err == nil {
		t.Fatal("frame 1 was accepted; it is the undrawn frame and writing it produces the " +
			"featureless PNG this command exists to prevent")
	}
	if !strings.Contains(err.Error(), "UNIFORM") {
		t.Errorf("the refusal does not say what is wrong with the frame: %v", err)
	}
}

// The negative control. A guard that refuses everything would pass the test above and
// make the flag useless, so a frame that DOES have the picture on it must come back.
func TestARealFrameIsAccepted(t *testing.T) {
	img, err := grabFrame(romPath("litmus_pf_allcols.bin"), 20)
	if err != nil {
		t.Fatalf("frame 20 was refused: %v", err)
	}
	mean, sd := bandStats(img)
	t.Logf("frame 20: mean %.2f sd %.2f", mean, sd)
	if sd < blankFloor {
		t.Errorf("frame 20's sd is %.2f, under the floor -- the ROM draws nothing and this "+
			"test is measuring nothing", sd)
	}
}

// The reason the guard is on the SPREAD and not on the brightness, stated as a test
// so the choice cannot quietly revert. The undrawn frame's mean is a property of the
// EMULATOR, not of a ROM: two unrelated ROMs read the same value at frame 1. Any
// brightness floor is therefore either under it (and passes the frame it was written
// to catch) or over it (and rejects dark pictures).
func TestTheUndrawnFrameLooksTheSameForEveryROM(t *testing.T) {
	means := map[string]float64{}
	for _, rom := range []string{"litmus_pf_allcols.bin", "litmus_48px.bin"} {
		e := mustFrame1Mean(t, romPath(rom))
		means[rom] = e
	}
	var vals []float64
	for _, v := range means {
		vals = append(vals, v)
	}
	t.Logf("frame-1 band means: %v", means)
	if vals[0] != vals[1] {
		t.Errorf("two different ROMs read %v and %v at frame 1; if the undrawn frame really "+
			"were ROM-specific a brightness floor could work, and this test is the reason "+
			"the guard does not use one", vals[0], vals[1])
	}
}

func mustFrame1Mean(t *testing.T, rom string) float64 {
	t.Helper()
	img, err := grabFrameUnguarded(rom, 1)
	if err != nil {
		t.Fatalf("%s: %v", rom, err)
	}
	mean, _ := bandStats(img)
	return mean
}
