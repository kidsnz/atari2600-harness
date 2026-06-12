package audio

import (
	"math"
	"testing"
)

// Stolberg のスポット値と式の一致（出典: frequency and waveform guide）。
func TestFreqSpotChecks(t *testing.T) {
	// AUDC=4 (square), AUDF=14 → ≈1047Hz (C6)
	f := Freq(4, 14, BaseClockNTSC)
	if math.Abs(f-1046.65) > 1.0 {
		t.Fatalf("square AUDF14 = %.2f, want ≈1046.65", f)
	}
	// AUDC=12 (lead), AUDF=4 → 同じく ≈C6（divider 6: 31399.5/5/6）
	f2 := Freq(12, 4, BaseClockNTSC)
	if math.Abs(f2-1046.65) > 1.0 {
		t.Fatalf("lead AUDF4 = %.2f, want ≈1046.65", f2)
	}
	// AUDC=1 (saw), AUDF=0 → ≈2093Hz (C7)
	f3 := Freq(1, 0, BaseClockNTSC)
	if math.Abs(f3-2093.3) > 2.0 {
		t.Fatalf("saw AUDF0 = %.2f, want ≈2093.3", f3)
	}
	// PAL は NTSC より低い
	if Freq(4, 14, BaseClockPAL) >= f {
		t.Fatal("PAL must be flatter than NTSC")
	}
}

func TestCanonicalDuplicates(t *testing.T) {
	pairs := map[int]int{11: 0, 5: 4, 10: 6, 9: 7, 13: 12}
	for dup, canon := range pairs {
		if Canonical(dup) != canon {
			t.Fatalf("Canonical(%d) = %d, want %d", dup, Canonical(dup), canon)
		}
		if Name(dup) != Name(canon) {
			t.Fatalf("Name(%d) != Name(%d)", dup, canon)
		}
	}
}

func TestNoteByteRoundtrip(t *testing.T) {
	for idx := 0; idx < 8; idx++ {
		for audf := 0; audf < 32; audf++ {
			if idx == 7 && audf == 31 {
				continue // $FF=休符と衝突（フォーマット固有の曖昧さ・doc 参照）
			}
			b := NoteByte(idx, audf)
			i2, f2 := DecodeNoteByte(b)
			if i2 != idx || f2 != audf {
				t.Fatalf("roundtrip (%d,%d) -> %02x -> (%d,%d)", idx, audf, b, i2, f2)
			}
		}
	}
	if i, f := DecodeNoteByte(0xFF); i != -1 || f != -1 {
		t.Fatal("0xFF must decode as rest")
	}
}

func TestMeasurePeriodSynthetic(t *testing.T) {
	// 周期 30（15 high / 15 low）の合成矩形波
	var s []uint8
	for i := 0; i < 600; i++ {
		if (i/15)%2 == 0 {
			s = append(s, 10)
		} else {
			s = append(s, 0)
		}
	}
	p := MeasurePeriod(s)
	if math.Abs(p-30) > 0.5 {
		t.Fatalf("measured %.2f, want 30", p)
	}
	// DC は 0
	if MeasurePeriod(make([]uint8, 100)) != 0 {
		t.Fatal("DC must measure 0")
	}
}

func TestNoteFreqAndFind(t *testing.T) {
	f, err := NoteFreq("A4")
	if err != nil || math.Abs(f-440) > 0.01 {
		t.Fatalf("A4=%f err=%v", f, err)
	}
	f, _ = NoteFreq("C4")
	if math.Abs(f-261.63) > 0.1 {
		t.Fatalf("C4=%f", f)
	}
	// C6 は square AUDF14 が ≈1047Hz（既存スポット値と整合）
	c, fr, cents, err := FindNote("C6", []int{4}, BaseClockNTSC)
	if err != nil || c != 4 || fr != 14 || math.Abs(cents) > 10 {
		t.Fatalf("C6 -> audc=%d audf=%d cents=%f err=%v", c, fr, cents, err)
	}
}
