package emu

import (
	"os"
	"path/filepath"
	"testing"
)

// twoChannelROM plays a fixed channel 0 and a channel 1 whose volume is the parameter,
// so "does this check see the second channel" is answerable by moving one byte.
func twoChannelROM(t *testing.T, audv1 uint8) string {
	t.Helper()
	prog := []byte{
		0xA9, 0x0A, 0x85, 0x19, // LDA #$0A / STA AUDV0
		0xA9, 0x09, 0x85, 0x17, // LDA #9   / STA AUDF0
		0xA9, 0x04, 0x85, 0x15, // LDA #4   / STA AUDC0  (square)
		0xA9, audv1, 0x85, 0x1A, // LDA #v  / STA AUDV1
		0xA9, 0x14, 0x85, 0x18, // LDA #20 / STA AUDF1
		0xA9, 0x0C, 0x85, 0x16, // LDA #12 / STA AUDC1  (lead)
		// the frame loop starts at $F018, so the registers are written once
		0xA9, 0x02, 0x85, 0x00,
		0x85, 0x02, 0x85, 0x02, 0x85, 0x02,
		0xA9, 0x00, 0x85, 0x00,
		0xA2, 0xFF, 0x85, 0x02, 0xCA, 0xD0, 0xFB,
		0xA2, 0x04, 0x85, 0x02, 0xCA, 0xD0, 0xFB,
		0x4C, 0x18, 0xF0,
	}
	rom := make([]byte, 4096)
	copy(rom, prog)
	rom[0x0FFC], rom[0x0FFD] = 0x00, 0xF0
	rom[0x0FFE], rom[0x0FFF] = 0x00, 0xF0
	p := filepath.Join(t.TempDir(), "twochannel.bin")
	if err := os.WriteFile(p, rom, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func hashesFor(t *testing.T, audv1 uint8) (audio, mixed string) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(twoChannelROM(t, audv1)); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioDigest(); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableMixDigest(); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e)
	e.ResetAudioDigest()
	e.ResetMixDigest()
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	return e.AudioHash(), e.MixHash()
}

// THE WITNESS, and the hole it was written for. Seven scenarios gate on `golden_audio`,
// and it hashes AudioChannel0 alone -- so a ROM can turn its second voice from silent to
// full and every one of them stays green.
func TestTheAudioGoldenIsBlindToChannelOneAndTheMixDigestIsNot(t *testing.T) {
	silent, silentMix := hashesFor(t, 0x00)
	half, halfMix := hashesFor(t, 0x07)
	full, fullMix := hashesFor(t, 0x0F)

	t.Logf("AUDV1 =  0  audio %s  mixed %s", silent[:16], silentMix[:16])
	t.Logf("AUDV1 =  7  audio %s  mixed %s", half[:16], halfMix[:16])
	t.Logf("AUDV1 = 15  audio %s  mixed %s", full[:16], fullMix[:16])

	// The hole, asserted so it cannot be quietly fixed upstream without this test noticing
	// and telling us the mix digest is no longer load-bearing.
	if silent != half || half != full {
		t.Errorf("the existing audio golden now MOVES with channel 1 (%s / %s / %s). That is an "+
			"improvement, but this test and the mix digest were written because it did not -- "+
			"re-check whether EnableMixDigest is still buying anything",
			silent[:16], half[:16], full[:16])
	}

	// The fix.
	if silentMix == halfMix || halfMix == fullMix || silentMix == fullMix {
		t.Errorf("the mix digest does NOT separate channel 1 at 0, 7 and 15 (%s / %s / %s); it "+
			"exists for exactly this and is worth nothing if it cannot",
			silentMix[:16], halfMix[:16], fullMix[:16])
	}
}

// The negative control. A check that changes every run would also pass the test above and
// would be useless as a golden, so the same ROM twice must hash the same.
func TestTheMixDigestIsDeterministic(t *testing.T) {
	a1, m1 := hashesFor(t, 0x07)
	a2, m2 := hashesFor(t, 0x07)
	if m1 != m2 {
		t.Errorf("the same ROM hashed %s then %s; a golden that moves on its own gates nothing",
			m1[:16], m2[:16])
	}
	if a1 != a2 {
		t.Errorf("the existing audio digest is non-deterministic too: %s vs %s", a1[:16], a2[:16])
	}
}

// The mix digest must also still catch everything the old one caught, or it is a
// replacement that loses coverage rather than adding it.
func TestTheMixDigestStillSeesChannelZero(t *testing.T) {
	var mixes []string
	for _, v := range []uint8{0x05, 0x0F} {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(buildAudioROM(t, 4, 9, v)); err != nil {
			t.Fatal(err)
		}
		if err := e.EnableMixDigest(); err != nil {
			t.Fatal(err)
		}
		warmupStable(t, e)
		e.ResetMixDigest()
		if err := e.RunFrames(10); err != nil {
			t.Fatal(err)
		}
		mixes = append(mixes, e.MixHash())
	}
	if mixes[0] == mixes[1] {
		t.Errorf("channel 0 volume 5 and 15 hash the same (%s); the mix digest cannot replace "+
			"the audio golden if it is blind to the channel that one could see", mixes[0][:16])
	}
}
