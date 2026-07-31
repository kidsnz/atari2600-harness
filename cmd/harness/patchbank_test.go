package main

import (
	"strings"
	"testing"
)

type nopEmu struct{ loaded []string }

func (n *nopEmu) LoadROM(p string) error { n.loaded = append(n.loaded, p); return nil }

// TestPatchDeclinesABankedImage pins that a temporary ROM patch refuses an image whose
// address space is shared by two banks, instead of quietly picking one.
//
// The old arithmetic was base = 0x10000 - len(rom). For an 8K cartridge that puts the
// base at $E000 — an address the 2600 never fetches from — and resolves every patch
// into the SECOND bank: $F123 becomes file offset $1123, which is inside the file, so
// the bounds check passes and nothing is reported. The error text for a genuinely
// out-of-range address described the ROM as "$E000-$FFFF", a range that does not exist
// on the machine.
//
// The consequence is a measurement taken on a ROM patched in the bank that was not
// running, which is worse than no measurement, because it is reported as one.
func TestPatchDeclinesABankedImage(t *testing.T) {
	prev := curROMPath
	defer func() { curROMPath = prev }()

	curROMPath = "../../roms/exerciser/exerciser.bin" // 8K
	e := &nopEmu{}
	restore, err := applyTempPatch(e, []PatchSpec{{Addr: 0xF123, Bytes: "EA"}})
	if err == nil {
		if restore != nil {
			restore()
		}
		t.Fatal("an 8K image was patched by address — $F123 exists in both banks, so the tool " +
			"chose one silently")
	}
	for _, want := range []string{"bank-switched", "$F000-$FFFF"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal should say why it cannot be done; %q missing from: %v", want, err)
		}
	}
	if len(e.loaded) != 0 {
		t.Errorf("the machine was reloaded despite the refusal: %v", e.loaded)
	}

	// The flat case must still work: refusing everything would pass this test for the
	// wrong reason.
	curROMPath = "../../roms/litmus/smoke.bin" // 4K
	e2 := &nopEmu{}
	restore, err = applyTempPatch(e2, []PatchSpec{{Addr: 0xF000, Bytes: "EA"}})
	if err != nil {
		t.Fatalf("a flat 4K image must still be patchable: %v", err)
	}
	if len(e2.loaded) != 1 {
		t.Errorf("the patched ROM was not loaded: %v", e2.loaded)
	}
	restore()
}
