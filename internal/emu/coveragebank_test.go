package emu

import "testing"

// TestCoverageDistinguishesTheSameAddressInTwoBanks pins that an instruction is
// identified by (bank, address) and not by address alone.
//
// Both banks of an 8K image decode $F000-$FFFF, so a bare address merges two
// different instructions into one entry, and it fails in both directions at once:
// as a count it under-reports what ran, and as a query it answers "covered" for the
// twin in the bank that never executed. The second is the flattering direction, and
// VV-3's coverage percentage and mutate's "covered" set both rest on it.
//
// Measured on the exerciser over 200k instructions before the fix: 319 distinct
// (bank, pc) pairs executed, PCCount reported 282, and 37 addresses ran in BOTH
// banks. cmd/cover's pc_executed went 268 -> 297 on the same command.
func TestCoverageDistinguishesTheSameAddressInTwoBanks(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/exerciser/exerciser.bin"); err != nil {
		t.Skipf("exerciser unavailable: %v", err)
	}
	e.EnableCoverage()

	pairs := map[[2]int]bool{}
	perAddr := map[uint16]map[int]bool{}
	for i := 0; i < 200000; i++ {
		pc := uint16(e.VCS.CPU.PC.Address())
		bk, _ := e.Bank()
		pairs[[2]int{bk, int(pc)}] = true
		if perAddr[pc] == nil {
			perAddr[pc] = map[int]bool{}
		}
		perAddr[pc][bk] = true
		if err := e.StepInstruction(); err != nil {
			break
		}
	}

	shared := 0
	var sampleAddr uint16
	var ranIn, didNotRun int
	for addr, banks := range perAddr {
		if len(banks) > 1 {
			shared++
			continue
		}
		// An address executed in exactly one bank: the OTHER bank's twin must not
		// read as covered. That is the query the old key got wrong.
		if sampleAddr == 0 {
			for b := range banks {
				ranIn = b
			}
			didNotRun = 1 - ranIn
			sampleAddr = addr
		}
	}
	if shared == 0 {
		t.Fatal("no address executed in more than one bank — this ROM no longer exercises the " +
			"collision, so the test proves nothing")
	}
	if len(pairs) != e.Coverage().PCCount() {
		t.Errorf("executed %d distinct (bank,pc) pairs but PCCount reports %d — the recorder is "+
			"merging instructions that share an address", len(pairs), e.Coverage().PCCount())
	}
	if sampleAddr != 0 {
		if !e.Coverage().SeenIn(ranIn, sampleAddr) {
			t.Errorf("$%04X ran in bank %d but SeenIn says otherwise", sampleAddr, ranIn)
		}
		if e.Coverage().SeenIn(didNotRun, sampleAddr) {
			t.Errorf("$%04X never ran in bank %d, but SeenIn reports it covered — this is the "+
				"flattering direction the address-only key had", sampleAddr, didNotRun)
		}
		if !e.Coverage().Seen(sampleAddr) {
			t.Errorf("Seen(addr) must stay 'covered in SOME bank' for compatibility")
		}
	}
	t.Logf("%d distinct (bank,pc) pairs, %d addresses executed in BOTH banks, PCCount=%d",
		len(pairs), shared, e.Coverage().PCCount())
}
