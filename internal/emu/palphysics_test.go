package emu

import (
	"math"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/clocks"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
	"github.com/kidsnz/atari2600-harness/pkg/design"
)

// TestPALChangesPhysicsBySquareOfTheRateRatio measures the premise underneath
// `subpixel-velocity.md`'s conversion factor, and then the consequence that page does not carry.
//
// The page says an NTSC increment must be **83.21%** of the PAL one (a PAL increment 120.18% of the
// NTSC one) to travel the same distance per second, computed from the 2600's own clocks. That is the LINEAR case. Anything that
// accelerates is worse, and a 2004 author found out by shipping a PAL60 build rather than retune:
//
//	"THE GRAVITY IN THE NTSC VERSION IS EFFECTIVELY 1.4x GREATER. IT'S THE ONE CONSTANT I
//	 COULDN'T CHANGE... So anyway, I'VE INCLUDED A PAL60 VERSION"  〔`200409/msg00309`〕
//
// 1.4 is not a coincidence. With `vel += g` and `pos += vel` once per frame, distance goes as the
// SQUARE of the frame count, so the same code falls **(60/50)² ≈ 1.44×** further per second.
//
// Two things are measured here and one is derived:
//
//	MEASURED  the same ROM produces byte-identical positions per frame under NTSC and PAL —
//	          nothing about the television standard reaches the arithmetic
//	MEASURED  the accelerating object's distance is quadratic in the frame count
//	DERIVED   therefore the per-second ratio is the rate ratio for constant velocity and its
//	          SQUARE for constant acceleration: 1.2018 and 1.4443 from the 2600's own clocks
//
// The first is the one that could have failed and is the reason the other two mean anything: if PAL's
// extra 50 scanlines had changed how much work fits in a frame, the whole conversion would be about
// something else. Found by the mailing-list distillation (helper-2).
func TestPALChangesPhysicsBySquareOfTheRateRatio(t *testing.T) {
	// pos16 returns the 16-bit fixed-point positions of the constant-velocity and the accelerating
	// object after n frames.
	pos16 := func(spec string, n int) (vel, acc int) {
		e, err := New(spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_pal_physics.bin"); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
		}
		lo, _ := e.PeekRAM(0x82)
		hi, _ := e.PeekRAM(0x83)
		vel = int(hi)<<8 | int(lo)
		lo, _ = e.PeekRAM(0x84)
		hi, _ = e.PeekRAM(0x85)
		acc = int(hi)<<8 | int(lo)
		return
	}

	// (1) The standard does not reach the arithmetic.
	for _, n := range []int{50, 100, 150} {
		vN, aN := pos16("NTSC", n)
		vP, aP := pos16("PAL", n)
		if vN != vP || aN != aP {
			t.Fatalf("after %d frames NTSC gives (%d, %d) and PAL gives (%d, %d). The ROM's "+
				"per-frame arithmetic is supposed to be untouched by the television standard; if "+
				"it is not, the conversion factor in subpixel-velocity.md is about the wrong thing",
				n, vN, aN, vP, aP)
		}
	}

	// (2) The accelerating object is quadratic: doubling the frames quadruples the distance.
	// Exact equality is not expected — the position lags the velocity by one frame — so the check
	// is that the ratio sits on 4 and nowhere near the 2 a linear object would give.
	_, a50 := pos16("NTSC", 50)
	_, a100 := pos16("NTSC", 100)
	if a50 == 0 {
		t.Fatal("the accelerating object never moved — nothing below is measuring anything")
	}
	ratio := float64(a100) / float64(a50)
	if ratio < 3.8 || ratio > 4.2 {
		t.Errorf("doubling the frames multiplied the accelerating object's distance by %.3f, want "+
			"about 4 (quadratic). A value near 2 would mean the ROM is not accelerating and the "+
			"whole point of this test is gone", ratio)
	}
	// And the control: the constant-velocity object must give 2, or "about 4" above proves nothing.
	v50, _ := pos16("NTSC", 50)
	v100, _ := pos16("NTSC", 100)
	if lin := float64(v100) / float64(v50); lin < 1.9 || lin > 2.1 {
		t.Errorf("the constant-velocity object scaled by %.3f over the same doubling, want about 2. "+
			"Without this the quadratic reading above could be an artefact of the measurement",
			lin)
	}

	// (3) The consequence, from the 2600's own clocks: the colour clock over 228 colour clocks per
	// line over the line count. NOT from the engine's `specification` package — its RefreshRate is
	// the broadcast line rate over the line count (`HorizontalScanRate: 15734.26` / 262 and
	// `HorizontalScanRate: 15625.00` / 312), a television's figure rather than the console's, and
	// its ratio is 1.1992.
	ntscClock := audio.BaseClockNTSC * 114 // 3,579,545 Hz
	palClock := audio.BaseClockPAL * 114   // 3,546,894 Hz
	// pkg/audio records no source for the PAL colour clock. The engine's CPU clocks are the in-tree
	// witness (`PAL = 1.182298`, `NTSC = 1.193182` in MHz, three colour clocks per CPU cycle, "Values
	// taken from" a taswegian.com Clock-Speeds page that is cited there and not verified here).
	if d := math.Abs(palClock - clocks.PAL_TIA*1e6); d > 1 {
		t.Errorf("pkg/audio's PAL colour clock %.0f Hz is %.0f Hz from the engine's %.0f", palClock, d,
			clocks.PAL_TIA*1e6)
	}
	if d := math.Abs(ntscClock - clocks.NTSC_TIA*1e6); d > 2 {
		t.Errorf("pkg/audio's NTSC colour clock %.0f Hz is %.0f Hz from the engine's %.0f", ntscClock, d,
			clocks.NTSC_TIA*1e6)
	}
	ntsc, pal := ntscClock/228/262, palClock/228/312
	if math.Abs(ntsc-design.NTSCFrameRateHz) > 1e-6 {
		t.Errorf("NTSC frame rate %.4f Hz here, %.4f Hz in design.NTSCFrameRateHz — the flicker ladder "+
			"and this conversion have drifted apart", ntsc, design.NTSCFrameRateHz)
	}
	rate := ntsc / pal
	if math.Abs(rate-1.2018) > 0.001 {
		t.Errorf("NTSC/PAL refresh ratio is %.4f, was 1.2018 — subpixel-velocity.md's 83.21%% is "+
			"1/this, so that number is stale too", rate)
	}
	if sq := rate * rate; math.Abs(sq-1.4443) > 0.002 {
		t.Errorf("the squared ratio is %.4f, was 1.4443. This is the number the 2004 author felt "+
			"as \"1.4x greater\" gravity", sq)
	}
}
