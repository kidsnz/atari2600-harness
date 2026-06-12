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

## MCP tool

`analyze_image {path}` runs the same pipeline live and returns the full report (structured) plus
the grid overlay inline; the overlay also lands at `$ATARI2600_INGEST_PATH` (default OS temp).
CLI equivalent: `cmd/ingest`.

## Honest limits

- One screenshot = **one frame of truth**: flicker-multiplexed objects (#10) appear half-missing;
  multi-frame techniques need multiple shots.
- An 8-px-wide, 4-clock-aligned shape is *undecidable* between playfield and sprite from pixels
  alone — extraction (M2/M3) emits confirmed data plus confidence-ranked candidates, and the
  final call stays with the author.
