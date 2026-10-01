# Technique ⑱ — RTS-stack modular kernel dispatch

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ0 = $06` — three copies at the wide spacing — so a base past ~96 puts the last copy over the edge. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

> Also known as: **modular kernel** (vitoco, AtariAge topic 313777). The **dynamic** sibling of zone
> multiplexing: the screen is composed of selectable vertical zones whose order is **data in RAM**, and the
> beam walks them by chaining zone routines through the **RTS-stack trick** at a constant ~6-cycle cost.

## Goal
A screen built from N vertical **zones**, where *which* zones appear and *in what order* is a **RAM list of
zone IDs** — not a fixed, hand-unrolled kernel. With many zone *types* a fixed kernel suffers a combinatorial
explosion; here a single dispatcher chains arbitrary zones with **no `case`/branch ladder** and a **constant**
per-zone switching cost. This is the dynamic generalization of `zone_multiplexing` (whose zones are fixed).

## Demo (`roms/techniques/rts_dispatch.asm`)
Four zone types, each a self-contained routine that draws a fixed `ZONE_H = 40` scanlines and ends in `RTS`:

| ID | routine | what renders (numerically verified, `read_row`) |
|----|---------|---------------------------------------------------|
| 0 | `ZoneSolid`  | solid blue band (`COLUBK $84` → `1D20CA`) |
| 1 | `ZonePF`     | yellow/red striped playfield band (`FFFF30`/`EC3333`) |
| 2 | `ZoneSprite` | brown band with 3 white diamond sprites (NUSIZ0 3-copy; `FFFFFE` columns over `FFAD37`) |
| 3 | `ZoneStripe` | per-scanline color-cycling band (`CD32A6` mid-band) |

The active screen is the RAM list `zonelist` (`$90..$93`). It boots as `[0,1,2,3]` (top→bottom) and is
**rewritten to `[3,2,1,0]` at frame 60** to prove the screen is genuinely data-driven — the rewrite is
reflected in both the dispatch trace and the rendered pixels.

## The RTS-dispatch mechanism (the actual trick)
On 6502, **`JSR` pushes `(return_addr − 1)`** (hi then lo) and **`RTS` pulls two bytes and adds 1** into PC.
So if you **push `(target − 1)` yourself** and execute `RTS`, you land on `target` — a computed jump in
**6 cycles**, no register clobber, no comparison ladder.

Per frame the dispatcher (clean-room reimplementation of vitoco's idea):

```
        lda #>(EndKernel-1)     ; 1) push the FINAL landing (last RTS returns here)
        pha
        lda #<(EndKernel-1)
        pha
        ldx #NLIST-1            ; 2) push each zone's (addr-1), list walked in REVERSE
PushLoop:
        lda zonelist,x         ;    zone ID
        asl                    ;    *2 -> word index into ZoneTbl
        tay
        lda ZoneTbl+1,y        ;    hi of (routine-1)
        pha
        lda ZoneTbl,y          ;    lo of (routine-1)
        pha
        dex
        bpl PushLoop
        rts                    ; 3) launch: RTS jumps to list[0]
```

Because the list is pushed **in reverse**, the stack pops **front-to-back**: `RTS` → `list[0]`; that zone's
trailing `RTS` → `list[1]`; … `list[NLIST-1]`'s `RTS` → `EndKernel`. Every zone-to-zone transition is one
`RTS` = constant ~6 cy regardless of which zone runs next. `ZoneTbl` stores `routine − 1` so the `RTS` `+1`
lands exactly on the routine. **Cost:** 2 stack bytes per zone (here 4 zones = 8 bytes) + 2 for the terminator.

## How total lines stay 262 (the hard part)
Variable zones could make the frame breathe; this demo nails it three ways:
1. **Fixed zone count & height.** The list is always `NLIST = 4` entries of `ZONE_H = 40` lines → visible
   zone area is always `4×40 = 160` lines whatever the IDs are. Different IDs change *appearance*, never *line
   count*.
2. **Constant-time frame logic.** The per-frame list rewrite runs inside **one dedicated `WSYNC`-bounded
   logic line** (`sta WSYNC` reserved every frame), so the swap frame costs exactly the same scanlines as any
   other — no ±1 jitter.
3. **Fixed budget = VSYNC 3 + VBLANK(logic 1 + 33) + zones 160 + EndKernel 63 = 262.** Verified by the
   scenario's `ntsc_frame_lines: 262`, including the swap-crossing run.

(vitoco's original keeps the arena height constant with **elastic blank spacer zones** — shrink an enemy
zone, grow a blank zone — so sprite vertical motion doesn't change the line total. This demo uses the simpler
fixed-height form; the elastic-spacer form is a documented extension.)

## CI / how the harness verifies it
`roms/techniques/scenarios/rts_dispatch.json` (golden + 9 asserts, exit 0):
- **Dispatch order trace.** Each zone writes its ID into `evid[slot]` (`$94..$97`) as it runs. Frame 0 asserts
  `evid == [0,1,2,3]` → the RTS chain executed every zone, in the list order.
- **RAM rewrite reflected.** Frame 57 (after the frame-60 swap) asserts `evid == [3,2,1,0]` → rewriting the
  zone-list RAM changes which zones render and in what order.
- **Distinct pixels locked.** `golden_frame: true` hashes the rendered frame chain; the four zones above were
  cross-checked numerically with `read_row` (distinct color/PF per zone range) and visually with
  `get_screen_annotated` (4 visually distinct horizontal bands — not blank).
- **Constant frame.** `ntsc_frame_lines: 262`.

## Verified facts
- **`JSR` pushes `addr−1`, `RTS` pops+`+1`** ⇒ pushing `(target−1)` + `RTS` = a 6-cycle computed jump
  (table stores `routine−1`).
- **Constant dispatch cost:** one `RTS` per zone transition, independent of zone type — no branch ladder.
- **Constant frame:** fixed `NLIST×ZONE_H` zone area + a `WSYNC`-bounded logic line keep the frame at **262**
  even when the RAM list is rewritten mid-run (verified across the swap frame).
- **Cost:** 2 bytes of stack per zone in the list (+2 for the terminator).

## Neighbours — the same stack for other jobs, and the list as something else

- **`RTS` or `JMP (ptr)`: the price is RAM, not ROM.** Rob Mundschau's single dispatch is `LDA
  TableHi,X / PHA / LDA TableLo,X / PHA / RTS`, tables holding address−1; he wrote that it is 2 bytes
  of ROM smaller than storing a vector and `JMP (Vector)`, and one cycle slower 〔stella-list
  `200301/msg00031`〕. By the opcode table (zero-page vector) the `RTS` form is 9 bytes and 20 cycles
  against 13 bytes and 19 cycles — 4 bytes, not 2 (**Not verified** — a hand count, not assembled).
  Thomas Jentzsch named the real difference: *"The little advantage of the JMP() version is, that it
  doesn't need extra stack RAM. Instead you can re(!)use two other currently unused bytes (e.g. a
  pointer)"* 〔`msg00039`〕. The stack bytes are claimed at that moment; the vector can be two bytes
  that mean something else outside the kernel. In this page's chain every zone's address sits on the
  stack for the whole kernel (the **Cost** line above). **Cited only, not verified**.
- **A variable delay, entered by `JMP (ptr)` and left by `RTS`.** shazz, timing a 48-px sprite into
  place, pushes the continuation and jumps into a run of `nop`s: `STA WSYNC / LDA #>ScanLineLoop / PHA /
  LDA #<ScanLineLoop-1 / PHA / JMP (DelayRoutine)`; the run ends in `RTS`, which lands on
  `ScanLineLoop` 〔AtariAge `topic/215637`, 2013〕. Where `DelayRoutine` points into the run is the
  number of cycles spent — 2 per `nop`, so he keeps separate even and odd tables. The push-(target−1)
  -then-`RTS` above, used once to come back rather than to chain zones; `SLEEP` elsewhere in this
  repository is a delay fixed at assembly time. **Cited only, not verified**.
- **The list as a timeline.** The zone list here is spatial — one entry per band of the screen. In a
  2025 thread on growing stalactites drawn in the playfield, two replies made the list temporal.
  SplendidNut: treat *"each PF byte as its own unique stack of bitmap changes"*, each entry a pattern
  and *"the countdown to the next change"* (16 bytes per PF column, 64 for a 32-pixel display).
  Bit Expander: about 32 bytes, *"each one being an opcode to be executed by the kernel"* — 3 bits of
  blocks to wait, 5 bits naming the PF bit to clear 〔AtariAge `topic/385470`〕. Thomas Jentzsch, who
  asked, used neither: he stored the plain playfield, *"32 (columns) / 8 * 16 (rows) = 64 bytes"*.
  **Cited only, not verified**.
- **Zone seams show in the picture.** Each zone sets its own colours, and anything drawn across a zone
  boundary shows the change. IanAjax's Impossible Mission mock-up (2026) staggers the side walls so
  platform zones and object zones alternate, and because every zone is the same height the colour
  steps read as *"a bit more like some design on the wall"*. Objects that use the ball also have a
  playfield version, so one of them can share a level with another ball object 〔AtariAge
  `topic/388301`〕. A mock-up, not a kernel; two days later the walls became a gradient
  pattern that changes the playfield colours only 12 times, *"12 zones to switch between"*.
  **Cited only, not verified**.

## See also
- `zone-multiplexing.md` — the **static** form (fixed zones); this is its data-driven generalization.
- `dynamic-multisprite.md` — sort/position/display + flicker (the general multi-sprite kernel).
- `roadmap.md` — full candidate list (this is candidate ⑱).

## References
- vitoco, *Tips and tricks for a modular kernel* — AtariAge topic **313777** (2020).
  Distilled notes: `reference/atariage/313777-modular-kernel/notes.ja.md`.
- 6502 `JSR`/`RTS` stack semantics (6502.org): `JSR` pushes return−1; `RTS` pulls and increments.
