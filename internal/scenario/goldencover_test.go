package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, j map[string]any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.json")
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The hole this gate closes, in the exact shape it was found in: an audio golden with no
// "frames" and no asserts hashes one frame, so it matches whatever it was recorded against.
// On a real work that scenario stayed green after the hi-hats were deleted from the ROM.
func TestAnAudioGoldenThatHashesNothingIsRefused(t *testing.T) {
	p := write(t, map[string]any{
		"rom":    "whatever.asm",
		"checks": map[string]any{"golden_audio": true},
	})
	_, err := Load(p)
	if err == nil {
		t.Fatal("a golden_audio scenario with no frames and no asserts loaded without complaint")
	}
	if !strings.Contains(err.Error(), "cannot fail") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
}

// The negative control, and the one that matters: a gate that refused everything would pass
// the test above. A scenario that DOES cover frames must load.
func TestAGoldenWithFramesLoads(t *testing.T) {
	p := write(t, map[string]any{
		"rom":    "whatever.asm",
		"frames": 460,
		"checks": map[string]any{"golden_audio": true, "golden_mix": true},
	})
	got, err := Load(p)
	if err != nil {
		t.Fatalf("a 460-frame audio golden was refused: %v", err)
	}
	// Assert the coverage the name claims, not merely that nothing errored: a Load that
	// silently dropped "frames" would pass this test while leaving the gate nothing to judge.
	if got.Frames != 460 || !got.Checks.GoldenAudio || !got.Checks.GoldenMix {
		t.Errorf("loaded frames=%d golden_audio=%v golden_mix=%v, want 460/true/true",
			got.Frames, got.Checks.GoldenAudio, got.Checks.GoldenMix)
	}
}

// Coverage does not come from "frames" alone — it is the larger of "frames" and the highest
// at_frame. This repository's own net relies on that: roms/techniques/sound_driver.json has no
// "frames" and reaches frame 70 through its asserts. Refusing it would be wrong.
func TestAssertsAloneCountAsCoverage(t *testing.T) {
	p := write(t, map[string]any{
		"rom": "whatever.asm",
		"asserts": []map[string]any{
			{"at_frame": 70, "field": "audio.ch0.volume", "op": "==", "value": 8},
		},
		"checks": map[string]any{"golden_audio": true},
	})
	got, err := Load(p)
	if err != nil {
		t.Fatalf("asserts reaching frame 70 should be coverage enough to load: %v", err)
	}
	// The point of the test is WHERE the coverage came from. "frames" must still be zero --
	// if a default filled it in, this scenario would load for a reason that has nothing to do
	// with its asserts, and the rule the name states would be untested.
	if got.Frames != 0 {
		t.Errorf("frames = %d, want 0: the coverage has to come from the assert alone", got.Frames)
	}
	if len(got.Asserts) != 1 || got.Asserts[0].AtFrame != 70 {
		t.Errorf("asserts = %+v, want exactly one at frame 70", got.Asserts)
	}
}

// A scenario with no golden at all is not this gate's business, however few frames it runs.
func TestTheGateOnlyJudgesGoldens(t *testing.T) {
	p := write(t, map[string]any{
		"rom":    "whatever.asm",
		"checks": map[string]any{"ntsc_frame_lines": 262},
	})
	got, err := Load(p)
	if err != nil {
		t.Fatalf("a scenario with no audio golden was refused: %v", err)
	}
	// And it loaded because there was no golden to judge, not because it happened to cover
	// enough frames: this scenario runs one frame, which the gate would refuse if it applied.
	if got.Checks.GoldenAudio || got.Checks.GoldenMix {
		t.Fatalf("the fixture is not the one this test needs: it has a golden")
	}
	if got.Frames > 1 {
		t.Errorf("frames = %d: the fixture no longer demonstrates that a SHORT run without a "+
			"golden is allowed", got.Frames)
	}
}
