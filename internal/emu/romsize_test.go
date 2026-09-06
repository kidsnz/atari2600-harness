package emu

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestEveryBuiltROMIsACartridgeSize catches a build accident that looks like a program bug.
//
// `internal/build/build.go` runs `dasm ... -f3`, which emits a raw image. **Drop the `-f3` and DASM
// defaults to `-f1`, which prepends the two-byte load address** — a 4096-byte cart becomes 4098
// bytes and every byte in it is displaced by two. The archive describes the symptom
// (AtariAge `160249`): *a black screen and a fast beep.* Nothing about that says "your file is two
// bytes long"; it says "your kernel is wrong", and the search goes into the ROM.
//
// Nothing here checked the size. `internal/build` has no size assertion, the gates have none, and
// `LoadROM`'s only floor is `minCartBytes = 0x100` — 4098 sails through, and so does a 5887-byte
// **partial output from an assembly that failed**, which is what was found in
// `roms/260816_transistor/bin/` on 2026-09-05 (`Origin Reverse-indexed`, `Aborting assembly`, and
// the partial image committed anyway).
//
// ★Measured across this repository's ROMs: 2048 x1, 4096 x175, 8192 x11, 16384 x2, 32768 x2, and
// **one deliberate exception** — `cart_dpc` at 10240, which is a DPC cartridge (8K of ROM plus 2K
// of display data) and genuinely not a power of two. So the rule "every built ROM is a power of two
// unless it is on this list" holds today with a single named entry, and 4098 fails it immediately.
//
// ★★Found by the AtariAge cross-check (helper-1). Its value is that the symptom and the cause do
// not resemble each other — the same shape as DASM's first-column rule, where a misplaced space
// makes eighteen instructions unknown.
func TestEveryBuiltROMIsACartridgeSize(t *testing.T) {
	// Sizes that are not powers of two, with the reason each is allowed.
	exceptions := map[string]string{
		"cart_dpc": "DPC cartridge: 8K of ROM plus 2K of display data, so 10240 by design",
	}

	var bad []string
	sizes := map[int64]int{}
	err := filepath.Walk("../../roms", func(p string, d os.FileInfo, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".bin") {
			return nil
		}
		n := d.Size()
		sizes[n]++
		name := strings.TrimSuffix(filepath.Base(p), ".bin")
		if _, ok := exceptions[name]; ok {
			return nil
		}
		// power of two, and big enough to be a cartridge at all
		if n < 2048 || n&(n-1) != 0 {
			bad = append(bad, fmt.Sprintf("%s (%d bytes)", p, n))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) == 0 {
		t.Fatal("walked roms/ and found no .bin at all — this test would pass over an empty tree")
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("these built ROMs are not a cartridge size:\n  %s\n\n"+
			"Two ways this happens and neither looks like itself. **A missing `-f3`** makes DASM "+
			"prepend the two-byte load address, so 4096 becomes 4098 and every byte is displaced by "+
			"two — the symptom is a black screen and a fast beep, which sends you looking inside the "+
			"kernel. **A failed assembly** still writes a partial image; check the `.lst` for "+
			"`Aborting assembly`. If a size is genuinely intended (a DPC cart is 10240), add it to "+
			"`exceptions` with the reason", strings.Join(bad, "\n  "))
	}
	var ks []int64
	for k := range sizes {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i] < ks[j] })
	var parts []string
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%d x%d", k, sizes[k]))
	}
	t.Logf("ROM sizes: %s", strings.Join(parts, ", "))
}
