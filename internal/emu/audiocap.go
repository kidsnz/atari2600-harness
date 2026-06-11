package emu

import (
	"fmt"

	"github.com/jetsetilly/gopher2600/hardware/television/signal"
)

// audioCapture は生の音声サンプル列をチャンネル別に蓄積する AudioMixer（V2-15）。
// TIA は 1 scanline あたり 2 サンプル生成（≈31.4kHz NTSC）。digest がハッシュしか持たないのに対し、
// こちらは波形そのもの＝ゼロ交差/自己相関で「音程」を数値測定できる（Slocum 音程表の反証可能化）。
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

// EnableAudioCapture は生サンプルの取得を開始する（冪等）。
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

// ResetAudioCapture は蓄積済みサンプルを破棄する（warmup 除外用）。
func (e *Emu) ResetAudioCapture() {
	if e.acap != nil {
		e.acap.Reset()
	}
}

// AudioSamples は取得済みの生サンプル（ch0, ch1）を返す。
func (e *Emu) AudioSamples() (ch0, ch1 []uint8) {
	if e.acap == nil {
		return nil, nil
	}
	return e.acap.ch0, e.acap.ch1
}
