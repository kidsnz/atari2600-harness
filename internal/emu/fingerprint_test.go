package emu

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestEveryCartridgeSizedROMFingerprints answers a question the distillation raised and
// nobody here had asked of the whole corpus: **how many ROMs can the mapper fingerprint
// NOT identify?**
//
// Answer, measured 2026-09-07: **none of them.** 157 cartridge-sized files under
// `reference/` load, and every one gets a mapper — 4K 95, F6 17, F8 15, F4SC 6, DPC+ 5,
// 2K 8, F4 3, E0 3, F8SC 2, 3E 1, F6SC 1, FA 1.
//
// ★The first version of this measurement reported **373 failures out of 535**, and the
// number is the finding. It fed the loader every `*.bin` under `reference/`, and most of
// them are not cartridges: `docs_atari/za2600/world/w2door3.bin` is **128 bytes** of game
// data. A loader that refuses a 128-byte file is behaving correctly; a test that calls that
// a fingerprinting failure is not. **Filtering to cartridge sizes takes the failures to
// zero**, which is the honest answer to the question that was asked.
//
// ★★So the assertion below is deliberately two-sided: the failures must be zero AND the
// corpus must still be large and varied. A filter tight enough to admit only files that
// happen to work would also produce zero.
func TestEveryCartridgeSizedROMFingerprints(t *testing.T) {
	out, err := exec.Command("bash", "-c",
		`find ../../../reference \( -name '*.bin' -o -name '*.a26' \) 2>/dev/null | grep -v litmus | grep -v techniques`).Output()
	if err != nil {
		t.Skipf("reference/ not reachable from here: %v", err)
	}
	cartSize := map[int64]bool{
		2048: true, 4096: true, 8192: true, 10240: true,
		12288: true, 16384: true, 32768: true, 65536: true,
	}
	var roms []string
	skipped := 0
	for _, f := range strings.Fields(string(out)) {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		if cartSize[st.Size()] {
			roms = append(roms, f)
		} else {
			skipped++
		}
	}
	if len(roms) == 0 {
		t.Skip("no cartridge-sized ROMs found — reference/ is local-only and not in CI")
	}

	byMapper := map[string]int{}
	var failed []string
	for _, f := range roms {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(f); err != nil {
			failed = append(failed, filepath.Base(f))
			continue
		}
		byMapper[e.VCS.Mem.Cart.ID()]++
	}

	keys := make([]string, 0, len(byMapper))
	for k := range byMapper {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	line := make([]string, 0, len(keys))
	for _, k := range keys {
		line = append(line, k+"="+strconv.Itoa(byMapper[k]))
	}
	t.Logf("%d cartridge-sized files (%d non-cartridge skipped), %d unidentified: %s",
		len(roms), skipped, len(failed), strings.Join(line, " "))

	for _, f := range failed {
		t.Errorf("no mapper for %s — the fingerprint has stopped covering the corpus", f)
	}
	// The other side: a corpus that shrank to a handful, or to one mapper, would pass the
	// line above while meaning nothing.
	if len(roms) < 100 {
		t.Errorf("only %d cartridge-sized ROMs — the sweep has lost its corpus and a zero "+
			"failure count says nothing", len(roms))
	}
	if len(byMapper) < 6 {
		t.Errorf("only %d distinct mappers across %d ROMs; the fingerprint is being exercised "+
			"on too narrow a set for zero failures to mean it works", len(byMapper), len(roms))
	}
}
