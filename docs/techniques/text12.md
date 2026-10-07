# Technique — 12-character text line (flicker-free)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** readable text for menus, messages, and titles: 12 characters per line, flicker-free,
using the same hardware-verified 48px VDEL 6-store choreography as the score kernel — with a
4×5 font packed two characters per player byte.

Demo: `roms/techniques/text12.asm` ("HELLO WORLD!" / "ATARI 2600.." on two lines).
CI: `scenarios/text12.json` (positions, packed-buffer bytes, 262 lines, golden).
Lineage: David Crane's 12-characters-per-line routine (Basic Programming, 1979) — the ancestor
of every 2600 text display; researched via the AtariAge 32-character-display thread, where the
width ladder is 12 (flicker-free) → 24 (column flicker) → 28 (Jentzsch) → 32 (interleaved
RESP re-strobing, solidcorp 2011). 12 is the sweet spot: zero flicker, no re-strobe timing
hazards, reuses the score kernel verbatim.

★**That ladder counts CHARACTERS, not width — the character width never changes.** Measured
2026-09-07 by reading the picture (`internal/emu/textwidth_test.go`): `text12` puts its twelve
characters in **clock 87..134 = 48 px**, and `text24` puts the *same* 48-px block at **two** X
positions — 39..86 and 87..134 — on alternate frames. **48 px is one 6-store sprite block, every rung
uses the same 4×5 font, and 48 / 12 = 4 px per character at every rung.** Climbing the ladder does not
narrow the letters; it adds another block at another X and pays in flicker. What a rung buys is how
much of the 160-px line carries text; what it costs is how often each block is drawn.

★★**So a glyph wider than 4 px is not a character in any of these kernels.** A 10-px letter needs the
48-px block used as a *picture* (`bitmap48.md`), where it is one of about four shapes on the line
rather than one of twelve. That is a different technique with a different cycle budget, not a wider
setting of this one.

★★★**Sampling one frame hides half of this.** One frame of `text24` shows one 48-px band and invites
the conclusion that 24 characters are squeezed into the same span at 2 px each. Both phases have to be
read — the same trap `scripts/phase_probe.py` exists for. Raised by the mailing-list distillation
(helper-2) from a 2003 thread where three people took apart David Crane's routine and disagreed over
whether letters could be 7 px or 8 px wide 〔`200309/msg00212`, `msg00216`, `msg00218`〕.

**Before designing anything wider, read [`restrobe-copies.md`](restrobe-copies.md) (technique #36).**
The rungs above 24 all use the mechanism named in the line above — a mid-line `RESP` re-strobe — and
#36 is where this harness measured it: a player in a copy mode draws **3 + k** slots with k mid-line
strobes at 6, 7 or 8 cycles of spacing, the ladder is **flat** at 3 and 5 cycles, and it climbs faster
at 12. This page's "12 is the sweet spot" is a statement about cost, not about a ceiling. A work in
the private `roms/` repository re-derived that ceiling from scratch in 2026-08 without reading #36,
and lost days to it; the one-way link (#36 pointed here, nothing pointed there) is why.

## The technique

- **Font**: 4×5 glyphs (39 chars: space, A-Z, 0-9, !, .), one nibble per row (bit 3 = leftmost),
  stored bottom-row-first (the kernel walks Y=4..0). 195 bytes of ROM (39 × 5).
- **Build (once, or per string change)**: for each of 6 character pairs, compose
  `Font[left]<<4 | Font[right]` per row into a **column-major zero-page buffer**
  (`buf[pair*5+row]`, 30 bytes per text line). Strings are stored pre-encoded as glyph indices.
- **Kernel**: exactly the score6 choreography — six `(zp),y` pointers set to `buf+pair*5`,
  4-burst stores completing at 55/58/61/64 cy, position P0=87/P1=95. Each text row is drawn on
  2 scanlines (the second line re-runs the store sequence with the same Y) → a text line is
  12 chars × 10 scanlines.

## Verified
- Both demo lines render legibly on first run (annotated screenshot), buffer composition is
  byte-exact (`buf[0] = H‹4|E = $9F` asserted), 262 lines every frame, golden-pinned.

## Notes / variants
- Per-line color: set COLUP0/1 before each text line (the choreography leaves room outside the
  burst). Scrolling: feed the window through the buffer build (the bitmap-minikernel idea).
- Wider displays (24/32 chars) need column flicker or RESPx re-strobing — recorded as catalog
  candidates with the measured constraints (9px strobe granularity, RESP-vs-GRP write conflicts)
  from the research thread; implement when a game needs them.
- **Rungs the ladder above leaves out.** It lists four rungs. Others from the list, sprite and
  playfield, all **Cited only, not verified**:
  - *13, flicker-free, from the playfield.* R Mundschau (December 2003) draws each letter as a hole in
    the playfield — *"the color of the letters is the COLUBK value"* — 3 PF blocks wide, packed with no
    blank column. All five objects, set to the COLUPF colour, lay a 4-px strip every 12 px over the
    2 px either side of each join: 14 strips from M0 (8 px, wrapping the screen) 2, P0 (quad width,
    `%01001001`) 3, P1 (two copies, wrapping) 2, M1 2, and the ball, re-strobed, the remaining 5
    〔stella-list `200312/msg00038`〕. His own list of costs: 3-px letters whose middle pixel is
    double the width of the other two, letters fixed horizontally, one colour, and a string that is
    *"rather computationally intensive"* to generate; on colour, *"I believe color changing could be
    added, but you would have to unwrap the loop"* 〔`msg00023`〕. Billy Eno saw it on a Supercharger as
    well as in PCAE and z26 〔`msg00013`〕. The other way it went: the routine needs the ball drawn at
    its new position as soon as `RESBL` is written, and Stella for Mac drew it a line later; a first
    version that wrote a `RESBL` copy outside zero page gave a black screen there, and he rewrote it
    〔`msg00011`, `msg00038`〕. On the 3-px glyphs, a post signed Manuel from the `cybergoth` address (the
    one Manuel Rotschkar's `200312/msg00111` and Manuel Polik's `200108/msg00398` carry) answered
    Mundschau's 3-px line: *"That's the main problem I assume: W, U, Y & V and M & N"*
    〔`200312/msg00028`〕; Paul Slocum: *"They look a little funny, but I think they'll be okay in the
    context of words and sentences. I played with them in Photoshop earlier. Q is a little tricky too."*
    〔`200312/msg00029`〕
  - *10 from the playfield.* Jim Nitchals, 1997: *"40/4 = 10 characters per line. It's done."*
    〔`199709/msg00299`〕. Roger Williams, 2001: *"6 lines of 10 characters, fairly readable, and with
    plenty of CPU time left"* 〔`200109/msg00220`〕.
  - *13 from sprites.* Paul Slocum (December 2003) adds a thirteenth character to the 48-px block from
    the two missiles and the ball: *"The playfield covers up the other two copies of the missiles"*
    〔`200312/msg00066`〕. Those copies exist because a missile follows its player's NUSIZ copies —
    rule 4 of `sprite-placement.md`, measured there; the covering is not. The next day: *"13
    characters plus two more characters on the side ... I rewrite NUSIZ at the end of the 13
    characters and change the player copying to wide. You could also move the missile/ball graphics
    over and do 12+3, or remove the ball/missiles and do 12+2 with more color."* 〔`200312/msg00102`〕
  - *10, for colour.* Slocum's RPG map (tile) kernel, his rewrite of Crane's text routine
    〔`200309/msg00216`〕, *"only displays 10 characters per line so that it can display the playfield
    behind the characters (for a little color.)"* 〔`200309/msg00230`〕.
  - *9 and 18.* Colin Hughes: an 18-character title line *"flickering between odd and even character
    spaced triple 3 wide spaced players"*, and a 9-character kernel without flicker — *"I never tested
    it on a real VCS, just pcae"* 〔`200207/msg00279`〕.
- **Lines per screen are the other axis.** Our reading of the posts below: the ladder counts
  characters per line and is bounded by cycles; how many lines fit is bounded by RAM against rebuild
  time, because every line's buffer has to be either held or rebuilt between lines. This page's demo
  holds two 30-byte buffers built once before the first frame (`text12.asm`, `bufA`/`bufB`), so no
  rebuild reaches its kernel; it would as soon as there are more text lines than buffers held in RAM.
  - Greg Troutman, 1997, on this page's packing: *"30 bytes for one 5 scanline high line of text"*;
    rebuilding it inside the frame *"would take well over a thousand cycles, I think, producing as
    many as 20 blank scanlines"*; *"You might use VBLANK to pre-load 3 lines of text using 90 bytes of
    RAM if you have them (unlikely for a game, but maybe possible in some special project like Jim is
    doing)"* 〔`199709/msg00300`〕. His best case for the build,
    with the glyph nibble duplicated so only a mask is needed, is 24 cycles a byte, 720 for the line,
    and he doubted it could be done under 1000 without vast amounts of ROM 〔`msg00311`〕.
  - Nitchals' playfield text: *"Processing the bitmaps only allows 6 or 7 lines of text per screen
    (there's not enough RAM to keep the bitmap around for more than one line of text at a time!)"*;
    he kept 6 to leave room for a password and editable text 〔`msg00299`〕. Of his player-graphics
    version two days later: *"There's enough CPU time to render 7 lines of text."* 〔`msg00321`〕
  - Christopher Tumber's Quadraside menu (2003): 24 characters per row and about 11 rows, *"double
    spaced to keep RAM usuage in check (data for the next row of text is pushed into RAM between
    rows)"*; single spacing for a couple of rows costs *"About 40 bytes per line"*
    〔`200307/msg00011`〕. Posting that setup screen again in December, he gets the 24 by drawing the
    48-px sprite every other frame 〔`200312/msg00069`〕.
  - Rebuilding between rows, December 2003: Andrew Towers, with two font copies, found *"it still
    takes about as long to generate the line of text as it does to display it"* 〔`200312/msg00070`〕,
    then trimmed it to 7 blank lines between text lines 〔`msg00075`〕. Thomas Jentzsch: the indirect
    text lookup Towers said a real game needs costs *"at least 13 additional cycles ((ind),y
    instead of abs,y)"*, and the thread ends with that version 3 cycles short 〔`msg00081`〕.
  - Width trades against spacing. Tumber, setting the 13-character routine against his flickering
    24: *"If 13 is enough then that's obviously the way to go, particularly since you can probably get
    almost as much text on screen given the fewer blank scanlines between rows"* 〔`msg00078`〕.
  - How many lines that came to. Slocum in April 2004, recommending it for a multicart menu: *"13
    chars with no flicker, can display 15 lines as is"* 〔`200404/msg00325`〕; the next day he said
    the link he had given, `200312/msg00070`, was *"an earlier version of the routine that could only
    display 14 lines"*, and pointed to `200312/msg00075`, the 7-blank-line version
    〔`200404/msg00351`〕. In the same thread Adam Thornton, on the Stellar Track engine as used in his
    Fellowship of the Ring (his hack of Greg Troutman's Dark Mage, with Thomas Jentzsch's improvements
    to the engine): *"IIRC the resolution is 13 lines of 12 characters each"* 〔`200404/msg00323`〕.
    **Cited only, not verified.**
  - B. Watson, 2001: 20 characters from two 5-digit routines on alternate frames. His non-optimised
    conversion of a string into the 25 display bytes takes about 30 scanlines; a text line is 5 bytes
    drawn twice plus one empty scanline, 11 in all. Advancing the message pointer every 4 frames
    scrolls a 128-byte message as a marquee, *"the last 20 bytes of which need to be spaces"*, and it
    *"worked on a real Atari the first time"* 〔`200109/msg00240`〕.

  **Cited only, not verified** — `BuildBuf` in `text12.asm` has not been cycle-counted.
- **A font, or the phrase itself.** Count the phrases before building a font. Nick Bensema, answering
  a hiragana font: *"In practice, though, we're probably only going to use one or two phrases anyway,
  so it might be more economical to just draw the characters in manually, like we do for English
  text"* — his example spells *kudasai* in 24 bytes 〔stella-list `199804/msg00141`〕. A font pays for
  its whole table, the build loop and the RAM buffer; a drawn phrase pays only its own bytes. One or two
  fixed phrases: draw them (`bitmap48.md`). Many, or text that changes: the font.
- **Glyphs wider than 4 px, and marks beside them.** The font Bensema was answering, Chris
  Cracknell's hiragana, is drawn in 8×8 blocks, each glyph the full width of a player, and he suggested
  the six-digit score routine to show it. The marks: *"the marks that give you the alternate
  pronounciation of a character had to be a seperate sprite placed beside the root character (since
  the sprites are only 8 bits wide I couldn't incorporate the marks into a single character with the
  root)"*; the table's comments put a small *ya* below
  〔stella-list `199804/msg00138`〕. Two days later his `sexp8.bin` moved Japanese characters around
  the screen as sprites 〔`199804/msg00163`〕. An 8-px glyph is the ★★ case at the top of this
  page, and a script with marks builds one character from two objects where this
  page packs two characters into one. **Cited only, not verified**.
- **Trim the font to the text.** Answering a card game whose letter images had reached 174 bytes
  〔stella-list `200111/msg00336`〕, Paul Slocum: *"Maybe you can reword some messages to avoid using less
  common letters like V or X, then drop the data for those characters"*, and *"you can
  combine a few letter graphics like "U" and "H", or "I" and "T""*, a trick from Dark Mage/Stellar Track 〔`200111/msg00338`〕 — in his
  listing U's last rows are H's first, two 6-byte glyphs in 9 bytes (our reading of the listing).
  Thomas Jentzsch, in his own reply: *"You might be able to overlap the data for some letters ... And
  do you really need *all* letters?"* 〔`msg00342`〕. The author, Erik Eid, called dropping rare letters
  *"pretty frequent"* advice and waited until the text was final; he could probably move the
  card-rank images into the letter table so J, Q, K and A were not stored twice — *"It's not
  necessary that the images be in "alphabetical" order. It just helped when creating the message
  data."* — and was not sure he could overlap glyphs, since *"the offsets into the letter image table
  are calculated as multiples of six"* 〔`msg00344`〕. Here `BuildBuf` already looks the offset up in
  the 40-byte `Mul5` table, but the same table also computes the output address (pair × 5), so
  overlapping glyphs would mean splitting off a glyph-only offset table. **Cited only, not verified**.
- **Duplicate each glyph nibble into both halves.** Every byte of the `Font` table in `text12.asm` is
  `$0`–`$F`, so storing `$99` instead of `$9` costs no ROM. Piero Cavina: *"the same 4-bit character
  twice in 1 byte, so that combining two of them into 1 byte can be done without shifting bits"*
  〔stella-list `199709/msg00308`〕 — the four `asl` that `BuildBuf` spends on the left character
  become a mask. Greg Troutman counted *"one mask operation (2 cycles) vs. 4 ASLs (8 cycles)"*, 180
  cycles over a 30-byte line 〔`msg00311`〕; but once both halves are filled the right character needs
  a mask too, so by the same count it is two `and`s against four shifts, 4 cycles saved per byte rather
  than 6. Jim Nitchals' alternative keeps two font tables, one pre-shifted, and needs no mask at all
  〔`msg00317`〕. In 2003 Paul Slocum proposed two font versions for his 13-character routine
  (*"that's a lot of ROM space too"*) and Andrew Towers built it: two copies of 144 bytes each, against
  182 bytes for the uncompressed block of text on that screen 〔`200312/msg00066`, `msg00070`〕.
  Christopher Tumber's menu keeps *"a high nibble and low nibble version"* of each character, *"which
  is probably why I've got a little less space between rows than Paul's"* 〔`200312/msg00069`〕.
  **Cited only, not verified** — `text12.asm` still shifts.
- **Taller rows 2 and 4.** Jim Nitchals, on his 5-row playfield text: *"Stretching rows 2 and 4 of the
  text gives it better shape without taking up extra room in the bitmap (6 bytes of playfield x 5
  lines.)"* 〔stella-list `199709/msg00299`〕. The stretch is in how many scanlines the kernel spends on
  a row, not in the data. This page draws every row on 2 scanlines; giving rows 2 and 4 more is the
  same trick. How many he used is not in the post. **Not verified**.
- **Proportional glyphs, and no feature one scanline tall.** karl-g's tiny proportional font (2020)
  starts from Andrew Davie's monospaced Glacier Belle (3 px × 12 lines) and changes only a few glyphs;
  2 px between words read as cramped, so at least 3. The rule for drawing any glyph that will
  **flicker**: no pixel feature one scanline tall — an LCD TV can swallow every other line of a
  flickering picture and take the feature with it; make it at least 2 lines tall (any height above 1,
  not necessarily even). A kernel that does not flicker, like this page's, is not affected
  〔AtariAge `topic/312815`〕. **Cited only, not verified** — read from distilled notes, not the thread.
- **Proportional width can ride in a spare byte.** Eckhard Stolberg's 1999 scrolling text uses a
  proportional font: *"I needed a counter for the shifting anyway to see when I have to copy the next
  character. Since each character is only seven scanlines high, adding a width value as the eighth
  byte"* cost little 〔stella-list `199907/msg00011`〕. This page's glyphs are 5 bytes, all 4 px wide,
  with no shift counter, so neither part comes free here. **Cited only, not verified**.
- **What a small font costs, and drawing it at size.** Nick Bensema's 3×5 font — A–Z, 0–9, space,
  comma, period and exclamation point, 40 glyphs — *"takes up only 82 bytes of space. That's barely two
  bytes per character."* 〔stella-list `199810/msg00048`〕; 40 × 15 bits is 75 bytes (our arithmetic),
  against this page's 195 for 39 4×5 glyphs. At 3×5, reveng on lowercase: *"the smaller ones like 3x5
  use RAISED descenders, and it looks fine to me"* 〔AtariAge `topic/323665`〕; this page's font has
  no lowercase. Manuel Polik, needing a 7×7 font for his Gunfight, rescaled the 8×10 Atlantis font by
  hand and tried an 8-pixel TTF font, found both ugly, and kept his own: *"At least all of its numbers
  fit to each other"* 〔stella-list `200108/msg00398`〕. **Cited only, not verified** — the AtariAge
  post read from distilled notes, not the thread.
