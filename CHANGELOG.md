# Changelog

The change history of this project. Format follows [Keep a Changelog](https://keepachangelog.com/);
versions follow [Semantic Versioning](https://semver.org/).

> Entries from v0.17.0 and earlier are condensed; the full detailed history (in Japanese) is kept locally
> in `CHANGELOG.ja.md`.

## [Unreleased]

### Planned
- Real game authoring on top of the 1.0 base (1.x).
- Stella oracle v2 (TIA/pixel compare, full keystroke automation); Slocum note-table transcription for composing.

## [1.57.1] - 2026-06-12

### Changed
- `aa_fetch.py` defaults to **lean storage**: raw HTML cache is deleted after parsing (Wayback
  itself is the permanent archive — re-fetchable anytime), attachments are listed in thread.md
  but only downloaded with `-attachments`, and `-keep-raw` opts back into caching. Keeps only
  the distillate (thread.md / gaps.md / notes). Demonstrated: a 2-page topic harvests to 80KB.

## [1.57.0] - 2026-06-12

### Added
- **`scripts/aa_fetch.py` — AtariAge thread-mining pipeline** (Wayback-first): the live forum
  sits behind a Cloudflare bot challenge, so the tool enumerates snapshots via the CDX API
  (both old/new domains), caches raw pages, parses IPB posts into a single `thread.md`
  (author/date/body), and recovers attachments (attachment.php redirects need
  status-filterless CDX + replay-URL following with retries). Gaps are reported for cookie/
  manual fallback (`AA_COOKIE` env supported; no passwords). First run: Medieval Mayhem topic —
  17/17 pages, 400 posts, dev-build ROMs recovered and analyzed with fieldtest/dissect
  (analysis artifacts stay in the non-repo reference/ area per the clean-room policy).

## [1.56.0] - 2026-06-12

### Changed
- Strengthening-run U wrap-up: summary section in `docs/improvement-roadmap.md` (P1-P4, 13
  harness releases + starshot v1.0 dogfood in the roms repo). Techniques catalog now covers the
  full real-game skeleton: score, SFX, sound driver, game states, bullets, paddle, procgen,
  bank template — every entry with a verified ROM + scenario + golden.

## [1.55.0] - 2026-06-12

### Added
- **`cmd/rammap`** (V2-18 closed): per-frame RAM diff over N frames → markdown usage map
  (address, change rate, value range, constant/per-frame hints). Feeds `docs/ram-maps.md` and
  audits our own ROMs' RAM budgets.
- **`scripts/check_gopher_pin.sh`** (F-2 closed): verifies the local Gopher2600 clone matches the
  CI-pinned SHA. Hardening-roadmap statuses updated (A-1/S-4/F-2/F-4/V2-18 all ✅).

## [1.54.0] - 2026-06-12

### Added
- **Stella oracle v2 — pixel compare** (`stellacheck -pixels` / `-snap`, F-4 closed): captures a
  Stella debugger `savesnap` PNG and compares it against Gopher2600's frame as TIA color codes,
  using a **measured Stella NTSC palette** (`internal/ingest/palette_stella.go`,
  `NewStellaNTSCQuantizer`) captured live from the new `litmus_palette.bin` (white marker + all
  128 colors, one per line). A shared quantizer misreads Stella's slightly-different RGB as
  ±1-luma errors (86.5%); with the measured palette: **100.00% agreement on litmus_pf
  (34,240 cells)**. `scripts/stella_oracle.sh <rom> <frames> pixels` runs it hands-free.

## [1.53.0] - 2026-06-12

### Added
- **Verification sweep — four documented-but-unverified facts closed** (`docs/fundamentals-audit.md`
  updated to ✅, each with a litmus + scenario):
  - `litmus_hmxx_freeze`: on Gopher2600, **HMxx is latched at the HMOVE strobe** — post-HMOVE
    rewrites (+6/+15/+33 cy) never alter in-flight movement. The 24-cycle rule stays as a
    real-hardware portability constraint.
  - `litmus_score_pfp`: **PFP dominates SCORE** — CTRLPF $06 renders identically to $04
    (PF in COLUPF on both halves, priority over players); SCORE coloring only without PFP.
  - `litmus_vdel_2lk`: the 2LK alignment relation pixel-exact — **VDELP0=1 shifts P0 +1 line**
    to align with odd-line-written P1 (read_row 137→138).
  - Shear-safe write window (cycles 0–22) closed by derivation from verified beam constants +
    litmus_48px6's measured mid-line choreography.

## [1.52.0] - 2026-06-12

### Added
- **`read_audio` note names** (A-1 closed): each channel now reports `note`/`cents` via
  `pkg/audio.NearestNote` — audio state is discussable by name ("ch0 is C5 +0.2¢"), not just
  raw AUDC/AUDF. Verified against the sound-driver ROM (C5/C4 exactly as composed).
- **Sprite shape in the annotated screenshot** (S-4 closed): `get_screen_annotated` draws the
  *current GRP bit pattern* (REFP-reflected, NUSIZ-width-scaled) at each player's marker
  position — mid-frame stops show exactly what byte the TIA is holding, cross-checked against
  `read_tia_registers.gfx_new` ($CC ⇒ the visible 2-2-2 pattern).

## [1.51.0] - 2026-06-12

### Added
- **Source-line debugging** (`internal/srcmap`, U-M9): `assemble_and_load` now assembles with
  DASM `-l`/`-s` and builds a PC → (nearest label + offset, source file:line) map. Tool outputs
  gain an `at` field: `assert_line_budget` (the overrunning code's location — e.g.
  `Burn+5 (litmus_overrun.asm:66)`), `trace_clocks` (every instruction), `watch_ram` (the
  writing instruction), `read_cpu` (current PC). `.bin` direct loads are unaffected (no map).
  Unit-tested parser + end-to-end coverage in `scripts/mcp_smoke.py` (overrun must report its
  source line). Flat 2K/4K only (banked ROMs return no `at`).

## [1.50.0] - 2026-06-12

### Added
- **Technique: bank-switched game structure** (`roms/techniques/banked_game.asm` +
  `docs/techniques/bankswitching.md`): the F8 template — per-bank reset stubs/vectors, a
  reusable `jsr $FF80` cross-bank trampoline, and the data-bank pattern (bank-1 loader copies
  level tables into zero page; bank-0 kernel renders from RAM). CI: `scenarios/banked_game.json`
  (load contents byte-exact, level switch, bank.number==0 at frame boundaries, golden).
  Recorded trap: **instruction fetch on $FFF8/$FFF9 switches banks** — placing the trampoline's
  `rts` on a hotspot caused a reboot loop (350-line frames); diagnosed via `watch_ram` writer PCs.

## [1.49.0] - 2026-06-12

### Added
- **Technique: procedural generation** (`roms/techniques/procgen_demo.asm` +
  `docs/techniques/procedural.md`): event-driven Galois LFSR (the litmus_lfsr form) mapped to
  spawn positions by mask+offset, with the sequence cross-checked against an off-target
  reference implementation. CI: `scenarios/procgen_demo.json` — four spawns assert RAM state
  AND rendered X exactly ($5A → $2D,$98,$4C,$26 / X 61,40,92,54), golden. Same seed = same world.

## [1.48.0] - 2026-06-12

### Added
- **Technique: paddle input** (`roms/techniques/paddle_demo.asm` + `docs/techniques/paddle.md`):
  the dump/charge/per-line-count kernel (VBLANK=$82 discharge → release at visible start →
  count lines until INPT0 D7) with the value mapped to a PosObject-placed bar. CI:
  `scenarios/paddle_demo.json` — paddle 0.1/0.25/0.5 measure exactly 0/63/170 lines (litmus
  transfer curve, shifted by the dump-release line) and the bar X follows (clamped), golden.

## [1.47.0] - 2026-06-12

### Added
- **Technique: missiles as bullets** (`roms/techniques/bullets.asm` +
  `docs/techniques/missiles-bullets.md`): RESMP spawn-at-player, sentinel-encoded row-range
  flight (kernel stays under the line budget on the active path), CXM0P hit handling.
  CI: `scenarios/bullets.json` (spawn at ship+4, flight, latch, hit bookkeeping, golden).
- **`litmus_resmp` — RESMP verified**: unlock places the missile at **player+4px** (1x center),
  follows HMOVE moves, and the lock must be **held ≥1 frame** (same-pass lock+unlock does not
  move the missile). Plus three recorded traps: collision *read* addresses decode the low nibble
  ($32 reads CXP0FB, not CXM0P=$30); PosObject fine adjust is `eor #7` (not `eor #$FF`); active-
  path-only line-budget overruns show up as frame-length changes (350-line frames).

## [1.46.0] - 2026-06-12

### Added
- **Technique: game state machine** (`roms/techniques/game_states.asm` +
  `docs/techniques/game-states.md`): title/play/game-over skeleton with edge-detected console
  switches, SELECT variants, difficulty-dependent round timing, attract mode, deterministic
  state entry, frame logic under TIM64T. CI: full-lifecycle scenario (~1100 frames, golden).
  Dogfooded: `fieldtest -auto` detects this ROM's title via `auto-start: reset`.
- **`litmus_swchb` — SWCHB read side verified** (D0/D1 active-low, D3 color, D6/D7 difficulty):
  `emu.SetPanel` extended with `color`/`p0pro`/`p1pro`, and scenario `inputs[]` now accepts panel
  actions (`reset`/`select`/`color`/`p0pro`/`p1pro`). `docs/fundamentals-audit.md` input section
  updated to verified.

## [1.45.0] - 2026-06-12

### Added
- **Technique: in-game sound driver** (`roms/techniques/sound_driver.asm` +
  `docs/techniques/sound-driver.md`): looping 2-voice music from jingle-compatible tables with
  **SFX preemption of channel 1 and automatic restore**; driver tick runs in overscan under
  TIM64T (constant calibrated by scenario line-count sweep). Verified by `dissect -audio`
  round-trip (transcription == composition on both voices) and frame-exact preemption/restore
  asserts. CI: `scenarios/sound_driver.json` (+ audio golden).

## [1.44.0] - 2026-06-12

### Added
- **Technique: sound effects** (`roms/techniques/sfx_demo.asm` + `docs/techniques/sound-effects.md`):
  SFX as frame tables (2 bytes/frame) generated by new `pkg/audio` helpers `PitchSweep` /
  `NoiseBurst` / `Blip` / `Arpeggio` / `EmitSFX` (unit-tested). Five standard recipes (laser,
  explosion, pickup, bounce, engine) + a ~40-cycle overscan player. CI: `scenarios/sfx_demo.json`
  — 14 register-exact asserts across all five effects (all passed first run) + audio-digest golden.

## [1.43.0] - 2026-06-12

### Added
- **Technique: 6-digit score kernel** (`roms/techniques/score6.asm` + `docs/techniques/score-kernel.md`):
  BCD 3-byte score + per-frame font-pointer build + the litmus_48px6 VDEL 6-store choreography with
  `(zp),y` fetches (stores at 55/58/61/64 cy → whole block repositioned +63px to P0=87/P1=95; gap
  relations preserved). `pkg/sprite.DigitFont()` for Go-side reuse. CI: `scenarios/score6.json`
  (positions, BCD carry at frames 99/150, 262 lines, golden).

## [1.42.0] - 2026-06-12

### Added
- **Music transcription** (`cmd/dissect -audio N`): samples TIA audio registers (AUDC/AUDF/AUDV,
  both channels) at frame granularity from reset and emits each channel as jingle notation
  ("D6:80 F6:40 R:6 ..."), with per-note AUDF/cents. New `pkg/audio.NearestNote` (12-TET inverse
  of `FindNote`, unit-tested). **Round-trip verified**: transcribing our own single- and two-voice
  fanfare ROMs reproduces the input melodies note-for-note on both channels (repeated equal
  pitches merge legato — register-identical, acoustically the same). Demo: a commercial title's
  theme transcribed with names + frame durations (output kept in inbox per clean-room policy).

## [1.41.0] - 2026-06-12

### Added
- **`cmd/dissect` bank-aware matching (F8/F6/F4)**: for carts >4K, matches are reported as
  "bank N $Fxxx-$Fxxx" (bank-relative in the $F000-$FFFF window) instead of a wrong flat address.
  Ground-truth verified with a purpose-built F8 ROM (Art table planted in bank 1 at $F200 →
  reported exactly as "bank 1 $F200-$F207"); field-checked on a commercial 8K title (asset tables
  resolved per bank, computed wireframe data correctly left unmatched). DiStella annotation is
  skipped with a note for banked carts (DiStella v2.10 supports 2K/4K only).

## [1.40.0] - 2026-06-12

### Changed
- **All generated output is now English**: ingest text reports (`internal/ingest/textreport.go`),
  fieldtest/dissect/stellacheck CLI messages, and jingle-generated ASM comments. Go source comments
  stay as-is (repo convention); only user-visible output strings changed. Existing inbox artifacts
  were regenerated/rewritten in English (reports, summaries, READMEs).

## [1.39.0] - 2026-06-12

### Added
- **`cmd/jingle` two-voice support** (`-notes2`/`-vol2`/`-type2`): both TIA channels driven
  independently (AUDC1/AUDF1/AUDV1, per-voice auto-picked sound type, automatic rest padding for
  loop sync). Verified numerically via `read_audio`: both channels sound the expected harmony
  pair (e.g. F6/A5) at the expected frames. Generated-ASM comments and CLI output are English.

## [1.38.0] - 2026-06-12

### Added
- **`cmd/dissect` — runtime trace × ROM byte matching** (disassembly-driven asset extraction; the
  preferred path when the ROM exists, superseding pixel analysis): instruction-steps N frames recording
  every TIA graphics-register store (GRP/PF/COLU) with PC + scanline, groups them into streams, and
  locates each table's **ROM address** (trying trimmed-blank / run-length-collapsed / reversed variants),
  rendering sprites as ASCII art. Constant streams are reported as immediates (false-positive guard).
  `-distella` merges `; dissect:` annotations into a DiStella disassembly at the nearest preceding label.
  Validated on ground truth (vertical_pos art table found at its exact address) and on a commercial
  title (player sprite incl. reversed storage + per-row color table + PF table; output kept local per
  the clean-room policy). Research notes + future ideas: `docs/improvement-roadmap.md`.
- `internal/emu`: CPU register accessors `PC`/`A`/`XReg`/`YReg` and `PeekROM` (memory peek without
  side effects) to support instruction-level tracing.

## [1.37.0] - 2026-06-12

### Added
- **fieldtest v2**: console panel switches (`emu.SetPanel` reset/select; `-press reset@30`),
  **auto-start escalation** (`-auto`: capture → if no dynamic objects, RESET → fire →
  fire+hold-right, reporting which attempt started the game — verified live: E.T. needed RESET,
  Outlaw needed fire+hold-right), and **inbox organize mode** (`-inbox dir`: each X.bin moves
  into X/ with overlay/report.txt/report.json inside — the standing structure, documented in
  inbox/README.txt). Batch-ran 9 ROMs end-to-end.

## [1.36.0] - 2026-06-12

### Added
- Recovery-run wrap-up: routing table entries (ram-maps, dynamic-multisprite), mcp_smoke now
  exercises all five new tools end-to-end, serverInfo version bump added to the release
  checklist in CLAUDE.md, open-backlog ledger CLEARED (remaining items are single user
  actions, each fully prepared). Summary at inbox/recovery_report.txt.

## [1.35.0] - 2026-06-12

### Added
- **Composing-session groundwork**: `pkg/audio.NoteFreq/FindNote` (12-TET note names →
  best (AUDC,AUDF) with cents error, Slocum tuning) and **`cmd/jingle`** — melody notation
  (`"C5:30 E5:30 G5:30 C6:60 R:30"`) → a playable looping ROM in one command (auto-picks the
  sound type that fits the whole melody within ±60 cents; assembles via dasm when present;
  per-note cents annotated in the generated source). Verified: register sequence
  AUDF 29→23→19→14 matches the documented C6 spot value; 262 lines held. The joint session is
  now "hum it → ROM in 30 seconds → listen together in Stella".

## [1.34.0] - 2026-06-12

### Added
- **`cmd/fieldtest` — ROM self-driving field tests (input contract v3).** Given a ROM file, the
  harness runs it in Gopher2600, captures K frames (with optional input injection
  `-press right@60,fire@90`), and emits the full multi-frame analysis (overlay/report.txt/json).
  Screenshots are no longer required when a ROM exists — F12 becomes the fallback. Verified
  end-to-end on dyn_multisprite (4 frames, fidelity ~100%).

## [1.33.0] - 2026-06-12

### Added
- **`scripts/stella_oracle.sh` — the Stella cross-check, hands-free.** Launches stellacheck and
  sends the debugger key to Stella via AppleScript in parallel; preflights the one-time
  Accessibility permission and prints setup instructions when missing (the manual-keypress flow
  remains as fallback). The last human step in the oracle loop is now a single one-time
  permission grant.

## [1.32.0] - 2026-06-12

### Added
- **MCP `trace_clocks`** — sub-instruction beam anatomy: each of the next N instructions with
  PC, opcode, CPU cycles, and start/end (scanline, color clock). The practical recovery of the
  parked step_clock (observation without suspension). **First catch:** the mid-line HMOVE
  table's strobe clocks were hand-estimates (≈1/73/130); trace_clocks measured 13/85/142 —
  fundamentals-audit corrected. Rule 2 extended to clocks.

## [1.31.0] - 2026-06-12

### Added
- **Ingest R3 — mid-scanline COLUPF as a first-class citizen.** Bands whose lit columns change
  color mid-half now carry `color_writes` ([{clock,color}] — faithful timed-write register
  semantics, exactly how you'd author it), the renderer replays them, and the text report prints
  them as `; COLUPF timed write: clock N -> $XX`. The previously "documented limit" is now
  modeled: **Pitfall's static layer 98.56% → 99.90%** (8 bands gained writes). Synthetic CI
  proof: a two-color half extracts write@clock48 with fidelity 100%. Half-boundary-only changes
  (score mode) still use ColorLeft/Right — no churn for existing data. inbox reports regenerated.

## [1.30.0] - 2026-06-12

### Changed
- **dyn_multisprite polish**: all five objects now have distinct X (DelTbl 1..5 — enabled by a
  −2-cycle state-flag dispatch: draw state = $80 so one `bmi` replaces cmp/beq); the documented
  position mapping now matches measurement (X = 33+15d on slot A, 36+15d on slot B; the 3px
  slot difference is the A/B dispatch asymmetry, now documented); scenario asserts strengthened
  with deterministic ys at two fixed frames; goldens regenerated.

## [1.29.0] - 2026-06-12

### Fixed
- **Exerciser scene-entry line transients eradicated** (debt since v1.2.0): title entry 263
  (music init moved into the half-empty HMCLR line), zone entry 264 (the 6-element X-table
  copies ran ~82 cycles — split 3+3 across the init's six lines), gradient entry 263 + a 263
  every 4th frame (the kick envelope's every-4th-frame branch jitter — now a branchless
  per-frame `AUDV0 = sfxTmr>>2` with identical envelope, and the entry-frame kick register
  writes moved past the first WSYNC, flagged by sfxTmr==40). **All five scenes now hold 262
  on every frame including entry** (full per-scene map probed). Goldens regenerated.

## [1.28.0] - 2026-06-12

### Added
- **V2-18 RAM-map audit** — `docs/ram-maps.md`, auto-extracted zero-page equates per ROM.
- CLAUDE.md tool list updated (analyze_screen / run_scenario / watch_ram; parked items noted);
  MCP serverInfo now tracks releases (1.28.0). Open-backlog ledger updated with v1.19–v1.28
  results. Overnight summary at `inbox/overnight_report.txt`.

## [1.27.0] - 2026-06-12

### Decided
- **Ingest M-I — static-layer residual diagnosed and documented, not papered over.** Pitfall's
  98.6% static reconstruction loses its 1.4% in canopy-fringe rows where two colors share one
  playfield half on one scanline = the game writes COLUPF mid-scanline. The band model keeps
  one color per half on purpose (per-column colors would misrepresent register semantics);
  the documented guidance is to author such rows as timed-write kernels. Diff-row histogram
  methodology recorded in docs/ingest.md.

## [1.26.0] - 2026-06-12

### Added
- **Ingest M-H — position-continuity union tracks + animated-PF hints.** The union links
  sprites across frames by proximity (≤20px; Pitfall's Harry runs up to 18px/frame) and shared
  colors — an animating mover is now ONE track with a `poses` count (Harry: 1 track, 4 poses,
  not flicker). `flicker` is redefined to "blinking in place across skipped frames" only
  (the four flicker balls keep their flag; vanished/appeared tracks count as gaps). Fully
  grid-aligned dynamic cells get an `animated_pf?` hint — the Exerciser's scrolling starfield
  is CI ground truth (mountains stay static reflect bands).

## [1.25.0] - 2026-06-12

### Added
- **Technique #10b — dynamic multi-sprite kernel, the full form** (`dynamic-multisprite.md`,
  demo `dyn_multisprite.asm`; suite now 50). 5 crossing objects through 2 players: 9-comparator
  sorting network (deterministic cycles), dynamic 2-of-N slot queues with 0-sentinels and
  per-frame fairness flip, mid-screen timed-RESP repositioning on the coarse grid, and a
  **TIM64T-managed VBLANK** (sort+assign vary 60–160 cycles by path — un-paddable; the
  real-game idiom now verified here). Zero visible budget spills over 10 frames by
  instruction-level interval enumeration; all 5 object colors proven rendered via multi-frame
  ingest. War stories recorded: a POSITION path at exactly 76 cycles (the closing WSYNC itself
  crossed) fixed by a fall-through reorder worth −3 cycles.

## [1.24.0] - 2026-06-12

### Added
- **VDEL odd/even verified** — `two_line_vdel.asm`: in a 2-line kernel (GRP0 on line A, GRP1 on
  line B), setting `VDELP0 = y&1` parks the GRP0 write in the shadow register until the GRP1
  write — the sprite starts on odd scanlines with the kernel unmodified. CI pixel proof: top
  edge moves exactly +1 scanline per frame (TestVDELOddEven). Suite now 49.

## [1.23.0] - 2026-06-12

### Added
- **skipDraw (DCP) verified** — `vertical_pos_dcp.asm`: the classic undocumented-opcode vertical
  trigger (`lda #H-1 / DCP sprDraw / bcs`), encoded via `.byte $C7` (DASM has no illegal
  mnemonics). Measured against the compare version on the same kernel: max line 40→38 cycles,
  sprite line 31→30 — modest here; the idiom's real value is freeing Y. Pixel-identical motion,
  CI-locked. Suite now 48.

## [1.22.0] - 2026-06-12

### Added
- **litmus_hmove_mid — mid-line HMOVE measured** (documented→verified). With HM registers
  cleared, strobes completing at visible clocks ≈1 and ≈73 shift nothing; ≈130 shifts **−5 px
  left**; no-strobe control 0. Pixel-confirmed (bar edge above/below the strobe line). The folk
  "right 1px/4CLK" summary did not reproduce — recorded as a non-monotonic function of strobe
  time in docs/fundamentals-audit.md; pinned in scenarios/hmove_mid.json. Suite now 47.

## [1.21.0] - 2026-06-12

### Added
- **litmus_bank_f6 / litmus_bank_f4 — F6 (16K/4-bank) and F4 (32K/8-bank) bankswitching
  hardware-verified** (generalizing the proven F8 pattern: vectors + identical reset stub in
  every bank, a byte-exact switch-zone chain at $FF00 visiting bank0→1→…→N→0 each frame).
  Each bank stamps its ID and counter; scenarios assert the last bank's mark, equal counters,
  and bank.number==0 at the frame boundary. Suite now 46. The F4 chain (~130 cycles) spills one
  overscan line — compensated explicitly (ldx #29) to keep 262.
- CLAUDE.md: bank constants note updated (F8/F6/F4 all verified).

## [1.20.0] - 2026-06-12

### Added
- **MCP `watch_ram`** — run until RAM[addr] changes; returns old/new value and the PC of the
  writing instruction (bounded by max_frames). Granularity is per-instruction; same-value
  stores are invisible (documented).

### Decided
- **step_clock parked with findings** (docs/mcp-tools.md): Gopher2600's colorClockCallback can
  observe but not suspend mid-instruction; a color-clock quantum needs an upstream CPU
  micro-instruction refactor. RunUntilBeam/read_cycles/assert_line_budget/watch_ram cover the
  practical cases.

## [1.19.0] - 2026-06-12

### Added
- **MCP `run_scenario`** — the regression runner's verdict callable from the live loop
  (paths[], returns pass/fail with failing-assertion details).
- **MCP `analyze_screen`** — the ingest analyzer applied to the *current emulator frame*
  (no file round-trip): PF bytes, sprite GRP + per-row colors, groups, fidelity, grid overlay.
  Supersedes the long-parked read_sprite_shape idea.
- `scripts/mcp_smoke.py` — sequential MCP smoke driver (the go-sdk serves tool calls
  concurrently; piping a batch races load_rom vs later calls — cost one debugging round).

## [1.18.0] - 2026-06-12

### Added
- **`report.txt` — the human-readable report is now an official tool output** (the author asked
  why the nice ASCII format was one-off). `cmd/ingest` writes it next to `report.json`/
  `overlay.png`: sprite ASCII art with per-row TIA color codes (duplicate rows compressed xN,
  NUSIZ stretch expanded), group list, playfield band table with 40-column previews and
  repaired/SCORE flags, and the DASM snippets. Multi-frame runs get the layered version
  (per-frame dynamic sprites + union + static layer).

## [1.17.0] - 2026-06-12

### Added
- **Image ingestion M9 — multi-frame everywhere.** `cmd/ingest -in a.png,b.png,c.png` and
  `analyze_image {paths: [...]}` run the M8 separation end-to-end; static objects carry
  interpretation hints (`pf_fringe?` when the color matches an adjacent PF band,
  `parked_object?` otherwise); input contract v2 documented (2-3 consecutive F12 shots for
  scenes with movement; N=3 recommended). MultiReport uses a named `static` field (Go embedded
  structs and the MCP schema generator don't mix — second schema gotcha after []uint8).

## [1.16.0] - 2026-06-12

### Added
- **Image ingestion M8 — multi-frame separation** (the author's architectural point: M7's
  reference-pattern repair doesn't generalize; this does). Feed N screenshots of the same scene:
  per-pixel voting builds the **static layer** (playfield/background/parked objects — leaf
  fringes, pit holes, ladders land here correctly as `static_*`), per-frame diffs give the
  **dynamic layer** (real sprites). No repeating-structure assumption. Bonus: **union across
  frames with flicker detection** — 30 Hz multiplexed objects read completely from 2 shots.
  N=2 ties fill from row background (recorded in `unresolved_share`); N=3 recommended.
- CI proofs from our own ROMs: flicker_multiplex 2 frames → all 4 balls in the union, each
  flagged flicker, per-frame fidelity 100%; sprite_anim → walker tracked moving +1px/frame, not
  misflagged; pf_modes static scene → bands identical to single-frame analysis, dynamic layer
  empty, unresolved 0.

## [1.15.0] - 2026-06-12

### Added
- **Image ingestion M7 — overlap repair (sprite-guided PF inpainting).** Where sprites cross
  playfield, ownership is locally undecidable; a clean reference band (the same structure
  repeating elsewhere) resolves it both ways: sprite pixels absorbed into PF return to the
  sprite's art, PF bits hidden under the sprite restore from the reference. Conservative: no
  reference → no touch. Synthetic CI proof: a frame sprite over a 3-cycle building pattern
  extracts bit-perfect with all bands repaired and fidelity 1.0.

### Fixed
- Context demotion (M6) demoted whole thin bands, dragging clean columns into the sprite layer
  (caught by the synthetic overlap test) — now **per-column** with per-column color matching.

### Result
- Pizza Boy: **fidelity 100.0%** (from 99.93%), zero contaminated/asymmetric bands; the pizza
  slice's body rows and the courier's belt row recovered exactly (author's two remaining
  complaints). All sprite/PF colors were already real TIA codes (COLUxx values) per row/band.

## [1.14.1] - 2026-06-12

### Fixed
- **annotate grid drew pink artifacts over bright backgrounds** — a latent bug since v0.5.0:
  the semi-transparent grid colors were invalid premultiplied `color.RGBA` values
  (channels > alpha, e.g. {255,255,255,30}); harmless over black (most 2600 screens) but the
  compositor produced pink streaks over bright areas (visible on Pizza Boy's cyan buildings;
  in hindsight also faint on the zone scene). Grid lines now use non-premultiplied
  `dc.SetRGBA`. Affects `get_screen_annotated` and all ingest overlays.

## [1.14.0] - 2026-06-12

### Added
- **Image ingestion M6 — context arbitration, stretch decomposition, grouping.** Thin "playfield"
  rows vertically touching same-colored sprite pixels are sprite strokes (the score digits'
  top/bottom bars) — they demote and the rings reassemble whole (synthetic 3-ring CI test).
  Components 9-16/17-32 px wide try NUSIZ 2x/4x hypotheses (≥90% row conformance) before
  empty-column splitting and 8px-window composites — everything gets GRP data now. Row-groups
  bundle score/gauge runs; identical shapes share an id. Overlay draws numbered bounding boxes.
- **Pizza Boy acceptance (author's checklist): all six criteria met.** Courier = one complete
  sprite (detached hand re-merged; 10 art rows × 2-line kernel), life gauge = one 3-copy entry,
  pizza = standalone sprite, **both cabs = player_2x with identical shape id (GRP'd)**, score =
  one row-group of complete digits, **fidelity 99.93%** (own-ROM suites stay at 100%).

## [1.13.0] - 2026-06-12

### Added
- **Image ingestion M5 — fidelity metric + fragment merging** (author feedback: "if the accuracy
  is too low to use, it's pointless" — so accuracy became a number first).
  - **Reconstruction fidelity**: the report (per-row background + PF bands + sprites) renders
    back to a 160×H plane and is pixel-compared with the normalized input; `fidelity` is in
    every report. CI asserts **100% on our own ROMs** (an extractor that can't reconstruct its
    own renderer's output is buggy); pf_modes allows 0.999 (sprite-over-PF assumption vs the
    priority region).
  - **Fragment merging**: connected components within a 2px gap sharing colors fuse before
    classification (the courier's detached hand, the cab's wheel, multi-part icons). Pizza Boy:
    16 components → 6 objects, fidelity **99.25%** (the remainder is exactly the still-GRP-less
    large objects = M6's job).

## [1.12.0] - 2026-06-12

### Added
- **Image ingestion M4 — `analyze_image` MCP tool.** The full pipeline (normalize → quantize →
  playfield bands → sprite candidates → DASM snippets) callable live; returns the structured
  report plus the TIA-grid overlay inline and at `$ATARI2600_INGEST_PATH`. Found and fixed a
  go-sdk structured-output gotcha: `[]uint8` marshals as base64 (Go `[]byte`) and fails the
  generated array schema — byte sequences in tool outputs are `[]int` now.
- docs/ingest.md (+ja) extended with the extraction layers and MCP usage; README section;
  CLAUDE.md routing + tool list. MCP serverInfo.version now 1.12.0.
- Field test: Pizza Boy F12 shot → 29 playfield bands + 16 sprite candidates end-to-end through
  the MCP tool (full report delivered to the author separately).

## [1.11.0] - 2026-06-12

### Added
- **Image ingestion M3 — sprite extraction.** 8-connected components over the residual layer
  (non-background, non-playfield) classified as player (width ≤8: GRP bytes in pkg/sprite bit
  order + per-row color table), missile/ball (≤4 solid), or large_object (low confidence);
  equal-shape groups at 16/32/64 spacing fold into one NUSIZ entry. DASM GRP tables emitted.
- **PF↔sprite reconciliation:** a grid-aligned sprite (the bouncing ball at x=80) was claimed by
  the playfield layer and fragmented — tiny PF bands (height ≤2, lit columns ≤2) now demote back
  to the sprite layer. Genuine 1-line playfield (starfields) survives via column count.
- Round-trip CI proofs: ball GRP == Art bit-for-bit with canonical colors; walker GRP matches
  phase art through the row-quadrupled kernel (32 rows); litmus_nusiz_copies folds to one
  3-copy/16-spacing entry.

## [1.10.0] - 2026-06-12

### Added
- **Image ingestion M2 — playfield extraction.** Per-row background estimation (global mode color
  with per-row fallback for COLUBK gradients — naive per-row mode inverted figure/ground on rows
  more than half-filled, caught by the mountain round-trip), 4-clock-aligned column folding,
  repeat/reflect/asymmetric half classification, score-mode flagging (same pattern, two colors),
  band compression, and DASM `byte` table emission reusing `pkg/playfield`'s verified bit order.
- Round-trip CI proofs: litmus_pf bands == $10/$80/$01 exactly; pf_modes score band ($66,
  $44-left/$86-right) and wall band ($10) found; Exerciser mountain bands match the live RAM
  band triples (PF0 masked to its displayed upper nibble) with reflect detected.
- Palette canonicalization: codes with identical RGB (e.g. $0C≡$0E here) report as the lowest
  code (`Quantizer.Canonical`).
- CI now assembles roms/techniques + roms/exerciser before `go test` (ingest tests use them as
  ground truth).
- Field result: Pizza Boy buildings extract as repeat-mode PF bands (blue $9E) with concrete
  PF0/PF1/PF2 bytes per 4-line band.

## [1.9.0] - 2026-06-12

### Added
- **Image ingestion M1 — screenshot → TIA raster** (`internal/ingest`, `cmd/ingest`,
  `docs/ingest.md`). The reverse pipeline begins: integer-scale auto-detection (any multiple of
  the 160-clock raster — decided with the author; 320×228 Stella F12 → 2×1), cell-majority
  normalization, palette quantization against the same Gopher2600 `Spec.GetColor` table the
  harness renders with (distance reported; Stella inputs show the expected small constant),
  TIA-coordinate grid overlay reusing `internal/annotate`. Round-trip CI tests: an emulator
  Snapshot upscaled 2×1/2×2 normalizes back **pixel-identical** with distance 0.
- **Image input contract** (docs/ingest.md + CLAUDE.md): grade A = Stella F12 PNG, unmodified,
  TV effects off (integer scale guaranteed, Retina-proof); OS screenshots = conversation grade,
  processed with warnings; hand-off point = umbrella `inbox/` (belongs to no repo).
- Real-image smoke test: Pizza Boy F12 shot → scale 2×1 detected, full color inventory
  (bg $00 79%, buildings $9E, score $FE, courier $CE, …).

## [1.8.0] - 2026-06-12

### Added
- **Technique #12 — Venetian Blinds** (`docs/techniques/venetian-blinds.md`, demo
  `roms/techniques/venetian.asm`, CI-locked; suite now 44). Intra-frame line interleaving: a white
  diamond and a red frame coexist in one 64-line zone through P0 alone — even lines draw A, odd
  lines B, shape *and* color swapped per line before the display window. Zero flicker (60 Hz
  stable), striped look — the Video Chess (Whitehead, 1979) technique. Adjacent rows pixel-verified
  (`[83+2 white]` ↔ `[80+8 red]`).

### Milestone
- **Techniques roadmap complete: 12 of 12 verified.** #1 zones, #2 animation, #3 vertical
  positioning, #4 2-line kernel, #5 48px+score, #6 sound driver, #7 LFSR, #8 PF modes,
  #9 ball+missiles, #10 flicker multiplexing, #11 F8 bank switching, #12 Venetian Blinds —
  each with a CI-locked demo or verified inside the Exerciser. Documented refinements (VDEL
  odd/even, dynamic Y-sort allocation, DCP skipDraw, F6+) remain on call for real games.

## [1.7.0] - 2026-06-12

### Added
- **Technique #10 — flicker multiplexing** (`docs/techniques/flicker-multiplexing.md`, demo
  `roms/techniques/flicker_multiplex.asm`, CI-locked; suite now 43). Four color-coded bouncing
  balls share two players by frame-parity subset rotation (30 Hz each) — the Pac-Man-ghost
  technique; overlap-safe since slots use the any-Y compare kernel (#3 ×2, ~49 cy/line) with
  per-subset colors and one shared HMOVE. **The alternation itself is CI-asserted** across three
  consecutive frames. The full dynamic form (Y-sort + 2-of-N allocation + fairness rotation)
  is documented for when a game needs it.

## [1.6.0] - 2026-06-12

### Added
- **Technique #8 completed — playfield score mode & priority** (`docs/techniques/pf-modes.md`,
  demo `roms/techniques/pf_modes.asm`, CI-locked; suite now 42). Three regions switch CTRLPF
  mid-frame; pixel-verified by read_row: in score mode the same PF1=$66 pattern reads back
  COLUP0-red on the left half and COLUP1-blue on the right; with priority off the red P0 column
  fully covers the yellow wall, with D2 set the wall splits the sprite (62+2/64+4/68+2).
  Together with the already-verified asymmetric PF and reflect, #8 is done.

## [1.5.0] - 2026-06-12

### Added
- **Technique #4 — 2-line kernel** (`docs/techniques/two-line-kernel.md`, demo
  `roms/techniques/two_line_kernel.asm`, CI-locked; suite now 41). Each art row spans two
  scanlines; line A carries P0's vertical compare + a COLUBK gradient, line B carries P1 +
  loop control — the standard headroom structure of real games. Two players staged then moved
  by **one shared HMOVE** (strobing per positioning line re-applies the earlier HMxx — a +3 px
  bug caught by read_tia and documented). Carry-hygiene note: an `adc` inheriting the sprite
  compare's flags jittered the gradient until it became an `ora`. VDEL odd/even (1-px vertical
  granularity inside a 2LK) left documented-only.

## [1.4.0] - 2026-06-12

### Added
- **Technique #3 — Vertical positioning** (`docs/techniques/vertical-positioning.md`, demo
  `roms/techniques/vertical_pos.asm`, CI-locked; suite now 40). Vertical has no hardware — the
  kernel compares `line − sprY` against the sprite height every scanline and feeds GRP0 art or
  zero (single unsigned `cmp` covers above *and* below via underflow; both paths converge on one
  store at ~21 cy). Demo bounces a ball Y 4⇔180 at X=80; pixel rows verified **bit-for-bit**
  against the art via `read_row`. DCP/skipDraw variant documented for cycle-starved kernels.
  Re-confirmed: **position calibration is kernel-specific** (`lda #imm` vs `lda zp` prologue =
  1 cy = 3 px; this ROM's XCAL is −5 where sprite_anim's is −8) — never copy constants, re-measure.

### Fixed
- **`read_row` y-coordinate was off by `visibleTop` (~29 lines)** from the annotated-grid labels
  the tool promises to match (grid = `visibleTop + image row`; the implementation indexed the
  cropped image directly). Static playfield checks were self-consistent, but grid-coordinate
  round-trips missed. `ReadRow` now subtracts `visibleTop` — the y you see on the grid is the y
  you pass. Found while pixel-verifying this technique's demo.
- MCP server `serverInfo.version` was stuck at "0.9.0"; now tracks releases (1.4.0).

## [1.3.0] - 2026-06-11

### Added
- **Technique #2 — Sprite animation** (`docs/techniques/sprite-animation.md`, demo
  `roms/techniques/sprite_anim.asm`, CI-locked by `scenarios/sprite_anim.json` + golden; suite now 39).
  4-phase walk cycle (frame-divided clock, `frameBase` staged in VBLANK, row-quadrupled kernel),
  ping-pong X with **free REFP0 horizontal flip** (asymmetric art so the flip reads), divide-by-15 +
  HMOVE-table positioner **calibrated to `pos(v) = v` exactly** (`XCAL=-8`, organic full-range sweep).
  Documented measurement subtlety: frame-boundary `hmoved_pixel` reads lag one frame (xpos∓1 by
  direction) — observation artifact, not a positioning error; and **calibrate with organic runs, not
  pokes** (poke timing vs frame-boundary anatomy mis-measured ±2 px twice).

### Changed
- `docs/techniques/roadmap.md` synced with reality: the Exerciser had already verified **#5 48px+score**,
  **#6 sound/music driver**, **#7 LFSR**, **#9 ball+missiles**, **#11 bank switching (F8)** (and parts of
  #8; VDEL prereq of #3 now ✅) — 7 of 12 techniques done. Next open items: #3 vertical positioning,
  #4 2-line kernel, #10 general multi-sprite kernel.

## [1.2.0] - 2026-06-11

### Changed
- **Exerciser Procedural scene redesigned: starfield over mountains** (author feedback: the old
  fixed-mask output "looks like a scrolling barcode" — the one-byte-seed magic wasn't visible).
  - Top 111 lines: sparse starfield — draw = (pair of LFSR steps ANDed) & previous line's pair
    (~6% density, any column), scrolling every frame. The old `and #$88/$11` masks confined stars
    to four fixed columns, which is what read as barcode.
  - Bottom 80 lines: a mirrored mountain ridge generated at scene entry from a one-byte seed by an
    AND-cascade (`band[b] = band[b+1] & (r1|r2)`, 10 bands of 8 lines; harsher `r1&r2` masks for the
    top bands, and the top two bands forced empty — consecutive LFSR steps are correlated, which
    otherwise lets a lucky column survive to the ceiling as a tower). Zero picture bytes in ROM.
  - The scene now owns all 192 scanlines explicitly (1+111+80). The old version only strobed 191
    WSYNCs and silently relied on the dispatch line spilling past 76 cycles for the 262 total; the
    rewrite's lighter pre-section broke that assumption (261 lines) before being caught by the
    line-count probe. Generation is spread across entry-frame lines (≤75 cycles each, one extra
    cycle over budget in an early draft was caught by the per-frame probe and moved to its own line).
- docs/exerciser.md: scene-4 row rewritten accordingly. 38 scenarios pass; goldens regenerated.

## [1.1.0] - 2026-06-11

### Changed
- **Exerciser polish from the author's play-test (three QA reports, all confirmed and fixed).**
  1. *Title logo & score were left of center* — the 48px blocks sat at the verified-recipe default (X=24).
     Now centered (P0=56/P1=64), which required **recalibrating the six-store choreography for the new
     display window** (timed stores 44/47/50/53 instead of 34/37/40/43) and rebalancing the kernel: B0/B1
     loads moved into the head, the tail slimmed to `dec row` + B5 staging, and the exit-line cleanups moved
     after their closing WSYNC (the combined exit line ran 77 cycles and spilled a scanline — caught by the
     line-budget probe).
  2. *Zone sprites never reached the right edge* — the drift wrap was `and #$7F` (0–127), inherited from the
     techniques demo. Now wraps properly at 0–159 (full width), with the drift loop re-split two zones per
     line to stay inside the 76-cycle budget.
  3. *The starfield's "reorganize every 64 frames" read as nothing happening* — one LFSR step per second
     only shifted the pattern a single line. The seed now advances every frame: a continuous upward-scrolling
     starfield. 38 scenarios pass; goldens regenerated.

## [1.0.1] - 2026-06-11

### Fixed
- **Exerciser: fire/scene-advance was dead in Stella — paddle scene removed.** Field report (the author,
  playing in Stella): Space did nothing, though it worked before M5 and every Gopher2600 scenario passes,
  including a real-user input-pattern probe. Root cause: **Stella's controller auto-detection** sees the
  ROM's INPT0 reads (the paddle scene), plugs paddles into the left port — and plugged paddles **hold INPT4
  permanently high**, so the joystick fire can never register (the property is also persisted per-ROM,
  which is why `-lc JOYSTICK` didn't rescue the first binary). Per the author's call, the paddle scene is
  removed from the Exerciser (5 scenes; paddle capability remains verified in `litmus_paddle` and the
  harness paddle input path). 38 scenarios pass.

## [1.0.0] - 2026-06-11

**The harness is 1.0.** The declared bar — a trustworthy loop (gaps A–E), a sourced fundamentals audit with
the unknowns measured, a verified techniques catalog, a two-emulator oracle, and **one artifact composing
every capability** — is met:

- **The Exerciser ROM is complete** (M1–M8, v0.56.0–v0.62.0): an 8K F8 cartridge whose six scenes compose
  the 48px six-store kernel + live BCD score + a 2-channel music driver, zone multiplexing over an
  asymmetric playfield, an interactive collision playground, paddle reading, per-scanline color + SFX, and
  LFSR procedural generation — all driven by input-timeline scenarios, locked by video/audio goldens, and
  green in CI on every push (39 scenarios; every scene provably inside the 76-cycle line budget via its
  262-line assertion).
- **Verification surface**: 26 litmus ROMs; the v2 fundamentals backlog closed (Tier 1–3, incl. VDEL, HMOVE
  side effects, asymmetric-PF windows, inputs incl. paddles, F8 bankswitching + `read_bank`, 6502/BCD
  precision, all 15 collision pairs, RIOT timers, mirrors, LFSR, audio sample capture + `pkg/audio`).
- **Cross-emulator agreement**: `cmd/stellacheck` RAM cross-checks PASS against Stella for `smoke` and the
  `litmus_6502` measurement suite (128/128 bytes each). The Exerciser cross-check additionally showed all
  structural state agreeing, with only per-frame counters phase-shifted by the emulators' differing
  frame-boundary cut points — measured and documented in `docs/stella-oracle.md` (sub-frame alignment = v2).
- **Docs**: routing-tabled deep dives (`fundamentals-audit`, `techniques/`, `exerciser`, `stella-oracle`,
  `verified-coverage`), each fact tagged verified/documented with sources.

## [0.50.0] - 2026-06-11

### Added
- **RESBL vs RESPx mid-line re-strobe litmus (v2 V2-11).** `litmus_resp_edge.asm` confirms Towers'
  TIA_HW_Notes: strobing **RESBL twice on one scanline draws two balls** (clocks 38 and 140 — the ball
  re-emits START, the multi-ball trick), while strobing **RESP0 twice draws a single 8px player** at the
  last position only (clock 107 — the player does not re-emit START until the 160-clock wrap). Locked by
  `scenarios/resp_edge.json` (position asserts + golden). 28 scenarios pass.

## [0.49.0] - 2026-06-11

### Added
- **Address-mirror litmus (v2 V2-12).** `litmus_mirror.asm` proves the memory map's mirroring: writing $5A to
  $0180 reads back at $0080 (and the reverse) — i.e. RAM $80–$FF is mirrored at $0180–$01FF, **which is why
  the stack works**; and setting the background through the TIA mirror $0049 colours COLUBK ($84 blue in
  `read_row`). Locked by `scenarios/mirror.json`. 27 scenarios pass.

## [0.48.0] - 2026-06-11

### Added
- **All 15 collision pairs verified in one ROM (v2 V2-8).** `litmus_collide_all.asm` overlaps P0/P1/M0/M1/BL
  (missiles width-8, ball width-8) with a lit PF0 at the left edge so every CXxx pair fires at once;
  `scenarios/collide_all.json` asserts all 15 (`p0_p1, m0_m1, m0_p0, m0_p1, m1_p0, m1_p1, p0_pf, p0_bl,
  p1_pf, p1_bl, m0_pf, m0_bl, m1_pf, m1_bl, bl_pf`) true — superseding the three single-pair litmus in
  coverage. 26 scenarios pass.

## [0.47.0] - 2026-06-11

### Added
- **RIOT timer litmus — answers the audit's open INTIM question (v2 V2-10).** `litmus_timer.asm` records
  INTIM/TIMINT snapshots to RAM: TIM1T counts down 1/cycle (consecutive reads −7 = the read-loop cost);
  after underflow INTIM wraps into the $FF range and keeps decrementing 1/cycle; **TIMINT D7 (timer-expired)
  is set before INTIM is read ($C0), and reading INTIM clears TIMINT ($00 afterward)** — the audit's open
  "does reading INTIM clear D7?" is now answered **yes**. Locked by `scenarios/timer.json`. 25 scenarios pass.

## [0.46.0] - 2026-06-11

### Added
- **LFSR litmus — procedural-generation foundation (v2 V2-9).** `litmus_lfsr.asm` runs an 8-bit Galois LFSR
  (`lsr / bcc / eor #$8E`, the form in DaveC's Random-Dungeon and common game RNGs) and proves its math
  numerically (pure `read_ram`, no rendering): the first 8 values from seed $01 are
  `01,8E,47,AD,D8,6C,36,1B` (matches hand calculation), it **never decays to $00** across a full sweep, and
  its **period is exactly 255** (returns to the seed). Locked by `scenarios/lfsr.json`. 24 scenarios pass.

## [0.45.0] - 2026-06-11

### Added
- **CTRLPF litmus — SCORE / priority / ball width, incl. the audit's open SCORE×PFP question (v2 V2-7).**
  `litmus_ctrlpf.asm` verifies five regimes: SCORE ($02) paints the left half COLUP0 / right half COLUP1
  (split at clock 80); default priority ($00) draws P0 over the playfield; PFP ($04) draws the playfield over
  P0 (player hidden); **SCORE+PFP ($06) renders the playfield as COLUPF — the SCORE colour substitution is
  *suppressed* under PFP — with the player hidden** (this corner is unspecified in the docs and a likely
  emulator-divergence point; recorded as a Gopher2600 measurement, flagged for the Stella oracle cross-check
  V2-17); ball width D4–5 doubles 1/2/4/8 px. Locked by `scenarios/ctrlpf.json`. 23 scenarios pass.

### Fixed
- **`smoke.asm` now clears collisions after init (CXCLR) — removes platform-dependent CI flakiness.** The
  zero-page clear loop incidentally strobes the TIA strobe registers (RESxx, HMOVE) whose effect depends on
  the power-on TIA state and reset beam timing, leaving sticky collision latches that differed across
  platforms (CI caught `TestReadCollisionsNoSprites` reporting M1-PF / BL-PF on the runner while it passed
  locally). A single CXCLR after init forces a clean, deterministic baseline; rendering (hence all goldens)
  is unchanged.

## [0.44.1] - 2026-06-11

### Changed (docs)
- **README reframed to match the evolved goal.** The project is no longer just "a loop to build games" with
  the five gaps A–E closed (phase 1); it is now a **general, verified 2600 capability base** (phase 2) — a
  fundamentals audit + a techniques catalog, each kept honest by the same numeric loop. Updated the opening
  and the gap-analysis section to name these two living documents and the current scope (20 tools, 20+
  regression scenarios), and to state the aim as *general verified competence, not any one game*.

## [0.44.0] - 2026-06-11

### Added
- **6502/6507 precision litmus — Tier 1 of the v2 backlog complete (V2-6).** `litmus_6502.asm` measures
  instruction facts *on the machine itself* via RIOT TIM1T (1 cycle/tick) and pins them in
  `scenarios/cpu6502.json`, all matching 6502.org exactly: **NMOS BCD** $99+$01 → A=$00 with C=1 correct
  while **Z=0 lies** (the documented NMOS unreliability, recorded); **JMP ($xxFF)** takes the page-bug path;
  **LDA abs,X** 4→5 cycles on page cross while **STA abs,X stays 5 fixed** (why store timing in kernels is
  deterministic); **BNE** 2/3/4 (not taken / taken / taken+cross); illegal **DCP zp = 5 cycles** (also
  certifies illegal-opcode support). 22 scenarios pass.

## [0.43.0] - 2026-06-11

### Added
- **F8 bankswitching verified + `read_bank` MCP tool + `bank.*` scenario fields (v2 backlog V2-5).**
  `litmus_bank.asm` is a best-practices 8K F8 ROM (vectors + an identical reset stub in *both* banks, a
  same-address switch zone whose instruction stream stays valid across the hotspot): every frame bank 0
  marks RAM and hotspot-reads $FFF9 → bank 1 writes its own sentinel and returns via $FFF8. Verified:
  Gopher2600 AUTO fingerprints the plain 8K dasm binary as F8; $80 ends every frame as bank 1's sentinel;
  both per-bank frame counters advance in lockstep; the kernel executes in bank 0 at the frame boundary.
  New `read_bank` MCP tool (20 tools now; `Cartridge.GetBank` at PC, with `is_ram`) and `bank.number` /
  `bank.is_ram` scenario fields; `bin/harness` rebuilt and smoke-tested (initialize + tools/list, no panic).
  Locked by `scenarios/bank.json`. 21 scenarios pass.

## [0.42.0] - 2026-06-11

### Added
- **Input-port litmus with an input-timeline scenario (v2 backlog V2-4).** `litmus_input.asm` samples
  SWCHA/INPT4 to RAM every frame; `scenarios/input.json` drives a press/release timeline and asserts the
  numeric readback: no input = SWCHA $FF, INPT4 $BC (D7=1 + open-bus noise — the documented reason to test
  with N only); P0 left = $BF (D6→0); fire = INPT4 $3C (D7→0); **the VBLANK D6 latch holds INPT4 at $3C
  frames after fire is released** while directions release immediately (the control). 20 scenarios pass.
  Paddle charge-timing verification split off as **V2-4b** (needs a paddle path in `set_input`).

## [0.41.0] - 2026-06-11

### Added
- **Asymmetric-playfield write-window litmus (v2 backlog V2-3).** `litmus_pf_async.asm` verifies woodgrain's
  `Playfield_Timing` tables to the pixel: **(A)** early PF1=$AA (cyc 5) + PF1=$55 at cyc 40 renders a true
  asymmetric playfield — left bits at clocks 16–43, right bits at 100–127, exactly as predicted;
  **(B)** a late write completing at cycle 33 while left PF1 is being drawn splits **per pixel**: the first
  5 bits show the old $FF (clocks 16–35 lit) and the last 3 the new $00 — reproducing woodgrain's worked
  example verbatim. Locked by `scenarios/pf_async.json`. 19 scenarios pass.

## [0.40.0] - 2026-06-11

### Added
- **HMOVE side-effects litmus (v2 backlog V2-2).** `litmus_hmove_side.asm` measures three regimes in one
  frame: **(a)** HMOVE right after WSYNC blanks the left 8px **even with all HMxx=0** (the comb — alternating
  strobe/no-strobe lines compared by `read_row`), confirming Towers' HBLANK+8CLK extension; **(b)** HMOVE
  mid-visible (~cycle 39) produces **zero displacement and no comb** for both HM=0 and HM=$10;
  **(c)** HMOVE at line end (~cycle 74) with HMP0=$10 (+1) moves P0 **left 9px per strobe = value+8**
  (the classic late-HMOVE +8 rule, measured numerically) with no comb. (b)/(c) are recorded as
  Gopher2600-measured values pending the Stella oracle cross-check (V2-17). Locked by
  `scenarios/hmove_side.json` (cumulative-position asserts + golden). 18 scenarios pass.

## [0.39.0] - 2026-06-11

### Added
- **VDEL litmus — verifies vertical delay's write-triggered shadow copies (v2 backlog V2-1).**
  `litmus_vdel.asm` proves all three paths in one frame, exactly as Stella PG §6.D describes:
  with VDELP0=1 a fresh GRP0=$FF stays hidden until **a GRP1 write copies P0's new→old** (then P0 renders
  $FF at X=3); with VDELBL=1 ENABL=on stays hidden until a GRP1 write (ball appears at X=2); with VDELP1=1
  GRP1=$3C stays hidden until **a GRP0 write copies P1's new→old** ($3C renders as 4px at clock 41).
  Locked by `scenarios/vdel.json` (vertical_delay asserts + golden). 17 scenarios pass. This is the
  prerequisite for the 48px score kernel and 2-line-kernel vertical positioning.

## [0.38.0] - 2026-06-11

### Added (docs)
- **Fundamentals audit — `docs/fundamentals-audit.md`.** Six parallel research passes over the local corpus
  (Stella Programmer's Guide, woodgrain wiki, Davie's *Newbies*, SpiceWare's Collect, 8bitworkshop,
  21 real-game disassemblies, DaveC's Random-Dungeon), ~22 owner-supplied links (AtariAge threads, 6502.org,
  Slocum's music guide, Stella debugger docs, Pitfall analyses), and independent web research (Towers'
  *TIA Hardware Notes*, Stolberg). Every domain is classified **verified / documented / unknown / caution**
  with sources. Headline corrections: the local cycle-counting guide's position math is approximate (never
  cite); Pitfall disassembly's LeftRandom comment is wrong (bit0, proven by simulation); SpiceWare Step 3
  vs 7 PF-window discrepancy (to settle by measurement); HMOVE comb/late-HMOVE behavior absent from the
  local shelf (Towers adopted as authority). Headline finds: VDEL's write-triggered cross-copy semantics;
  woodgrain's definitive asymmetric-PF write-window tables; Slocum's complete AUDC/tuning data (the parked
  audio-authoring blocker was already on our shelf); F8-first bankswitching consensus + Gopher2600 already
  auto-fingerprints 8K as F8 and exposes `GetBank()` (a `read_bank` tool candidate); Stella debugger is
  scriptable for automated oracle cross-checks (F-4 design v1).
- **`hardening-roadmap.md` § v2 backlog** — 18 prioritized follow-ups (V2-1…V2-18) in three tiers
  (VDEL, HMOVE side effects, asymmetric-PF windows, input, bankswitch + `read_bank`, 6502 precision; then
  matrix completion; then capabilities: audio sample capture, `pkg/audio`, Stella oracle automation).
- **CLAUDE.md constants hardened**: 24-cycle HMxx freeze after HMOVE; stores never pay page-cross
  penalties; NMOS decimal mode C-only; CLD mandatory at init; cycle-counting-guide caution. Routing tables
  link the audit.

## [0.37.1] - 2026-06-11

### Changed (docs)
- **Roadmap reframed as a general-capability TODO (de-anchored from any single game).** The main goal is a
  general, verified, reusable technique toolkit — not one specific game. `docs/techniques/roadmap.md` now
  prioritizes by **general/foundational value × difficulty × prereqs-verified** (instead of "relevance to a
  particular game"), and is an explicit checklist (`- [ ]`) ordered foundational/easy-wins first
  (animation → vertical positioning/VDEL → 2-line kernel → 48-px score → sound → …). A concrete game can be
  picked flexibly as a per-technique testbed; it is no longer the organizing principle.

## [0.37.0] - 2026-06-11

### Added / Changed (docs)
- **Technique #1 promoted to its formal name + a sourced techniques roadmap.** Researched AtariAge / the
  local `reference/docs_atari/` corpus and Wikipedia: confirmed the formal name is **sprite multiplexing**
  (the loop is a **multi-sprite kernel**); DaveC's "zone" is the common vertical-band term, and our demo is
  the *static-zones* form of the general *sort/position/display + flicker* kernel. Rewrote
  `docs/techniques/zone-multiplexing.md` with a formal-name/taxonomy section, a "Refinements & limits"
  section (2-per-line limit, flicker, single- vs 2-line kernel, positioning cost), See-also (48-px sprite,
  Venetian Blinds), and a sourced References list — marking *documented* vs *verified*. Added
  `docs/techniques/roadmap.md`: a prioritized survey of ~12 next techniques (48-px score, 2-line kernel,
  vertical positioning/VDEL, sound, animation, playfield tricks, LFSR, general flicker kernel, Venetian
  Blinds, bank switching) ranked by North-Star (Frogger) value, difficulty, and prereq-verified status.
  Catalog index links the roadmap. Docs-only; no code change (tests/scenarios unchanged).

## [0.36.1] - 2026-06-11

### Fixed
- **Deterministic emulator power-on state — eliminates CI test flakiness at the root.** Gopher2600
  randomizes the CPU/RAM power-on state (`vcs.Env.Random`, used by `CPU.Reset`), so a fresh `emu.New`
  varied run-to-run; cycle/timing tests (`TestCycleCounterExcludesWsyncStall`, `TestStepScanline`) passed
  locally but flaked in CI. `emu.New` now calls Gopher2600's official `vcs.Env.Normalise()`
  (`Random.ZeroSeed = true` + prefs defaults), the method intended for regression testing, before the
  cartridge-attach reset. Result: identical initial state every run (verified 5×/10× stable). Goldens are
  unaffected (the ROMs clear RAM on boot).

## [0.36.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 gets per-zone background colors — a landscape look.** Each zone sets `COLUBK` from a
  `ZoneBG` table (sky-blue → cyan → green → brown), set in HBLANK so it doesn't disturb the per-zone
  positioning, giving 6 colored bands behind the 12 moving sprites. Golden regenerated; 262 lines preserved.

## [0.35.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 now animates — 12 *moving* sprites.** Each zone's X moved from ROM tables into RAM
  (`zx0`/`zx1`) and is updated every frame (P0 drifts right, P1 left, wrapping `and #$7F`), so all 12 sprites
  animate. Demonstrates RAM-backed motion verifiable purely by `read_ram` (the position bytes change frame to
  frame). VBLANK line count retuned to keep the frame at 262 (the per-frame update loop is absorbed). The
  scenario now locks the frame by `golden_frame` only (robust to the moving positions); all scenarios pass.

## [0.34.0] - 2026-06-11

### Added
- **Techniques catalog + #1 Zone (vertical) sprite multiplexing.** Establishes a repeatable pipeline for
  absorbing 2600 authoring techniques: learn (from `reference/`, local) → clean-room implement
  (`roms/techniques/`) → verify numerically (harness) → cross-check (Stella) → lock in (scenario + golden +
  CI) → optionally promote to `pkg/`. First entry: `roms/techniques/zone_multiplex.asm` puts **12 sprites**
  on screen (6 zones × P0+P1) from a 2-player machine by repositioning P0/P1 per zone (divide-by-15 + HMOVE,
  the harness-verified method). Verified on Gopher2600 + cross-checked in Stella; locked by
  `scenarios/zone_multiplex.json`. CI now runs `roms/techniques/scenarios/` too. Catalog index at
  `docs/techniques/`, linked from the routing tables.

## [0.33.0] - 2026-06-10

### Added
- **Coverage batch: NUSIZ quad-width + missile-player collision.**
  - `litmus_nusiz_quad.asm` (`NUSIZ0=$07`, QuadWidth) → `read_row` shows a **32px** continuous span (8px ×4),
    completing the NUSIZ width modes (double/quad) and copy modes (close/three).
  - `litmus_collide_mp.asm` overlaps an 8px-wide missile0 with player0 → `read_collisions` reports
    **`m0_p0=true`** (CXM0P), extending collision coverage to the missile-player pair. (Also documents the
    1px left-edge offset between missile clamp X=2 and player clamp X=3.)
  - Locked by `scenarios/nusiz_quad.json` and `scenarios/collide_mp.json`. 15 litmus scenarios pass.

## [0.32.0] - 2026-06-10

### Added
- **P0-P1 collision litmus (CXPPMM) — extends collision coverage.** `roms/litmus/litmus_collide_pp.asm`
  overlaps player0 and player1 (both clamped to X=3 via HBLANK strobes) drawing `$FF`; `read_collisions`
  reports **`p0_p1=true`**. Verifies the player-player pair the Frogger `OnPad` check actually uses (previously
  only BL-PF was litmus-verified). Locked by `scenarios/collide_pp.json`. 13 litmus pass.

## [0.31.0] - 2026-06-10

### Added
- **REFP (reflected sprite) litmus — rounds out the sprite track.** `roms/litmus/litmus_refp.asm` draws the
  asymmetric ramp with `REFP0=$08`; `read_tia_registers` shows `player0.reflected=true` and `read_row` shows
  the ramp mirrored (row0 `0x80` lights clock 10 = the right end; row4 `0xF8` lights clock 6–10 = right 5px) —
  the mirror image of the non-reflected `litmus_sprite`. Confirms REFP and `pkg/sprite.Reflect` (data-side
  mirror) are equivalent. Locked by `scenarios/refp.json`. 12 litmus pass.

## [0.30.0] - 2026-06-10

### Added
- **Missile/ball position litmus.** `roms/litmus/litmus_missile.asm` enables and positions missile0 and the
  ball in the visible region; `read_tia` reads **missile0=38 / ball=140** and `read_row` shows a 1px vertical
  line at each clock — verifying the harness reads the missile/ball object-position family (the `X = 3N − 55`
  side, complementing the player `X = 3N − 54` litmus_pos). Locked by `scenarios/missile.json`. 11 litmus pass.

## [0.29.1] - 2026-06-10

### Fixed
- **Flaky `TestStepScanline` (surfaced by CI).** The test asserted every single scanline step consumes
  >0 CPU cycles, but a scanline can legitimately be a pure WSYNC-stall pass-through (0 instructions executed)
  depending on beam-phase alignment — not an invariant. Relaxed to assert the **cumulative** cycles across
  40 scanlines is >0 (the CPU makes progress), which is robust. Keeps the CI badge reliable.

## [0.29.0] - 2026-06-10

### Added
- **NUSIZ multi-copy litmus coverage (extends S-2).** `roms/litmus/litmus_nusiz_copies.asm` renders an 8px
  solid sprite at `NUSIZ0=$03` (ThreeCopiesClose); `read_row` confirms **three 8px white spans at clock
  3/19/35 (16px copy spacing)**. Locked by `scenarios/nusiz_copies.json` (golden + `player0.nusiz=3`).
  Deepens verified coverage of the NUSIZ helper beyond double-width. 10 litmus scenarios now pass.

## [0.28.0] - 2026-06-10

### Added
- **CI via GitHub Actions (hardening-roadmap F-1).** `.github/workflows/ci.yml` runs on every push/PR:
  Ubuntu + Go (from `go.mod`) + DASM, clones Gopher2600 at the pinned commit `5d532e88` into `./Gopher2600`
  (the `replace` target), assembles the litmus ROMs (`.bin` are gitignored), then `CGO_ENABLED=0`
  build/vet/test and runs all litmus regression scenarios. No SDL needed — the harness only imports the
  SDL-free Gopher2600 packages, so a static (cgo-off) build covers it. A CI badge is on the README.
  Verified green on Actions (build/vet/test + 9 scenarios, ~1m).

## [0.27.0] - 2026-06-10

### Added
- **PAL frame verification (hardening-roadmap F-3).** `roms/litmus/litmus_pal.asm` emits a proper PAL frame
  (VSYNC 3 / VBLANK 45 / visible 228 / Overscan 36 = 312 lines) and `scenarios/pal.json` (with
  `tv_spec: "PAL"`) asserts the harness drives/counts it as **312 lines** (plus a RAM sentinel). Confirms the
  harness is not NTSC-only; `ntsc_frame_lines` counts the actual per-frame line total (312 for PAL).

## [0.26.0] - 2026-06-10

### Added
- **Golden-audio regression `checks.golden_audio` (hardening-roadmap A-2).** Mirrors the video golden for
  sound: a sha1 audio-chain (Gopher2600 `digest.Audio`) over the timeline is compared against
  `<scenario>.audio.golden`. `internal/emu` gains `EnableAudioDigest`/`ResetAudioDigest`/`AudioHash`
  (symmetric to the video digest); `internal/scenario`'s golden eval is generalized to share video/audio.
  Verified with `roms/litmus/scenarios/audio.json` on `litmus_audio.asm` (deterministic record→match, plus
  numeric AUDC/AUDF/AUDV asserts). All 8 litmus scenarios pass. CLI only; MCP binary unchanged.

## [0.25.0] - 2026-06-10

### Added
- **`pkg/sprite` NUSIZ helper (hardening-roadmap S-2).** `PlayerSize` (OneCopy … DoubleWidth … QuadWidth) /
  `MissileSize` enums and `NUSIZ(player, missile)` / `NUSIZPlayer(player)` compose a NUSIZx byte from intent
  instead of raw bits. **Verified on Gopher2600** with `roms/litmus/litmus_nusiz.asm`: an 8px solid sprite at
  `NUSIZ0=$05` (DoubleWidth) renders **16px wide** (`read_row` clock 4–19 = white len 16) and
  `read_tia_registers` shows `player0.nusiz=5`. Locked by `scenarios/nusiz.json`. Completes the sprite
  authoring trio (S-1 encoder + S-2 NUSIZ + S-3 P0+P1 combine).

## [0.24.0] - 2026-06-10

### Added
- **`pkg/sprite.SplitWide` + P0+P1 16px combine litmus (hardening-roadmap S-3 — flagship).** Split a
  16-wide ASCII design into P0 (left 8) + P1 (right 8) GRP tables, then place P1 exactly +8px to the right
  of P0 for a seamless up-to-16px (or multicolor) character. `roms/litmus/litmus_p0p1.asm` positions the two
  sprites by strobing RESP0→RESP1 three cycles apart in the visible region (= +9px; an HBLANK strobe would
  clamp both to the left edge) then HMOVE P1 left 1 → exactly +8px.
  **Verified on Gopher2600:** `read_tia` shows player0=69 / player1=77 (exactly +8); `read_row` shows the
  solid-16 rows as a **single continuous 16px white run (clock 69–84, no seam gap/overlap)**, with P0-only /
  P1-only / far-edge rows byte-exact. Locked for regression by `scenarios/p0p1.json` (position asserts 69/77
  + golden frame). This proves sprite placement is as numerically trustworthy as playfield — the headline
  capability of the sprite track.

## [0.23.0] - 2026-06-10

### Added
- **`pkg/sprite` — ASCII → player GRP encoder (hardening-roadmap S-1).** A mirror of `pkg/playfield` for
  player graphics: 8-wide ASCII rows → GRP bytes (`EncodeRow`/`Encode`, D7 = leftmost = standard TIA bit
  order), plus `Reflect` for REFP-less mirroring / P0+P1 right halves. Reuses `playfield.ParseASCIIRow`.
  Unit-tested, including that `..XXXX..` = `0x3C` matches the existing hand-coded Monet Frogger lily-pad byte.
- **`roms/litmus/litmus_sprite.asm` + `scenarios/sprite.json` (+golden) — numeric hardware proof.** An
  asymmetric ramp sprite (top `0x80` 1px → bottom `0xFF` 8px) rendered by player0 at X=3. Verified on
  Gopher2600 via `read_row`: the white span widens 1→2→…→8 px from clock 3 (visible lines 96–103), proving
  D7 = leftmost and top→bottom row order are byte-exact. Locked for regression with a golden-frame scenario;
  all litmus scenarios PASS. First step of the sprite track toward the P0+P1 16px flagship (S-3).

### Added
- **Strengthening roadmap (`docs/hardening-roadmap.md`).** A prioritized roadmap for the next phase —
  making the harness stronger beyond gap-closing. Theme A: deepen authoring + verification into the thin
  domains (S = sprites, incl. `pkg/sprite` ASCII→GRP, NUSIZ helper, and the ★ P0+P1 two-sprite combine for
  up to 16px / multicolor characters placed numerically via the X(N) calibration; A = audio, incl. note/
  timbre names in `read_audio` via Gopher2600 `tracker`, a `digest.Audio` golden, and a `pkg/audio` SFX
  helper). Theme B: harden the foundation (★ CI via GitHub Actions, optional Gopher2600 version pin,
  PAL/SECAM verification, Stella oracle cross-check, completing `step_clock`/`watch|trap`/`run_scenario`).
  Theme C: wire upstream Gopher2600 libraries (`recorder`/`regression`/`reflection`). Each item lists where
  to touch + how to verify + size. Cross-linked from the routing tables in CLAUDE.md / README /
  improvement-roadmap. No code changes (implementation in separate sessions).

## [0.22.1] - 2026-06-10

### Added
- **GPL-3.0 `LICENSE`.** The harness embeds Gopher2600 (GPL-3.0) as a library, so the combined work is
  GPL-3.0-or-later. Added copyright and an Acknowledgements section to the README.

### Changed
- **Public-readiness: the published repo is now English-only.** Translated the public surface
  (README + `docs/`×7 + CHANGELOG + CLAUDE.md) to English. The author works in Japanese, so Japanese copies
  are kept locally as `*.ja.md` sidecars (gitignored, never published). Calibrated the prior-art wording to
  "no Atari 2600 MCP found in a public search (2026-06; Atari Lynx = gearlynx exists)" rather than claiming
  "first". Removed the README provenance section. No code changes; build/vet/test green.

## [0.22.0] - 2026-06-10

### Changed
- **Physical spinoff: split the base into a standalone repo `atari2600-harness` (game ROMs move to a
  separate repo `atari2600-roms`).** Under an umbrella folder `260609_atari2600-dev/`, place `harness/`
  (this repo, history preserved) and `roms/` (new repo) as siblings, bound by `go.work`. Remove
  `roms/frogger` from the harness (moved to the roms repo); `roms/litmus` stays as the harness's own
  verification ROMs. **Eradicate the harness→game dependency:** repoint the scenario/emu unit tests from
  frogger ROMs to litmus, and add a new fixture `roms/litmus/scenarios/golden.json` (+`.golden`).
  `.mcp.json`/`.claude` move up to the umbrella (read at Claude Code's project root). Updated CLAUDE.md's
  structure/dev sections to the post-spinoff reality. Verified: harness `go vet`/`go test` green, 4 litmus
  scenarios PASS; on the roms side `gen` + 3 frogger scenarios PASS.
- **Renamed the Go module `github.com/kidsnz/atari2600-dev` → `github.com/kidsnz/atari2600-harness`
  (spinoff prep).** `go.mod` and 9 import files replaced. build/vet/test green, all scenarios PASS.
- **Promoted `internal/playfield` → `pkg/playfield` (spinoff prep).** Go can't import `internal/` across
  modules, so the playfield encoder (universal Atari 2600 knowledge) became a public package. Updated the
  only cross-package importer (`roms/frogger/gen`). Regenerated all scenes (header-comment-only diffs).
  Verified green; all scenarios (3 frogger + 3 litmus) PASS.
- **Documentation freshness audit (spinoff preamble).** Rewrote `README.md` to v0.21.0 reality (old diagram
  = `cmd/probe` + `internal/emu` only → 4 cmds, 6 internals, roms/<game>, 19 MCP tools, gaps A–E all
  closed; fixed the smoke.asm path to `roms/litmus/`). Fixed minor staleness in `improvement-roadmap`,
  `mcp-tools`, `tool-landscape`, and a stale `cmd/genpf` comment in `roms/frogger/gen/asmgen.go`.

### Added
- **Improvement roadmap document (`docs/improvement-roadmap.md`).** Prioritizes next moves to make authoring
  more accurate, from every angle. Central observation = the position litmus is closed but the timing
  *budget* verification is open (gap B is the biggest hole in the real loop). P0 = cycle exposure +
  per-scanline budget guard, P1 = TIA shadow / collision register reads, P2 = verification automation,
  P3 = build-loop shortening, each annotated with verified Gopher2600 API symbols
  (`CPU.LastResult.Cycles`, `TIA.Video.*`, `Collisions`). Also added "untapped reference veins" (R-1 Freeway
  architecture port, R-2 audio recipes, R-3 cycle-cost table, R-4 real-game structure index) and "external
  research" (the biggest finding: Gopher2600 already implements the hardest items as libraries —
  `recorder`/`regression`/`tracker`/`reflection`/`digest`/`rewind` are usable standalone, shrinking P2/R-2
  from "build" to "wire"; License = GPL-3.0; an Atari 2600 MCP was not found in a public search = no known
  prior art, not a claim of being first; G-2 C64 MCPs, G-3 test DSLs, G-4 authoring-tool integration).

## [0.21.0] - 2026-06-10

### Added / Changed
- **A `.asm` source can be specified directly as a scenario `rom` (gap E fully closed).** If a scenario's
  `rom` is `.asm`, it is assembled with dasm before running = "one source → assemble → run → numeric
  asserts → verdict" in one command (`go run ./cmd/scenario foo.json`). Gap E reaches its ideal form.
- **Consolidated dasm invocation into `internal/build` (DRY).** `assemble_and_load` (harness) and the
  scenario `.asm` feature share `build.Assemble`/`build.BinPathFor`. Assemble failures are returned as
  errors (dasm output including the failing line), not swallowed. Sample: `roms/litmus/scenarios/smoke_src.json`.

## [0.20.0] - 2026-06-10

### Added
- **Automatic calibration of horizontal X(N) (B-4 / gap B fully closed).** Turns litmus from a one-off
  manual job into a reproducible sweep→auto-fit. A cooperating ROM (`litmus_pos`: delay `DELAY=$80`,
  SBC/BCS = 5 CPU cycles/unit) is poked across delays, `player0.ResetPixel` is measured each frame, and a
  linear regression recovers slope and offset numerically. Implementation: `internal/calibrate` (`Sweep`,
  `Fit` — robust to the 160 wrap and left-edge saturation via median-delta unwrapping of the longest
  consistent run). Result on litmus_pos: **slope = 3.0000 px/CPU-cycle** (matches the authoritative 3),
  R²=1.0, kernel offset = −18. Verified in `calibrate_test.go`.

## [0.19.0] - 2026-06-10

### Added
- **Golden-frame regression (P2 D-3 / gap D fully closed).** Adding `checks.golden_frame: true` compares the
  timeline's **rendered frame-chain hash** against `<scenario>.golden` = pixel-level regression detection of
  rendering (complements the D-1/D-2 logic/timing regression). Implementation: wire Gopher2600's exported
  `digest.Video` into `internal/emu` (`EnableVideoDigest`/`ResetVideoDigest`/`VideoHash`); `internal/scenario`
  enables it for golden scenarios, resets after warmup (deterministic), and compares to `.golden`.
  `cmd/scenario -update` records/updates the baseline. Sample: `roms/frogger/scenarios/golden.json` +
  committed `golden.golden`. CLI only; `bin/harness` (MCP) unchanged.

## [0.18.0] - 2026-06-10

### Added
- **Scenario runner (P2 / gap D = first step of verification automation. D-1 assertions + D-2 input replay).**
  Declares an "input timeline + numeric assertions" in one JSON and auto-passes/fails it against a ROM.
  `go run ./cmd/scenario <file.json> ...` (exit 0 on all pass, 1 on failure) = a regression base that runs
  in CI **without MCP**. Key design: the assertion vocabulary (`field` strings) maps one-to-one to
  `internal/emu`'s read methods (dogfooding the observation tools as the regression vocabulary). Unknown
  fields are an error (no swallowing typos). Whole-run measurements with side effects are separated into
  `checks{ntsc_frame_lines, max_line_budget}`. Structure: `internal/scenario` (parse + vocab + Run,
  ROM-agnostic) / `cmd/scenario` (thin CLI). Samples under `roms/litmus/scenarios/` and
  `roms/frogger/scenarios/` (including `hop` = `up` input drives FrogY 144→128). CLI only; MCP unchanged.

## [0.17.0] - 2026-06-10
### Added
- **`read_audio` MCP tool (R-2 / audio verification path).** Returns the current TIA audio registers
  AUDC/AUDF/AUDV for both channels as numbers (extends rule 1 "verify with numbers" to audio). Uses
  Gopher2600's exported `Audio.PeekChannels()`. Verification ROM `roms/litmus/litmus_audio.asm`; exact match
  in `emu_audio_test.go`.

## [0.16.0] - 2026-06-10
### Added
- **`assemble_and_load` MCP tool (P3 / build-loop shortening).** Takes an asm path, runs `dasm -f3` via
  `os/exec`, and loads the output `.bin` on success — collapsing `edit→dasm→load_rom`. On failure returns a
  structured `ok=false` + `dasm_output` (failing line) instead of an MCP error, so the model can fix in place.

## [0.15.0] - 2026-06-10
### Added
- **`step_instruction` / `step_scanline` MCP tools (B-2 / intra-frame granularity).** `step_instruction`
  runs exactly one CPU instruction (returns its cycles + coords); `step_scanline` runs until scanline +1
  (returns cycles consumed). A color-clock-granular `step_clock` is unimplemented (`Step` is per-instruction).

## [0.14.0] - 2026-06-10
### Added
- **`read_tia_registers` MCP tool (P1 / closes the rest of gap A).** Returns current values of write-only TIA
  registers directly from Gopher2600 internals (measure instead of inferring color from `read_row`). Confirmed
  PF0=$F0 (upper-nibble-only) behavior.
- **`read_collisions` MCP tool (P1).** Structures the 8 collision latches (CXxx, $30–$37) into named boolean
  pairs. Bit assignment verified against Gopher2600's `collisions.go`; BL-PF positive on `litmus_collide.asm`.

## [0.13.0] - 2026-06-10
### Added
- **`assert_line_budget` MCP tool (the crux of gap B / B-3 = per-scanline cycle budget guard).** Numerically
  catches the failure that silently killed Pong v2 (per-scanline overrun → screen roll). Detection: a WSYNC
  strobe = a `RdyFlg` true→false transition; the scanline delta between strobes = physical lines consumed by
  that logical line. Implemented with exported `RdyFlg` + beam coords in `internal/emu`'s own step loop
  (no debugger driver). Verified with `roms/litmus/litmus_overrun.asm` (`over=true`, `line_cycles=152`); no
  false positives on smoke / frogger.

## [0.12.1] - 2026-06-10
### Fixed
- **`read_cycles` double-counted spinning during a WSYNC stall (v0.12.0 bug).** During a WSYNC stall the CPU
  doesn't execute but leaves `LastResult` in place, so the old per-boundary accumulation over-counted on any
  WSYNC-using ROM. Fix: unify progress through a `stepInstr()` primitive and accumulate only when `RdyFlg`
  was true before the Step (i.e. a real instruction ran). Regression test `TestCycleCounterExcludesWsyncStall`.

## [0.12.0] - 2026-06-10
### Added
- **`read_cycles` MCP tool (gap B = wiring timing into the real loop, P0 step 1 / B-1).** Gets CPU cycles
  from the simulator numerically (first embodies rule 2 outside litmus). Returns `last_instruction_cycles`,
  `cycles_since_mark`, `total_cycles`. Source = `CPU.LastResult.Cycles` accumulated at instruction
  boundaries across all progress paths. Verified via the invariant "executed cycles × 3 == color clocks" on
  WSYNC-free `litmus_cycles.asm` (1 frame = 263×76 = `total_cycles 19988`).

## [0.11.0] - 2026-06-10
### Changed
- **Monorepo reorg: root = harness base / `roms/<game>/` = ROMs (spinoff Phase 1).** Demonstrated the
  game→harness one-way dependency and separated without surgery. Moved game-specific kernel generation
  (`cmd/genpf` + asmgen) into `roms/frogger/gen/` (package main importing `playfield`); litmus under
  `roms/litmus/`. All builds/tests green; `litmus_pf` read_row identical after the reorg.

## [0.10.1] - 2026-06-10
### Added
- **Frogger polish.** Game over / restart (Lives→0 resets Lives=3/Score=0); visual zones (top = goal band,
  bottom = start bank, middle = Monet water).

## [0.10.0] - 2026-06-10
### Added
- **🎉 Playable Monet Frogger (M5).** A frog crosses a river on flowing lily pads over Monet water. A full
  game kernel (`GenerateFroggerASM`) handles ride/drown/win/lives via a state machine; collisions via CXPPMM.
  The model **played it itself** (set_input + peek/read_tia) to numerically verify every mechanic — and found
  and fixed a fatal landing-frame timing bug that way (1-frame grace via `PrevY`).

## [0.9.3] - 2026-06-09
### Added
- **Frog vertical hop.** player0 drawn at variable scanline `FrogY`; edge-detected up/down jumps it ±16 (one
  lane) on press (no auto-repeat). The model operates/observes/judges in a closed headless loop.

## [0.9.2] - 2026-06-09
### Added
- **Collision check (the Frogger core: on a pad vs in the water).** Per-frame `CXCLR` strobe; CXPPMM read via
  `peek $37` (no new tool needed). Set/clear verified frame-by-frame.

## [0.9.1] - 2026-06-09
### Added
- **Full-scene integration.** Flowing lily (player0) + controllable frog (player1) coexist over Monet water
  (per-scanline COLUBK), with separate motion applied to both via one HMOVE.

## [0.9.0] - 2026-06-09
### Added
- **`set_input` tool = joystick injection.** `poke` doesn't work for input (RIOT redrives SWCHA each frame),
  so inject via Gopher2600's `Ports.HandleInputEvent`. Control ROM verifies "input → frog moves" headlessly.
### Fixed
- A `set_input` jsonschema tag starting with `0=…`/`true=…` made go-sdk panic in AddTool; reworded the tags.

## [0.8.1] - 2026-06-09
### Added
- **Monet water + flowing lily sprite integration (M3 step 2).** Per-scanline COLUBK (water) + per-scanline
  GRP0 (lily) both resolved in HBLANK to dodge cycle criticality; drift via per-frame HMOVE.

## [0.8.0] - 2026-06-09
### Added
- **M2/M3 animation groundwork.** Per-frame color-table animation (`GenerateAsymmetricShimmerASM`, with
  TIM64T-timed VBLANK/Overscan) and smooth sprite horizontal motion = water flow (`sprite_flow.asm`, +1px/frame
  via per-frame HMOVE) — establishing that smooth horizontal motion on the 2600 is the sprite's (HMOVE) job.

## [0.7.2] - 2026-06-09
### Changed
- **Promoted the Monet still (M1) to an asymmetric version** (left/right-independent playfield + per-row water
  color). Per-row water (COLUBK) + constant lily (COLUPF), since the asymmetric loop has budget for only one
  per-row color channel.

## [0.7.1] - 2026-06-09
### Added
- **Asymmetric (left/right-independent) playfield capability, hardware-verified.** Transcribed ABB's "repeated"
  asymmetric kernel (72 cy/line, `tay`/`sty` timing). read_row proves one-sided lighting (impossible with reflect).

## [0.7.0] - 2026-06-09
### Added
- **M1 "quiet pond" — the rendering pipeline opens end to end.** First milestone of the north-star ROM. Path:
  ASCII art + color → EncodeSymmetric → asmgen(kernel) → dasm → load_rom → read_row check. `GenerateSymmetricASM`
  generates a self-contained reflect-playfield still with per-row COLUBK water.

## [0.6.0] - 2026-06-09
### Added
- **`read_row` tool (read playfield-lit columns / per-scanline color numerically).** RLE `{clock,len,hex}` of a
  visible scanline. Playfield bit-order litmus (`litmus_pf.asm`) and per-scanline color litmus (`litmus_color.asm`)
  pass; the verified bit-order table is burned into `docs/resources.md` / `CLAUDE.md`. The `internal/playfield`
  package (`EncodeSymmetric`/`EncodeAsymmetric`) self-verifies against the real litmus values in go test.

## [0.5.1] - 2026-06-09
### Added
- **`get_screen_annotated` also saves the PNG to a file** (env `ATARI2600_SCREEN_PATH`) so clients that don't
  render inline images (CLI terminals) can still open the latest frame; VS Code auto-reloads on change. Returns
  `png_path` in the structured Out.

## [0.5.0] - 2026-06-09
### Added
- **`get_screen_annotated` implemented (the user↔model comms channel), as a first-class citizen.** Captures the
  frame to `image.RGBA` (PixelRenderer), draws a TIA-coordinate XY grid + axis labels + sprite markers (Fixed
  Debug Colors) at ×3 nearest-neighbor, and returns **image (ImageContent PNG) + numbers together**. Enables the
  "user points on the image → model translates to registers" round trip.

## [0.4.1] - 2026-06-09
### Changed
- **Distilled core constants into CLAUDE.md (Phase 4):** the beam-coord convention `Clock` = HBLANK −68..−1 /
  visible 0..159, horizontal position (3px/cycle, coarse 15px, 160 wrap, leftmost X=3; offset is kernel-specific,
  final verdict via `read_tia.HmovedPixel`), the fully hardware-verified HMOVE table, and the annotated screenshot
  redefined as the primary user↔model channel.

## [0.4.0] - 2026-06-09
### Added
- **Litmus test fully passed (Phase 3) — the harness proven real, numerically (rule #4).** Coarse
  (`litmus_pos.asm`): 1 loop = 5 CPU cycles = 15px, linear over DELAY 3–11 (`ResetPixel = 15·DELAY − 18`),
  160 wrap, leftmost X=3. Fine (`litmus_hmove.asm`): all 16 HMP0 nibbles match the CLAUDE.md HMOVE table at 1px
  granularity. Coarse 15px + fine 1px = any X numerically predictable/placeable/verifiable. Detoxes Pong's
  failures #1/#3 (gap B).

## [0.3.0] - 2026-06-09
### Added
- **Harness plumbing verified (Phase 2.1)** — Gopher2600 embedded as a library, driven fully headless and
  numerically on a real ROM. `internal/emu` driver wrapper; `cmd/probe` numeric CLI; `roms/smoke.asm` confirms
  262 lines / RAM `$80`=$42 / PC.
- **Minimal MCP prototype (Phase 2.2)** — `cmd/harness` exposes 8 tools over stdio; JSON-RPC confirmed numerically.
  Official `modelcontextprotocol/go-sdk` v1.6.1, typed Out auto-generates JSON Schema. Spec in `docs/mcp-tools.md`.
### Decisions
- **Drive via direct `hardware.VCS` embedding, not terminal/PushedFunction** — `hardware`/`television`/`setup` are
  pure Go (no SDL/cgo), so library embedding is more deterministic/simple/fast. The terminal driving the research
  docs assumed was unnecessary.
- **★ Beam clock convention settled on hardware:** `GetCoords().Clock` = HBLANK −68..−1 / visible 0..159 (the
  spec's tentative "0–227" was wrong); same coordinate system as `HmovedPixel`.

## [0.2.0] - 2026-06-09
### Added
- **macOS / Apple Silicon environment set up.** Go 1.26.4, cc65/sim65, pkgconf, Gopher2600 built
  (`go build -tags=release .`), DASM / Stella / SDL2.

## [0.1.0] - 2026-06-09
### Added
- **Project founded.** Defined the goal as "an environment where the model can author the Atari 2600 in 6502
  assembly accurately." Initial `docs/gap-analysis.md` (gaps A–E from the past-Pong post-mortem),
  `docs/tool-landscape.md`, `docs/resources.md` (horizontal formula `X = 3N − 55`, HMOVE table, frame budget,
  collision registers), README, CHANGELOG.
### Decisions
- **Engine = Gopher2600** (the only high-accuracy 2600 emulator drivable at CPU + color-clock granularity on
  macOS), wrapped in a thin Go MCP. **BizHawk not adopted** (no macOS). Regression layer = sim65 / 6502profiler;
  oracle = Stella; top-priority gap = B (timing). MCP SDK = official `modelcontextprotocol/go-sdk`; design follows
  mcp-gameboy. Image overlay in-house Go (no ImageMagick shell-out). Regression around Gopher2600's record/replay
  + `regress`.
### Changed
- Renamed the directory from `Stella-MCP` to `atari2600-dev` (the engine isn't limited to Stella, and the
  deliverable is a whole environment, not a single MCP).
