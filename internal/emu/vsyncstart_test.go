package emu

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestWhatVSYNCsyncedOnStartHides answers a worry with a measurement instead of leaving it open.
//
// `Gopher2600/hardware/television/television.go` will not move the picture's vertical origin until
// the frame is Stable, and `VSYNCsyncedOnStart` — **default true** — is what gates that:
//
//	if tv.state.frameInfo.Stable || !tv.env.Prefs.TV.VSYNCsyncedOnStart.Get().(bool) {
//	    tv.state.vsync.topScanline = ...
//	}
//
// with `stabilityThreshold = 6`. So for the first six frames the engine does not show the vertical
// displacement a real television would. **And 42% of this repository's measurement points sit
// inside that window** (722 time references across 122 scenarios; 303 of them at frame < 6, with
// `warmup_frames` = 3 in 83 of them) — counted by the mailing-list distillation (helper-2), who
// raised it as exposure rather than damage and asked for it to be measured.
//
// ★Measured across 192 ROMs rendered twice at SIX different read points, whole-frame hash:
//
//	after 2 frames  0 differ        after 5 frames  1 differ   cart_f4sc
//	after 3 frames  0 differ        after 6 frames  1 differ   pf_wraps
//	after 4 frames  0 differ        after 8 frames  0 differ
//
// **At most one ROM differs at any read point, WHICH ROM depends on the read point, and by frame 8
// everything agrees again.** So the effect is real, transient, and self-correcting.
//
// ★★The first version of this test read at six frames only and reported "one ROM differs:
// pf_wraps". That was one sample of a moving quantity — exactly the error `check_instruments`
// exists to catch — and it was caught by the distillation (helper-2), who pointed out that six is
// the boundary itself (`stabilityThreshold = 6`) and asked for four to be read as well. They also
// predicted `cb_roll` would differ; it does not, at any of the six read points.
//
// ★★★So the honest statement of the 42% exposure is: the window is real, and nothing in this
// corpus depends on it for more than a frame or two. This is a test rather than a note so that a
// future ROM which DOES depend on it fails here and is named, instead of the figure sitting in a
// document as a permanent unresolved worry. Family: `known-traps.md`'s "TRUE OF HARDWARE, AND NOT
// OF WHAT WE MEASURE WITH".
func TestWhatVSYNCsyncedOnStartHides(t *testing.T) {
	if testing.Short() {
		t.Skip("renders the whole ROM corpus twice")
	}
	var roms []string
	if err := filepath.Walk("../../roms", func(p string, d os.FileInfo, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".bin") {
			roms = append(roms, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(roms)
	if len(roms) < 100 {
		t.Fatalf("found only %d ROMs — a sweep over nothing agrees with any expectation", len(roms))
	}

	digest := func(rom string, synced bool, frames int) (string, bool) {
		e, err := New("NTSC")
		if err != nil {
			return "", false
		}
		e.VCS.Env.Prefs.TV.VSYNCsyncedOnStart.Set(synced)
		if err := e.LoadROM(rom); err != nil {
			return "", false
		}
		if err := e.RunFrames(frames); err != nil {
			return "", false
		}
		h := sha1.New()
		for line := 0; line < 262; line++ {
			runs, _, err := e.ReadRow(line)
			if err != nil {
				continue
			}
			for _, r := range runs {
				fmt.Fprintf(h, "%d:%d:%s,", r.Clock, r.Len, r.Hex)
			}
		}
		return fmt.Sprintf("%x", h.Sum(nil)), true
	}

	// ★Read at several points, because the answer moves. `want[n]` is what differs after n frames.
	want := map[int]map[string]bool{
		2: {}, 3: {}, 4: {},
		5: {"cart_f4sc": true},
		6: {"pf_wraps": true},
		8: {},
	}
	points := []int{2, 3, 4, 5, 6, 8}
	for _, n := range points {
		got := map[string]bool{}
		swept := 0
		for _, rom := range roms {
			a, ok1 := digest(rom, true, n)
			b, ok2 := digest(rom, false, n)
			if !ok1 || !ok2 {
				continue
			}
			swept++
			if a != b {
				got[strings.TrimSuffix(filepath.Base(rom), ".bin")] = true
			}
		}
		for name := range got {
			if !want[n][name] {
				t.Errorf("after %d frames %s renders differently with VSYNCsyncedOnStart off and is "+
					"not in the witness list. It has started depending on the hidden start-up "+
					"window: before the frame is Stable the engine does not move the vertical "+
					"origin, so whatever this ROM does to VSYNC would show on a television and does "+
					"not show here", n, name)
			}
		}
		for name := range want[n] {
			if !got[name] {
				t.Errorf("after %d frames %s no longer differs. That may be a fix, but a witness "+
					"list that has drifted is worse than none", n, name)
			}
		}
		t.Logf("after %d frames: swept %d ROMs twice, %d differ", n, swept, len(got))
	}
}
