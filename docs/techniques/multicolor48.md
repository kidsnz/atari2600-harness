# Technique — multicolor 48px graphic (per-row color)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** the 48px-wide image (the standard 3×NUSIZ + P1 offset + VDEL technique used by
`score-kernel` / `bitmap48`) but **multicolored vertically** — `COLUP0`/`COLUP1` are rewritten
every scanline from a color table, so a single 48px graphic spans many colors top-to-bottom
without any extra sprites.

Demo: `roms/techniques/multicolor48.asm` (a rainbow-banded heart icon, 48×16).
CI: `scenarios/multicolor48.json` (positions 87/95, two distinct per-row colors mirrored to RAM,
262 lines, golden).
Lineage: AtariAge topic/209137 (SeaGtGruff's 76-cycle multicolor 48px kernel) laid on top of the
hardware-verified 6-store choreography (`litmus_48px6`, v0.52.0).

## The technique

**48px construction** is unchanged from the family: `NUSIZ0=NUSIZ1=$03` (3 copies close),
`VDELP0=VDELP1=1` (double-buffer), P0 reset then P1 reset + `HMP1=$10` (P1 nudged +8px right) so
the six 8px slices abut into one 48px band. Positions land at **P0=87 / P1=95** (same prologue as
score6: litmus recipe + SLEEP 21).

**Per-row color = one table fetch in HBLANK.** The color rewrite is squeezed into the front of the
line, before the first GRP store, so it costs nothing in the visible store window:

```
Krow:   sta WSYNC
        ldy row        ; 3
        lda ColorTab,y ; 7
        sta COLUP0     ; 10
        sta COLUP1     ; 13   ← color set for this row (still in HBLANK)
        lda (p0),y     ; 18   sta GRP0 ; 21   B0
        lda (p1),y     ; 23   sta GRP1 ; 26   B1
        lda (p2),y     ; 31   sta GRP0 ; 34   B2
        lda (p3),y     ; 39   sta tmp  ; 42
        lda (p4),y     ; 47   tax      ; 49
        lda (p5),y     ; 54   tay      ; 56
        lda tmp        ; 59
        sta GRP1 ; 62  stx GRP0 ; 65  sty GRP1 ; 68  sta GRP0 ; 71 (junk)
        dec row        ; 76
        bpl Krow       ; runs 3cy into HBLANK, halted by next WSYNC
```

**Per-line budget ≈ 73 cycles of work** before `dec row` completes at cycle 76 (the WSYNC
boundary); `bpl` spills harmlessly into the next HBLANK where the next `sta WSYNC` re-syncs. One
row = exactly one scanline, no overrun — confirmed by the frame staying at 262 lines (an overrun
would push it to 276). The 10-cycle color burst (`lda/sta/sta`) lives entirely inside HBLANK, so
the four GRP stores still complete at 21/26/34 then 62/65/68/71 — the same gap relations as the
monochrome kernel, just shifted +7 cy vs score6 because of the inserted color fetch.

**Data layout:** six column tables (`Col0..Col5`, one per 8px slice) and a `ColorTab`, all stored
**bottom-row-first** (the kernel walks `row = HEIGHT-1 → 0`) within one ROM page (fixed pointer
high bytes → `lda (p),y` is a deterministic 5 cy). `ColorTab[row]` is the COLUPx value for that
scanline; vary it to taste (here a 16-step rainbow).

## Verified

- Positions `read_tia` hmoved_pixel **87 / 95**, VDEL on.
- **Multicolor proven numerically:** the kernel mirrors the top row's color ($64) and bottom row's
  color ($44) into RAM `$82`/`$83`; the scenario asserts they are distinct (`$82==100`, `$83==68`,
  `$82!=68`). `read_row` confirms different colors on different scanlines (e.g. `3F81FF` cyan at
  line 46, `FFFF2F` yellow at line 52).
- Recognizable, non-blank shape (rainbow heart band on the annotated screen), golden-pinned, 262
  lines every frame.

## Notes / variants

- The color table is independent of the graphic data — swap `ColorTab` for a flashing/cycling
  effect by offsetting the index per frame.
- For pixel-exact X placement of the band (1px instead of the 3px coarse grid), combine with the
  ÷3 coarse/fine table + clockslide from topic/209137 (technique ledger ⑯); orthogonal to the color trick.
- Budget headroom is small (~73 cy used). Adding more per-row work (e.g. a second color register
  for the PF) needs an illegal-opcode tightening (LAX to fuse LDA+TAX).
  **Shipped precedent, added 2026-09-03: this was not a proposal.** Thomas Jentzsch used it in
  Thrust and said why 〔stella `200006/msg00037`〕: *"i'm using `LAX (ptr),Y` to get the data faster
  into the X-register (5 vs 7 clocks). This gives me the time to do the color-cycling"* — the same
  purpose this line proposes it for, in June 2000. His figures check out against our own instruction
  table (`Gopher2600/hardware/cpu/instructions/definitions.json`): `LDA (zp),Y` 5 cy / 2 bytes plus
  `TAX` 2 cy / 1 byte against `LAX (zp),Y` 5 cy / 2 bytes, so **2 cycles and 1 byte**. `LAX` is
  hardware-stable — `known-traps.md` lists it with `SAX/SBX/DCP` ★on original NMOS silicon. `SBX` and `ARR` are reported dead on the **Flashback 2** (AtariAge `113732-clean-assembly`), which is a reimplementation rather than a 6507 — unmeasured here, and stated because this is a technique page telling an author something is safe, unlike `LXA/XAA`.
  "Verify on Gopher2600 first" still stands for our own kernel; what has changed is that the idea
  is no longer untried.
