package main

import "testing"

// TestRomAddrOfUsesTheWindowNotTheFileLength pins that a ROM offset resolves to an
// address the machine can actually fetch from.
//
// The naive origin (0x10000 - len) is right for a flat image and meaningless for a
// banked one: an 8K cartridge would start at $E000, which the 2600 never fetches,
// because both 4K banks live in the same $F000-$FFFF window. This file used to carry
// two copies of the mapping — one that knew that and one that did not — and the one
// that did not was matching annotations to DiStella labels by address, so on an 8K
// ROM every offset in bank 0 resolved to $Exxx, matched nothing, and its annotation
// vanished silently.
func TestRomAddrOfUsesTheWindowNotTheFileLength(t *testing.T) {
	cases := []struct {
		name   string
		romLen int
		off    int
		want   int
	}{
		{"4K first byte", 4096, 0x000, 0xF000},
		{"4K last byte", 4096, 0xFFF, 0xFFFF},
		{"2K first byte", 2048, 0x000, 0xF800},
		{"2K last byte", 2048, 0x7FF, 0xFFFF},
		{"8K bank 0 first byte", 8192, 0x0000, 0xF000},
		{"8K bank 0 last byte", 8192, 0x0FFF, 0xFFFF},
		{"8K bank 1 first byte", 8192, 0x1000, 0xF000},
		{"8K bank 1 last byte", 8192, 0x1FFF, 0xFFFF},
		{"16K bank 3 first byte", 16384, 0x3000, 0xF000},
	}
	for _, c := range cases {
		if got := romAddrOf(c.romLen, c.off); got != c.want {
			t.Errorf("%s: romAddrOf(%d, $%04X) = $%04X, want $%04X",
				c.name, c.romLen, c.off, got, c.want)
		}
	}
	// The specific failure: bank 0 of an 8K image must not land outside the window.
	for off := 0; off < 0x1000; off += 0x137 {
		if a := romAddrOf(8192, off); a < 0xF000 || a > 0xFFFF {
			t.Fatalf("8K bank 0 offset $%04X resolved to $%04X, outside $F000-$FFFF — this is the "+
				"address that matched no DiStella label and dropped the annotation", off, a)
		}
	}
}
