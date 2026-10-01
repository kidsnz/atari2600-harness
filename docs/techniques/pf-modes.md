# Technique #8 — Playfield modes: score mode & priority

**Goal:** two CTRLPF bits that change what the playfield *means*. **Score mode (D1)** paints the
same PF pattern with COLUP0 on the left half and COLUP1 on the right — the classic way one
playfield draws both players' scores/territory in their own colors. **Priority (D2)** puts the
playfield *in front of* players/missiles — sprites pass behind walls, bridges, HUD frames.

Learned from (clean-room): Stella Programmer's Guide CTRLPF; spiceware Step 7. Demo:
`roms/techniques/pf_modes.asm`, locked in CI by `scenarios/pf_modes.json`. (Asymmetric PF and
reflect — the other halves of "playfield tricks" — were already verified by litmus_pf_async,
the Exerciser zone scene, and the procedural mountains.)

## The technique
- `CTRLPF` D0=reflect, **D1=score**, **D2=priority** (D4-D5 = ball size). The bits are live —
  you can switch them mid-frame per region, as games do (score bar on top, gameplay below).
- In score mode COLUPF is ignored; the PF takes COLUP0/COLUP1 by *screen half* — so it pairs
  naturally with the left/right player split of versus games.
- **Score mode moves PRIORITY too, not only colour.** The left half of the PF takes P0's priority
  as well as COLUP0, the right half P1's — so on the left half P1 goes *behind* the playfield
  (still in front of the ball). alex_79 reports it as absent from the Stella Programmer's Guide;
  per the same thread Stella did not emulate it until around 4.x, and SwordQuest Waterworld relies
  on it (stephena, from memory) (AtariAge `topic/301780`; Cited only, not verified). The engine
  draws it that way: the score-mode branch of `Gopher2600/hardware/tia/video/video.go` quotes
  supercat — *"in score mode, the left half of the playfield has priority over the player/missile
  1 sprites"* (AtariAge `topic/166193`) — and puts P1/M1 above the ball there (read from the
  source; Not verified). No fixture here measures it: `pf_modes.asm` and `litmus_ctrlpf.asm`
  never write GRP1, and `litmus_score_pfp.asm`, which does draw P1 in score mode, puts it at
  x=104 — on the right half only, where P1 is in front of the PF either way. With D2 set score mode is off whatever D1 says — that part is
  measured (`docs/fundamentals-audit.md`, "SCORE×PFP interaction measured").
- Priority is global PF-over-players; for per-object layering you reorder *which* objects you
  draw with PF vs sprites.

## Verified here (pixel-level, Gopher2600, locked in CI)
Three horizontal regions, modes switched mid-frame:
- **Score region:** PF1=$66 blocks read back **CC2121 (COLUP0 red) on the left half, 2D32EA
  (COLUP1 blue) on the right** — same pattern, two colors (`read_row`).
- **Normal region:** red P0 column (X=62) fully covers the yellow wall (PF2 bit4, clocks 64-67):
  row reads `62+8 red`, wall invisible.
- **Priority region:** same overlap reads `62+2 red / 64+4 yellow / 68+2 red` — the wall now
  splits the sprite = PF in front.
- `tiareg.playfield.ctrlpf`, P0 color and position asserted; 262 lines; golden frame.
