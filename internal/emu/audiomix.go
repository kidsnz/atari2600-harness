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
