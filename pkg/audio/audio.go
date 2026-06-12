// Package audio は Atari 2600 TIA 音声の普遍知識（公開・再利用可）。
// 出典: Paul Slocum "Atari 2600 Music And Sound Programming Guide" v1.02（権威・ローカル保有）、
// Eckhard Stolberg "frequency and waveform guide"、Stella Programmer's Guide。
// 検証: harness の audio capture（生サンプル→周期測定, internal/emu）で数値裏取り（V2-14/15）。
package audio

import (
	"fmt"
	"math"
	"strings"
)

// BaseClockNTSC は TIA 音声クロック（Hz）。カラークロック/114 = 2 サンプル/scanline。
const BaseClockNTSC = 3579545.0 / 114.0 // ≈ 31399.5
// BaseClockPAL は PAL の音声クロック（≈13 セント低い）。
const BaseClockPAL = 3546894.0 / 114.0 // ≈ 31113.1

// Name は AUDC 値の通称（Slocum 命名）。重複は正準値の名を返す。
func Name(audc int) string {
	switch Canonical(audc) {
	case 0:
		return "silent"
	case 1:
		return "saw"
	case 2:
		return "rumble"
	case 3:
		return "engine"
	case 4:
		return "square"
	case 6:
		return "bass"
	case 7:
		return "pitfall"
	case 8:
		return "noise"
	case 12:
		return "lead"
	case 14:
		return "low bass"
	case 15:
		return "buzz"
	}
	return "unknown"
}

// Canonical は重複 AUDC（{0,11} {4,5} {6,10} {7,9} {12,13}）を正準値へ畳む。
// 注（V2-14 実測, Gopher2600）: {0,11} {4,5} {12,13} は【サンプル一致】の完全重複。
// {6,10} と {7,9} は【同調律・同周期だが波形が論理反転】（hi/lo が相補）＝耳には同じ、サンプル列は別。
// 文書（Stolberg/Slocum）の「重複」は調律の意味では正しいが、サンプル単位では2種類に分かれる。
func Canonical(audc int) int {
	switch audc & 0x0F {
	case 11:
		return 0
	case 5:
		return 4
	case 10:
		return 6
	case 9:
		return 7
	case 13:
		return 12
	}
	return audc & 0x0F
}

// Divisor は AUDC 波形の繰り返し長 D（音声クロック tick 単位）。周波数 = base/(AUDF+1)/D。
// 0/11 は DC（無音）= 0。8 はノイズ（511bit poly, 音程感なし）。
func Divisor(audc int) int {
	switch Canonical(audc) {
	case 1:
		return 15
	case 2, 3:
		return 465
	case 4:
		return 2
	case 6, 7:
		return 31
	case 8:
		return 511
	case 12:
		return 6
	case 14, 15:
		return 93
	}
	return 0
}

// Freq は (AUDC, AUDF) の基本周波数（Hz）。音程の無いモード（DC/ノイズ）は 0。
func Freq(audc, audf int, baseClock float64) float64 {
	d := Divisor(audc)
	if d == 0 || Canonical(audc) == 8 {
		return 0
	}
	return baseClock / float64(audf+1) / float64(d)
}

// PeriodSamples は (AUDC, AUDF) の理論周期（サンプル数=音声クロック tick 数）。
func PeriodSamples(audc, audf int) int {
	d := Divisor(audc)
	if d == 0 {
		return 0
	}
	return (audf + 1) * d
}

// MeasurePeriod は生サンプル列から支配的な繰り返し周期（サンプル数）を測る（矩形波系向け:
// 値の遷移間隔の平均×2）。遷移が 3 未満なら 0（無音/DC）。
func MeasurePeriod(samples []uint8) float64 {
	if len(samples) < 4 {
		return 0
	}
	var transitions []int
	for i := 1; i < len(samples); i++ {
		if samples[i] != samples[i-1] {
			transitions = append(transitions, i)
		}
	}
	if len(transitions) < 3 {
		return 0
	}
	// 遷移間隔の平均 = 半周期
	first, last := transitions[0], transitions[len(transitions)-1]
	half := float64(last-first) / float64(len(transitions)-1)
	return half * 2
}

// IsPeriodic は samples が厳密に period で繰り返すか（s[i]==s[i+period]）を最低 minPeriods 周期ぶん検査する。
// poly 波形（saw/pitfall/engine 等＝遷移が多く MeasurePeriod が使えない波形）の周期検証はこちらを使う。
func IsPeriodic(samples []uint8, period, minPeriods int) bool {
	if period <= 0 || len(samples) < period*(minPeriods+1) {
		return false
	}
	n := period * minPeriods
	for i := 0; i < n; i++ {
		if samples[i] != samples[i+period] {
			return false
		}
	}
	return true
}

// NoteByte は Sequencer Kit / slocum-tracker 互換の音符バイト（上位3bit=音色 idx, 下位5bit=AUDF）。
// idx は soundTypeArray のインデックス（既定: 4,6,7,8,15,12,1,3）。$FF は休符。
// 形式固有の曖昧さ: (idx=7, AUDF=31) は $FF＝休符と衝突するため使用不可（フォーマットの仕様）。
func NoteByte(soundTypeIdx, audf int) uint8 {
	return uint8((soundTypeIdx&0x07)<<5 | (audf & 0x1F))
}

// DecodeNoteByte は音符バイトを (音色 idx, AUDF) に戻す。$FF は (-1, -1)（休符）。
func DecodeNoteByte(b uint8) (soundTypeIdx, audf int) {
	if b == 0xFF {
		return -1, -1
	}
	return int(b >> 5), int(b & 0x1F)
}

// --- 音名 → TIA（R7: 作曲セッション支援）---

var noteOffsets = map[string]int{
	"C": -9, "C#": -8, "DB": -8, "D": -7, "D#": -6, "EB": -6, "E": -5,
	"F": -4, "F#": -3, "GB": -3, "G": -2, "G#": -1, "AB": -1, "A": 0,
	"A#": 1, "BB": 1, "B": 2,
}

// NoteFreq は 12 平均律の音名（"C4", "F#3", "Bb2" 等, A4=440Hz）を周波数へ。
func NoteFreq(name string) (float64, error) {
	n := strings.ToUpper(strings.TrimSpace(name))
	if len(n) < 2 {
		return 0, fmt.Errorf("bad note %q", name)
	}
	octIdx := len(n) - 1
	pitch := n[:octIdx]
	oct := int(n[octIdx] - '0')
	if oct < 0 || oct > 9 {
		return 0, fmt.Errorf("bad octave in %q", name)
	}
	off, ok := noteOffsets[pitch]
	if !ok {
		return 0, fmt.Errorf("bad pitch %q", name)
	}
	semis := float64((oct-4)*12 + off)
	return 440.0 * math.Pow(2, semis/12.0), nil
}

// FindNote は音名に最も近い (AUDC, AUDF) を返す（セント誤差つき）。
// types を空にすると実用音色 {4(square), 6(bass), 7(buzz), 12(lead)} から探す。
func FindNote(name string, types []int, baseClock float64) (audc, audf int, cents float64, err error) {
	target, err := NoteFreq(name)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(types) == 0 {
		types = []int{4, 6, 7, 12}
	}
	best := math.Inf(1)
	for _, c := range types {
		for f := 0; f < 32; f++ {
			got := Freq(c, f, baseClock)
			if got <= 0 {
				continue
			}
			ct := 1200 * math.Log2(got/target)
			if math.Abs(ct) < math.Abs(best) {
				best, audc, audf = ct, c, f
			}
		}
	}
	return audc, audf, best, nil
}
