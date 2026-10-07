# Technique — large or flicker-free fighters (four tricks reported in 1998 about commercial fighting games)

**Goal:** draw a fighter that is tall, or that does not flicker, out of player graphics that are 8
bits wide. One 1998 post describes what several commercial fighting games did — one game with tall
characters, others with characters that do not flicker; this page collects what that post reports.

**Status:** the only source is one mailing-list post describing what its author saw in commercial
ROMs. This harness has no ROM, no scenario and no measurement for any game or trick here, and none
of the games named was run here. Every claim about the games below is **Cited only, not verified**.
**Source:** Kurt Woloch, 〔stella-list `199804/msg00042`〕 (dated Thu, 9 Apr 1998, +0200), posted again
with the same text as `199804/msg00095` (Tue, 14 Apr 1998). The first copy's subject marks it as a
branch of the list's 1998 "big sprite" thread (a message from that thread is quoted in
`bitmap48.md`). He is answering Songbird, who asked in a post dated 8 April whether a cart called
*"Karate"* or something like it *"featured really large characters"*. Not part of this
repository's measured catalogue, so it is not in `README.md`'s table.

## What the post reports

Four tricks, each tied by the post to particular games. The post gives a height figure for one game
only, and says *"don't flicker"* about a group of them; it does not plainly say both of the same game
(see "Not verified" below).

**1. Coarse vertical resolution.** *"Karate characters are one color, 4-scanline-resolution, and
8-pixels wide (double width), Still, they fill about 80-100 scanlines."* Our reading: a new graphics
row every fourth scanline, so a character 80-100 scanlines tall is about 20-25 rows of graphics (our
arithmetic). The post does not say whether the 8 pixels are counted before or after the doubling.

**2. Bending the player with HMOVE.** Of Atari's *Realsports Boxing*, whose characters *"also seem
to use the missile"* (trick 3), *"and some HMOVE's to bend the head of a character that's hit"* —
the post's *"seem to"* covers this too. Of Absolute Entertainment's *Pro Wrestling*: it *"also bends the
characters when they're lifted by the opponent, so they seem about 20 pixels wide, but only 8 in
each scanline! (or was it that they are double-widthed to 16?)"* The post names HMOVE only for the
boxing game, says *"some"* rather than every line, and describes the bend as something that happens
on a hit or a lift, not as the standing shape. Its author was not sure of the wrestling game's width.

**3. A missile as a limb.** *Realsports Boxing* characters *"also seem to use the missile for the
boxer's glove"*, and in Activision's *Kung-Fu Master* *"the character is also expanded when it hits
the opponent, again using its missile."* Of Activision's *Boxing*, whose characters are *"monochrome,
but definitely more than 8 pixels wide, and still don't flicker"*: *"I think they also used some
missiles, but I don't know how."*

**4. Colour changes inside the player.** *"All these only have 8-pixel characters, but they don't
flicker, and have some in-player color changes to look really realistic."* The post does not say
whether a change falls between scanlines or within one.

## Price

Not measured here; the source gives no cost — no cycles per line, no RAM bytes and no ROM bytes for
any of the four tricks. The only resources the post points to are objects: one player per
character (our reading; the post's word is *"in-player"*), in two games that character's missile
(for the boxing game hedged with *"seem to"*), and in a third (Activision's *Boxing*) missiles that
its author thinks were used but does not know how. Our reading: with two fighters on screen, the two
players are both taken, which is why the post's limit is *"8-pixel characters"*.

## How this differs from the measured neighbours

- **`nusiz-shaping.md`** sets NUSIZ and strobes HMOVE on every line to make one player a wider,
  irregular shape, measured on 40 of 40 scanlines (`internal/emu/nusizshape_test.go`); that page
  states about 36 cycles per scanline for one object, without naming a measurement for the figure.
  The wrestling description above (*"about 20 pixels wide, but only 8 in each scanline"*) looks like
  the edge-shifting half of that idea, with or without a width change by its author's own doubt (our
  reading; the post does not say how that game makes the bend).
  What this page adds is a list, from different games, of: coarse vertical rows (*Karate*), a bend
  applied when a character is hit or lifted (*Realsports Boxing*, *Pro Wrestling*), a missile as a
  glove or an expansion (*Realsports Boxing*, *Kung-Fu Master*), and colour changes. The post names
  no game that does all four. That page's 36-cycle figure is for its own kernel and does not price
  any game here.
- **`hmove-slope.md`** moves a one-pixel missile or ball along a straight line of any slope: an
  accumulator decides on each line whether a fractional slope owes one more clock, and a steeper
  slope adds a whole part to every line's move, up to what one HMOVE can carry. Here the object bent
  is the 8-bit player graphic, and the post gives no rule for how each line's move is chosen.
- If a bend strobes HMOVE on visible lines, `nusiz-shaping.md`'s comb bullet (HMOVE on a visible
  line blanks visible clocks 0-7; that bullet names no measurement) applies to it (our reading; the
  post does not mention it).
- *Boxing* is also named, in 2022, among games using size and/or line-by-line shifting, in a
  SpiceWare quote held in `nusiz-shaping.md`'s "Two 2x players shifted line by line" bullet; he says
  only *"Boxing"*, and that it is Activision's is our reading.
  The 1998 post says only that its author did not know how *Boxing* used its missiles.

## Not verified

- That *Karate* draws its characters at 4-scanline resolution and fills about 80-100 scanlines.
- Whether *Karate* flickers: the post's *"don't flicker"* follows its list of *Kung-Fu Master*,
  *Realsports Boxing* and *Pro Wrestling*, and in our reading *"All these"* means those three.
- Heights for any game other than *Karate*: the post gives none.
- That *Realsports Boxing* bends a hit character's head with HMOVE writes, and how many lines it
  moves.
- How *Pro Wrestling* bends a lifted character, and whether it is 8 or 16 pixels wide on a line.
- That the missile draws *Realsports Boxing*'s glove and *Kung-Fu Master*'s expansion on a hit, and
  how either is sized or placed.
- When and how the in-player colour changes are written.
- Any cost: cycles, RAM, ROM, or the HMOVE blank.
