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
  Within one line the bits do not all take effect alike. Brad Mott, from tests he ran on when
  playfield changes really occur (in his count cycle 48 is pixel 76): *"Change the reflect bit at
  or before cycle 48 and the display will correspond to the reflect value you specify"*, after
  cycle 48 *"the display will correspond to the old reflect value"*, while *"Changes to the score
  bit and the priority bit take effect at the end of the cycle they are changed (even in the
  middle of a 4 pixel wide playfield block)"* 〔stella-list `199805/msg00153`〕. He adds, of his
  results in that post, *"I'm pretty sure these were for an unreflected playfield"*. No fixture here
  writes CTRLPF mid-line. **Cited only, not verified.**
- Kevin Horton, from a first reading of the TIA schematics (2001): the playfield is *"a 20 bit
  left/right shift register"*, which *"allows both "normal" and "reflected" versions to be
  produced"*, clocked *"directly from the clock to the horizontal scan counter"* 〔stella-list
  `200109/msg00291`〕. **Cited only, not verified.**
- In score mode COLUPF is ignored by the playfield; the PF takes COLUP0/COLUP1 by *screen
  half* — so it pairs naturally with the left/right player split of versus games.
  **The ball is not switched: it keeps drawing in COLUPF.** SeaGtGruff drew a piano keyboard with
  the ball and a score-mode playfield, *"so the ball uses the COLUPF color and the playfield uses
  the COLUP0 color"* (AtariAge `topic/201572`); our notes of AtariAge `topic/166193` say the same,
  and the engine leaves the ball in COLUPF in score mode (`design-principles.md`, "The ball is not
  a colour of its own"; read from the source). No fixture here reads the ball's pixels in score
  mode — `litmus_ctrlpf`'s ball band runs with D1 clear; `internal/oracle/testdata/tiaprobe.asm`
  sets score and priority together (`$27`, where priority wins) with the ball on, and
  `tiaprobe2.asm` sets score (`$12`) with the ball off; both compare registers only. **Cited only,
  not verified.**
  Per the same notes of `topic/166193`, omegamatrix described the *shine* in Strat-O-Gems: the
  display area surrounded with playfield, score and reflect on, the graphics pre-inverted so the
  digits are background, and the ball moved with HMOVE every line so a gloss runs across the digits
  (read from our notes, not the thread). **Cited only, not verified.**
- **The price of score mode is that COLUP0/COLUP1 are the players' colours too.** Thomas Jentzsch,
  on a kernel that rewrote COLUP0, COLUP1 and CTRLPF every line to colour the playfield through
  score mode: *"you are wasting way to many cycles (24!) for coloring the playfield ... And your
  colour changes will change the players colors too. You should use COLUPF for this. (But I bet,
  that would still cost to many cycles.)"* [sic] 〔stella-list `200111/msg00186`〕. The author answered that the shared
  colour was intentional — each player's home side would be drawn in the other player's colour
  〔`200111/msg00191`〕. **Cited only, not verified.**
- **On hardware the split may not be clean.** Nick Bensema, 1997: *"On my 2600jr, color from the
  right side bleeds about one pixel into the left side"*, and of the emulators then, *"PC Atari
  does not implement it at all; both sides appear as Player 0's color. A26 does, though"*
  〔stella-list `199703/msg00015`〕. Gopher2600 splits at clock 80 with no bleed (`litmus_ctrlpf`,
  `docs/fundamentals-audit.md`), so a one-pixel bleed would not show in any fixture here.
  **Cited only, not verified.**
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
- **Priority as a mask.** Eckhard Stolberg's 1998 "18 extra lives" demo shows 18 sprites *"On a
  real VCS"* and hides the unwanted ones *"by putting the playfield and the ball over them"*:
  *"the sprites can only be 5 pixels wide"*, but *"each sprite could be turned on and off
  individualy"* and they can be animated 〔stella-list `199808/msg00103`〕. The source he posted
  sets CTRLPF priority with an 8-pixel ball, keeps the mask as five RAM bytes labelled
  `PF1 PF2 PF0 PF1 PF2`, and saves the stack pointer during the 18-sprite display
  〔`199809/msg00001`〕. He reports that the emulators of the day drew it wrongly (Stella showed 20
  sprites) 〔`199808/msg00103`, `199809/msg00002`〕. **Cited only, not verified.**
- **The picture as holes.** R Mundschau, after an intro screen of HMOVEd vertical lines, proposed
  for a next game: *"invert the playfield graphics so the images are holes showing the background
  color. Then the lines pass behind and fill in the form of the image as they pass"*, with
  background and foreground the same colour 〔stella-list `200306/msg00023`〕. A proposal, not a
  shipped effect, and the post does not say which objects draw the lines or how they are put behind
  the playfield. **Cited only, not verified.**

## Reflect per row: asymmetric, but not arbitrary (from the lists; Cited only, not verified)
- **The idea.** Erik Mooney, 1999, to someone who wanted a full 40-bit playfield: *"in your
  playfield storage scheme, store only 20 bits per row but also indicate whether to reflect or
  repeat - with creative terrain design, you could probably make that invisible to the player"*
  〔stella-list `199907/msg00130`〕.
- **In a game kernel.** Zach Matley, 2003, on two screens of Looping (screenshots on AtariProtos)
  that look asymmetric: *"The kernal changes between duplication and reflection several times per
  frame. This allows a playfield that is not symmetrical in the strictest sense, and allows sprites
  to move freely"* 〔stella-list `200309/msg00226`〕.
- **Per row, per layer.** Christopher Tumber's Fade Out does *"both symmetrical and copied PF.
  (asymetrical but not arbitrary)"*, the symmetrical/copy status of *"a row's background and
  platforms"* at the time controlled independently, by *"tweaking CTRLPF just before the
  platforms"*. The same post adds that *"the top row of backgrounds is asymmetrical"*, and that
  *"the changes to PF values happen midscanlines but are masked as blank (black) scanlines"*
  〔stella-list `200312/msg00088`〕.
- Our reading: the reflect switch itself is one CTRLPF write per zone, where a true asymmetric
  playfield rewrites the PF registers in the middle of every line (Fade Out, above, still rewrites
  PF mid-line between rows, hidden on black lines). If the switch lands mid-line, Mott's cycle-48 rule above
  applies to the reflect bit.
- **When the playfield IS fully asymmetric, positioning is the hard part.** Thomas Jentzsch, 2003:
  *"For a full width playfield I would suggest using the non-reflected mode. That gives you a bit
  more flexibility"*; split the positioning code *"into (at least) two parts, one for positioning
  on the left and one for the right side"*; *"Worst case would be 11 different parts, one for each
  coarse positioning value"*; *"I'd suggest some striped PF like most (all?) 2600 with that feature
  do"* 〔stella-list `200302/msg00023`〕. Manuel Polik in the same thread: *"If you don't have any
  void lines in the PF layout, 100% free positioning is not possible"* 〔`200302/msg00021`〕, and
  Clay Halliwell, looking again at Ms. Pac-Man, found *"it only does sprite positioning on lines
  with no dots"* 〔`200302/msg00027`〕.

## The playfield as a large coloured object (from the lists; Cited only, not verified)
- Glenn Saunders, 1998, on a ship demo: *"A large symmetrical object with slices of colors like
  this is something that is well-suited to the 2600's playfield. I don't know many examples of the
  playfield being used to paint a large multicolored MOVING object like this"* 〔stella-list
  `199806/msg00082`〕.
- **Colour bands from three registers.** Christopher Tumber, for a 3D corridor, first proposed
  building it from all the sprites, then concluded that *"if using sprites to do this, the pros
  don't really outweigh the cons (...) unless you're really prepared to use PF graphics for
  enemies &etc."*, the con being *"You've used up all the sprites and are limited to playfield
  graphics"*, and instead: *"preloading X, Y and A and shoving them directly into COLUPF"*, with
  colour cycling for motion. His caveats: *"There's currently only 14 cycles available per
  scanline"*, and moving the tunnel sideways is *"limited to 1 cycle increments"*
  〔stella-list `200303/msg00335`, `200303/msg00341`〕.

## Verified here (pixel-level, Gopher2600, locked in CI)
Three horizontal regions, modes switched mid-frame:
- **Score region:** PF1=$66 blocks read back **CC2121 (COLUP0 red) on the left half, 2D32EA
  (COLUP1 blue) on the right** — same pattern, two colors (`read_row`).
- **Normal region:** red P0 column (X=62) fully covers the yellow wall (PF2 bit4, clocks 64-67):
  row reads `62+8 red`, wall invisible.
- **Priority region:** same overlap reads `62+2 red / 64+4 yellow / 68+2 red` — the wall now
  splits the sprite = PF in front.
- `tiareg.playfield.ctrlpf`, P0 color and position asserted; 262 lines; golden frame.
