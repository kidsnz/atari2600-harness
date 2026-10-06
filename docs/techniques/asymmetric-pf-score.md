# Technique — asymmetric-playfield two-digit score (left/right independent, PONG-style)

**Goal:** big blocky per-player scores (2 BCD digits each side) drawn with the PLAYFIELD —
players/missiles stay free for game objects — by rewriting PF1/PF2 mid-scanline so the left
and right screen halves show different digits (CTRLPF repeat mode, reflect OFF).

**Status:** verified **in-game** (PONG `sandbox/practice/pong/steps/pong_top_paddle_pf2_*.asm`,
first landed in `pf2_score-2digit-playfield` 2026-07-01); standalone demo ROM + CI scenario = TODO.
**Source:** in-house, PONG pf2 work 2026-07-01 (session fa891501), digit layout matched to the
real Video Olympics look (4-px digits above the top wall). All digit cells verified with `read_row`.

## The pattern
- **CTRLPF D0=0 (repeat):** the right half re-reads PF0/PF1/PF2 in the same order → write the
  registers twice per line (left values in HBLANK, right values mid-line after the beam passes
  the left half) = 4 independent digit fields per line.
- **Digit geometry (per side):** tens + ones, each 4 PF pixels (=16 clocks) wide with a 1-PF-px
  gap, 5 zones × 4 scanlines tall (the classic chunky look). Layout used:
  - left tens = PF1 bits 4-1 (clock 28-43), bit0 = inter-digit gap
  - left ones = PF2 bits 0-3 (clock 48-63; PF2 is LSB-left → bit-reversed glyphs)
  - right tens = PF1 bits 7-4 (clock 96-111), bit3 = gap
  - right ones **straddles registers**: PF1 bits 2-0 (clock 116-127) + PF2 bit 0 (clock 128-131)
- **5 glyph tables** (one per field; the straddling digit needs a hi/lo pair), 8 bytes/digit,
  page-aligned so `(zp),y` never page-crosses. Pointers = `digit*8 + table base`, computed in
  VBLANK from the packed BCD scores.
- **Kernel line:** `WSYNC → PF1←(ScLt),y → PF2←(ScLo),y → ~10 nop → PF1←(ScRt)|(ScRoH),y →
  PF2←(ScRoL),y` — the mid-line rewrite must complete after the beam draws the left half
  (clock >79) and before it reads PF1 for the right half (clock 96).
- **Font pipeline:** glyphs auto-generated from the hand-painted 8×20 master font
  (`sandbox/practice/pong/tools/pong_font_gen_pf.py`, OR-pair 8px→4px) so one painting feeds both the sprite-font
  and playfield-font versions.

## Another glyph store: two digits in one PF1 byte, merged at run time (cited, not built here)
The 5 pre-split tables above need no masking in the kernel. The other way round, for a layout
where two narrow digits sit in PF1's two nibbles (MSB-first, so the high nibble is the left digit),
keeps ONE table with each glyph drawn in both nibbles and merges per line (AtariAge `topic/169927`,
2010, a Monaco GP scoreboard; Cited only, not verified):
- **AND/ORA (seagtgruff):** `lda (first),y / and #$F0 / sta result / lda (second),y / and #$0F /
  ora result / sta result` → `result` goes to PF1.
- **EOR fold (bogax):** from bytes `ab` and `cd` (letters = nibbles), `lda ab / eor cd / and #$F0
  / eor cd` leaves `ad` in A with no temporary — the second EOR cancels `c` and restores `d`.
- nukey-shay's choice guide in the same thread: when RAM and time come first, split the digits
  into separate tables so no mask is needed (four in his case; five in this file); when ROM comes
  first, keep a reversed copy in the low nibble and align it with LSR/ASL. None of these forms has
  been cycle-counted here.
- **Merged in the kernel, one line ahead (Outlaw):** Manuel Polik, 2001, on the playfield score of
  *Outlaw*, which he had analysed: it shows *"two different 2-digit numbers, the actual graphical data
  is 10 Bytes, but the RAM usage is only 2 Bytes(!). There's two blank lines at the beginning, then
  every line creates the data for the next line on the fly"* 〔stella-list `200102/msg00021`〕. In his
  posted loop PF1 is stored twice a scanline, once in HBLANK and once after the left PF1 is done (the
  last line exits before its second store), so the two scores are the left and right copies of PF1,
  and the next row's two bytes are merged with `and`/`ora` as above while the current row shows; his
  2 bytes are the two line buffers, and the loop also increments four offset variables (our reading of
  the code). The same source, priced, is in `docs/integration-density-playbook.md`. **Cited only,
  not verified.**
- **A mirrored copy in the same byte:** JeremiahK, 2018, explaining the text of his entry for the
  Nordlicht 512-byte demo competition, drew how *"both the forward and reverse versions of characters are encoded into the
  same table"* for a 3-pixel-wide 'R': forward copies in positions 1-3 and 5-7 of the byte as he draws
  it, and a *"REVERSE COPY (MUST BE SHIFTED INTO POSITION)"* in positions 3-5, sharing its outer
  positions with them 〔AtariAge `topic/280442`〕. That post does not say which registers the glyphs
  go to; his 2020 description of the same layout for a 3×5 playfield score, with its byte count, is
  in `docs/integration-density-playbook.md` 〔AtariAge `topic/311006`〕. **Cited only, not verified.**

## Verified numbers (PONG)
- All four digit fields verified per zone with `read_row` (e.g. "83 38": left-8 bar clock 28-43
  = $1E pattern, right-ones straddle at clock 116-131).
- Budget: the score-band line ≈ well under 76cy (two pointer reads per half + nop delay);
  `assert_line_budget` over=false with the full game running.
- Layout matches the real-PONG reference screenshot (digit centers symmetric about the net).

## Wider than two digits a side (cited, not built here)
Three archive posts put different data in the two playfield halves for more than a two-digit
score: lines of text, a full-width picture, and ten digits. None of them has been assembled or
measured here.
- **Ten characters a line of text (Jim Nitchals, 1997).** His playfield text, its 10 characters a
  line, its 6-or-7-lines limit and its taller rows 2 and 4 are quoted in `docs/techniques/text12.md`
  (under *10 from the playfield.*, *Nitchals' playfield text* and **Taller rows 2 and 4.**)
  〔stella-list `199709/msg00299`〕. The six playfield bytes he gives for each row are both halves'
  PF0/PF1/PF2, i.e. this page's left/right rewrite (our reading). **Cited only, not verified.**
- **A 40×24 picture (Eckhard Stolberg, 1998).** Asked by John Harvey how *Cookie Monster Munch* and
  *Big Bird's Egg Catch* put their words on screen without flicker, he answered *"They are playfield
  graphics"* and posted a loop that *"will display a 40*24 pixel picture"*: repeat mode (`CTRLPF` = 0),
  six tables PF0-L … PF2-R *"stored bottom up"*, `ldy #23` for the rows and `ldx #7` for 8 scanlines a
  row (24 × 8 = 192 lines), and each right-half value stored as soon as the left half has finished
  with that register — his comments put the stores at cycles 28, 39 and 50 〔stella-list
  `199812/msg00033`〕. Those counts add up when no table read crosses a page (our arithmetic). Unlike
  the PONG line above, which waits for the whole left half, PF1 is rewritten while the left PF2 is
  still being drawn. **Cited only, not verified.**
- **Ten digits, split by scanline (Jake Patterson, 2001).** *"It simply displays the even placed
  digits on even scanlines and the odd placed digits on odd scan lines"* — interlaced *"in more or less
  the same way as the scores in Space Invaders"*, with *"no flicker"*. *"There is enough time left over
  to read a color for each pair of scan lines from a table"*; the tables (200 digit bytes and 5 colour
  bytes, per his source comment) take *"205 byts of ROM for the tables, plus considerably more for the
  code, since the kernel is partially unwrapped"*
  〔stella-list `200109/msg00241`〕. In his posted kernel every digit is drawn on every frame, on
  alternate scanlines (our reading of the code). He thought it *"could probably be adapted to
  alphanumerics fairly easily"*. **Cited only, not verified.**
