package build

import (
	"fmt"
	"os"
)

// ROMBytesUsed reports a LOWER BOUND on how much of a cartridge image the program occupies, and
// separately the run of fill bytes at the end.
//
// ★It is a lower bound and not a range, and that took four attempts. All three that failed looked
// right, and each was caught only by asking a question with a known answer:
//
//  1. **Count the bytes DASM's listing displays.** The listing truncates any line of more than four
//     bytes with a `*`, so a sixteen-byte table counts as four. Short on every ROM with data in it.
//  2. **Take the address delta to the next emitted line.** Exact until a directive is the last thing
//     before an `org`, where there is no next address to subtract from. Calibrated against a ROM of
//     known size — 7 bytes of code, a 100-byte table, 4 vector bytes = **111** — the tool said
//     **11**: the `ds 100` collapsed to one byte.
//  3. **Bracket it: non-$FF as the floor, capacity minus the trailing $FF run as the ceiling.** The
//     floor is sound. **The ceiling is not a ceiling.** A ROM whose data ENDS in $FF has that data
//     swallowed by the trailing run: 3 bytes of code plus 200 bytes of deliberate $FF plus vectors
//     is 207, and the "bracket" came out 7..9 — the true answer outside it. A bound that excludes
//     the answer is worse than no bound, so the ceiling is gone.
//
// ★★What survives is exact in its direction. DASM fills unwritten space with **$FF** (measured),
// so every byte that is not $FF is one the program wrote: `Used` can only understate, never
// overstate, and it understates by exactly the number of $FF bytes the program emitted on purpose.
// On a ROM whose data avoids $FF it is the answer — calibrated at 111.
//
// ★★★`TrailingFF` is reported beside it because it answers the question people actually ask —
// "how much room is left at the end" — and it is exact as long as the program's last byte is not
// $FF. The two numbers disagree about a ROM full of solid sprite rows, and that disagreement is
// information rather than noise.
//
// The author's works are why this exists. Twelve of `260809_technojacket`'s cover ROMs have **10
// bytes** of trailing fill out of 4096, and 35 of 188 built work ROMs have under 512.
func ROMBytesUsed(binPath string) (used, trailingFF, capacity int, err error) {
	b, err := os.ReadFile(binPath)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(b) < 8 {
		return 0, 0, len(b), fmt.Errorf("%s is %d bytes, which is not a cartridge image", binPath, len(b))
	}
	for _, x := range b {
		if x != 0xFF {
			used++
		}
	}
	body := b[:len(b)-6] // the reset/IRQ vectors are always written, so they are not "trailing fill"
	for trailingFF < len(body) && body[len(body)-1-trailingFF] == 0xFF {
		trailingFF++
	}
	return used, trailingFF, len(b), nil
}
