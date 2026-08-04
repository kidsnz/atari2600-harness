package srcmap

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// DASM's listing layout, transcribed from litmus_bank_f4.lst and banked_game.lst with
// the tabs intact, because the tabs are what separate a row that PUT BYTES SOMEWHERE
// from a row that merely printed the PC:
//
//   - `0000 ????` — the row produced no output. Comments, `processor` and `=` equates
//     all look like this, and the address beside `????` is whatever the PC was, which
//     before the first ORG is offset $0000 = bank 0's first byte.
//   - `0000` + tabs + source text — ORG/RORG or a label alone on its line: a position,
//     no bytes.
//   - `0000` + two tabs + the byte column — the row assembled those bytes there.
const bankedLstFix = "      1  0000 ????\t\t\t\t\t\t; a header comment, assembling nothing\n" +
	"      2  0000 ????\t\t\t\t      processor\t6502\n" +
	"      3  0000 ????\t       00 00\t   VSYNC      =\t$00\n" +
	"      4  0000\t\t\t\t\t      ORG\t$0000\n" +
	"      5  0000\t\t\t\t\t      RORG\t$F000\n" +
	"      6  0000\t\t\t\t   Start\n" +
	"      7  0000\t\t       78\t\t      sei\n" +
	"      8  0001\t\t       d8\t\t      cld\n" +
	"      9  1000\t\t\t\t\t      ORG\t$1000\n" +
	"     10  1000\t\t\t\t\t      RORG\t$F000\n" +
	"     11  1000\t\t       a9 b1\t   Work\t      lda\t#$B1\n"

const bankedAsmFix = `; a header comment, assembling nothing
        processor 6502
VSYNC   = $00
        ORG  $0000
        RORG $F000
Start:
        sei
        cld
        ORG  $1000
        RORG $F000
Work:   lda #$B1
`

func writeFixtureAsm(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fixture.asm")
	if err := os.WriteFile(p, []byte(bankedAsmFix), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A line number is a claim that THIS SOURCE LINE ASSEMBLES TO THAT ADDRESS. A listing
// row marked `????` makes no such claim: it emitted nothing, and the address printed
// next to it is stale. Measured before this rule existed, on litmus_bank_f4: bank 0
// $F000 resolved to line 1 — the file's opening comment — because the comment's row
// carries offset $0000 and first-occurrence wins over the `sei` that really lives
// there. The equates, which start in column 1, were placed as bank 0 labels at $F000
// on top of that.
func TestABankedListingRowThatAssembledNothingDefinesNoLine(t *testing.T) {
	asm := writeFixtureAsm(t)
	m := ParseBanked(bankedLstFix, asm, 2)

	if got, ok := m.Line(0, 0xF000); !ok || got != 7 {
		t.Errorf("Line(bank 0, $F000) = %d, %v — want line 7 (`sei`), the line that assembles "+
			"there. Line 1 is a comment and line 4 is an ORG; neither puts a byte at $F000", got, ok)
	}
	if got, ok := m.Line(0, 0xF001); !ok || got != 8 {
		t.Errorf("Line(bank 0, $F001) = %d, %v — want line 8 (`cld`)", got, ok)
	}
	if got, ok := m.Line(1, 0xF000); !ok || got != 11 {
		t.Errorf("Line(bank 1, $F000) = %d, %v — want line 11 (`lda #$B1`); bank 1's own first "+
			"byte must not inherit bank 0's rows", got, ok)
	}
	// An equate is not a code label. It starts in column 1 like one, and its listing row
	// carries a stale address, so placing it names an address it has nothing to do with.
	if b, ok := m.LabelBank("VSYNC"); ok {
		t.Errorf("LabelBank(VSYNC) = bank %d — an `=` equate was placed as a code label; "+
			"its row is a `????` row and its address is not where it lives", b)
	}
	// A label alone on its line emits no bytes either, but it DOES have an address, so it
	// still has to be placed — dropping every by-position row would cost every such label.
	if b, ok := m.LabelBank("Start"); !ok || b != 0 {
		t.Errorf("LabelBank(Start) = bank %d, %v — want bank 0; a label on a line of its own "+
			"still names an address", b, ok)
	}
	if got := m.Locate(0, 0xF000); got != "bank 0 Start (fixture.asm:7)" {
		t.Errorf("Locate(bank 0, $F000) = %q — want %q", got, "bank 0 Start (fixture.asm:7)")
	}
}

// Source lines that cannot be the line that assembles to an address, decided from the
// SOURCE FILE rather than from the listing, so the verdict is independent of the parse
// under test.
var (
	fixEquate    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*=`)
	fixDirective = regexp.MustCompile(`(?i)^\s*(ORG|RORG|SEG|PROCESSOR|INCLUDE|IFCONST|ENDIF|ELSE)\b`)
	fixLabelOnly = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*:?\s*(;.*)?$`)
)

func assemblesNothing(text string) (bool, string) {
	t := strings.TrimRight(text, " \t\r")
	switch {
	case strings.TrimSpace(t) == "":
		return true, "blank line"
	case strings.HasPrefix(strings.TrimSpace(t), ";"):
		return true, "comment"
	case fixEquate.MatchString(t):
		return true, "equate"
	case fixDirective.MatchString(t):
		return true, "directive"
	case fixLabelOnly.MatchString(t):
		return true, "label with no instruction"
	}
	return false, ""
}

// Images whose listing carries no usable offset, recorded by name with the reason, so
// the limitation is a stated fact instead of a silent zero.
//
// bank = offset>>12 needs the listing's address column to BE the physical offset, which
// holds when the source's first `org` is 0. litmus_superchip orgs at $D000, so every row
// reads $Dxxx/$Exxx, lands past `banks*4096`, and is dropped. Attributing it would mean
// inferring the base from the lowest address seen — a guess that puts every line in the
// wrong bank when a source deliberately leaves its first bank empty, and a wrong line is
// worse than none. Nothing is lost in practice: the static analysis DECLINES F8SC by name
// (`banked_declined: cartridge is mapper F8SC ...`) and returns before a per-bank map is
// ever built for it.
var resolvesNothing = map[string]string{
	"litmus_superchip.asm": "its source orgs at $D000, so the listing's address column is not a " +
		"0-based physical offset (and the analysis declines F8SC before building a map at all)",
}

// bankedCorpus finds every bank-switched image in the repo's own ROMs: a .bin larger
// than one 4K bank with a sibling .asm. Discovered rather than listed, so a bank ROM
// added later is covered without editing this file.
func bankedCorpus(t *testing.T) []string {
	t.Helper()
	var out []string
	root := filepath.Join("..", "..", "roms")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".bin" {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.Size() <= 4096 {
			return nil
		}
		asm := strings.TrimSuffix(p, ".bin") + ".asm"
		if _, err := os.Stat(asm); err == nil {
			out = append(out, asm)
		}
		return nil
	})
	if err != nil {
		t.Skipf("cannot walk %s: %v", root, err)
	}
	return out
}

// The whole point of a per-bank line map is that its answers are FACTS about the image.
// This walks every (bank,address) the map resolved on every bank-switched ROM in the
// corpus and checks the line it names could actually have assembled there — a comment,
// an equate, an ORG or a bare label cannot. A wrong line costs more than no line: the
// author goes and reads a line that has nothing to do with the address.
//
// Measured over 11 bank-switched images: 91 of 878 EXECUTED (bank,PC) pairs named such
// a line before the `????`/byte-column rule, 0 of 878 after.
func TestBankedSourceLinesNameALineThatAssemblesThere(t *testing.T) {
	roms := bankedCorpus(t)
	if len(roms) == 0 {
		t.Fatal("no bank-switched ROM found under roms/ — this test would pass while checking nothing")
	}
	checked, bad := 0, 0
	for _, asm := range roms {
		bin := filepath.Join(t.TempDir(), filepath.Base(asm)+".bin")
		_, lst, _, err := build.AssembleWithListing(asm, bin)
		if err != nil {
			t.Fatalf("%s: %v", asm, err)
		}
		img, err := os.ReadFile(bin)
		if err != nil {
			t.Fatal(err)
		}
		banks := len(img) / 4096
		src, err := os.ReadFile(asm)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")

		m := ParseBanked(lst, asm, banks)
		resolved := 0
		for b := 0; b < banks; b++ {
			for a := 0xF000; a <= 0xFFFF; a++ {
				ln, ok := m.Line(b, uint16(a))
				if !ok {
					continue
				}
				resolved++
				checked++
				if ln < 1 || ln > len(lines) {
					bad++
					t.Errorf("%s: bank %d $%04X names line %d, which is outside the file (%d lines)",
						filepath.Base(asm), b, a, ln, len(lines))
					continue
				}
				if no, why := assemblesNothing(lines[ln-1]); no {
					bad++
					t.Errorf("%s: bank %d $%04X names line %d, a %s (%q) — that line assembles "+
						"no byte at that address, so the location is a fabrication",
						filepath.Base(asm), b, a, ln, why, strings.TrimSpace(lines[ln-1]))
				}
			}
		}
		if why, known := resolvesNothing[filepath.Base(asm)]; known {
			if resolved != 0 {
				t.Errorf("%s: resolved %d addresses, but it is recorded as resolving NOTHING because "+
					"%s — the limitation was lifted and this exception is now hiding whatever the new "+
					"answers are; check them and delete the entry", filepath.Base(asm), resolved, why)
			}
			continue
		}
		if resolved == 0 {
			t.Errorf("%s: the per-bank map resolved NOTHING — every line number silently absent",
				filepath.Base(asm))
		}
	}
	if checked == 0 {
		t.Fatal("no (bank,address) resolved anywhere in the corpus — this test proved nothing")
	}
	t.Logf("checked %d resolved (bank,address) line numbers across %d bank-switched ROMs; %d name a "+
		"line that assembles nothing", checked, len(roms), bad)
}

// Three named instructions in BANK 1 of litmus_bank_f4, spot-checked against the source
// text rather than against the listing that produced the map. Bank 1's $FF03/$FF05/$FF07
// are the addresses the machine executes there (the flat map could reach none of them:
// it stored them under $1F03 as if the physical offset were a CPU address).
func TestBankOneInstructionsNameTheirOwnSourceLine(t *testing.T) {
	asm := filepath.Join("..", "..", "roms", "litmus", "litmus_bank_f4.asm")
	if _, err := os.Stat(asm); err != nil {
		t.Skipf("%s not present (%v)", asm, err)
	}
	bin := filepath.Join(t.TempDir(), "litmus_bank_f4.bin")
	_, lst, _, err := build.AssembleWithListing(asm, bin)
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(asm)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(src), "\n")
	m := ParseBanked(lst, asm, 8)

	want := []struct {
		addr uint16
		line int
		text string
	}{
		{0xFF03, 70, "lda #$B1"},
		{0xFF05, 71, "sta $90"},
		{0xFF07, 72, "inc $91"},
	}
	for _, w := range want {
		got, ok := m.Line(1, w.addr)
		if !ok {
			t.Errorf("bank 1 $%04X has no source line", w.addr)
			continue
		}
		if got != w.line {
			t.Errorf("bank 1 $%04X names line %d (%q); want line %d (%q)",
				w.addr, got, strings.TrimSpace(lines[got-1]), w.line, w.text)
			continue
		}
		if txt := strings.TrimSpace(lines[got-1]); !strings.HasPrefix(txt, w.text) {
			t.Errorf("bank 1 $%04X names line %d, whose text is %q; the source moved and this "+
				"fixture is stale — expected it to start with %q", w.addr, got, txt, w.text)
		}
	}
}
