package ocr

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// scoreFont mirrors roms/litmus/score2.asm's FONT table (the ground-truth spec).
var scoreFont = Font{
	{0x3C, 0x42, 0x46, 0x5A, 0x62, 0x42, 0x3C, 0x00}, // 0
	{0x08, 0x18, 0x08, 0x08, 0x08, 0x08, 0x1C, 0x00}, // 1
	{0x3C, 0x42, 0x02, 0x0C, 0x30, 0x40, 0x7E, 0x00}, // 2
	{0x3C, 0x42, 0x02, 0x1C, 0x02, 0x42, 0x3C, 0x00}, // 3
	{0x0C, 0x14, 0x24, 0x44, 0x7E, 0x04, 0x04, 0x00}, // 4
	{0x7E, 0x40, 0x7C, 0x02, 0x02, 0x42, 0x3C, 0x00}, // 5
	{0x1C, 0x20, 0x40, 0x7C, 0x42, 0x42, 0x3C, 0x00}, // 6
	{0x7E, 0x02, 0x04, 0x08, 0x10, 0x10, 0x10, 0x00}, // 7
	{0x3C, 0x42, 0x42, 0x3C, 0x42, 0x42, 0x3C, 0x00}, // 8
	{0x3C, 0x42, 0x42, 0x3E, 0x02, 0x04, 0x38, 0x00}, // 9
}

func decode(t *testing.T, rom string) (Result, byte) {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}
	img, _ := e.Snapshot()
	ram, err := e.PeekRAM(0x80)
	if err != nil {
		t.Fatal(err)
	}
	return ReadScore2(img, scoreFont), ram
}

// TestScoreOCRSelfTest is VV-9's falsifiable self-test: the genuine ROM decodes
// to its RAM score; a font-index mutation (glyph '8' copied over glyph '4', so
// the tens digit renders as 8 while RAM still says 4) is caught as displayed != RAM.
func TestScoreOCRSelfTest(t *testing.T) {
	// 1) genuine ROM: displayed digits == decode(RAM score $42).
	res, ram := decode(t, "../../roms/litmus/score2.bin")
	if !res.OK {
		t.Fatalf("OCR did not find the digit band")
	}
	if res.Tens != 4 || res.Ones != 2 {
		t.Fatalf("decoded %d%d, want 42 (dists %d/%d)", res.Tens, res.Ones, res.TensDist, res.OnesDist)
	}
	if res.ExpectedBCD() != ram {
		t.Fatalf("displayed BCD 0x%02X != RAM 0x%02X", res.ExpectedBCD(), ram)
	}

	// 2) font-index mutation: copy glyph 8 over glyph 4 in the ROM's font.
	bin, err := os.ReadFile("../../roms/litmus/score2.bin")
	if err != nil {
		t.Fatal(err)
	}
	sig := []byte{0x3C, 0x42, 0x46, 0x5A, 0x62, 0x42, 0x3C, 0x00} // glyph 0 = FONT start
	fontOff := bytes.Index(bin, sig)
	if fontOff < 0 {
		t.Fatalf("font table not found in ROM")
	}
	copy(bin[fontOff+4*8:fontOff+5*8], bin[fontOff+8*8:fontOff+9*8]) // glyph4 := glyph8
	mut := filepath.Join(t.TempDir(), "score2_mut.bin")
	if err := os.WriteFile(mut, bin, 0o644); err != nil {
		t.Fatal(err)
	}
	res2, ram2 := decode(t, mut)
	if !res2.OK {
		t.Fatalf("OCR did not find the band in the mutant")
	}
	if res2.ExpectedBCD() == ram2 {
		t.Fatalf("font-index bug not caught: displayed 0x%02X == RAM 0x%02X (decoded %d%d)",
			res2.ExpectedBCD(), ram2, res2.Tens, res2.Ones)
	}
}
