package emu

import "testing"

// TestAUDCSilentCarriers measures which AUDC settings produce no tone at all — only a DC level at
// the volume written to AUDV. This project builds TIA PCM on AUDC=0 alone (`docs/techniques/
// tia-pcm.md`, `roms/litmus/litmus_pcm.asm`), and stella-list 199902/msg00036 (Eckhard Stolberg)
// says there are two:
//
//	"If you set the AUDCx register to 0 or 11, the output will always be high."
//
// He is right. A second silent carrier is worth having: it is the escape route when AUDC is wanted
// for something else on the channel being used as a DC output.
//
// **The interesting part of this measurement is how it was got wrong first, twice.**
//
// Attempt 1 built sixteen small ROMs, each writing AUDC/AUDF once at boot, and sampled 60 frames.
// It reported that AUDC 2, 6, 10 and 14 are ALSO constant at AUDF=31 — and that was passed on as a
// finding before it was checked. Widening the window to 300 frames killed it: bisected, AUDC=2 at
// AUDF=31 holds one value for **89 frames and breaks on the 90th**. "Constant" was a claim about
// the window.
//
// Attempt 2 assumed the 89 frames were a property of the setting and made the short window a
// control. It is not a property of the setting. This ROM rewrites the registers every frame, so its
// polynomial counters reach AUDF=31 through a different history — and here the same AUDC=2 breaks
// **in a single frame**. Same register values, same emulator, two ROMs, 89 frames apart.
//
// So the fact worth keeping is not the near-miss, it is why the near-miss existed: **a value
// measured in one state cannot be told apart from a constant.** Sixty frames of one boot path said
// "silent carrier" about a channel that is audibly toggling. The claim survives only because 0 and
// 11 hold in both ROMs, at every frequency, out to 1000 frames, while all fourteen others break
// within 4.
//
// The ROM takes both values from RAM and writes them with the CPU, so the register write is a real
// store and only the choice comes from here.
func TestAUDCSilentCarriers(t *testing.T) {
	const (
		ctrlAddr = 0x82
		freqAddr = 0x83
		volume   = 10
		// Long enough to clear the 89-frame near-miss with 3.4x of margin.
		windowFrames = 300
		// The window the first measurement used, kept as the control below.
		shortFrames = 60
	)
	freqs := []int{0, 1, 8, 31}

	distinct := func(t *testing.T, ctrl, freq, frames int) int {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM("../../roms/litmus/litmus_audc_carrier.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if err := e.EnableAudioCapture(); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(ctrlAddr, uint8(ctrl)); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(freqAddr, uint8(freq)); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		e.ResetAudioCapture()
		if err := e.RunFrames(frames); err != nil {
			t.Fatal(err)
		}
		ch0, _ := e.AudioSamples()
		if len(ch0) < frames*8 {
			t.Fatalf("only %d samples over %d frames for AUDC=%d AUDF=%d — too few to call "+
				"anything constant", len(ch0), frames, ctrl, freq)
		}
		seen := map[uint8]bool{}
		for _, s := range ch0 {
			seen[s] = true
		}
		// A silent carrier must park at the volume written, not at silence: that is the property
		// PCM depends on (AUDV IS the amplitude).
		if len(seen) == 1 && !seen[volume] {
			for v := range seen {
				t.Errorf("AUDC=%d AUDF=%d is constant at %d, want %d — a silent carrier is only "+
					"useful if it parks at the volume written", ctrl, freq, v, volume)
			}
		}
		return len(seen)
	}

	silent := map[int]bool{}
	for c := 0; c < 16; c++ {
		constantEverywhere := true
		for _, f := range freqs {
			if distinct(t, c, f, windowFrames) != 1 {
				constantEverywhere = false
			}
		}
		if constantEverywhere {
			silent[c] = true
		}
	}
	if len(silent) != 2 || !silent[0] || !silent[11] {
		got := []int{}
		for c := range silent {
			got = append(got, c)
		}
		t.Errorf("silent carriers over %d frames = %v, want exactly {0, 11} — stella-list "+
			"199902/msg00036: \"If you set the AUDCx register to 0 or 11, the output will always "+
			"be high\"", windowFrames, got)
	}

	// The control: every one of the fourteen others must break QUICKLY. Worst measured is 4 frames
	// (AUDC=10); anything that starts needing a long window here means the channel has stopped
	// toggling and "only 0 and 11 are constant" above has quietly become true by default rather
	// than by measurement.
	for c := 0; c < 16; c++ {
		if c == 0 || c == 11 {
			continue
		}
		if distinct(t, c, 31, 8) != 2 {
			t.Errorf("AUDC=%d at AUDF=31 did not toggle within 8 frames; worst measured is 4 "+
				"(AUDC=10). If this channel has gone quiet, the result above is true by default", c)
		}
	}

	// And the two survivors must survive a window three times longer again — the failure this whole
	// file exists to prevent is calling something constant because nobody looked for long enough.
	for _, c := range []int{0, 11} {
		if distinct(t, c, 31, 1000) != 1 {
			t.Errorf("AUDC=%d stopped being constant at 1000 frames", c)
		}
	}
}
