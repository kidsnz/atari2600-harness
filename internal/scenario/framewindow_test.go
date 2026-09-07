package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// windowFacts is the slice of a scenario this test cares about, read from the JSON rather than
// through Scenario so that a field being dropped from the struct cannot silently empty the check.
type windowFacts struct {
	Inputs []struct {
		Frame int `json:"frame"`
	} `json:"inputs"`
	Asserts []struct {
		AtFrame *int `json:"at_frame"`
	} `json:"asserts"`
	Checks struct {
		FrameLinesStable *struct {
			Frames int `json:"frames"`
			Lines  int `json:"lines"`
		} `json:"frame_lines_stable"`
	} `json:"checks"`
}

// reach returns the stability window and the last frame the scenario says anything about.
func (w windowFacts) reach() (window, last int, has bool) {
	if w.Checks.FrameLinesStable == nil {
		return 0, 0, false
	}
	window = w.Checks.FrameLinesStable.Frames
	for _, i := range w.Inputs {
		if i.Frame > last {
			last = i.Frame
		}
	}
	for _, a := range w.Asserts {
		if a.AtFrame != nil && *a.AtFrame > last {
			last = *a.AtFrame
		}
	}
	return window, last, true
}

// TestFrameLineWindowCoversEveryStateTheScenarioReaches closes a gap named on the mailing list and
// never given a shape here.
//
// A 2005 report, found by eye rather than by any check: *"The screen also **jumps during gameplay on
// some, but not all, of the screens**"* 〔`200505/msg00099`〕. `ntsc_frame_lines` looks at one frame
// and `frame_lines_stable` looks at a window; neither knows anything about **states**. A ROM that
// holds 262 lines on its title screen and loses them in play would pass both if the window stopped
// before the game started.
//
// The invariant that makes the window meaningful is cheap: **it must reach at least as far as the
// last frame the scenario itself talks about.** Every input and every assert is the author saying
// "the ROM is somewhere interesting at frame N"; if the line-length check stopped before N, it never
// looked at that somewhere.
//
// Measured 2026-09-07: **205 scenarios declare `frame_lines_stable` and 0 fall short.** The property
// held everywhere and nothing enforced it, which is the state a rule is cheapest to add in.
//
// The zero is guarded below by a synthetic scenario that must be caught.
func TestFrameLineWindowCoversEveryStateTheScenarioReaches(t *testing.T) {
	var files []string
	for _, g := range []string{"../../roms/*/scenarios/*.json", "../../../roms/*/scenarios/*.json"} {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	if len(files) < 50 {
		t.Fatalf("the walk found %d scenarios — too few for the result below to mean anything", len(files))
	}

	checked := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var w windowFacts
		if err := json.Unmarshal(b, &w); err != nil {
			continue // scenarios this test's narrow struct cannot read are not its business
		}
		window, last, has := w.reach()
		if !has {
			continue
		}
		checked++
		if window < last {
			t.Errorf("%s: frame_lines_stable watches %d frames but the scenario acts or asserts as "+
				"late as frame %d. Everything after frame %d is unchecked for line length — which is "+
				"exactly where a state this scenario drives the ROM into would be.",
				filepath.Base(f), window, last, window)
		}
	}
	if checked < 50 {
		t.Fatalf("only %d scenarios declare frame_lines_stable; the sweep is looking at too little", checked)
	}

	// The control: a scenario whose window stops before its own last input must be caught, or the
	// clean result above is a check that cannot fire.
	var bait windowFacts
	if err := json.Unmarshal([]byte(`{
		"inputs": [{"frame": 900, "action": "reset", "pressed": true}],
		"asserts": [{"at_frame": 5, "field": "ram.0x80", "op": "==", "value": 0}],
		"checks": {"frame_lines_stable": {"frames": 100, "lines": 262}}
	}`), &bait); err != nil {
		t.Fatal(err)
	}
	window, last, has := bait.reach()
	if !has || window >= last {
		t.Fatalf("the counter-bait was not caught (window=%d last=%d has=%v) — the sweep above "+
			"would have reported zero whatever the corpus contained", window, last, has)
	}
}
