package motion

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestAnalyzePureMath(t *testing.T) {
	// constant velocity glide -> zero 2nd difference -> jerk 0, monotonic.
	g := Analyze([]int{0, 1, 2, 3, 4})
	if g.JerkRMS != 0 || g.MaxAccel != 0 || g.MaxStep != 1 || !g.Monotonic {
		t.Fatalf("glide: %+v (want jerk0/maxaccel0/maxstep1/monotonic)", g)
	}
	// stutter +2,0,+2,0 -> velocity alternates -> accel ±2 -> jerk_rms 2.
	s := Analyze([]int{0, 2, 2, 4, 4, 6})
	if s.JerkRMS != 2 || s.MaxAccel != 2 {
		t.Fatalf("stutter: jerk=%v maxaccel=%d (want 2/2)", s.JerkRMS, s.MaxAccel)
	}
	if !s.Monotonic { // a 0 step is not a sign change
		t.Fatalf("stutter should stay monotonic: %+v", s)
	}
	// a one-frame snap-back -> sign change (not monotonic) + a big max_accel spike.
	b := Analyze([]int{0, 1, 2, 5, 3, 4})
	if b.Monotonic || b.MaxAccel < 5 {
		t.Fatalf("snap-back: monotonic=%v maxaccel=%d (want false / >=5)", b.Monotonic, b.MaxAccel)
	}
}

// track is the warmup+TrackObject the CLI does, factored for the test.
func track(t *testing.T, rom string) Track {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatalf("load %s: %v", rom, err)
	}
	if err := e.RunFrames(5); err != nil { // settle the RESBL strobe / first-frame transient
		t.Fatal(err)
	}
	tr, err := TrackObject(e, "BL", 40, 40, 230)
	if err != nil {
		t.Fatalf("track %s: %v", rom, err)
	}
	return tr
}

// TestMotionSelfTest is the planted-discrepancy gate: a clean constant-velocity
// glide must score jerk 0, and an otherwise-identical ROM with a deliberate
// +2,0,+2,0 stutter must score well above it. If the stutter does NOT score
// higher, the metric is vacuous. (Both ROMs assembled by CI before `go test`.)
func TestMotionSelfTest(t *testing.T) {
	glide := track(t, "../../roms/litmus/motion_glide.bin")
	stutter := track(t, "../../roms/litmus/motion_stutter.bin")

	if glide.Missing != 0 || stutter.Missing != 0 {
		t.Fatalf("ball not tracked every frame (glide miss=%d stutter miss=%d)", glide.Missing, stutter.Missing)
	}
	// clean glide: rendered top advances by a constant +1 -> exactly smooth.
	if glide.Top.JerkRMS != 0 || glide.Top.MaxAccel != 0 {
		t.Fatalf("clean glide should be perfectly smooth: top jerk=%v maxaccel=%d\n top=%v",
			glide.Top.JerkRMS, glide.Top.MaxAccel, glide.Top.Positions)
	}
	// planted judder: must be clearly above the clean glide.
	if stutter.Top.JerkRMS <= glide.Top.JerkRMS {
		t.Fatalf("VACUOUS METRIC: stutter jerk %v not above glide jerk %v",
			stutter.Top.JerkRMS, glide.Top.JerkRMS)
	}
	if stutter.Top.JerkRMS < 1.5 {
		t.Fatalf("stutter jerk %v too low (expected ~2 from a +2,0,+2,0 stutter)", stutter.Top.JerkRMS)
	}
	// X is fixed in both -> horizontal must read as exactly still.
	if glide.X.JerkRMS != 0 || stutter.X.JerkRMS != 0 {
		t.Fatalf("fixed-X ball should have X jerk 0 (glide %v stutter %v)", glide.X.JerkRMS, stutter.X.JerkRMS)
	}
}
