package emu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// pcmROM rebuilds litmus_pcm with a chosen AUDC0, changing that and nothing else.
func pcmROM(t *testing.T, audc string) string {
	t.Helper()
	src, err := os.ReadFile("../../roms/litmus/litmus_pcm.asm")
	if err != nil {
		t.Fatal(err)
	}
	// The litmus feeds one `lda #0` to three stores, so replacing it would move AUDF0 and AUDV0
	// as well. Split it first: that confound is the reason this helper exists.
	const old = "        lda #0\n        sta AUDC0\n        sta AUDF0\n        sta AUDV0"
	if !strings.Contains(string(src), old) {
		t.Fatal("litmus_pcm.asm's setup block has changed shape; this test would be editing " +
			"something other than AUDC0")
	}
	out := strings.Replace(string(src), old,
		"        lda #"+audc+"\n        sta AUDC0\n        lda #0\n        sta AUDF0\n        sta AUDV0", 1)

	dir := t.TempDir()
	asm := filepath.Join(dir, "p.asm")
	if err := os.WriteFile(asm, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	// The litmus includes from its own directory, so assemble it in place of the original.
	inPlace := filepath.Join("../../roms/litmus", "zz_pcm_variant.asm")
	if err := os.WriteFile(inPlace, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(inPlace) })
	bin := filepath.Join(dir, "p.bin")
	if o, err := build.Assemble(inPlace, bin); err != nil {
		t.Fatalf("AUDC0=%s did not assemble: %v\n%s", audc, err, o)
	}
	return bin
}

// TestZeroAndElevenBothParkTheToneGenerator settles a 1999 sentence that `tia-pcm.md` uses half of.
//
// Eckhard Stolberg, asked how to get one-bit sound out of the TIA: *"If you set the AUDCx register to
// **0 or 11**, the output will **always be high**. You can generate complex waves by quickly changing
// the AUDVx register for that voice"* 〔`199902/msg00036`〕.
//
// `docs/techniques/tia-pcm.md` takes the first half — *"Silence the tone generators. AUDC=0 on both
// channels"* — and `litmus_pcm.asm` says *"AUDC0 = 0 and AUDF0 = 0 for the whole run: AUDV0 IS the
// amplitude"*. **11 appears nowhere.**
//
// Measured 2026-09-07 by rebuilding the litmus with five AUDC0 values and comparing the audio mix
// digest over ten frames:
//
//	AUDC = 0    d323059a…   the reference
//	AUDC = 1    1e6efd69…   different
//	AUDC = 4    d33c513f…   different
//	AUDC = 11   d323059a…   IDENTICAL
//	AUDC = 12   160c6c9a…   different
//
// ★**Exactly 0 and 11, and both of their neighbours differ.** So the precondition for PCM is not
// "AUDC must be zero" but "AUDC must be **0 or 11**", and a driver that already has 11 in the
// register does not have to write anything.
//
// ★★**The confound this nearly had**: the litmus feeds one `lda #0` to three stores — AUDC0, AUDF0
// and AUDV0 — so changing that literal would have moved the frequency and the volume too, and the
// digests would have differed for reasons having nothing to do with the tone generator. The helper
// splits the load first.
//
// Found by the mailing-list distillation (helper-1).
func TestZeroAndElevenBothParkTheToneGenerator(t *testing.T) {
	digest := func(audc string) string {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(pcmROM(t, audc)); err != nil {
			t.Fatal(err)
		}
		if err := e.EnableMixDigest(); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
		e.ResetMixDigest() // exclude the warmup
		if err := e.RunFrames(10); err != nil {
			t.Fatal(err)
		}
		return e.MixHash()
	}

	base := digest("0")
	if base == "" {
		t.Fatal("the AUDC=0 run produced no digest; nothing below is comparing anything")
	}
	if d := digest("11"); d != base {
		t.Errorf("AUDC=11 gives %s and AUDC=0 gives %s. Stolberg's sentence says both park the tone "+
			"generator, and tia-pcm.md's precondition would then be wrong in the other direction",
			d, base)
	}
	// Both neighbours, so "0 and 11" is a pair of points and not a range.
	for _, other := range []string{"1", "4", "12"} {
		if d := digest(other); d == base {
			t.Errorf("AUDC=%s also matches AUDC=0. If more than 0 and 11 silence the generator the "+
				"finding is bigger than the source claims and should be re-derived, not widened here",
				other)
		}
	}
}
