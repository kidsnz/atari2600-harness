# Image ingestion — screenshot → TIA data

The reverse pipeline: feed a game screenshot in, get TIA-coordinate analysis out — a grid
overlay you can point at, the color inventory as real `COLUxx` values, and (M2/M3) playfield
bytes and sprite GRP data ready to paste into DASM source.

CLI: `go run ./cmd/ingest -in shot.png -out report_dir/` → `overlay.png` + `report.json`.

## Input contract (what to feed it)

| Grade | Source | Use |
|---|---|---|
| **A — extraction grade** | **Stella's own snapshot (F12), PNG, unmodified, TV effects OFF** | pixel-exact extraction. Integer scale guaranteed; bypasses macOS Retina scaling entirely |
| **B — conversation grade** | OS screenshots (full screen / window) | "look at this" pointing and discussion. Extraction still runs but warns: non-integer scale and window chrome degrade precision |
| **C — not usable** | photos of a screen, resized/filtered images, JPEG-artifacted shots | colors and the pixel grid are destroyed; expect garbage |

**Some swatches are the same colour.** The palette this pipeline quantises against holds 128 codes
but only **91 distinct colours** — 28 groups collide, 37 codes are unreachable, and four codes
(`$26 $28 $F6 $F8`) are a single tone. The two adjacent grey pairs (`$08`/`$0A`, `$0C`/`$0E`) matter
most: a luminance ramp that looks smooth on the canvas has steps in it that do not exist. `Nearest`
cannot warn about this — by the time it runs, the two swatches have already collapsed — so ask
`Quantizer.Aliases()` first if you want to check a palette before drawing with it.
**This is the measured Stella table's shape, not the TIA's**: the engine's own palette gives 126–127
distinct colours in 1–2 groups (the count is host-dependent; the gap to Stella's 91 is not). Which of the two a real television resembles is not established here.

**And the palette is per TV standard, with one point checked against hardware.** The same byte is a
different colour on PAL — `$36` orange becomes green, `$C6` green becomes violet — so a picture
designed here is designed *for NTSC*. A 2001 post from someone playing on a PAL console through a
Cuttle Cart reports *"the red cards are green (changed RED to `$62` and RED is red for PAL)"*, and
that reproduces exactly: `$62` renders purple (`521196`) on NTSC and red (`7C0A15`) on PAL. It is the
one datum here that comes from outside the emulators rather than from comparing them with each other.
It does not give a conversion table — one point is one point, and the 1997 source warns there is *"not
necessarily a corresponding hue"* — but it does mean **a PAL release is a re-choice of colours, not a
re-mapping of them.**

**And the budget is different, counted 2026-09-04:** rendering all 128 even values gives **NTSC 126–127 distinct colours (host-dependent), PAL 104, SECAM 8**. A 1997 post claiming PAL has *"half the colours"* was hedged
with *"I vaguely remember"*, and the hedge was the accurate part — 104 is about 83% of NTSC's, not 50%.
**SECAM is the one to know about**: eight colours, because the standard carries luminance only and
assigns each level a fixed hue. A picture designed here does not degrade on SECAM, **it is replaced**.
Pinned in `internal/emu/palspec_test.go`, so if any of the three numbers moves, the work designed
against it fails loudly.
The count covers three of the engine's five standards: `SpecList`
(`Gopher2600/hardware/television/specification/specifications.go`) also lists PAL60 and PAL-M, and neither
`internal/emu/palspec_test.go` nor `internal/ceiling/palpalette_test.go` counts those two, so no colour
budget for either is recorded here.

**What SECAM replaces it with is a fixed table, by luminance only.** The Stella Programmer's Guide
(*PAL/SECAM conversions*): `0` black, `2` blue, `4` red, `6` magenta, `8` green, `A` cyan, `C` yellow,
`E` white. The same section says a SECAM console *"takes the PAL software"* with the colour/B&W switch
*"hardwired as black & white"*, so what appears there is the game's **black-and-white** table read
through those eight hues. The engine's SECAM palette (`legacySECAMfromStella` in
`Gopher2600/hardware/television/colourgen/legacy.go`, taken from Stella) holds the same eight hues in the
same order — read from its hex values, not pinned by a test; only the count of eight is.
**Cited only, not verified** on a SECAM console.

**The canvas is not the screen. A 2600 pixel is wide.** Anything drawn at 100 % on a
square-pixel canvas — the form this project's artwork arrives in — is stretched horizontally on a
real display by somewhere between **1.60× and 1.82×**; the range is not measurement noise, it is the
question being underspecified, and `design-principles.md` explains why and records that it was
settled by deleting the constant rather than picking a value. The useful form for whoever is
drawing is Erik Mooney's (stella-list, 2001-10): **an object 8 pixels tall and 5 pixels wide reads
as a square.** So a circle is an oval on the canvas, taller than it is wide, and letterforms need
about half as many pixels across as down.
For a tile the same range puts an 8-pixel-wide cell at about 13–15 rows to read square
(8 × 1.60–1.82, at one scanline per row), and the advice given to someone drawing an 8×8 Link in 2014
moved that way: raindog, *"you might want to try 8x10 instead of 8x8 … you may even want to go up to
8x12 pixels per tile"*; gemintronic, *"I'd seriously consider going at around 14 pixels high"* (AtariAge
`topic/228629`; **Cited only, not verified**).

**On a PAL console it is worse, and on a different axis.** The pixel aspect is
`visible_lines / 120`, so PAL's 240 lines give **2.00** against NTSC's 1.60–1.82 — a letter keeps its
height and gains a fifth of its width again. A 45° line reads as **27°** there against **31°** on
NTSC. So "PAL support" changes the drawing twice over: the colours are re-chosen, and the proportions
move.
It changes the motion too: PAL runs 50 frames a second to NTSC's 60, and a port may retune each moving
thing separately. Destroyer's first PAL build (0.5b, 2026-01) set the destroyer's and the submarines'
speeds to be *"similar in both versions"* while *"the charges are moving a little quicker in the NTSC
version for now"*; a week earlier, at 0.4b, its author had said his console checks were on an LCD,
*"not the best to test real hardware"* (AtariAge `topic/386546`; **Cited only, not verified**). The Guide's way to make the change a
table swap is in `docs/techniques/subpixel-velocity.md`.

None of that has to be settled to work. The loop is *draw → build → look at it on the emulator →
adjust*, and the eye closes the gap in one pass; the number only matters for the first guess. What
would bite is trusting a square-dot preview and never looking at the render — so **look at the
render**, always, before deciding a shape is finished.

Why F12: Stella saves straight from its render buffer, so the image is an exact integer
multiple of the 160-clock TIA raster (e.g. 320×228 = 2×1) regardless of window size or Retina
display. An OS screenshot of the same window goes through the compositor and is rarely integer.

**Size rule (decided 2026-06-12): any integer multiple of the 160-clock raster is accepted** —
320, 480, 640 wide etc.; the scale is auto-detected. The contract is about the *source* (F12,
unmodified), not a fixed pixel size.

Checklist for grade A:
1. Stella → Options → Video & Audio → **TV effects: Disabled** (phosphor/blending shift colors).
   **That setting is for MEASURING, and it is the wrong one for LOOKING.** Everything this pipeline
   reports is pixel-exact and pre-television; the 2600's own style — colour striping read as a blend —
   only exists after the blur. Turn the effects back **on** for at least one pass before deciding a
   picture is finished, and treat that as a different activity from verifying it. See
   `design-principles.md`, *"Everything measured here is measured before the television."*
2. Press **F12** in-game; find the PNG via Options → Snapshot settings (save directory shown there).
   Tip: point the snapshot directory anywhere under `~/Documents` and give the path — the tools take
   a directory argument.
3. Leave the file where it is and give its path — as-is, no cropping, no resizing, no
   format conversion. That folder is the standing hand-off point ("put it here and Claude sees it").

## What the analyzer reports

- **Normalization**: detected scale (e.g. 2×1), TIA raster size (160×H). Vertical coordinates
  are *image-relative* — the absolute scanline cannot be known from pixels alone.
- **Palette quantization**: every pixel mapped to the nearest NTSC entry of the same palette
  table the harness renders with (Gopher2600 `specification.Spec.GetColor`). `avg_palette_dist`
  ≈ 0 means Gopher2600-rendered input; Stella inputs land a small constant distance away
  (different palette tables — expected, reported, harmless).
- **Color inventory**: every color as the byte you'd write to `COLUxx`, with screen share.
- **Warnings** instead of refusals: non-integer scale, low cell uniformity (filtered input),
  high palette distance.

## Extraction layers (M2/M3)

- **Playfield bands**: per-row background estimation (global mode color, per-row fallback for
  COLUBK gradients), 4-clock-aligned column folding, repeat/reflect/asymmetric halves,
  score-mode flag (same pattern, two colors), band compression, DASM `byte` tables in
  `pkg/playfield`'s verified bit order.
- **Sprites**: connected components of what's left → player (GRP bytes + per-row colors),
  missile/ball, or low-confidence large_object; equal shapes at 16/32/64 spacing fold into one
  NUSIZ entry. Reconciliation pass: tiny grid-aligned "playfield" (height ≤2, ≤2 columns)
  demotes back to the sprite layer.
- All of it is **round-trip proven in CI**: our own ROMs rendered, pseudo-Stella upscaled,
  re-extracted, compared against the source constants (litmus_pf exact bytes; pf_modes score +
  wall; Exerciser mountains vs live RAM; ball/walker GRP bit-for-bit; NUSIZ 3-copy fold).

## ROM field-testing (R6 — contract v3, the best input of all)

If you have the **ROM file**, skip screenshots entirely:
`go run ./cmd/fieldtest -rom game.bin [-warmup N -shots K -gap G -press right@60,fire@90]`
runs it in Gopher2600, captures K frames, and produces the full multi-frame analysis
(overlay/report.txt/report.json) in one shot. Point `-inbox` at whatever directory holds the ROMs (
nothing gets committed). F12 screenshots remain the fallback when only a running Stella exists.

## Multi-frame separation (M8/M9 — the general solution)

Single screenshots have a principled limit: where a sprite overlaps playfield, pixel ownership
is locally undecidable, and 30 Hz flicker objects are half-missing. **Feed 2–3 screenshots of
the same scene instead** (`analyze_image {paths: [...]}` / `cmd/ingest -in a.png,b.png,c.png`):

- per-pixel voting builds the **static layer** — playfield, backgrounds, parked objects
  (ladders, pit holes, leaf fringes) come out as `static_*` with a hint (`pf_fringe?` /
  `parked_object?`), never confused with moving sprites;
- per-frame diffs give the **dynamic layer** — true sprites, per frame, plus a **union of
  position-continuity tracks** (an animating, moving object — Pitfall's Harry at up to 18px/frame —
  is one track with a `poses` count); **flicker** now means only "blinking in place across
  skipped frames"; fully-grid-aligned dynamic cells carry an `animated_pf?` hint (scrolling
  starfields and the like);
- no repeating-structure assumption (this is what the reference-based repair of M7 could not
  promise); `unresolved_share` reports pixels that never settled (background animation).

**Contract v2:** for scenes with movement, press F12 two-three times in a row (don't resize the
window between shots) and give the directory holding the sequence. N=3 resolves ties that N=2 cannot.
Known limits: a sprite that never moves melts into the static layer (space the shots out);
*animated playfield* (e.g. scrolling starfields) lands in the dynamic layer as objects — true
to the pixels, noisy in semantics.

## Accuracy machinery (M5/M6)

- **Reconstruction fidelity**: every report carries `fidelity` — the report rendered back to a
  160×H plane and pixel-compared with the input. Own-ROM round-trips assert **100%** in CI;
  the Pizza Boy field image scores **99.93%**.
- Fragment merging (≤2px gaps, shared colors), context-aware PF↔sprite arbitration (thin
  "playfield" rows vertically touching same-colored sprite pixels are sprite strokes — score
  digits reassemble into complete rings), NUSIZ stretch hypotheses (2x/4x with ≥90% row
  conformance), empty-column splitting for digit strips, row-groups (score/gauge bundles),
  shape ids for identifying the same object appearing twice (the two cabs).
- The overlay draws numbered bounding boxes for every sprite — answer-check by eye.
- **Overlap repair (sprite-guided inpainting):** where a sprite crosses playfield, pixel
  ownership is locally undecidable — but if the same PF structure repeats elsewhere on screen,
  a clean reference band resolves it: sprite pixels absorbed into PF return to the sprite
  (restoring its art), PF bits hidden under the sprite are restored from the reference. Context
  demotion is per-column (a whole-band demotion dragged clean columns along — caught by the
  synthetic overlap test). Repairs only when a reference exists; otherwise it leaves things
  alone and says so via confidence. Pizza Boy: **fidelity 100.0%**, zero contaminated bands.

## MCP tool

`analyze_image {path}` runs the same pipeline live and returns the full report (structured) plus
the grid overlay inline; the overlay also lands at `$ATARI2600_INGEST_PATH` (default OS temp).
CLI equivalent: `cmd/ingest`.

## Static-layer residual — diagnosed (M-I)

Pitfall's static layer reconstructs at **98.6%**; the residual concentrates in canopy-fringe
rows 68–76 where leaf green ($D6) and trunk dark ($10) coexist **in the same playfield half on
the same scanline** — hardware-wise that requires a **mid-scanline COLUPF write**, which the
band model (one color per half) deliberately does not express. Modelling per-column PF colors
would misrepresent the register semantics, so this stays a documented limit: when you see a
low-confidence multi-color band, the game is doing mid-line color splits — read those rows with
`read_row` and author them as a timed-write kernel, not as band data.

## Honest limits

- One screenshot = **one frame of truth**: flicker-multiplexed objects (#10) appear half-missing;
  multi-frame techniques need multiple shots.
- An 8-px-wide, 4-clock-aligned shape is *undecidable* between playfield and sprite from pixels
  alone — extraction (M2/M3) emits confirmed data plus confidence-ranked candidates, and the
  final call stays with the author.
- Narrower than a playfield column leans the other way. A playfield pixel is a whole 4-clock column, 1/40 of
  the line, and `analyzeRowPF` (`internal/ingest/segment.go`) takes a column as playfield only when all four of
  its pixels are one non-background colour, handing every other non-background pixel to the sprite layer. A
  playfield column with a sprite over part of it fails the same test (the overlap repair above), so narrow is a
  lean, not proof. Glenn Saunders gave the by-eye form in 1997: *"Any graphics you see narrower than 1/40th of
  the screen is sprite usage. This includes the missile trails in Missile Command and the asteroids in
  Asteroids."* — answering Erik Mooney, who had guessed, unsure, at playfield for the trails and for at least
  the big rocks 〔stella-list `199703/msg00091`, `199703/msg00093`〕. **Cited only, not verified** for the two
  games.
- The playfield table comes out in **one layout**: a `byte` line per band holding every register
  (`PF0,PF1,PF2`, or six for an asymmetric band) — `DASMPlayfield` in `internal/ingest/emit.go`. The other
  common layout — **one labelled array per register** (`mountainsPF1: .byte …`) — is not emitted, and
  the inputs are images or a ROM (`cmd/ingest -in`, `cmd/fieldtest -rom`), never assembler source. masswerk's Tiny
  Playfield Editor reads and writes both layouts (its author: *"byte orders either per row (as before) or
  by labeled arrays per playfield register, both for import and for export"*, AtariAge `topic/305741`;
  **Cited only, not verified**), and `tools/research-w1-tooling.md` records that shape as worth adopting.
  Here, transposing into it is done by hand.
- **Sprite labels are numbers, not names.** `DASMSprites` (`internal/ingest/emit.go`) labels the tables
  `Spr0Gfx`, `Spr1Gfx`, … by position in the report's sprite list; the input file's name does not reach the
  emitted source. Andrew Davie took the other route for Fu Kung! in 2003: *"The frame 'names' come direct from
  the filename of the original graphics, so I now don't need to worry about frame numbering - I just use the
  mnemonic and it will assemble correctly, even if the frame table has extra frames added."* 〔stella-list
  `200301/msg00475`〕. So a label pasted from this output names a position, and it moves to another shape
  when one more or one fewer is found ahead of it; naming frames after the drawing files is done by hand.
  **Cited only, not verified.**

## Fitting the picture to the machine — decisions that go back to the artwork

Everything above reads a picture that already runs. Before that, someone fits a drawing to the objects
by hand, and the worked examples on AtariAge show the fitting changing the **drawing**, not only the
code. The rule is in `design-principles.md` (*"Do not fix the picture first and then assign objects"*);
these are cases. All are **Cited only, not verified**: none was built or run here, and the images in
the threads were not seen.

- **An assignment written down until it runs out.** BladeJunker (2011), fitting a duck-hunting screen:
  tree, trunk and ground as an asymmetric playfield *"with 3 color changes from the top to the bottom"*;
  the reticle as *"a flopped Player0 sprite with a couple side extensions using the Missle0 bit copied
  and set to 8 times width"*; the duck as Player1 with per-line colour plus *"a few Missle1 bits
  overlayed for any scanlines with 2 colors"*; then *"I ran out of objects for the buckshot pixels"*,
  with a way out — make the clouds' colour from the playfield and the ball is free. He was *"no
  programmer"*; a month earlier SeaGtGruff had called the screen *"pretty doable"* on the assumption of
  a custom kernel that changes background or playfield colours mid-line (AtariAge `topic/188233`).
- **Move the drawing onto the playfield grid.** johnnywc (2023) translated a Bruce Lee mockup element by
  element: the score as the standard 48-pixel sprite; the mountain tops as playfield plus a 48-pixel
  sprite plus the missiles *"(3 copies each) to smooth things out"*, at the price of HMOVE lines on the
  left; the buildings all playfield over a grey background; and the ladder — *"use PF so you don't use
  up a sprite, so I would change it so it would line up with the PF boundaries"*. The artist took it:
  *"PF resulution/alignment woud be perfect. Actually most ladder in the game have "space" to the left
  and right of them"*. That list also recommended CDFJ+ with the ARM; splendidnut's running prototype
  in the same thread began with a symmetric playfield and no ladders, and later refused to simplify the
  playfield graphics to stay inside 4K — *"a non-starter for me"* (AtariAge `topic/347106`).
- **Check a mockup against that grid before building.** SpiceWare (2015): *"There's 40 PF pixels across
  the screen. I made a 40x2 checkerboard image, scaled it to 640 across to match your mockup, and
  overlayed it on the mockup … The pixels of your tree do not line up with PF pixels"*; and for a
  mirrored playfield, *"chop the image exactly in half and the trunk is now 1/2 a PF pixel"* — the trunk
  *"needs to be 2 pixels wide, not 1"* (AtariAge `topic/242131`). This pipeline's overlay is not that
  check: its grid lines fall every 10 clocks (`internal/annotate`), so only the lines at multiples of 20
  land on a playfield-pixel edge, and a mockup is not grade-A input.
- **Gaps between tiles are a separator, not only coarseness.** Pitkat's R3 (2021) removed the gaps; its
  author MarcoJ kept R2 available (*"It could come down to taste"*), and later said *"the gaps do help
  convey separation for monochrome coloured objects side by side and also vertically"*, and that in R3 *"character
  objects and ladders"* stayed 7px wide, off centre, and going gapless *"helps the soil/concrete and
  bitmaps gel together moreso than the characters"*. One player, Pat Brady, preferred the gaps in play
  and gapless on the title and selection screens (AtariAge `topic/308669`).
- **Compressibility can be the budget.** deater78's Myst (16K E7 with 2K RAM, 2022–23): a scene is 448
  bytes and had to compress with ZX02 to 256 or fewer, decompressed into cartridge RAM, for about 60
  scenes to fit — *"it can't be too complex. The viewing pool scene I had to hack a lot of the detail out
  before it would compress small enough"*. Decompressing took more than 262 scanlines, so it was split
  up and paced by the timer to keep VSYNC (AtariAge `topic/338659`).
  Asked what a scene holds, he broke the 448 down (June 2023): *"16 bytes used to describe the scene,
  6\*48 bytes for the playfield, 48 bytes for foreground color, 48 bytes for the overlay, 48 bytes for
  overlay color"*. The kernel they feed: *"a 40x48 asymmetric playfield. There is a fixed background
  color, but each 4-scanline high line can have its own foreground color"*, overlaid by *"sprite1 that's
  an 8x48 strip of blocks (each overlay line can be its own color)"*, plus one vertical line in the
  pointer's colour from missile0 and an optional alternating background colour on the right side. A
  few days earlier in the same thread he had put it at *"512 bytes"* per scene (AtariAge `topic/338659`).
  **Cited only, not verified.**
- **A lattice from copies.** grafixbmp (2011): one player at *"3 close"*, the other at *"2 close"*
  placed between its copies, both drawing a tall strip of hexagons with the two-copy one offset half a
  hexagon down, gives a honeycomb; the colours are two — *"either one color if both are set to the same
  thing or alternating pattern of 3 blue and 2 red"* (AtariAge `topic/179163`). No ROM was posted.
