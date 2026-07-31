package emu

// A truncated download must fail to load, not kill the process.
//
// Found by sweeping every .bin under the umbrella through LoadROM (542 images,
// 2026-07-31): two of them PANICKED rather than returning an error — a 12-byte
// `Combat.bin` and a 5-byte `skeleton_test.bin`, both partial downloads left in a
// mined reference archive. The fault is in the engine's fingerprinter, which slices
// `d[:0x80]` on every 4K window without checking the length, but the consequence
// lands here: `load_rom` and `assemble_and_load` take a path from the MCP caller and
// `cmd/fieldtest -inbox` walks a directory the user drops files into, so a panic ends
// the server rather than one call.
//
// The fixtures carry the OBSERVED BYTES, not files of that length. The first version
// of this test wrote zero-filled files of the same sizes, and removing the guard did
// not make them panic — they were quietly accepted as tiny cartridges instead. So the
// test proved the size check refuses short files while proving nothing at all about
// the crash it is named after. The real content is twelve and five bytes of 6502
// prologue (`sei` / `cld` / `ldx #$FF` / `txs` ...), and it is that content reaching
// the fingerprinter that faults. With the guard removed, both of these panic; the
// zero-filled ones do not.
//
// The empty file and the byte below the threshold are kept as well, so a guard that
// special-cased two known sizes rather than a range would still fail.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadROMRejectsTruncatedImagesWithoutPanicking(t *testing.T) {
	dir := t.TempDir()

	// The two panicking images, byte for byte, as found in
	// reference/disassemblies/bjars_site_archive/public_html/source/.
	combatTruncated := []byte{0x78, 0xd8, 0xa2, 0xff, 0x9a, 0xa2, 0x5d, 0x20, 0x07, 0xf6, 0xa9, 0x10}
	skeletonTruncated := []byte{0x78, 0xd8, 0xa2, 0x00, 0x8a}

	for _, c := range []struct {
		name string
		data []byte
	}{
		{"combat-truncated.bin", combatTruncated},        // panicked, 12 bytes
		{"skeleton_test-truncated.bin", skeletonTruncated}, // panicked, 5 bytes
		{"empty.bin", nil},
		{"one-below-threshold.bin", make([]byte, minCartBytes-1)},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name)
			if err := os.WriteFile(path, c.data, 0o644); err != nil {
				t.Fatalf("writing fixture: %v", err)
			}

			e, err := New("NTSC")
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			// A panic escaping here fails the test by crashing it, which is the
			// behaviour being ruled out; no recover, because recovering would hide
			// exactly what this test exists to detect.
			err = e.LoadROM(path)
			if err == nil {
				t.Fatalf("a %d-byte file loaded successfully; a malformed image must be reported, "+
					"not accepted", len(c.data))
			}
			if !strings.Contains(err.Error(), "bytes") {
				t.Errorf("error does not say what is wrong with the file: %v", err)
			}
		})
	}
}

// TestLoadROMStillLoadsARealImage is the other half: the guard must not have been
// bought by refusing things it should accept. Without this, returning an error
// unconditionally would pass the test above.
func TestLoadROMStillLoadsARealImage(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const rom = "../../roms/litmus/litmus_pos.bin"
	if _, statErr := os.Stat(rom); statErr != nil {
		t.Fatalf("fixture %s missing: %v", rom, statErr)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatalf("a valid litmus ROM was refused: %v", err)
	}
	if id, banks := e.CartInfo(); id == "" || banks < 1 {
		t.Fatalf("loaded but reports no cartridge: id=%q banks=%d", id, banks)
	}
}
