package emu

import (
	"fmt"

	"github.com/jetsetilly/gopher2600/hardware/television/signal"
)

// audioCapture is an AudioMixer that accumulates the raw audio sample stream per channel
// (V2-15). The TIA generates 2 samples per scanline (≈31.4kHz NTSC). Where a digest holds
// only a hash, this holds the waveform itself = "pitch" can be measured numerically by
// zero-crossing / autocorrelation (which makes Slocum's pitch table falsifiable).
type audioCapture struct {
	ch0 []uint8
	ch1 []uint8
}

func (c *audioCapture) SetAudio(sig []signal.AudioSignalAttributes) error {
	for _, s := range sig {
		c.ch0 = append(c.ch0, s.AudioChannel0)
		c.ch1 = append(c.ch1, s.AudioChannel1)
	}
	return nil
}

func (c *audioCapture) EndMixing() error { return nil }
func (c *audioCapture) Reset()           { c.ch0 = c.ch0[:0]; c.ch1 = c.ch1[:0] }

// EnableAudioCapture starts collecting raw samples (idempotent).
func (e *Emu) EnableAudioCapture() error {
	if e.acap != nil {
		return nil
	}
	if e.VCS == nil {
		return fmt.Errorf("no VCS")
	}
	e.acap = &audioCapture{}
	e.TV.AddAudioMixer(e.acap)
	return nil
}

// ResetAudioCapture discards the samples accumulated so far (to exclude the warmup).
func (e *Emu) ResetAudioCapture() {
	if e.acap != nil {
		e.acap.Reset()
	}
}

// AudioSamples returns the raw samples collected so far (ch0, ch1).
func (e *Emu) AudioSamples() (ch0, ch1 []uint8) {
	if e.acap == nil {
		return nil, nil
	}
	return e.acap.ch0, e.acap.ch1
}
