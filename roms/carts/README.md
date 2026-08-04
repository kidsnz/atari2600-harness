# roms/carts — cartridge-FORMAT fixtures

Every ROM here exists to exercise a **bank-switching scheme**, not a TIA behaviour. Each one is the
smallest image the Gopher2600 engine will fingerprint as its scheme and still boot and run.

| fixture | mapper | banks | bank size | origins | window is not the image because |
|---|---|---|---|---|---|
| `cart_f6sc.asm` | F6SC | 4 | 4096 | 1 | superchip RAM overlaid on $F000-$F0FF |
| `cart_f4sc.asm` | F4SC | 8 | 4096 | 1 | superchip RAM overlaid on $F000-$F0FF |
| `cart_3e.asm` | 3E | 4 | 2048 | 1 | `sta $3E` maps 1K of cartridge RAM over $1000-$17FF |
| `cart_3eplus.asm` | 3E+ | 4 | 1024 | 4 | `sta $3E` maps RAM into any of four 1K segments |
| `cart_dpc.asm` | DPC | 2 | 4096 | 1 | $1000-$107F is the data-fetcher / RNG / music register file |

Measured 2026-08-04. The numbers are asserted by `internal/emu.TestAdvancedCartridgesLoadWithTheRightGeometry`,
the refusals by `internal/cyclebound.TestAdvancedCartridgesAreDeclinedByNameAndReason`, and the banking
geometry `cmd/dissect` prints by `cmd/dissect.TestGeometryComesFromTheMapperNotTheFileLength`.

## Why this is a separate directory rather than more of `roms/litmus`

`internal/oracle`'s Stella TIA oracle covers **all** of `roms/litmus` and `roms/techniques`, and a ROM
with no capture fails it. Capturing needs Stella's GUI and takes the user's screen for ~13 s per ROM.
These fixtures paint one flat colour; a TIA-register capture of them would cost five screen takeovers and
add no TIA information, because what they are *for* is the mapper. So they are a corpus of their own,
graded by mapper-level assertions, and the oracle's corpus is unchanged.

They are still wired: `scripts/check_wiring.py` scans this directory alongside `roms/litmus`, so a fixture
that nothing runs fails CI here exactly as it does there.

## What has no fixture, and why

- **DPC+ / CDF / ACE** — recognised and *runnable* by the engine (7 real DPC+ cartridges in the umbrella's
  reference archive load and render), but a fixture would need real ARM Thumb driver code, not just the
  four fingerprint bytes. The refusal is exercised on the real cartridges out of repo; it is not pinned here.
- **ELF / bus stuffing** — bus stuffing is implemented only by the ELF and ACE mappers
  (`grep -rl "func.*BusStuff" Gopher2600/hardware/memory/cartridge/`), and no ELF or ACE cartridge exists
  on this machine (0 of 493 umbrella images). A synthesised 4096-byte ELF header is *rejected* by the
  engine (`mismatched ELF version 'EV_NONE'`), so there is nothing to test against.
- **AR (Supercharger)** — one real cartridge exists out of repo and is refused correctly; an in-repo
  fixture would have to be a tape image rather than a cartridge dump.
