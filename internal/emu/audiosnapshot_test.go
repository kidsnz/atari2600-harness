package emu

import "testing"

// audioAfterRestore plays `rom`, saves `extraLines` scanlines past a frame boundary, optionally
// takes a detour, restores, and returns the samples captured from the restore onward.
func audioAfterRestore(t *testing.T, rom string, extraLines, detourFrames int) []uint8 {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioCapture(); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e)
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < extraLines; i++ {
		if err := e.StepScanline(); err != nil {
			t.Fatal(err)
		}
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

// TestSaveRestoreIsNotBitExactForAudio measures what a save/restore round trip does to the sound.
//
// `save_state` is documented as a "whole-machine snapshot ... CPU/RAM/TIA/RIOT/cart/TV plus the
// rendered framebuffer and the cycle counters". For the picture that holds. For the audio it does
// not, and this test pins how badly, so that a measurement taken across a restore is not quietly
// believed.
//
// ★THE CAUSES (measured 2026-09-05 by probing the vendored engine, `tia_pcm.bin`, save at a frame
// boundary, 20-frame detour — 34 of 1048 samples differ as shipped):
//
//	as shipped ......................................... 34
//	with Audio.sampleSum deep-copied ................... 33
//	with Television.audioSignals cleared on Plumb ....... 1
//	with both .......................................... 0
//
// Two causes, additive, and together the whole of it.
//
//   - **`Television.audioSignals`.** The TV batches one `AudioSignalAttributes` per scanline and
//     flushes to the mixers when the batch reaches `TotalScanlines` (`television.go:489-507`). The
//     field is on `Television`, **not on `State`** — `State` starts at `television.go:42` and
//     `Snapshot()` returns `tv.state.Snapshot()` — so the partial batch is neither saved nor
//     restored, and whatever the detour left in it is emitted after the restore.
//   - **`Audio.sampleSum`.** The volume averager's running total is a slice and `Snapshot()` is
//     `n := *au`, so snapshot and live machine share one backing array. Deep-copying is this
//     engine's convention — 20 of the 92 `Snapshot` functions under `hardware/` call `copy()` —
//     and Audio is the exception. Its blast radius is one averaging window (36→150 on the
//     free-running 228 clock, 38 CPU cycles), hence exactly one sample. Read from the source by the
//     mailing-list distillation (helper-2), who predicted "at most one sample" before it was run.
//
// ★★HOW BAD, ACROSS THREE ROMS AND FOUR SAVE POSITIONS. The first version of this test measured
// ONE ROM at ONE save position, reported "34 samples, then it re-converges", and that was wrong as
// a general statement — 34 is this ROM's number and re-convergence is this save position's. The
// sweep (differing / of / last differing index):
//
//	                 save at frame boundary   +1 line        +5 lines        +40 lines
//	tia_pcm            34/1048 last 33        38 last 1015    54 last 1015   114/786 last 491
//	sound_driver       19/1048 last 19        87 last 1020   366 last 1047   530/786 last 780
//	music_driver        6/1048 last 5         76 last 1034   266/786 last 785 526/786 last 785
//
// ★★★So the honest statement is: **at a frame boundary the damage is a short head (6-34 samples
// here) and heals; anywhere else it is not a head at all** — up to **530 of 786 samples, running to
// the last one**. "Discard the first N" is only a workaround for a frame-boundary save. If you are
// measuring sound, save at a frame boundary and discard the head, or do not restore.
//
// ★★★★The engine is vendored and has never been modified here (`git log -- Gopher2600/` is
// empty), which is what lets "our engine does X" mean "upstream does X". Both probes above were
// applied, measured and reverted. So this test ASSERTS THE DEFECT: if it starts passing cleanly,
// someone has fixed one of the two causes and this comment needs re-measuring, not deleting.
func TestSaveRestoreIsNotBitExactForAudio(t *testing.T) {
	roms := []string{
		"../../roms/techniques/tia_pcm.bin",
		"../../roms/techniques/sound_driver.bin",
		"../../roms/techniques/music_driver.bin",
	}

	type cell struct{ diff, last, n int }
	got := map[string]map[int]cell{}

	for _, rom := range roms {
		got[rom] = map[int]cell{}
		for _, extra := range []int{0, 1, 5, 40} {
			base := audioAfterRestore(t, rom, extra, 0)
			detour := audioAfterRestore(t, rom, extra, 20)
			n := len(base)
			if len(detour) < n {
				n = len(detour)
			}
			if n == 0 {
				t.Fatalf("%s at +%d lines: captured no audio, so this would compare two empty "+
					"slices and pass for the wrong reason", rom, extra)
			}
			diff, last := 0, -1
			for i := 0; i < n; i++ {
				if base[i] != detour[i] {
					diff++
					last = i
				}
			}
			got[rom][extra] = cell{diff, last, n}
			t.Logf("%-42s +%2d lines: %4d of %4d differ, last at %4d",
				rom, extra, diff, n, last)
		}
	}

	// 1) The defect exists at all. Every cell in the sweep must differ somewhere.
	for _, rom := range roms {
		for extra, c := range got[rom] {
			if c.diff == 0 {
				t.Errorf("%s at +%d lines: save/restore is now bit-exact for audio, which it was "+
					"not on 2026-09-05. One of the two causes has been fixed: Television.audioSignals "+
					"not being part of television.State, or Audio.Snapshot sharing sampleSum's "+
					"backing array. Re-measure the table in this comment, update the `known-traps` "+
					"row for save_state, and only then relax this assertion — do not delete it",
					rom, extra)
			}
		}
	}

	// 2) The shape claim the documentation rests on: a frame-boundary save damages only a head.
	//    This is the assertion that makes "discard the head" a legitimate workaround, and it is
	//    the one the first version of this test got wrong by measuring a single ROM.
	for _, rom := range roms {
		c := got[rom][0]
		if c.last > c.n/4 {
			t.Errorf("%s saved at a frame boundary: the divergence reaches sample %d of %d, which "+
				"is no longer a head — `known-traps.md` and `mcp-tools.md` tell the reader to "+
				"discard the head after a frame-boundary save, and that advice is now wrong",
				rom, c.last, c.n)
		}
	}

	// 3) And the claim that makes the warning worth printing: away from a frame boundary it is
	//    NOT a head. If this ever stops being true the warning is over-stated and should be cut.
	worst := 0
	for _, rom := range roms {
		for _, extra := range []int{1, 5, 40} {
			if c := got[rom][extra]; c.last > worst {
				worst = c.last
			}
		}
	}
	if worst < 100 {
		t.Errorf("away from a frame boundary the divergence now stops at sample %d, so it has "+
			"become a head everywhere and the stronger warning in the docs is no longer earned",
			worst)
	}
}
