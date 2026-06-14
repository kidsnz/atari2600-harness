# TIA Studio — Web Research: Related Tools & Resources (W1 tooling)

Research date: 2026-06-13. Goal: build "TIA Studio" — a canvas tool to design a full Atari 2600 screen
(sprites/missiles/ball/PF/per-scanline color/composite placement) and generate kernels.

License via `gh api repos/<owner>/<repo> --jq .license.spdx_id`. "Adopt" = what TIA Studio should take.
Already-owned (do not re-evaluate as new): 8bitworkshop, javatari, PlayerPal, masswerk.

---

## Axis 1 — 2600 graphics/sprite/PF/screen/kernel editors

### PlayerPal 2600 (Kirk Israel / kisrael) — ALREADY OWNED
- URL: https://alienbill.com/2600/playerpal.html , v2.2: https://alienbill.com/2600/playerpalnext.html
- License: web tool (alienbill); Kirk's tools generally GPL on GitHub.
- Player sprite editor, animation frames, exports ready-to-run ASM + batari BASIC. Multicolor.

### PlayfieldPal (Kirk Israel) — ALREADY OWNED-adjacent
- URL: http://alienbill.com/2600/playfieldpal.html
- Playfield editor, symmetric/asym, exports asm.

### atari-background-builder (Kirk Israel / kirkjerk) — NEW, relevant
- URL: https://alienbill.com/2600/atari-background-builder/ — repo: https://github.com/kirkjerk/atari-background-builder
- License: **GPL-3.0**
- Playfield-only large graphic editor. **Imports JPG/PNG and shrinks to PF.** Exports batari Basic,
  asm (handles asym/mirror/repeat bit-flipping), and 48px splash minikernel.
- Adopt: image->PF import pipeline logic (connects to our cmd/ingest); asym/mirror/repeat bit-flip export;
  48px minikernel generation pattern. GPL => study, do not copy code into a non-GPL TIA Studio.

### bB Playfield Editor (RandomTerrain, based on background-builder) — NEW
- URL: https://www.randomterrain.com/bb-playfield-editor.html
- License: web tool (derived from kisrael GPL work).
- JS playfield editor for batari, more DPC+ row options than original. Adopt: DPC+ row-count UI ideas.

### masswerk Tiny VCS Sprite + Playfield Editors — ALREADY OWNED
- URL: https://www.masswerk.at/vcs-tools/
- License: Copyright N. Landsteiner 2018-2020, **no OSS license** (do not copy).
- Sprite editor (pattern only, no color); Playfield editor (sym/asym, repeat/mirror), exports asm
  by-rows or labeled-arrays-per-register. Plus Studio2600 (TIA sound, uses Javatari).
- Adopt (concept only): the "labeled arrays per PF register" export shape is clean for our generator.

### pfed — Atari 2600 Playfield Editor (mauri) — NEW
- URL: https://mauri.github.io/pfed/  (no public GitHub repo found / 404 on api)
- License: unknown (no repo metadata).
- Browser PF editor keyed to PF0/PF1/PF2. Minor; concept reference only.

### ocarneiro playfield editor gist — NEW (minor)
- URL: https://gist.github.com/ocarneiro/d4dd29af63e4990b85a377f5ad93c5f2
- License: gist, unstated. Small reference impl of PF bit packing.

### spritemate (Esshahn) — NEW, strong impl reference
- URL: https://github.com/Esshahn/spritemate (live: spritemate.com)
- License: **MIT**
- Browser sprite editor. **C64-targeted**, but it is the editor that chunkypixel/atari-dev-studio
  embeds. Clean modular JS canvas sprite editor.
- Adopt: MIT-licensed, embeddable canvas sprite-editor architecture we can fork/learn from directly.

### atari-dev-studio (chunkypixel) — NEW
- URL: https://github.com/chunkypixel/atari-dev-studio
- License: **GPL-3.0**
- VS Code extension for 2600/7800; bundles a spritemate-based sprite editor + tooling. GPL => reference.

### RT bB Sprite Editor (RandomTerrain) — NEW (minor)
- URL: https://www.randomterrain.com/2600bbsprite.html — sprite+animation for batari.

### SourceForge "Atari VCS (2600) Graphics Editor" — NEW (minor, legacy)
- URL: https://sourceforge.net/projects/vcs-graphics/ — desktop graphics editor, dated. License per SF page.

---

## Axis 2 — Composition / scene / kernel generation (closest to our M3 composite canvas)

### vcs-game-maker (haroldo-ok) — NEW, CLOSEST EXISTING TO M3
- URL: https://github.com/haroldo-ok/vcs-game-maker (live: haroldo-ok.itch.io/vcs-game-maker)
- License: **MIT**
- No-code 2600 maker. Uses **Blockly** for logic, generates **batari Basic**, compiles via
  **batari-basic-js** (batari Basic in browser), previews with **Javatari**.
- Caveat: composition is via batari standard kernel (limited multi-sprite); not a free pixel-placement
  composite canvas. Docs do not confirm a drag-drop scene editor — it is block-logic + bB assets.
- Adopt (HIGH): the *full browser pipeline* is exactly our shape — MIT-licensed reference for
  batari-basic-js compile-in-browser + Javatari preview wiring. Our M3 differs by doing true per-object
  composite placement + kernel gen rather than bB standard kernel.

### batari Basic standard/multisprite kernels (concept)
- The "compose multiple objects -> kernel" problem on 2600 is solved in practice by bB's standard kernel
  (2 players + missiles + ball + PF) and multisprite kernel (up to ~6 sprites via multiplexing).
- Adopt: study bB kernel object model as the semantic target our composite canvas serializes to.
- No single OSS "visual composite scene -> hand-rolled kernel generator" tool was found. **This is the
  genuine gap TIA Studio M3 fills.** Closest neighbors are vcs-game-maker (bB) and the per-tool exporters
  in PlayerPal/PlayfieldPal/background-builder, none of which compose a whole screen of heterogeneous
  objects with per-scanline color into one generated kernel.

### aloan "Atari 2600 Graphics Simplified" — NEW but proprietary/not relevant
- URL: https://aloan.neocities.org/atari_graphics_simplified
- License: proprietary (Clickteam Fusion 2.5 based, OneDrive distributed). Not code-generating, not OSS.
  Mentioned only to mark as a dead end for our purposes.

---

## Axis 3 — In-browser 2600 emulator / TIA renderer cores (embed? pixel readback? license?)

### Stellerator-embedded / 6502.ts (DirtyHairy / 6502ts) — NEW, BEST EMBED CANDIDATE
- URL: https://github.com/6502ts/6502.ts , docs https://6502ts.github.io/typedoc/stellerator-embedded/
- License: **MIT**
- Purpose-built **embeddable** TIA/VCS emulation library: `new Stellerator(canvas, 'stellerator.js')`,
  `.run(rom, TvMode.ntsc)`, web-worker backend, save states, config (TV emu/phosphor/scanlines).
- Pixel readback: renders to a canvas you supply -> we can `getImageData()` off that canvas for pixel
  comparison/readback even though there is no documented raw-framebuffer API.
- Adopt (HIGHEST for embed): MIT license = safe to embed in TIA Studio; TypeScript; clean public API;
  canvas-based so readback is trivial. **Top pick for the in-page emulator/preview pane.**

### nostalgist.js (arianrhodsandlot) — NEW, alt embed path
- URL: https://github.com/arianrhodsandlot/nostalgist (docs nostalgist.js.org)
- License: **MIT**
- Wraps RetroArch/Emscripten cores (Stella for 2600). Programmatic launch; exposes
  `getEmscriptenModule()`/`getEmscriptenFS()` low-level APIs. Renders to canvas (=> getImageData readback).
- Caveat: underlying Stella libretro core is GPL; nostalgist wrapper is MIT. Heavier than 6502.ts.
- Adopt: fallback if we want full-accuracy Stella in-browser; MIT wrapper, but ships GPL core.

### javatari.js (ppeccin) — ALREADY OWNED — LICENSE WARNING
- URL: https://github.com/ppeccin/javatari.js (javatari.org)
- License: **AGPL-3.0** (!!). Embedding AGPL in a network-served TIA Studio likely triggers source-
  disclosure of the whole app. **Flag: prefer 6502.ts (MIT) over javatari for embedding.**

### jsAtari (docmarionum1) — NEW
- URL: https://github.com/docmarionum1/jsAtari — License: **GPL-3.0**. JS 2600 emulator; older. Reference.

### atari2600-wasm (ColinEberhardt) — NEW — LICENSE WARNING
- URL: https://github.com/ColinEberhardt/atari2600-wasm
- License: **NONE / unset** => all rights reserved, do NOT use. AssemblyScript->WASM 2600 emu. Interesting
  tech (WASM core) but legally unusable without contacting author.

### 8bitworkshop emulator (sehugg) — ALREADY OWNED
- URL: https://github.com/sehugg/8bitworkshop — License: **GPL-3.0** (samples CC0). Its VCS path historically
  used Javatari.js. Reference, not to be linked into a non-GPL app.

### EmulatorJS — NEW (heavy, GPL)
- URL: https://github.com/EmulatorJS/EmulatorJS — License: **GPL-3.0**. RetroArch frontend; overkill.

### Other cores (reference only)
- TomHarte/CLK (MIT, C++ multi-system, not browser), rejunity/tiny-atari-2600 (Apache-2.0, Verilog SoC),
  JetSetIlly/Gopher2600 (GPL-3.0 — note: this is OUR harness engine), stella-emu/stella (GPL-2.0).

---

## Axis 4 — Image -> 2600 converters / sprite & PF data formats

- **atari-background-builder** (GPL-3.0): image (PNG/JPG) -> playfield, with shrink + bit-flip export.
  Best existing image->PF reference; connects directly to our cmd/ingest goal. (See Axis 1.)
- **abc — Amiga & Atari bitmap converter** (arnaud-carre / AnimaInCorpore): https://github.com/arnaud-carre/abc
  — PNG->bitplanes/sprite sheets, but targets Amiga/Atari ST (bitplanes), NOT 2600 TIA format. Format
  model differs; concept only.
- Data-format conventions to standardize on in TIA Studio export:
  - Player/sprite: one byte per scanline, MSB-left, top-to-bottom (PlayerPal convention).
  - Playfield: PF0 (bits 4-7, reversed), PF1 (bits 7-0), PF2 (bits 0-7) per the PF0/PF1/PF2 nibble/byte
    ordering — masswerk's "labeled arrays per register" and background-builder's bit-flipping are the
    canonical references.
  - Missiles/Ball: width via NUSIZ/CTRLPF, position via RESMx/RESBL + HMxx fine adjust.
- No clean OSS "image -> full TIA composite (sprite+missile+ball+PF+color)" converter exists; our cmd/ingest
  + composite canvas is novel here.

---

## Axis 5 — Canvas pixel-editor implementation frameworks/libraries

### spritemate (Esshahn) — MIT — best small-canvas-editor reference
- https://github.com/Esshahn/spritemate — modular vanilla-JS canvas sprite editor; MIT means we can fork.

### Piskel (juliandescottes / piskelapp) — NEW
- URL: https://github.com/juliandescottes/piskel — License: **Apache-2.0**
- Mature web pixel/sprite editor. HTML5 canvas, MVC + Observer, layer system, animation frames,
  tools-left/canvas-center/settings-right layout.
- Adopt: layer model, frame/animation model, canvas rendering loop, MVC+Observer separation. Apache-2.0
  is permissive (compatible). Heavier than we need but the best-architected reference.

### Pixelorama (Orama-Interactive) — NEW
- URL: https://github.com/Orama-Interactive/Pixelorama — License: **MIT**
- Godot-based desktop pixel editor. MIT. Useful for tool UX patterns (layers, palettes, tile mode) but
  Godot/GDScript stack — not directly reusable in a web TIA Studio.

### Eloquent JS canvas / vanilla approach
- For TIA Studio's bespoke per-scanline + per-object model, a thin custom canvas layer (single <canvas>,
  ImageData blits, an object/scanline model) is likely better than adopting a full editor framework.
  Use Piskel/spritemate as architecture references, not dependencies.

---

## License quick-reference (for adoption decisions)
- SAFE to embed/fork (permissive): 6502.ts/Stellerator (MIT), nostalgist.js (MIT, ships GPL core),
  spritemate (MIT), vcs-game-maker (MIT), Pixelorama (MIT), Piskel (Apache-2.0).
- STUDY-ONLY (copyleft, don't link into non-GPL app): javatari.js (**AGPL-3.0** — strongest caution),
  8bitworkshop / jsAtari / atari-dev-studio / atari-background-builder / EmulatorJS / Stella / Gopher2600
  (GPL-2.0/3.0).
- DO NOT USE (no license): ColinEberhardt/atari2600-wasm (all rights reserved); masswerk tools
  (copyrighted, no OSS license); aloan tool (proprietary).
