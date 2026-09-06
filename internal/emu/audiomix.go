package emu

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/jetsetilly/gopher2600/hardware/television/signal"
	"github.com/jetsetilly/gopher2600/hardware/tia/audio/mix"
)

// mixDigest is an AudioMixer that hashes what a SPEAKER would receive: both channels,
// through the TIA's own non-linear output stage.
//
// WHY THIS EXISTS, and it is not a refinement of the existing golden audio -- it covers
// two things that had no coverage at all.
//
//  1. THE EXISTING AUDIO GOLDEN CANNOT SEE CHANNEL 1. Gopher2600's digest.Audio hashes
//     `s.AudioChannel0` and nothing else (Gopher2600/digest/audio.go:78). Measured on a
//     ROM playing a fixed channel 0 against a channel 1 swept silent / half / full, the
//     hash is byte-identical at all three -- 44cc324ba5783a68 every time -- while the
//     control moves correctly when channel 0's volume changes. Seven scenarios gate on
//     `golden_audio`; not one of them can see half of the sound.
//
//  2. NOTHING HERE HAD EVER EXERCISED THE MIXER. `mix.Mono` is the TIA's output stage and
//     it is NOT a sum: `mono[c0+c1] >> 1`, indexed by the SUM of the two 4-bit volumes
//     into a hyperbolic curve. Superposition fails by up to 25% (Mono(15,15) = 16383
//     against Mono(15,0)+Mono(0,15) = 21844), and a loud channel 1 squashes channel 0 --
//     stepping channel 0 from 0 to 8 adds 6898 in silence but only 3297 against a full
//     channel 1, 48% of it. That squashing IS the two-channel interference that older
//     emulators are criticised for omitting, and Gopher2600 models it (its audio is
//     Chris Brenner's circuit-derived implementation, not Ron Fries' -- the Fries tables
//     in polynomials.go are dead code no caller references). We simply never looked at it:
//     grep finds no use of the mix package anywhere outside Gopher2600, and every audio
//     tool here reads the raw pre-mix 4-bit channels.
//
// So a ROM could change what a listener hears -- a second voice appearing, a balance
// shifting, one channel drowning the other -- and every audio check would stay green.
type mixDigest struct {
	h   [sha1.Size]byte
	buf []byte
}

// ★`mix.Mono` has a SILENT failure mode, and this note is here because someone will read the
// engine's comment and take it at face value. Past a channel sum of `0x1e` the engine returns
// **zero — silence, not clipping**:
//
//	// boundary check. in very rare instances, the sum will be more than 0x1e so we
//	// check and return zero if it is
//	// … it is acceptable to return zero and not worry about the root cause too much
//	// update: this should no longer happen because the average channel volume is
//	// now masked to a maximum of four bits (see audio.Step() function)
//
// ★★A guard was written here on 2026-09-06 and REMOVED the same hour, because it could not be
// made to fire. Three probes, each applied to the vendored engine, measured and reverted:
// widening `audio.Step()`'s `& 0x0f` to `& 0x1f`; widening the register write's
// `Volume = data.Value & 0x0f` likewise; and both at once, with a ROM writing `AUDV = $1F` to
// both channels. **The sum reaching this function stayed inside `0x1e` every time.** So there is
// a third cap on the path between the register and the mixer that this session did not locate,
// and the engine's comment credits a mask that is at best one of several.
//
// ★★★What that means for anyone extending this: the branch is real, its symptom is the sound
// going AWAY, and a digest of silence is a perfectly stable digest — but no ROM available here
// can reach it, so a guard would be a branch nothing walks. Find the third cap first; then the
// guard has a witness and is worth having.
//
// Found by the mailing-list distillation (helper-2) while chasing a 2004 report of a two-voice
// tune that was *"brilliant and crystal clear"* on an emulator and *"horribly distorted"* on real
// hardware, curable by *"turning the volume down"* to 10 and 8 〔stella-list `200405/msg00275`,
// cybergoth〕. That thread also settles its own question: the PAL-mono hypothesis was disproved
// two days later — *"I did your test … on a PAL and a NTSC 2600 Jr. I got the same results on
// both consoles"* 〔`msg00286`, Eckhard Stolberg〕 — and the word offered for the symptom was
// *"clipping"* 〔`msg00281`, B. Watson〕.

func (d *mixDigest) SetAudio(sig []signal.AudioSignalAttributes) error {
	for _, s := range sig {
		v := mix.Mono(s.AudioChannel0, s.AudioChannel1)
		d.buf = append(d.buf, byte(v>>8), byte(v))
	}
	if len(d.buf) >= 4096 {
		d.flush()
	}
	return nil
}

// flush chains the buffered samples into the running hash, the same shape as the video
// and audio digests: hash = sha1(previous hash || new samples).
func (d *mixDigest) flush() {
	if len(d.buf) == 0 {
		return
	}
	d.h = sha1.Sum(append(d.h[:], d.buf...))
	d.buf = d.buf[:0]
}

func (d *mixDigest) EndMixing() error { d.flush(); return nil }
func (d *mixDigest) Reset()           { d.h = [sha1.Size]byte{}; d.buf = d.buf[:0] }

// EnableMixDigest starts hashing the mixed output of both channels (idempotent).
func (e *Emu) EnableMixDigest() error {
	if e.mdigest != nil {
		return nil
	}
	if e.VCS == nil {
		return fmt.Errorf("no VCS")
	}
	e.mdigest = &mixDigest{}
	e.TV.AddAudioMixer(e.mdigest)
	return nil
}

// ResetMixDigest restarts the chain from zero, to exclude the warmup.
func (e *Emu) ResetMixDigest() {
	if e.mdigest != nil {
		e.mdigest.Reset()
	}
}

// MixHash returns the chained hash of the mixed output so far ("" if not enabled).
func (e *Emu) MixHash() string {
	if e.mdigest == nil {
		return ""
	}
	d := *e.mdigest // hash without disturbing the running chain
	d.flush()
	return hex.EncodeToString(d.h[:])
}
