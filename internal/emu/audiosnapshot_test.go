package emu

import "testing"

// TestSaveRestoreIsNotBitExactForAudio measures what a save/restore round trip does to the sound.
//
// `save_state` is documented as a "whole-machine snapshot ... CPU/RAM/TIA/RIOT/cart/TV plus the
// rendered framebuffer and the cycle counters". For the picture that holds. For the audio it does
// not, and this test pins the size and the shape of the gap so that a measurement made across a
// restore is not quietly believed.
//
// ★Measured 2026-09-05 with `roms/techniques/tia_pcm.bin`, which writes AUDV every few cycles and
// so shows any phase error immediately. Same ROM, same warm-up, same two frames of capture; the
// only difference is a 20-frame detour between SaveState and RestoreState:
//
//	as shipped ......................................... 34 of 1048 samples differ
//	with Audio.sampleSum deep-copied ................... 33
//	with Television.audioSignals cleared on Plumb ....... 1
//	with both .......................................... 0
//
// So there are exactly two causes, they are additive, and together they are the whole of it:
//
//   - **`Television.audioSignals` (33 samples).** The TV batches one `AudioSignalAttributes` per
//     scanline and flushes to the mixers when the batch reaches `TotalScanlines`
//     (`television.go:489-507`). The field is on `Television`, **not on `State`** — `State` starts
//     at `television.go:42` and `Snapshot()` returns `tv.state.Snapshot()` — so the partial batch
//     is neither saved nor restored. Whatever the detour left in it is emitted after the restore.
//   - **`Audio.sampleSum` (1 sample).** `audio.go` keeps the running total the volume average is
//     taken from in a slice, and `Snapshot()` is `n := *au`, so the snapshot and the live machine
//     share one backing array. Deep-copying is this engine's convention — 20 of the 92 `Snapshot`
//     functions under `hardware/` call `copy()` — and Audio is an exception to it. The blast radius
//     is one averaging window (36→150 on the free-running 228 clock, 38 CPU cycles), hence one
//     sample. Read from the source by the mailing-list distillation (helper-2), who predicted "at
//     most one sample" before it was run; the measurement above is that prediction landing exactly.
//
// ★★The engine is vendored and has never been modified here (`git log -- Gopher2600/` is empty),
// which is what lets "our engine does X" mean "upstream does X". Both probes above were applied,
// measured and reverted. So this is a test that ASSERTS THE DEFECT: if it starts failing, someone
// has fixed one of the two, and the `known-traps` row plus this comment need updating rather than
// the code.
//
// ★★★The practical rule, which is why this is worth a test: audio measured across a restore is
// wrong for the first ~17 scanlines and then re-converges. Reset the capture AFTER the restore and
// discard the head, or do not restore at all.
func TestSaveRestoreIsNotBitExactForAudio(t *testing.T) {
	run := func(detourFrames int) []uint8 {
		t.Helper()
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/techniques/tia_pcm.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.EnableAudioCapture(); err != nil {
			t.Fatal(err)
		}
		warmupStable(t, e)
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		s := e.SaveState()
		if detourFrames > 0 {
			if err := e.RunFrames(detourFrames); err != nil {
				t.Fatal(err)
			}
		}
		if err := e.RestoreState(s); err != nil {
			t.Fatal(err)
		}
		e.ResetAudioCapture()
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		ch0, _ := e.AudioSamples()
		return ch0
	}

	base := run(0)
	detour := run(20)

	if len(base) == 0 {
		t.Fatal("captured no audio at all — the fixture is not making sound, so this test would " +
			"pass by comparing two empty slices")
	}
	if len(base) != len(detour) {
		t.Fatalf("sample counts differ (%d vs %d): the runs are not comparable", len(base), len(detour))
	}

	var diff, lastDiff int
	firstMatch := -1
	for i := range base {
		if base[i] != detour[i] {
			diff++
			lastDiff = i
		} else if firstMatch < 0 {
			firstMatch = i
		}
	}
	t.Logf("samples=%d differing=%d last-differing-index=%d", len(base), diff, lastDiff)

	if diff == 0 {
		t.Fatal("save/restore is now bit-exact for audio, which it was not on 2026-09-05. " +
			"One of the two causes has been fixed upstream: Television.audioSignals not being part " +
			"of television.State, or Audio.Snapshot sharing sampleSum's backing array. Re-measure " +
			"the decomposition in this comment, update the `known-traps` row for save_state, and " +
			"then relax this assertion — do not simply delete it")
	}

	// The divergence must be a PREFIX that heals. If it ever runs to the end of the capture the
	// damage is no longer transient, and "discard the head" stops being an adequate answer.
	if lastDiff == len(base)-1 {
		t.Errorf("the audio never re-converged after the restore (differs through sample %d of %d): "+
			"the documented workaround — discard the first ~17 scanlines — assumes a transient",
			lastDiff, len(base))
	}
}
