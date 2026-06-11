# Hardening & deepening roadmap — strengthening the harness

The base gaps A–E are all closed (v0.21.0), and the harness is spun out as a standalone public repo. This
document is the **prioritized roadmap for the next phase: making the harness stronger** — not closing the
original gaps, but (Theme A) **deepening authoring + verification coverage into the domains that are still
thin (sprites, audio)** and (Theme B) **hardening the foundation (CI, trust, completing stub tools)**.
Implementation happens in separate sessions. Each item lists **where to touch (real files, verified
Gopher2600 symbols) + how to verify + size** so work can start without guesswork (same style as
`improvement-roadmap.md`).

## Central observation — coverage is uneven

The `pkg/playfield` line is deep: an encoder (ASCII → PF0/1/2), a litmus ROM, and `read_row` numeric
verification. But the same "spec → ASM + numeric litmus" pattern has **not** been extended to two domains
the model authors constantly:
- **Sprites have no authoring tool.** `pkg/` contains only `playfield`. Game sprites are hand-written raw
  bytes (`roms/frogger/gen/main.go`: `pad := []byte{0x3C,0x7E,...}`). The **read side is already rich**
  (`read_tia_registers` exposes `GfxNew`/`GfxOld`/`Nusiz`/`SizeAndCopies`/`Reflected`/`VerticalDelay` at
  `internal/emu/emu.go:310-367`; `read_tia` gives `ResetPixel`/`HmovedPixel`; `read_row` exists) — so adding
  an authoring helper means it can be verified numerically immediately.
- **Audio is read-only.** `read_audio` (`internal/emu/emu.go:412`, `Audio.PeekChannels`) returns raw
  AUDC/AUDF/AUDV, but there is no authoring side and no semantic/golden verification. Gopher2600's `tracker`
  (`lookupMusicalNote`/`lookupDistortion`/`NoteToPianoKey`) and `digest/audio.go` are **already vendored** —
  realizable by wiring.
- **No CI** (`.github/workflows` absent) — a public repo with no automated verification.
- `step_clock` / `watch|trap` are unimplemented; `run_scenario` is CLI-only (not an MCP tool).

Unifying axis: **extend the proven `pkg/playfield` "spec → ASM + numeric litmus" pattern into the thin
domains (sprites, audio)**, and **shore up the public repo's defenses (CI, trust)**.

---

## Theme A — deepen authoring + verification into thin domains

### S. Sprites

#### S-1. `pkg/sprite` (ASCII → GRP)  — size: small–medium — ✅ DONE (v0.23.0)
Implemented: `pkg/sprite` `EncodeRow`/`Encode`/`Reflect`; `roms/litmus/litmus_sprite.asm` + `scenarios/sprite.json`
prove a ramp sprite byte-exact via `read_row` (white span widens 1→8 px from clock 3).

- **Problem:** no encoder for player graphics; sprites are hand-coded byte arrays.
- **Proposal:** a mirror of `pkg/playfield`. Convert an 8px-wide × N-row ASCII design (`.`/`X`) into a GRP
  byte table + a kernel fragment. Support REFP (reflect) and VDEL (vertical delay). One byte per scanline,
  MSB = leftmost pixel.
- **Where to touch:** new `pkg/sprite/sprite.go` (alongside `pkg/playfield/playfield.go`); reuse the
  `ParseASCIIRow` idea. Game side imports it from `<game>/gen` like playfield.
- **Verify:** `read_tia_registers.GfxNew` equals the encoded byte per scanline; `read_row` shows the lit
  span matching the design. A `roms/litmus/litmus_sprite.asm` proving an ASCII shape renders byte-exact.

#### S-2. NUSIZ helper  — size: small — ✅ DONE (v0.25.0)
Implemented: `PlayerSize`/`MissileSize` + `NUSIZ()`/`NUSIZPlayer()`; `litmus_nusiz.asm` + `scenarios/nusiz.json`
prove DoubleWidth (`NUSIZ0=$05`) renders 16px (`read_row` len 16, `player0.nusiz=5`).

- **Problem:** NUSIZ (copies/width) is set by raw value; intent ("3 copies, wide spacing", "double/quad
  width") is opaque.
- **Proposal:** a helper mapping design intent → NUSIZ value (and back), covering the 8 size/copy modes for
  players and missile/ball width.
- **Where to touch:** `pkg/sprite` (or `pkg/tia` constants). 
- **Verify:** `read_tia_registers.SizeAndCopies` + `read_tia` positions of each rendered copy.

#### S-3. ★ Two-sprite combine (P0+P1) = up to 16px / multicolor characters — size: medium — FLAGSHIP — ✅ DONE (v0.24.0)
Implemented: `pkg/sprite.SplitWide`; `litmus_p0p1.asm` + `scenarios/p0p1.json` place P1 exactly +8px from P0
(RESP0→RESP1 3cy apart in the visible region = +9px, HMOVE −1 = +8) and prove a **seamless continuous 16px run**
(`read_row` clock 69–84 len 16; `read_tia` player0=69/player1=77).

- **Problem:** a single player is 8px wide and one color. Wider or multicolor characters require combining
  P0 and P1 — the classic technique — but there is no tooling to lay out and **place them seamlessly**.
- **Proposal:** author a ≤16px (or multicolor) design as two 8px halves — P0 = left 8, P1 = right 8 — and
  use the verified X(N) calibration (`cmd/calibrate` / `internal/calibrate`) to place **P1 exactly +8px to
  the right of P0** so the halves abut with no seam. For multicolor, P0/P1 carry separate COLUP. For wide
  characters, also expose the NUSIZ double/quad-width path (S-2).
- **Where to touch:** `pkg/sprite` (split a 16-wide design into two GRP tables + emit the RESP/HMOVE offset
  to place P1 at P0+8, computed from the kernel's calibrated offset); a litmus ROM `roms/litmus/litmus_p0p1.asm`.
- **Verify (numeric, no eyeballing):** `read_row` shows a **single continuous lit span up to 16px wide** (no
  gap/overlap at the seam); `read_tia` shows `player0`/`player1` `HmovedPixel` exactly 8 apart. This is the
  headline capability and the proof that sprite placement is as numerically trustworthy as playfield.

#### S-4. Sprite-shape verification extensions  — size: medium
- **Problem:** `get_screen_annotated` marks only sprite X; it doesn't show shape, and there's no direct
  GRP-bytes-vs-rendered-pixels cross-check.
- **Proposal:** overlay the sprite **shape / bounding box** in the annotated screenshot; and/or a
  `read_sprite_shape` that scans `read_row` vertically over a sprite's scanline range and reconstructs the
  rendered rows to compare against the GRP table.
- **Where to touch:** `internal/annotate` (overlay), `internal/emu` (vertical row scan), `cmd/harness` (tool).
- **Verify:** reconstructed rows == encoded GRP for `litmus_sprite`.

### A. Audio

#### A-1. Semantic verification = note/timbre names in `read_audio`  — size: small (tracker in-tree)
- **Problem:** `read_audio` returns only raw AUDC/AUDF/AUDV; "is this a C-4 square or a noise sweep?" isn't
  answerable numerically.
- **Proposal:** wire Gopher2600 `tracker.lookupMusicalNote` / `lookupDistortion` (and `NoteToPianoKey`) so
  `read_audio` returns **note name + timbre name** alongside the raw values.
- **Where to touch:** `internal/emu/emu.go` (`ReadAudio` augments `AudioChannel` with `note`/`timbre`),
  `cmd/harness` (`read_audio` Out); source = `Gopher2600/tracker/descriptions.go` (`lookupMusicalNote`,
  `lookupDistortion`), `Gopher2600/tracker/pianokeys.go`.
- **Verify:** known writes on `roms/litmus/litmus_audio.asm` map to expected note/timbre names.

#### A-2. `digest.Audio` golden  — size: small–medium (digest/audio.go in-tree) — ✅ DONE (v0.26.0)
Implemented: `checks.golden_audio` compares a `digest.Audio` chain hash against `<scenario>.audio.golden`
(emu `Enable/Reset/AudioHash`, shared golden eval). Verified on `litmus_audio`.

- **Problem:** golden regression covers only video (`digest.Video`, v0.19); audio has no golden.
- **Proposal:** mirror the video golden for audio. Add `checks.golden_audio` to scenarios, comparing a
  sha1 audio-chain against `<scenario>.audio.golden`.
- **Where to touch:** `internal/emu` (wire `Gopher2600/digest/audio.go` like the existing `digest.Video`
  wiring `EnableVideoDigest`/`VideoHash`), `internal/scenario` (new check, `docs/scenarios.md`).
- **Verify:** deterministic audio hash across runs; mismatch fails.

#### A-3. `pkg/audio` authoring helper  — size: medium
- **Problem:** no way to author sound; effects would be hand-poked.
- **Proposal:** note→AUDF/AUDC/AUDV tables (Paul Slocum's 8 common timbres + pitch table) and minimal SFX
  recipes (hop = short Square, drown = descending Noise, win = rising arpeggio) emitted as ASM data + a tiny
  driver fragment (hit AUDx in spare VBLANK/Overscan cycles).
- **Where to touch:** new `pkg/audio/audio.go`; references `docs/resources.md` (Slocum guide), the past
  `za2600/audio.asm` driver structure (idea only, clean-room).
- **Verify:** `read_audio` over frames matches the intended note/timbre/volume envelope (uses A-1 names).

---

## Theme B — harden the foundation (robustness / trust)

#### F-1. ★ CI (GitHub Actions)  — size: small–medium — HIGH PRIORITY — ✅ DONE (v0.28.0)
Implemented: `.github/workflows/ci.yml` (ubuntu + go-from-go.mod + DASM, clones Gopher2600 at the pinned
commit, assembles litmus ROMs, `CGO_ENABLED=0` build/vet/test + litmus scenarios). No SDL needed. Badge on
README. (Already paid off: it immediately surfaced a flaky `TestStepScanline`, fixed in v0.29.1.)

- **Problem:** no automated verification; regressions can land silently on a public repo.
- **Proposal:** a workflow running `go build ./...` + `go vet ./...` + `go test ./...` + the litmus
  scenarios on push/PR.
- **Where to touch:** new `.github/workflows/ci.yml`. Catch: Gopher2600 is gitignored, so CI must `git clone`
  it (pin a commit) — or adopt F-2 to drop the local clone entirely (cleaner CI).
- **Verify:** the workflow goes green on a clean checkout; a deliberately broken test fails it.

#### F-2. Pin Gopher2600 to a version (optional; makes F-1 trivial)  — size: medium (verification risk)
- **Problem:** `go.mod` uses `replace github.com/jetsetilly/gopher2600 => ./Gopher2600` (a local clone),
  which complicates CI and clones.
- **Proposal:** replace it with a pinned tagged module dependency, **if** the exported API the harness uses
  exists in a release (needs checking — the project currently tracks a local/nightly clone). Removes the
  ~big local `Gopher2600/` folder and makes CI a plain `go test`.
- **Where to touch:** `go.mod` (drop `replace`, add a pinned `require`); README setup section.
- **Verify:** `go build ./... && go test ./...` green against the pinned version; all litmus scenarios PASS.

#### F-3. PAL/SECAM verification  — size: small–medium — ✅ DONE (v0.27.0, PAL)
Implemented: `litmus_pal.asm` (312 = 3/45/228/36) + `scenarios/pal.json` (tv_spec PAL) assert 312 lines.
(SECAM not separately covered; same mechanism.)

- **Problem:** the harness is NTSC-centric; constants list PAL (312 lines) but there's no PAL litmus/tests.
- **Proposal:** a PAL litmus ROM and tests asserting the 312-line / 3/45/228/36 budget, plus position
  behavior under PAL.
- **Where to touch:** `roms/litmus/` (PAL ROM), `internal/emu` tests; `load_rom` already accepts `tv_spec`.
- **Verify:** PAL scenario asserts 312 lines and correct beam coords.

#### F-4. Stella oracle cross-check  — size: medium–large
- **Problem:** `gap-analysis.md` flags that Gopher2600 annotation pixels are never cross-checked against the
  Stella oracle.
- **Proposal:** a cross-check harness that drives Stella (`-sssingle -ss1x -dbg.script` + `dump`) and
  compares its dump against Gopher2600's `read_*` for the same ROM/frame.
- **Where to touch:** new `cmd/oracle` or `internal/oracle`; Stella flags per `docs/resources.md`.
- **Verify:** agreement within tolerance on litmus ROMs; a planted discrepancy is reported.

#### F-5. Complete the stub tools  — size: small–medium each
- `step_clock` (color-clock granularity) — needs a finer Gopher2600 hook than per-instruction `Step`.
- `watch|trap` (halt on a RAM/collision condition) — `breakif` covers beam position; Gopher2600's
  `debugger/halt_*` types are unexported (per `improvement-roadmap.md` G-1), so implement in
  `internal/emu`'s own step loop using exported state.
- `run_scenario` as an MCP tool — scenarios are CLI-only; share logic with `cmd/scenario`.
- **Where to touch:** `internal/emu`, `cmd/harness`, `internal/scenario`.

---

## Theme C — wire upstream Gopher2600 libraries (continuation of G-1)

Low cost, high leverage — already noted in `improvement-roadmap.md` G-1:
- `recorder` / `regression` — battle-tested record/replay + a regression DB to harden D-2/D-3 instead of the
  homegrown path.
- `reflection` — per-video-step element attribution → annotate "which object drew this pixel" (feeds S-4).

---

## Recommended order of attack
1. ~~**S-1 / S-2 / S-3 sprites** (flagship S-3 P0+P1 16px).~~ ✅ DONE (v0.23.0–v0.25.0). Sprite trio complete.
2. ~~**F-1 CI**.~~ ✅ DONE (v0.28.0). Reliable after fixing a flaky test it surfaced (v0.29.1).
3. ~~**A-2 audio golden** / **F-3 PAL** / use pkg/sprite in Frogger.~~ ✅ DONE (v0.26.0 / v0.27.0 / roms v0.1.2).
4. **Remaining (need the user or more design):**
   - **A-3 audio authoring + A-1 note/timbre names** — parked: needs authoritative Slocum reference data to
     name notes/timbres correctly, and SFX quality is best judged by ear (Stella) with the user.
   - **F-5 `run_scenario` MCP** — parked: rom-path resolution when invoked from the MCP server's CWD is a
     design fork for the user.
   - **S-4 sprite shape / Theme C `reflection` annotation** — image-side; harder to verify numerically autonomously.
   - **F-2 Gopher2600 version pin / F-4 Stella oracle cross-check / VDEL & more collision-pair litmus** — open.

> Verified coverage added this phase also includes: NUSIZ double-width (v0.25) + multi-copy (v0.29),
> missile/ball position (v0.30), REFP reflect (v0.31). The sprite/object verification surface is now broad.
> When implementing any MCP-tool change, follow CLAUDE.md "Smoke-test harness before reconnect".

---

## v2 backlog — from the 2026-06 fundamentals audit

Distilled from `fundamentals-audit.md` (six research squads over the local corpus + ~22 owner links + web).
Priority = general/foundational value × cost × how much it unlocks. Each item ships via the usual pipeline
(branch → litmus/numeric verify → CI → tag).

### Tier 1 — foundational gaps that block techniques
- [x] **V2-1 VDEL litmus.** ✅ DONE (v0.39.0, `litmus_vdel` + `scenarios/vdel.json`). Verify the write-triggered cross-copies (GRP0→P1 shadow; GRP1→P0 *and* ENABL
  shadows), VDELxx D0 selecting the old copy, and the 2LK alignment relation (VDELP0=1/VDELP1=0 → same Y;
  0/1 → Y+1). Prereq for 48px score and vertical positioning. Src: Stella PG §6.D; SpiceWare Step 4/5.
- [x] **V2-2 HMOVE side-effects litmus.** ✅ DONE (v0.40.0, `litmus_hmove_side`). (a) 8px left blank on HMOVE-after-WSYNC lines (incl. all-HMxx=0);
  (b) mid-line HMOVE calibration sweep (expect rightward "plugging" ≈1px/4CLK; Cosmic Ark family);
  (c) HMxx writes inside the 24-cycle freeze — capture Gopher2600's table. Src: Towers TIA_HW_Notes.
- [x] **V2-3 Asymmetric-PF write-window litmus.** ✅ DONE (v0.41.0, `litmus_pf_async`). Boundary probes around woodgrain's conservative table
  (LPF0≤21, LPF1≤27, LPF2≤37, RPF0 27–48, RPF1 37–53, RPF2 48–64; reflected RPF2 == 48 exactly); verify
  per-pixel split on late writes; settle the SpiceWare Step3(66) vs Step7(71) discrepancy.
- [x] **V2-4 Input litmus (joystick + latch).** ✅ DONE (v0.42.0, `litmus_input` + input-timeline scenario).
- [ ] **V2-4b Paddles.** Add a paddle path to `set_input` (plug Gopher2600 paddle peripheral), then verify
  INPT0–3 dump/charge (VBLANK D7) with a scanline-count calibration of the transfer curve.
- [x] **V2-5 Bankswitch F8 litmus + `read_bank` MCP tool.** ✅ DONE (v0.43.0, `litmus_bank` + `bank.*` scenario fields). 8K two-bank ROM (per-bank sentinels/colors,
  vectors+stub+trampoline in both banks); assert bank via RAM sentinels and the new tool
  (Gopher2600 `Cartridge.GetBank()` already exposes it). Then F6 variant; F8SC later.
- [x] **V2-6 6502 precision litmus.** ✅ DONE (v0.44.0, `litmus_6502`) — **Tier 1 complete.** STA abs,X fixed-5 vs LDA abs,X +1 page-cross; branch +1/+2 (measured
  from next instruction); NMOS BCD (C valid, Z/N counterexample, SED/CLD); JMP ($xxFF) bug; skipdraw
  18-cycle constancy incl. illegal `dcp` (also certifies illegal-opcode support).

### Tier 2 — completing the matrix
- [x] **V2-7 CTRLPF litmus.** ✅ DONE (v0.45.0, `litmus_ctrlpf`; SCORE×PFP→COLUPF flagged for Stella). SCORE mode colors, PFP priority, ball width bits; measure the unspecified
  SCORE×PFP interaction.
- [x] **V2-8 Collision pairs (all 15).** ✅ DONE (v0.48.0, `litmus_collide_all`). Remaining 12 CXxx pairs; verify the BIT N/V two-pairs-per-read idiom.
- [x] **V2-9 LFSR litmus.** ✅ DONE (v0.46.0, `litmus_lfsr`; Galois period 255 + never-zero). Pitfall Fibonacci both directions from $C4 (verified prefixes, period 255,
  N-right-then-N-left identity) + Galois `eor #$8E` / `eor #$B4` periods. Pure read_ram asserts.
- [x] **V2-10 RIOT timer litmus.** ✅ DONE (v0.47.0, `litmus_timer`; reading INTIM clears TIMINT D7). TIM64T expiry timing, post-underflow $FF/1-per-cycle behavior, TIMINT D7
  (+ does INTIM read clear it), first-decrement offset.
- [ ] **V2-11 RESPx/RESBL edge litmus.** Double-strobe in one line; RESBL mid-line restart (vs RESPx wrap);
  missile-unlock offset from RESMP.
- [x] **V2-12 Mirrors litmus.** ✅ DONE (v0.49.0, `litmus_mirror`). TIA via $0040/$2000, RAM via the $0180 stack page, ROM odd-$x000 mirrors.
- [ ] **V2-13 48px GRP window litmus.** Derive the 6-store cycle map ourselves (score6.asm recipe; no source
  documents it) — after V2-1.

### Tier 3 — capabilities (tools/packages)
- [ ] **V2-14 Audio: duplicate-AUDC digest-equality scenarios** ({4,5} {6,10} {7,9} {12,13} {0,11} hash-equal)
  — possible today, zero new code.
- [ ] **V2-15 Audio sample capture.** Second `AddAudioMixer` hook capturing raw per-channel samples →
  zero-crossing/autocorrelation pitch measurement → falsify Slocum note tables in CI (±cents asserts,
  calibrated to the emulator's actual sample rate). Unlocks A-3/pkg/audio with verification.
- [ ] **V2-16 `pkg/audio`.** AUDC names+duplicate map, `freq(audc,audf,spec)`, Slocum's three tuning setups,
  note-byte codec (Sequencer Kit/slocum-tracker interop), SFX recipe helpers. (Resumes parked A-3 — the
  authoritative data was in our `reference/` all along.)
- [ ] **V2-17 F-4 Stella oracle automation v1.** `<rom>.script` (`frame N / tia / riot / dump 80 ff 7 /
  saveSes`) + driver that launches `Stella -debug`, scrapes the session file, kills the process; compare
  RAM + TIA registers vs harness at frame N. One-time frame-numbering calibration probe. Image compare = v2.
- [ ] **V2-18 RAM-map audit helper.** Symbols → read/write coverage (catches dead variables à la Pitfall's
  `cxHarry`); cheap pass over existing trace hooks.

### Constants to adopt into CLAUDE.md (from the audit)
24-cycle HMxx freeze after HMOVE · stores never pay page-cross penalties (deterministic kernels) ·
NMOS decimal mode: only C valid · CLD mandatory at init · never cite cycle_counting_guide for positions.
