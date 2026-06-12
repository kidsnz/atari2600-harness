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

Why F12: Stella saves straight from its render buffer, so the image is an exact integer
multiple of the 160-clock TIA raster (e.g. 320×228 = 2×1) regardless of window size or Retina
display. An OS screenshot of the same window goes through the compositor and is rarely integer.

**Size rule (decided 2026-06-12): any integer multiple of the 160-clock raster is accepted** —
320, 480, 640 wide etc.; the scale is auto-detected. The contract is about the *source* (F12,
unmodified), not a fixed pixel size.

Checklist for grade A:
1. Stella → Options → Video & Audio → **TV effects: Disabled** (phosphor/blending shift colors).
2. Press **F12** in-game; find the PNG via Options → Snapshot settings (save directory shown there).
   Tip: point the snapshot directory at the project's **`inbox/`** folder (next to `harness/`).
3. Drop the file in **`260609_atari2600-dev/inbox/`** as-is — no cropping, no resizing, no
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
window between shots) and drop the sequence into `inbox/`. N=3 resolves ties that N=2 cannot.
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
