package emu

import (
	"bytes"
	"path/filepath"
	"testing"
)

// stubSelectsBank reports whether the reset stub in this bank reaches a
// bank-select hotspot within its first few instructions. Byte-level rather than a
// full decode on purpose: a boot stub is a handful of instructions and the question
// is only "does it touch a hotspot before doing anything else".
func stubSelectsBank(data []byte, hotspots map[uint16]string) bool {
	if len(data) < 0x1000 {
		return false
	}
	off := int((uint16(data[0x0FFC]) | uint16(data[0x0FFD])<<8) & 0x0FFF)
	for k := 0; k < 16 && off+k+2 < len(data); k++ {
		switch data[off+k] {
		case 0xAD, 0x8D, 0x2C: // lda / sta / bit, absolute
			a := uint16(data[off+k+1]) | uint16(data[off+k+2])<<8
			if _, ok := hotspots[0x1000|(a&0x0FFF)]; ok {
				return true
			}
		}
	}
	return false
}

// TestEveryBankCanBeBootedInto covers the F8/F6/F4 random-boot-bank trap at ROM
// level. The console's power-on bank is undefined, so a bank-switched cartridge has
// to end up in the same place whichever bank it wakes in — every bank needs a reset
// stub that selects, except the one it is all heading for.
//
// THE OBVIOUS RULE IS WRONG and measuring caught it before the check shipped. "Every
// bank's stub must select a bank" fails litmus_superchip, which is correct: its bank
// 1 stub does `lda $FFF8` and jumps, and bank 0 — the destination — has no reason to
// select itself. banked_game selects in both banks, which is merely redundant. So the
// rule is that AT MOST ONE bank may omit the select.
//
// Measured over the corpus, all 10 multi-bank ROMs pass: 9 select in every bank,
// litmus_superchip in 1 of 2. Byte-identical banks are exempt outright — booting into
// a copy is booting into the same program.
func TestEveryBankCanBeBootedInto(t *testing.T) {
	var files []string
	for _, pat := range []string{"../../roms/techniques/*.bin", "../../roms/litmus/*.bin"} {
		f, _ := filepath.Glob(pat)
		files = append(files, f...)
	}
	checked := 0
	for _, f := range files {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(f); err != nil {
			continue
		}
		id, banks := e.CartInfo()
		if banks < 2 {
			continue
		}
		cs, err := e.CopyBanks()
		if err != nil || len(cs) < 2 {
			continue
		}
		hotspots, published := e.BankSwitchHotspots()
		if !published || len(hotspots) == 0 {
			continue // the switch is not address-driven; this check cannot see it
		}
		checked++

		identical := true
		for i := 1; i < len(cs); i++ {
			if !bytes.Equal(cs[i].Data, cs[0].Data) {
				identical = false
				break
			}
		}
		if identical {
			continue
		}

		var strays []int
		for _, c := range cs {
			if !stubSelectsBank(c.Data, hotspots) {
				strays = append(strays, c.Number)
			}
		}
		if len(strays) > 1 {
			t.Errorf("%s (%s, %d banks): banks %v boot without selecting one, and the banks differ. "+
				"The power-on bank is undefined, so at most ONE bank — the destination they all switch "+
				"to — may omit the select", filepath.Base(f), id, banks, strays)
		}
	}
	if checked == 0 {
		t.Fatal("no multi-bank cartridge with published hotspots was checked — this test would pass " +
			"while looking at nothing")
	}
	t.Logf("checked %d multi-bank ROMs; every one can be booted into from any bank", checked)
}
