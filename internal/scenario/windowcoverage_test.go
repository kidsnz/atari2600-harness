package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFrameLinesStableCoversEveryEvent fails when a scenario asserts something, or presses a
// button, AFTER its own `frame_lines_stable` window has closed.
//
// The window began life as a default — 130 frames — and scenarios grew past it without anyone
// re-reading it. Measured 2026-09-04: **ten scenarios had events outside their window**, the worst
// being `game_states.json`, which drives a full lifecycle out to frame 1100 and checked line counts
// for the first 130. Every state transition it exists to exercise was outside the line-count check.
//
// The corpus says exactly why that matters. Andrew Davie, stella-list 2002, on Qb Special Edition:
// *"I have manufactured 100 cartridges. With a showstopper bug in them. I didn't see it in my own
// testing. The scanline count is mostly 262, but sometimes jumps to 288."* Thomas Jentzsch's
// diagnosis: it happened **only when a new game started** — a long timer polled too late, missing
// the first appearance of zero. A month earlier the same failure had hit `warring worms`, also
// intermittent, also missed by its author, also found by someone else.
//
// So the failure this guards is not hypothetical, and it has a shape: **intermittent, tied to a
// transition, invisible to an average or to a representative frame.** `frame_lines_stable` is the
// right check — it is ∀ over frames, unlike `ntsc_frame_lines`, which samples one — but a check is
// only ∀ over the frames it runs.
//
// ★What this test does NOT do (corrected 2026-09-29). It was written as if the window started at
// frame 0, so that `frames` past the last event would put every event inside it. The window starts
// where the timeline ENDS (scenario.go steps it after the timeline, on the same emulator), so no event
// frame is ever in it, however large `frames` is — measured 2026-09-15, see docs/scenarios.md. What
// the rule still buys is a window at least as long as the last input or assert frame number, on the
// state the run settled into (it does not look at `frames`, `fuzz` or warmup). The transitions
// themselves, which the Qb report above is about, are unchecked for line count.
//
// Found by the mailing-list distillation (helper-1), who asked the one question that mattered:
// does the golden cover every frame, or the important ones?
func TestFrameLinesStableCoversEveryEvent(t *testing.T) {
	roots, err := filepath.Glob("../../roms/*/scenarios/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) == 0 {
		t.Skip("no scenarios found")
	}
	checked := 0
	for _, p := range roots {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var doc struct {
			Asserts []struct {
				AtFrame int `json:"at_frame"`
			} `json:"asserts"`
			Inputs []struct {
				Frame int `json:"frame"`
			} `json:"inputs"`
			Checks struct {
				FrameLinesStable *struct {
					Frames int `json:"frames"`
				} `json:"frame_lines_stable"`
			} `json:"checks"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if doc.Checks.FrameLinesStable == nil {
			continue
		}
		checked++
		last := 0
		for _, a := range doc.Asserts {
			if a.AtFrame > last {
				last = a.AtFrame
			}
		}
		for _, in := range doc.Inputs {
			if in.Frame > last {
				last = in.Frame
			}
		}
		if w := doc.Checks.FrameLinesStable.Frames; last > w {
			t.Errorf("%s: frame_lines_stable runs %d frames, fewer than its last event's frame number "+
				"(%d). The window starts after the timeline, so raising `frames` does not put the "+
				"events in it; it only watches the settled state for longer.",
				filepath.Base(p), w, last)
		}
	}
	if checked == 0 {
		t.Error("no scenario declared frame_lines_stable — this test is checking nothing")
	}
	t.Logf("checked %d scenarios that declare frame_lines_stable", checked)
}
