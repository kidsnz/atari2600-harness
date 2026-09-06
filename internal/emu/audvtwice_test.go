package emu

import (
	"os"
	"path/filepath"
	"testing"
)

// buildAudvTwiceROM emits a 262-line ROM that writes AUDV0 once (or twice) per scanline.
//
// The line is: WSYNC, `lda #v1 / sta AUDV0`, `pad` cycles of filler, optionally
// `lda #v2 / sta AUDV0`, then the loop's DEX/BNE. Everything is timed from the WSYNC, so the
// two variants differ in exactly one thing: whether a second write lands `pad` cycles later.
func buildAudvTwiceROM(t *testing.T, audc, audf, v1, v2 uint8, pad int, second bool) string {
	t.Helper()
	if pad < 0 || (pad > 0 && pad < 2) {
		t.Fatalf("pad %d is not emittable: the shortest filler is a 2-cycle NOP", pad)
	}

	filler := []byte{}
	rem := pad
	if rem%2 == 1 {
		filler = append(filler, 0x24, 0x80) // BIT $80 — 3 cycles, reads RAM, no TIA side effect
		rem -= 3
	}
	for ; rem > 0; rem -= 2 {
		filler = append(filler, 0xEA) // NOP
	}
	if rem != 0 {
		t.Fatalf("pad %d did not decompose into 2- and 3-cycle fillers (left %d)", pad, rem)
	}

	body := []byte{0x85, 0x02, 0xA9, v1, 0x85, 0x19} // STA WSYNC / LDA #v1 / STA AUDV0
	body = append(body, filler...)
	if second {
		body = append(body, 0xA9, v2, 0x85, 0x19) // LDA #v2 / STA AUDV0
	}
	body = append(body, 0xCA) // DEX
	rel := -(len(body) + 2)   // BNE back to the top of body
	if rel < -128 {
		t.Fatalf("loop body of %d bytes is out of branch range", len(body))
	}
	body = append(body, 0xD0, byte(int8(rel)))

	prog := []byte{
		0xA9, 0x02, 0x85, 0x00, // LDA #2 / STA VSYNC
		0x85, 0x02, 0x85, 0x02, 0x85, 0x02, // STA WSYNC ×3
		0xA9, 0x00, 0x85, 0x00, // LDA #0 / STA VSYNC
		0xA9, audf, 0x85, 0x17, // LDA #audf / STA AUDF0
		0xA9, audc, 0x85, 0x15, // LDA #audc / STA AUDC0
		0xA2, 0xFF, // LDX #255
	}
	prog = append(prog, body...)
	prog = append(prog,
		0xA2, 0x04, // LDX #4
		0x85, 0x02, 0xCA, 0xD0, 0xFB, // loop2: STA WSYNC / DEX / BNE
		0x4C, 0x00, 0xF0, // JMP $F000
	)

	rom := make([]byte, 4096)
	copy(rom, prog)
	rom[0x0FFC], rom[0x0FFD] = 0x00, 0xF0
	rom[0x0FFE], rom[0x0FFF] = 0x00, 0xF0
	p := filepath.Join(t.TempDir(), "audvtwice.bin")
	if err := os.WriteFile(p, rom, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// audvTwiceRun plays one variant and returns the distinct sample values it produced on channel 0,
// with their counts, plus what ReadAudio reports at the end.
func audvTwiceRun(t *testing.T, audc, audf, v1, v2 uint8, pad int, second bool) (map[uint8]int, uint8) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(buildAudvTwiceROM(t, audc, audf, v1, v2, pad, second)); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioCapture(); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e)
	e.ResetAudioCapture()
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	ch0, _ := e.AudioSamples()
	hist := map[uint8]int{}
	for _, s := range ch0 {
		hist[s]++
	}
	return hist, e.ReadAudio().Channel0.Volume
}

// TestAudvWrittenTwiceInOneWindowIsAveragedNotReplaced answers a question the register tools
// cannot: what does the machine PLAY when AUDV0 is written twice inside one averaging window?
//
// ★The mechanism. `hardware/tia/audio/audio.go` sums `(pulseCounter & 1) * Volume` every step and
// takes the AVERAGE at fixed points on a free-running 228 counter — `phase1` closes the window at
// `clock228` **36** and **150**, 114 colour clocks = **38 CPU cycles** apart, twice per scanline.
// So a second write does not replace the first; it DILUTES it, in proportion to how much of the
// window each value held. `read_audio`/`read_audio_trace` read `PeekChannels`, which is the
// register, and the register is simply the last value written.
//
// ★★Measured 2026-09-05 with AUDC=0 (constant output, so there is no waveform to confound the
// average), v1=$0F, v2=$00, sweeping the padding between the two writes. Distinct sample values on
// channel 0, and what `read_audio` said:
//
//	one write only ... {15}                 read_audio 15
//	pad  0 ........... {0, 1}               read_audio 0
//	pad  4 ........... {0, 3}               read_audio 0
//	pad  8 ........... {0, 1, 3}            read_audio 0
//	pad 18 ........... {0, 3, 5}            read_audio 0
//	pad 26 ........... {0, 3, 8}            read_audio 0
//	pad 34 ........... {0, 3, 11}           read_audio 0
//	pad 42 ........... {0, 3, 15}           read_audio 0
//
// ★★★Two things are visible at once. The played value climbs monotonically with the padding —
// 1, 3, 5, 8, 11, 15 — which is the weighted average and not noise. And **`read_audio` says 0 at
// every single one of them**: the register tools report a volume the machine never plays. Two
// distinct non-zero values appear per line because there are two windows (36 and 150) and one of
// them catches the first write while the other catches the second.
//
// ★★★★Designed by the mailing-list distillation (helper-2) from the source, with the prediction
// stated before it was run — "an intermediate value for 0 < k < 38, and read_audio disagreeing for
// at least one k". Both landed; the disagreement is at every k tested. The design has to compare
// two runs of the same instruction stream rather than an absolute position in a line, because the
// audio clock is free-running (see `known-traps.md` §E) — that is what this ROM's two variants are.
func TestAudvWrittenTwiceInOneWindowIsAveragedNotReplaced(t *testing.T) {
	const (
		audc = 0x00 // constant output: `actualVolume` has no waveform to confound the average
		audf = 0x00
		v1   = 0x0F
		v2   = 0x00
	)

	single, regSingle := audvTwiceRun(t, audc, audf, v1, v2, 0, false)
	t.Logf("one write per line: samples=%v  read_audio volume=%d", single, regSingle)

	var intermediate []int
	for _, pad := range []int{0, 2, 4, 6, 8, 10, 14, 18, 22, 26, 30, 34, 38, 42} {
		hist, reg := audvTwiceRun(t, audc, audf, v1, v2, pad, true)
		mid := 0
		for v, n := range hist {
			if v != v1 && v != v2 {
				mid += n
			}
		}
		if mid > 0 {
			intermediate = append(intermediate, pad)
		}
		t.Logf("pad=%2d: samples=%v  intermediate=%4d  read_audio volume=%d", pad, hist, mid, reg)
		if reg != v2 {
			t.Errorf("pad=%d: read_audio reports volume %d, but the last write was %d — the "+
				"register read is supposed to be exactly the last value written", pad, reg, v2)
		}
	}

	if len(intermediate) == 0 {
		t.Fatal("no padding produced a sample that is neither the first nor the second value. " +
			"Either the two writes never land in one averaging window (widen the sweep), or this " +
			"engine replaces rather than averages — in which case `read_audio` and the samples " +
			"agree, and the warning in mcp-tools.md is not earned")
	}
	t.Logf("★ intermediate samples at pad = %v — the second write DILUTES the first, and "+
		"read_audio reports only the second", intermediate)

	// ★It is an AVERAGE, not noise: holding v1 for longer must raise what is played. Compare the
	// loudest sample at a short pad against a long one. Without this the test would pass on any
	// engine that merely produced *some* other number.
	loudest := func(pad int) uint8 {
		hist, _ := audvTwiceRun(t, audc, audf, v1, v2, pad, true)
		var max uint8
		for v := range hist {
			if v > max {
				max = v
			}
		}
		return max
	}
	near, far := loudest(4), loudest(34)
	t.Logf("loudest sample: pad=4 → %d, pad=34 → %d", near, far)
	if far <= near {
		t.Errorf("holding the first value %d cycles longer did not make the played sample louder "+
			"(%d → %d). The result is then not a time-weighted average, and the explanation in this "+
			"comment is wrong even though the numbers differ", 30, near, far)
	}

	// ★★Negative control: write the SAME value twice. If anything here were an artefact of writing
	// AUDV0 twice per line — an extra strobe, a reset, a lost cycle — this would show it.
	for _, pad := range []int{4, 18, 34} {
		hist, reg := audvTwiceRun(t, audc, audf, v1, v1, pad, true)
		if reg != v1 {
			t.Errorf("control, pad=%d: read_audio reports %d after two writes of %d", pad, reg, v1)
		}
		for v, n := range hist {
			if v != v1 && v != 0 {
				t.Errorf("control, pad=%d: writing %d twice produced sample value %d (%d times) — "+
					"the intermediate values above are then an artefact of the second WRITE, not of "+
					"the second VALUE", pad, v1, v, n)
			}
		}
	}
}
