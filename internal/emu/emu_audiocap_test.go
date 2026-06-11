package emu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// buildAudioROM は (AUDC,AUDF,AUDV) を鳴らし続ける最小 262 行 ROM を手組みで生成する（V2-14/15 用）。
// dasm 不要＝テストが自己完結。フレーム: VSYNC3 + 255 + 4 = 262 行。
func buildAudioROM(t *testing.T, audc, audf, audv uint8) string {
	t.Helper()
	prog := []byte{
		0xA9, 0x02, // LDA #2
		0x85, 0x00, // STA VSYNC
		0x85, 0x02, 0x85, 0x02, 0x85, 0x02, // STA WSYNC ×3
		0xA9, 0x00, // LDA #0
		0x85, 0x00, // STA VSYNC
		0xA9, audv, // LDA #audv
		0x85, 0x19, // STA AUDV0
		0xA9, audf, // LDA #audf
		0x85, 0x17, // STA AUDF0
		0xA9, audc, // LDA #audc
		0x85, 0x15, // STA AUDC0
		0xA2, 0xFF, // LDX #255
		0x85, 0x02, // loop1: STA WSYNC
		0xCA,       //        DEX
		0xD0, 0xFB, //        BNE loop1
		0xA2, 0x04, // LDX #4
		0x85, 0x02, // loop2: STA WSYNC
		0xCA,       //        DEX
		0xD0, 0xFB, //        BNE loop2
		0x4C, 0x00, 0xF0, // JMP $F000
	}
	rom := make([]byte, 4096)
	copy(rom, prog)
	rom[0x0FFC] = 0x00
	rom[0x0FFD] = 0xF0
	rom[0x0FFE] = 0x00
	rom[0x0FFF] = 0xF0
	p := filepath.Join(t.TempDir(), "audio.bin")
	if err := os.WriteFile(p, rom, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func audioHashFor(t *testing.T, audc uint8) string {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(buildAudioROM(t, audc, 0x14, 0x0A)); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioDigest(); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e)
	e.ResetAudioDigest()
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	return e.AudioHash()
}

// V2-14: サンプル一致の重複 AUDC ペアは音声 digest が一致するはず（{0,11} {4,5} {12,13}）。
// {6,10}/{7,9} は実測で【波形反転】（同調律・同周期、hi/lo 相補）と判明＝digest は別、下のテストで検証。
func TestDuplicateAUDCDigestEquality(t *testing.T) {
	pairs := [][2]uint8{{0, 11}, {4, 5}, {12, 13}}
	for _, p := range pairs {
		h1 := audioHashFor(t, p[0])
		h2 := audioHashFor(t, p[1])
		if h1 != h2 {
			t.Fatalf("AUDC %d vs %d: digests differ (%s vs %s) — duplicate table wrong?", p[0], p[1], h1, h2)
		}
	}
	// 陰性対照: square(4) と bass(6) は当然違う
	if audioHashFor(t, 4) == audioHashFor(t, 6) {
		t.Fatal("AUDC 4 and 6 must differ")
	}
}

// {6,10} と {7,9}: 同周期（=同調律）かつ波形が論理反転（hi サンプル数が相補）であることを実測で固定。
func TestInvertedTwinPairs(t *testing.T) {
	for _, pair := range [][2]uint8{{6, 10}, {7, 9}} {
		testInvertedTwins(t, pair[0], pair[1])
	}
}

func testInvertedTwins(t *testing.T, c1, c2 uint8) {
	capture := func(audc uint8) []uint8 {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(buildAudioROM(t, audc, 0x09, 0x0A)); err != nil {
			t.Fatal(err)
		}
		if err := e.EnableAudioCapture(); err != nil {
			t.Fatal(err)
		}
		warmupStable(t, e)
		e.ResetAudioCapture()
		if err := e.RunFrames(10); err != nil {
			t.Fatal(err)
		}
		ch0, _ := e.AudioSamples()
		return ch0
	}
	a, b := capture(c1), capture(c2)
	// poly 系は遷移が多く平均遷移間隔は使えない → 厳密周期性（s[i]==s[i+310]）で検証
	want := audio.PeriodSamples(int(c1), 9) // 310
	if !audio.IsPeriodic(a, want, 10) {
		t.Fatalf("AUDC %d: not periodic at %d samples", c1, want)
	}
	if !audio.IsPeriodic(b, want, 10) {
		t.Fatalf("AUDC %d: not periodic at %d samples", c2, want)
	}
	n := 3100 // 周期の整数倍で hi 数を数える
	hi := func(s []uint8) (c int) {
		for _, v := range s[:n] {
			if v > 0 {
				c++
			}
		}
		return
	}
	ha, hb := hi(a), hi(b)
	if ha+hb != n {
		t.Fatalf("AUDC %d/%d duty not complementary: %d + %d != %d", c1, c2, ha, hb, n)
	}
}

// V2-15: 生サンプル取得→周期実測が理論値 (AUDF+1)×D と一致するはず。
func TestAudioCapturePitch(t *testing.T) {
	cases := []struct {
		audc, audf uint8
	}{
		{4, 14},  // square: period (14+1)×2 = 30
		{4, 30},  // square: 62
		{12, 14}, // lead:   (14+1)×6 = 90
		{6, 9},   // bass:   (9+1)×31 = 310
	}
	for _, c := range cases {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(buildAudioROM(t, c.audc, c.audf, 0x0A)); err != nil {
			t.Fatal(err)
		}
		if err := e.EnableAudioCapture(); err != nil {
			t.Fatal(err)
		}
		warmupStable(t, e)
		e.ResetAudioCapture()
		if err := e.RunFrames(20); err != nil {
			t.Fatal(err)
		}
		ch0, _ := e.AudioSamples()
		if len(ch0) < 1000 {
			t.Fatalf("AUDC=%d: too few samples (%d)", c.audc, len(ch0))
		}
		want := float64(audio.PeriodSamples(int(c.audc), int(c.audf)))
		got := audio.MeasurePeriod(ch0)
		if got < want*0.97 || got > want*1.03 {
			t.Fatalf("AUDC=%d AUDF=%d: measured period %.2f, want %.0f (±3%%)", c.audc, c.audf, got, want)
		}
	}
}
