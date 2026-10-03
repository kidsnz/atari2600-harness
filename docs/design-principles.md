# Atari 2600 visual design principles (design-principles)

The canon of "graphics-design principles that can be reduced to rules", obtained from mining (AtariAge) plus
web research. Purpose = (1) explicit rules for Claude's design judgement (roms/EVALUATION.md's ⑥craft)
(2) the basis of `pkg/design` feasibility judgements (also reusable for the frozen TIA Studio templates).
Detailed sources = `tools/research-w2-design.md` + `docs/mining-digest.md` (mined-thread index) + `reference/atariage/*/notes.ja.md`.

**The executable judgements are already "absorbed" into `pkg/design/`** (machine-checked before any asm is written).
Rules that can be quantified name their function at the end of the line with `→ func`. Judgement rules that
cannot be quantified are collected in the final section, "Judgement rules no machine can decide".
Mapping: colour bands / gradients / mixing = `color.go` / horizontal position = `position.go` / PF windows, 2-colour score, scrolling = `pf.go` /
multiplexing = `multiplex.go` / character count = `text.go` / budget = `budget.go` / drawing craft = `craft.go`.

## Colour (most important)
- **Hold colour as a register value / symbolic name (hue = upper nibble × lum = lower nibble), never as RGB**. Don't scatter raw hex.
  Luminance is effectively 8 steps (bit0 has no effect). Design goal for PAL/NTSC = two parallel sets (N_xx/P_xx) switchable in one line. 〔Davie S11, symbolic-color-names〕
  - **The two sets are not one set shifted: each PAL value has to be chosen, and some names have none.**
    Thomas Jentzsch, 2017, answering how to give colour values names in DASM, keeps one switch and two
    tables under conditional assembly (`NTSC_COL = 1` / `IF NTSC_COL` … `ELSE` … `ENDIF`), fifteen
    names from `YELLOW` to `OCHRE`. Under NTSC the names step through the hue nibble in order (`YELLOW =
    $10` … `OCHRE = $F0`); under PAL the same names land out of order — `RED` `$40`→`$60`, `CYAN`
    `$B0`→`$70`, `BLUE` `$80`→`$D0`, `GREEN` `$C0`→`$50` — and three carry his comment *"no real
    equivalent"*: `YELLOW` and `OCHRE` both `$20`, `OCHRE_GREEN` `$30`, each a neighbour's value reused.
    〔mining 266202 any-tips-for-a-beginning-atari-2600-programmer〕 So a colour named on NTSC may not
    exist on PAL; compare the palette-side measurement in `visual-ceiling.md` (four of sixteen PAL hues
    are one grey). **Cited only, not verified** — his values have not been checked against the PAL
    table here.
  - **Which set to use can be measured instead of switched, given an ARM on the cartridge.** SpiceWare,
    2017, timing one frame with a Harmony/Melody timer, after ZackAttack's idea of telling the consoles
    apart by their clocks: the-maxx's NTSC 2600 read `11d32A`, `11d32E` and `11d326`; Thomas
    Jentzsch's PAL 2600 *"11Fd00..11Fd0F"*; alex_79's PAL 2600 Jr `11fd15`–`11fd20` and PAL 7800
    `11fd31`–`11fd35`. SpiceWare: *"I'll use the middle of the extremes, (11d311+11fd20)/2 = 0x11E818
    as the comparison point."* Two days later his menu was *"using the timer results to set the
    initial value for TV-Type"*. It cannot tell a 2600 from a 7800: an NTSC 2600 reading of `11d312`
    was *"within the range of my 7800, so we definitely cannot use the timer for 2600/7800
    detection."* 〔mining 263219 ntsc-vs-pal-detection〕 The PAL readings are about 1.009 times the
    NTSC ones, the ratio of the two colour clocks quoted at the top of the thread (3.546894 and
    3.579545 MHz) (our arithmetic). **Cited only, not verified** — none of the readings was taken here.
  - **Keep colours in their own table, and the colour half of a PAL/NTSC conversion stays a table edit
    (our reading).** Thomas Jentzsch,
    2002, after 21 PAL→NTSC conversions in which he *"changed all the colors too"*: *"The name "Ikari
    Warriors" still raises some bad memories, because they stored graphic data and colors interleaved!.
    That really was a painful conversion."* 〔`200202/msg00056`〕 The masked-sprite rule under Sprites
    already reads colour from its own pointer (`lda (color),y`); this is a reason to keep it there.
    **Cited only, not verified.**
  - **What the hue nibble selects (NTSC): one tap of a delay line on the colour clock.** Chris Wilkson,
    2003: *"The pot adjusts the control voltage going into a digital delay line. ... The input to the
    delay line is the color clock. ... Each stage delay is the same. One of the 15 outputs is chosen by
    (COLU & $f0) to be sent to the output pin. Color 1 has no delay...it is the same as the color
    burst."* So the hues are evenly spaced and hue 1 is the burst's phase. The spacing is the pot's, not
    the chip's: the pot sets the delay for colour 15, divided by 14, and *"Most 2600s are calibrated
    initially so that color 15 and color 1 are the *same*"*, so *"a better number for the inter-color
    delay would be (360/14) instead of 24 degrees"* 〔`200312/msg00123`〕. On such a console hue 15
    repeats hue 1. **Cited only, not verified.**
- **Add colours VERTICALLY = rewrite COLUPx per scanline** (one colour horizontally). **Horizontal multi-colour is expensive** (only the fakes: PF score / Chronocolour / flicker / stacking). 〔Hugg, Davie S21〕
  - **Minimum width of a horizontal colour band = store-instruction cycles × 3 colour clocks**. An arbitrary
    colour costs ~6cy per band (about 8 bands per line is the ceiling). There is also the trick of borrowing SP
    (`txs`/`tsx`) as a 4th colour register. 〔170018 multiple-colors-per-scanline〕 `→ design.MinColorBandWidthPx/CheckColorBands`
  - **How many colours a line holds depends on how the stores are fed, and a 2013 challenge counted
    the rungs.** Andrew Davie's thread on how much colour a still picture can carry, as the
    distillation notes record it: Tjoppen's kernel of consecutive zero-page `STA`/`STX`/`STY`/`SAX`
    changes colour every 9 pixels with four colours a line, the fourth being the `SAX` AND of two
    loaded registers; omegamatrix's stack abuse reached 14 a line, but without running code from RAM
    only the hues with an odd high nibble (`$1x`, `$3x` … `$Fx`); enthusi, changing the base colours
    along the line and mixing with `SAX`, 13 a line in a 19×192 single frame; and bus stuffing, with
    cartridge hardware replacing the stored value, 18 unique background colours a line. 〔mining 217629
    programming-challenge〕 The 9 is the `STA zp` floor above (3 cycles × 3 clocks). The parent line's 8 is for an
    arbitrary colour placed per band with `LDA #`/`STA`; these counts feed the stores another way. The 13 and 14
    are more than the "3 colours (4 by borrowing SAX)" in the craft section, which counts colours held
    in registers for a line; these rewrite the registers along it (our reading). **Cited only, not
    verified** — only the distillation notes are held here, not the thread, and
    `fundamentals-audit.md` lists bus stuffing as not verified.
  - **Two separate numbers govern a PF-aligned band, and the source welded them with an `=` that is false**
    (resolved 2026-08-06; it read "multiples of 4 colour clocks (= 12px)"). Both figures are right about their
    own thing, and both are already machine-locked in this repo rather than taken from the thread:
    - **WHERE a boundary can fall: multiples of 4 colour clocks.** One playfield pixel is 4 colour clocks wide
      (40 columns × 4 = 160), so a PF-aligned edge cannot land anywhere else. Pinned at the pixel by
      `TestEveryPlayfieldColumnLandsWhereTheTableSays` — `litmus_pf_allcols` lights one column per band and all
      20 column positions are re-measured, not the leftmost-bit-of-each-register sample the older `litmus_pf` took.
    - **HOW WIDE the narrowest band can be: 3 colour clocks per CPU cycle**, so a 4-cycle `STx.w` buys 12.
      Pinned by `cmd/calibrate`'s sweep of `litmus_pos`: slope 3px per CPU cycle, R² = 1.000000.
    **The two compose, which is what the original was reaching for**: 12 is a multiple of 4, so a band written
    with a 4-cycle `STx.w` is automatically on the PF grid — 12 colour clocks is exactly 3 PF pixels. A 3-cycle
    `STA zp` buys 9 clocks, which is NOT a multiple of 4 and therefore cannot start and end on the grid.
    - **The 9 turns up as a planning number.** johnnywc, 2023, on ianajax's mock-up of *Bruce Lee*:
      *"The ladder you would want to also use PF so you don't use up a sprite, so I would change it so
      it would line up with the PF boundaries.  You could in theory change the PF color right before
      the ladder is drawn and again right after it's drawn (the minimum spacing between color changes
      is 9 pixels)."* ianajax: *"Actually most ladder in the game have "space" to the left and right of
      them."* 〔mining 347106 bruce-lee-2600-concept-mockup〕 The 9 is the `STA zp` figure above; a
      change off the PF grid can fall unseen in the space beside a ladder (our reading). **Cited only,
      not verified** — "in theory" is his, and no such kernel was built.
  - **A playfield colour stencil puts three differently coloured shapes on one line with two players.**
    SpiceWare's answer to jab (2021), who wanted three sprites in three colours: turn on the reflected
    playfield and light it only behind each shape, invert the player graphics (a 0 bit becomes 1) and
    leave the players black (in front of the playfield, the default priority), then rewrite `COLUPF`
    mid-line between the shapes. The shape is the hole the inverted player leaves, and the colour seen
    through it is the playfield's, so the colour count no longer depends on `COLUP0`/`COLUP1`.
    omegamatrix's addition: what remains is balancing the `COLUPF` writes against the beam. The price is
    the playfield's: each shape must sit on its 4-clock grid and the reflection, and colour edges are
    playfield-resolution. 〔mining 323770 three-sprites-color〕 **Cited only, not verified** — taken from
    the distillation notes; the thread copy and omegamatrix's `Test3Sprites(mod).asm` were not read
    here.
  - **Chronocolour need not rotate, and then it does not flicker.**
    Andrew Davie, 2004: Interleaved Chronocolour (ICC) shows each line's red, green and blue pixels in
    three successive frames; the variant he proposed keeps *"a single colour per line, with each
    successive line alternating between red, green, blue"*, each three lines forming an RGB triad.
    *"The memory requirements are 1/3 of the original interleaved chronocolor(TM) technique, but this
    technique is a lot less flickery.  Actually, it doesn't flicker at all."* 〔`200408/msg00065`〕
    For the interlaced two-field form he proposed next, Davie claimed *"improved perceived resolution
    (we're doubling vertical resolution with the interlacing anyway)"*; Thomas Jentzsch, replying to
    that line: *"Yes, but IMO you are also reducing the effective vertical resolution by combining 3
    lines."* 〔`200408/msg00070`〕 What Davie then posted as ROMs was an interlaced two-field version,
    which he said uses 2/3 of ICC's ROM for the image 〔`200408/msg00071`, `msg00083`〕. Jentzsch, on an AV-modded PAL
    console, preferred ICC — *"much better colors (more brilliant, higher saturation) and a higher
    vertical resolution (the SpastiColour version looks quite "striped", the black lines are too
    obvious)"* 〔`200408/msg00076`〕 — and so did Darrell Spice 〔`200408/msg00081`〕; Lee Fastenau called it
    *"a valid, flicker-free(ish) alternative to ICC"* 〔`200408/msg00082`〕; one PAL TV showed both in
    black and white 〔`200408/msg00084`〕. Davie: *"I think the preference is for ICC;  but in a crunch a
    very acceptable result can be achieved by the new method."* 〔`200408/msg00083`〕 So in the parent
    line Chronocolour is a fake of horizontal colour but not necessarily a flickering one (our reading).
    **Cited only, not verified** — the single-field triad was described, not posted as a ROM, and
    nothing here was run.
  - **A Chronocolour picture is made before the converter: split the colours, then dither each to one
    bit.** Davie, in the same 2004 post: ICC *"actually managed to look OK by preprocessing of the
    image -- separating an image into separate colour planes and then (and this is the REALLY important
    bit) colour-reducing these planes to a single bit per pixel WITH DITHERING."* 〔`200408/msg00065`〕
    In 2003 Adam Thornton asked for a tool that *"resizes it to 48x128, does the splitting into three
    color channels, and then runs it through banker"* (Davie's converter); Davie: *"Generally the
    process of producing good Interleaved Chronocolour images is an iterative one - though I do get
    excellent efforts first-time, these days.  Have good contrast seems to be the key."*
    〔`200303/msg00090`, `msg00092`〕 Thornton then did the steps by hand — sized, split, *"dither the
    channels down to monochrome"*, Banker on each — and got illegal instructions or *"an image that
    bears no obvious relation to what I wanted"* 〔`200303/msg00093`〕; no answer to that is in the thread
    as held here. **Cited only, not verified.**
  - **Or tell the states apart without colour.** Andrew Davie's 1998 *PUSH* demo, a Sokoban-like
    puzzle drawn wholly in the playfield, used his four-colour playfield (*"I CANNOT choose any colours
    I want"*). Eckhard Stolberg, on a PAL console: *"the light green and the dark purple are very hard to
    distinguish from the normal green and purple"* and *"For an action type of game it would be OK, but
    if I'm trying to concentrate on a level in a puzzle game, the flicker would annoy me quite a bit."*
    His alternative: *"Maybe you could use animated monochrome playfield graphics for this game. Use
    checkered blocks for the walls, full blocks for the boxes, blocks, that flip left and right for
    empty goals and blocks that flip up and down for filled goals."* 〔`199805/msg00214`,
    `199805/msg00216`〕 Four states in one colour, two of them told apart by the direction of their
    animation (our reading). It stayed a suggestion: Davie, a day or two later, *"Reaction to
    PUSH has convinced me to abandon that particular project."* 〔`199805/msg00236`〕 **Cited only, not
    verified** — never built, there or here.
- **There is no "one correct RGB"**: Stella generates the palette from YIQ dynamically, so the same register value differs by a dozen up to 0x20 between emulators and settings.
  For us the running table `internal/ingest/palette_stella.go` is authoritative (100% match against Stella). 〔rgb-color-values, 118495〕
- **hue ↔ colour map**: hue1 = yellow / hue4 = red / hue8 = blue / hue12 = green (hue15 ≈ hue1). hue1 is the standard choice for yellow. 〔132561〕
- **The higher the luminance the lower the saturation — it washes out toward white** (bright blue in particular stops being identifiable) → **place colours you want to read as vivid at mid-to-low luminance**. Saturation and luminance trade off. 〔132561〕 `→ design.Hue/Luminance/WashoutRisk, HueName, GradientSameHue, SameLuminance`
  - **Luminance can fade a hue to white, never to black.** Nick Bensema, 1997: *"The truth about Atari
    hue-luminance video is that the luminance controls saturation, not brilliance. $20 makes a deep
    red, which could easily fade to white by incrementing to $2E and shifting to $0E. But you cannot
    fade to black from it."* 〔`199703/msg00183`〕 The running table agrees: in
    `internal/ingest/palette_stella.go` the darkest code of each of hues 1–15 has a brightest channel
    between 42 and 139 of 255 (`$20` is RGB(106, 28, 7)), while `$00` is RGB(6, 6, 6) — computed from
    that table 2026-09-30. So a fade that has to reach black ends with a jump from the hue's darkest
    code to `$00`, unless it is grey all the way (our reading). The same thread's PAL question, Glenn
    Saunders: *"there are no orangy hues in PAL, as these tend to come out "grey""*
    〔`199703/msg00176`〕 — is measured in `visual-ceiling.md` (four of sixteen PAL hues are one grey).
- **The atom of the colour data model is "colour per scanline" = `colorPerRow[]`**: holding an array of scanline index → COLUPx value instead of a single `color` expresses vertical multi-colour (the cheapest multi-colour) directly. TIA Studio's M1 design decision converged on this too. 〔research w4 / `tools/research-w4-m1-open-questions.md`〕
  - **A colour table takes a decision out of the kernel, not only cycles.** nukey-shay, 2016, to
    just-jeff, whose kernel chose a sprite colour mid-line: *"That is burning unnecessary cycle time by
    having to decide which color to use mid-scanline. Without careful timing, the color data and/or
    sprite data will be updated too late...leading to shearing or miscolored lines. It's probably
    better to use a color table and update it every line as part of your sprite drawing routine."* On
    keeping a RAM byte of colour for every line: *"This is not necessary, and wasteful of resources.
    It's far easier to change a single pointer to a different color table (i.e. pattern) whenever you
    need to. The point I was making is that your kernel should not be deciding to do something twice
    ("Are we drawing the character?" followed by "Is it time to change colors?") Using color tables,
    the second question is eliminated."* 〔mining 254832 how-to-make-a-multi-color-sprite〕 The choice
    of colours moves out of the kernel into which table the pointer names. **Cited only, not
    verified.**
- **Background "shimmer / noise texture" is just streaming bits of the random seed into `COLUBK` every scanline (no dedicated RAM)**: water shimmer, sandstorm, twinkling stars — copy bits of the LFSR/randomSeed you already run into `COLUBK` per band and get them at **almost zero cost**. 〔Fishing Derby `.colorWaterShimmer` = a water effect that streams randomSeed bits into per-line COLUBK〕
  - **The playfield version of the same noise is NOT near-zero cost.** Yars' Revenge's neutral zone is
    random-looking — the original draws its own code as data — and batari's LFSR reproduction does four
    things on every scanline — read the data, write PF, write the colour, step the LFSR — with the PF
    write exactly timed; in the batari Basic standard kernel he was answering for, that costs the whole
    playfield (a limit of that kernel, not a price the TIA sets). The zone is also shown only
    every other frame, alternating with Qotile's shield, so it needs a second, PF-less kernel for the
    other frames. The `COLUBK` stream above is the cheap end of this. **Cited only, not verified** —
    his kernel (`ion.zip`) was not fetched. 〔mining 120680 yars-ion-field〕
  - **The original's way costs no graphics bytes: the program is the pattern.** A second thread asks
    how to recreate the zone, and raindog's recipe (2010, as the distillation notes record it, which
    also call it the original game's method): point at any stretch of code in the ROM and, on each of
    about 32 lines, read four bytes into `PF0`, `PF1`, `PF2` and `COLUPF`, reflected or repeated,
    whichever looks better. The code bytes are the random pattern, so there is no table. wickeycolumbus, in reply: the result is horizontal
    bands, since a PF pixel is 4 clocks wide and a line gets one `COLUPF` unless cycles are spare.
    〔mining 166082 yars-safety-zone〕 **Cited only, not verified** — only the distillation notes are
    held here, not the thread.
  - **Background bands can carry motion when nothing else on screen moves.** Verdant, 2026, on
    vacuous_b's *Onward*: *"you could change the background colour in alternating bands which scroll
    down the screen to keep the illusion of vertical movement going (and as a way to gauge the
    player's speed) when there are no horizontal barriers on the screen."* vacuous_b: *"I'll have to
    think of a way to maintain the feeling of speed when most or all of the other objects are off the
    screen."* Later the same day: *"I tried to use the bg color to make a paralax effect and give the
    illusion of constant movement of some kind. I kind of like it. I'll leave it in for testing and
    see how it feels."* 〔mining 389551 new-game-late-beta-onward-2026〕 Here `COLUBK` carries motion
    rather than texture, and no object is spent on it (our reading). **Cited only, not verified** —
    his screen recording was not watched here.
- **Take the hue away before calling a picture readable — a game that honours the colour/B&W switch
  shows what is left.** The switch only sets `SWCHB` D3 (`litmus_swchb`; the engine flips it with the
  `"color"` panel event in `internal/emu/emu.go`); what B&W mode looks like is whatever the game then
  writes to its colour registers. Glenn Saunders on Seawolf 2, 2004: *"You have to be careful how you
  alter the color registers for B&W. In B&W mode the bottom-most ship is almost invisible if not for
  the stripe and the top tips, and the top ship's tips disappear. It's the whole contrast issue."*
  〔`200401/msg00048`〕 A design separated by hue alone passes every colour check and fails
  here. It is the cheapest look at the quantity `pkg/design/color.go` says is *not measured anywhere in
  this tree* — how small a luminance difference still reads. **Cited only, not verified**: nothing here
  has rendered a design with its hue removed.
- **Attract-mode colour cycling and the B&W switch can share one store path, with no branch.** Nick
  Bensema, 1997: *"Most games use an EOR or ADC on every write to color registers against a memory
  location in RAM which holds zero when there's no attract mode"*, naming Combat, Defender and Pitfall.
  In Pitfall, *"every color register went through something like this"*: `LDA ColorToGet` /
  `EOR $88` / `AND $87` / `STA ColorRegister`. Outside attract mode `$88` *"contained a zero, which
  meant no bits were changed"*; `$87` held *"$FF in color mode, or $0F in B&W mode"*, so in B&W *"all
  the bits that controlled hue were stripped, leaving a grayscale version of the original color"*
  〔`199707/msg00023`〕. Both modes are data, so the store costs the same whatever the mode, and the
  B&W result is the hue-free picture the rule above asks to look at. nukey-shay gave the same recipe on
  AtariAge in 2007, with the updated timer as the `EOR` operand: the table colour, `EOR` the timer, then
  `AND` by the B&W switch state (`AND #$FF` colour; `AND #$0F` for B&W is our reading, since the note's
  quote breaks off at *"AND #$0F ="*) 〔mining 108824
  code-for-screen-color-changing; only the distillation notes are held here〕. Combat's attract colour cycle is
  in the Minimal-UI rule below. **Cited only, not verified** — Bensema gives the code as *"something like
  this"* (so perhaps recalled rather than copied — our guess), and nothing here builds it.
- **A whole screen in ONE hue is a style, and can be offered as an option.** The craft section has the
  single-object case (8 px monochrome → spend it on the silhouette); this is the screen-wide one. One hue
  × the 8 luminance steps above is the palette — bladejunker: *"Technically an eight color gradient"* —
  and SpiceWare: *"While Space Rocks defaults to multicolor, you can select any chroma value for a
  monochrome palette."* The claimed advantage is budget: *"if the color values are uniform across the
  scanlines it falls in line with the limits of those graphic objects and their color output limit."*
  **Cited only, not verified** — the cycles freed were not counted. 〔mining 317744 green-monochrome〕
- **The ball is not a colour of its own — it is drawn in `COLUPF`.** Outside SCORE mode, on a line where
  the playfield is yellow, the ball is yellow (in SCORE mode the engine paints the PF halves in
  `COLUP0`/`COLUP1` and leaves the ball in `COLUPF` — `Gopher2600/hardware/tia/video/video.go`). atari2600land, 2021, wanting a letter drawn with the ball: *"can I make it
  another color or does it have to be yellow since the COLUPF is yellow when I'm drawing the sides of the
  wall?"* — and a post later, *"It must be yellow."* 〔mining 319062 x-vs-o〕 Budget the ball as **one
  more shape in the playfield's colour, not one more colour**. The ink is measured here:
  `internal/emu/respxphase_test.go` reads the ball's pixels as `COLUPF`'s colour — in the engine, with
  the playfield empty and SCORE off (`litmus_respx_phase.asm`).
  - **So the ball spent on a picture costs a playfield colour there, and buys a second colour on a
    player's line.** littaum, in a 2022 thread asking why games do not use the ball and missiles for
    extra colour: *"In addition to CPU cycles, the ball also takes the color of whatever the playfield
    color is (there's some trickery that can be done when playfield is in score mode, but it's not
    very clean). Halo 2600 uses the ball for the Master Chief face mask, at the cost of having the
    boundaries be the same color"*; ecernosoft, later in the thread, that the ball still has to be
    drawn, *"which takes more CPU cycles"*. 〔mining 337214 using-ballmissile-sprites-for-added-color〕
    ZeroPage Homebrew, 2023, on *Meooow*: *"Great use of two colours on the same line using the
    ball."* 〔mining 352320 meooow-released-game-for-atari-2600〕 A player is one colour on a line; the
    ball adds `COLUPF`'s, and anything else drawn in `COLUPF` on those lines takes the same colour
    (our reading). **Cited only, not verified** — neither screen was looked at here.
    - **The ball as the eye.** gauauu, 2018, asked how to liven up the player sprite of *Robo-Ninja
      Climb*. LS_Dracon: *"Do you have spare cycles in the kernel to enable Ball? I think you can use it
      for an extra color for the eyes."* gauauu: *"I've got time for either the ball or multiple
      colors. I'll give the ball a try."* 〔mining 281341 sprite-graphics-adviceassistance〕 The same
      kernel time buys either the ball's colour or colour changes down the player (our reading).
      **Cited only, not verified** — LS_Dracon's mock-up was not looked at here.
- **Black is a colour you need, not the absence of one.** Aloan (2015) asked why the palette has a black
  when the beam can simply be off. seagtgruff: blanking cannot draw fine pixels — *"It takes a minimum
  of 3 CPU cycles to turn blanking on or off"*, so *"the smallest "pixel" you can draw this way (by
  turning blanking on, then immediately turning it back off) is 9 color clocks wide"*, it can start
  *"only on every third color clock"*, and *"you can't move anything across it"*, because the beam draws
  nothing while blanked. Thomas Jentzsch: *"Sometimes you display black objects over a colored
  background. There you need black."* SpiceWare: switching the signal off gives *"a black that's darker
  than black"*, visible on a set turned up too bright. 〔mining 244898
  to-be-black-or-not-to-be-black-color-question〕 The 9 is the same arithmetic as the band floor at the
  top of this section (`STA zp` = 3 cycles × 3 clocks). **Cited only, not verified.** Drawing with
  blanking presumes a store to `VBLANK` takes effect within the line: the distillation notes of 〔mining
  192183〕 record a one-colour-clock delay, as does the delay list in the WSYNC rule below, and the
  bundled Gopher2600 schedules the change one colour clock late. The one colour clock has not been
  measured here.

## Sprites (P0/P1)
- 8 dots wide, one register (GRP 8-bit, MSB = leftmost). Width via NUSIZ 1x/2x/4x. 〔2k6specs, Davie S21〕
- **Horizontal position = two stages**: coarse ÷15 (5cy loop) → fine HMOVE. Granularity 3px per CPU cycle (matches litmus). 〔Davie S22〕 `→ design.PositionSplit/CoarseIterations` `→ design.HMoveReachable`
  - **The coarse-positioning line IS the kernel's tightest interval**: the `sta WSYNC`→`sta WSYNC` interval holds the ÷15 loop plus the fine tail (`eor` / ASL×4 / `sta`), so it **gets heavier in proportion to X**, and at maximum X the **worst case exceeds 76cy → the picture rolls** (a lucky test at small X passes; only a ∀ proof surfaces it). `prove_line_budget`'s `; @amax N` must sit on the line of the **`sta WSYNC` that OPENS the interval** to bind it (it has no effect on the `sbc` line). Ways to fit: **restrict the range of movement**, or **turn the ÷15 into a lookup table so it becomes constant-time** (tabulation is the standard density move). Constraining the loop alone still overruns while the fine tail remains. 〔density-ladder rung2 measured / prove_line_budget roll_free 2026-07-24〕 A third, from Thomas Jentzsch, once the fine and coarse values are computed outside the kernel: *"two different codes for positioning objects on the left and right, giving you more free cycles for other stuff"* 〔`200410/msg00016`〕 — the range is not narrowed, the path is split by it (**Cited only, not verified**).
  - **Positioning BETWEEN kernel bands costs lines, and a line count that varies is a frame that
    varies.** easmith (2018, *Alien Revenge*): 268 lines, 270 when firing right, 269 up-right or
    down-right. ZackAttack: the ÷15 routine *"will take 1 or 2 scanlines depending on the position of
    the object, which isn't a problem during vblank if you're using the timer, but will cause problems
    if you're using it to position something in between display kernels"*; the fix is to *"modify the
    routine to always consume 2 lines or use the timer again to make the number of lines for positioning
    all those objects constant"*, or to position in VBLANK the objects the score does not use 〔mining
    280240 alien-revenge-wip-but-help-needed〕. The same defect is measured here, in a reproduction:
    pizza-boy's frame length tracked sprite X because the loop costs an extra line past X=105
    (`capability-gap-audit.md`), and `TestNoRomBreathesAcrossFrames` sweeps the corpus for it. Paying
    the worst case every time is the lodging rule's "same line count on every path" applied to one
    routine. **Cited only, not verified** for the always-two-lines routine.
  - **Coarse plus fine can set a length, not only a position.** Christopher Rydberg, 2001, asked how
    a health bar is drawn; Glenn Saunders, answering: *"You start out with an asymmetrical playfield
    routine and you set up the bits to represent the coarse health (4 pixel chunks) and then you can
    fill in the gaps (1-3 pixels if necessary) with a sprite colored the same as the playfield."* And:
    *"I would align the sprite on the PF boundary and adjust the shape to use between 0 and 3 bits
    rather than actually overlaying it, so that when the health is near zero you won't be stuck with a
    sprite that's too fat to represent 1-3."* *"You can position the missiles to draw a line
    representing the maximum health"*, a blue line above and below frames the bar, and *"As the health
    deteriorates you can adjust the color registers, maybe shifting from green to yellow to red."*
    〔`200109/msg00385`〕 Roger Williams, the same day, called it probably the best suggestion but one
    that would make his knees knock as a beginner — *"Mixing playfield and sprite graphics and lining everything up creates whole
    minefields in which bugs can live."* — adding *"I may be overstating the difficulty"* — and offered
    a bar character set for the 48-pixel score routine instead 〔`200109/msg00388`〕. Erik Mooney:
    *"If you leave out the part where you put two health bars on the same line, Glenn's solution is
    quite elegant, workable, and good as an intro to object placement and manipulation."*
    〔`200109/msg00390`〕 Saunders, to Williams: *"But if you did that you'd only have one health bar
    available per scanline."* 〔`200109/msg00391`〕 Joe Grand's *SCSIcide* bar: *"It was a fairly coarse
    counter, so I just used playfield graphics"* 〔`200110/msg00010`〕. Here the two stages are the
    playfield's 4-pixel grid and the sprite's shape, not ÷15 and `HMOVE` (our reading). **Cited only,
    not verified** — no bar has been built here.
  - **Steps of mixed sizes are one symptom of a positioning routine that is not working.** Chris Cracknell, 1998, asking
    how to move several copies of a sprite independently, on the routine he took from *SoundX*: *"I
    couldn't even get it to move ONE sprite the way I wanted it to. It would move three or four big
    chunks, then two little chunks, then pixel by pixel for awhile and repeat the process in reverse on
    the way back."* 〔`199804/msg00171`〕 The thread held here gives no diagnosis and no answer to his
    question. That the big chunks are coarse steps and the single pixels `HMOVE` is our reading.
    Earlier the same day, in another thread, Eckhard Stolberg had diagnosed a different symptom of
    Cracknell's non-`HMOVE` version — sprites pausing on the left: *"your delay loop is too short with
    low values. The STA RESP0 ends within the horizontal blanking time. In that case the sprite gets
    positioned on the left side of the screen. If you have several values, that are too low, the sprite
    stays there for several moving rounds."* 〔`199804/msg00166`〕 **Cited only, not verified** —
    nothing was reproduced here.
  - **The coarse strobes can go in the VSYNC lines.** Thomas Jentzsch, 2001: *"I'm positioning (except
    HMOVE) during VSYNC in Thrust and Jammed. Nobody reported any problems with that so far."*, adding
    that during development he *"mostly had to rely on the nearly perfect emulation"* of z26.
    〔`200109/msg00371`〕 The replies are cautions about positioning in general, not about VSYNC. John
    Saeger: *"I've never thought much about what happens during VSYNC"* — his worry was *"the dreaded
    24 cycles at the start of a scanline"* and the multi-sprite effect, which he said differs between a
    classic 2600, the rainbow model and a 7800 — *"I'd be careful about doing positioning at any time."* (his emphasis on "any")
    〔`200109/msg00373`〕 Eckhard Stolberg: *"while you can access the positioning registers at any time,
    you should be carefull with doing any too fancy stuff, or your program might not display properly on
    some of the consoles out there"* 〔`200109/msg00374`〕. `cmd/framegen` and
    `techniques/sprite-animation.md` both put positioning on lines of their own after VSYNC; this would
    move it into lines the frame spends anyway (our reading). **Cited only, not verified.**
- **Vertical position has no register at all.** SpiceWare: *"TIA's a scanline based video chip, as such
  it has no concept of Y. Therefore it's up to your program to keep track of which row of the screen is
  currently being drawn, and whether or not the sprite shows up on that row."* 〔mining 246500
  help-with-movable-objects〕 Horizontal position is a strobe plus a nibble the chip keeps; vertical
  position is only which lines the kernel writes a non-zero `GRPx` on, which is why every vertical
  technique here (`techniques/vertical-positioning.md`, `techniques/two-line-kernel.md`) is bookkeeping
  in RAM. **Cited only, not verified.**
  - **Vertical resolution has no register either — the kernel sets it.** Robert M, answering 82-ta's
    question about the standard resolution: horizontally the machine fixes it (40 PF blocks of 4
    pixels at fixed positions; sprites 8 wide, placeable at any of 160 positions), while *"The vertical
    blockyness of all graphics is a function of your kernel code. For example, Combat has a vertical
    resolution on the tank sprites of 2 scanlines. the PF blocks have a vertical size of 8 scanlines (I
    think). Single scanline graphics are possible, but not much can be going on at the same time (see
    Atlantis)"* — in Atlantis *"there is never more than 2 sprite objects in any horizontal band of the
    game screen. That is not by accident, but by design."* 〔mining 62722 couple-of-design-questions〕
    Vertical detail is bought from the kernel's line budget rather than set once. **Cited only, not
    verified** — Combat's 2 and 8 are his recollection ("I think"), not measured here.
  - **A sprite that has to cross every band can be drawn at the coarsest band's resolution.**
    tokumaru's 2010 Sonic mock-up: *"you may have noticed that Sonic has half of the vertical
    resolution, this is so that he can "play nice" with all the mini-kernels, since he can be anywhere
    in the level. Player 1 is full resolution though. Sonic uses its missile to have 9 pixels across
    instead of 8."* 〔mining 170135 sonic-the-hedgehog-on-the-2600〕 The object that can be anywhere
    pays in detail so that no mini-kernel has to make room for it, and the other keeps full resolution
    (our reading). The 9 pixels are the player-plus-missile width in the Multiplexing section. **Cited
    only, not verified** — a mock-up, not a running kernel.
  - **So a sprite that slips a line is a program bug, and the cheap fix is less state.** gradualgames,
    2018, had a missile nudged when it started on the player's line. tokumaru: *"The hardware doesn't
    care about vertical positioning, that's all done in software, so this is definitely a bug in the
    program and not a limitation of the system"*, and with 76 cycles a line, *"it looks like your code
    will take more than that when all branches are not taken"*. His advice: *"The more you think like a hardware designer, the better"*; the NES PPU
    subtracts a sprite's Y from the scanline number and, if the result is smaller than the height,
    draws that row, which *"needs less state (no need for individual counters for each sprite) and
    solves multiple problems in a single operation. The 2600 can draw sprites in a similar way"* —
    SkipDraw (`techniques/vertical-positioning.md`). nukey-shay: *"most games don't rely on separate
    counters or buffering. They'll utilize the existing scanline counter and draw the objects before or
    after their visible position/use the delay registers for auto buffering"*, and nanochess's rewrite
    in the thread had *"a logic result serving double-duty as a bitmap value"*. 〔mining 275171
    p0-on-same-scanline-as-m0-nudges-it-up-or-down-slightly〕 **Cited only, not verified.**
- **★RESxx's internal draw delay (first suspect in any position mismatch)**: the `RESxx` strobe resets the counter immediately, but **the object actually starts drawing later = player +5 / missile and ball +4 colour clocks** (if RESP0 completes at cycle 46, X ≈ 75). Measured 2026-09-03 for strobes in the visible area, 1x player (`roms/litmus/litmus_respx_phase.asm`, `internal/emu/respxphase_test.go`); AtariAge 294398 reports the same from Stella's source (`renderCounterOffset`). **When a target X is off by ~5px, suspect this first.** RESxx granularity is 3 colour clocks. 〔mining 294398, 283075, 305780, 172089, 137739, 329611, 304182〕 (this is the quantity that explains the codified `X=3N−54/55` from behind; the measurement is recorded in `docs/fundamentals-audit.md`)
- **Position formula and write window for multiple objects**: `RESxx` while the beam is visible is forbidden (the immediate reset bends the picture) = look ahead in HBLANK or on the previous line. A shared loop walks consecutive `RESP0,x`/`HMP0,x` with `DEX/BPL` (`design.shared_setxpos`, implemented). N objects cost N+1 scanlines only while every input stays inside a limit: 340965's PosWorldObjects caps X at 0..134 to guarantee it. Past the limit the next `sta WSYNC` spills onto the following line, and the code after the strobe sets the limit — 134 when the `sta WSYNC` comes right after the `rts` (308513), 89 when the next call of `shared_setxpos`'s loop comes first (measured, `docs/techniques/shared-setxpos.md`, where the loop's last object holds to 134). 340965 and 308513 are Cited only, not verified (mining notes). 〔mining 67045, 308513, 340965, 311795 (RESxx × HMOVE race = implemented in Gopher2600)〕
- **Burn one cycle to land RESP where you want it**: when coarse positioning has no NOP to spare, `sta.wx HMP0,x` (dasm `.w`/`.FORCE` forces Absolute,X = 5cy; ZP,X is 4cy) adds 1cy so the RESP0 strobe lands on **the cycle you intended**. 〔mining blog SpiceWare 12538〕
- **Masked sprite drawing = 21cy** (cheaper than DoDraw's 26cy): `lda (img),y / and (mask),y / sta GRPx / lda (color),y / sta COLUPx` = **clip the shape and update the per-row colour at the same time**. Zero-pad so different sizes share a single mask. 〔mining blog SpiceWare 10890, 339509〕
  - **DoDraw's 26 is the bottom rung of a ladder, and the rungs above it are cartridge hardware.**
    SpiceWare, 2016, one multicolour-sprite line (graphics + colour): DoDraw **26** cycles (50 left) →
    Activision's DPC **14** (`LDA DF0DATAW` / `STA GRP0` / `LDA DF1DATA` / `STA COLUP0`, 62 left) →
    DPC+ **10** (the same with immediate loads, `LDA #<DF0DATAW`, 66 left) → bus stuffing **6**
    (`STY GRP0` / `STY COLUP0`, 70 left). 〔mining 255177 so-what-is-dpc〕 **Cited only, not verified**:
    all four are his annotations, the 26 above is the same author's figure (not an independent check),
    and `fundamentals-audit.md` lists DPC+ and bus stuffing as not verified here.
  - **DoDraw is 26 on BOTH paths by construction, and that is what makes it usable in exact-time
    kernels.** SpiceWare: *"DoDraw is written so the cycles are exactly the same no matter which path
    through the code is taken, which makes it very useful for kernels that require precise timing like
    this"* — "this" being a reflected asymmetric PF whose right-hand PF2 write ends at exactly cycle 48.
    〔mining 254684 asymetrical-reflected-playfield〕 A skip-draw is not balanced by default: the one
    measured here, `roms/techniques/vertical_pos_dcp.asm`, takes **20 cycles on drawing lines and 17 on
    skipped ones** (`fundamentals-audit.md`). Balance is a property of how the routine is written, not of
    its name. **Cited only, not verified** for DoDraw's balance.
  - **What the mask costs, from someone who built one.** ZackAttack, 2018, a one-line-kernel masked
    draw: *"The mask only takes 256 bytes because it exploits the natural wrap around of the 8 bit Y
    register. Keeping everything aligned properly is critical to avoiding a 1 cycle penalty for crossing
    pages. So the net is a couple bytes of RAM, 150 bytes of ROM, and 2 cycles during the kernel."*
    〔mining 276058 some-guidance-stuck-with-masked-sprites〕 The one-cycle penalty on a page-crossing
    read is machine-locked here (`TestPageCrossPenaltyRules`); the 256, the 150 and the 2 are his.
    **Cited only, not verified.**
  - **Without a mask the zeros go into the graphic itself, and the bill moves to ROM.** Ben Larson,
    2002, drawing with `LDA (ptr),Y` / `STA GRP0`: *"this approach requires me to put 154 blank bytes
    before and after the player graphic. Because of this, it's inevitable that a page is going to be
    crossed at some point, since the whole 'graphic' is going to use up 314 bytes"* 〔`200201/msg00063`〕.
    Thomas Jentzsch: *"It's the fastest way, that's true (in my counting 8 cycles not 11-12), but you
    are wasting a lot of ROM. So you can't add nice animations etc."* (his emphasis on "lot" dropped) —
    and the main loop's 120 lines
    would fit in one page, so the crossing can be kept out of the kernel 〔`200201/msg00066`〕.
    SpiceWare's arithmetic for a 37-line image in a 192-line 1LK has the same shape: 155 + 37 + 155 =
    347 bytes, more than a page; Stay Frosty split the screen into zones with their own pointers so far
    fewer zeros were needed 〔mining 276058〕. **Cited only, not verified.**
  - **Alignment can fail the build instead of the frame.** Dionoid, 2022, splitting a game into include
    files: a `CHECK_PAGE` macro, placed after a block, compares the high byte of the last byte emitted
    with the high byte of the block's start label and, when they differ, ECHOes the block's name with
    *"crosses page"* and executes DASM's `ERR`, so the assembly stops 〔mining 345618
    breaking-up-a-code-monolith-into-modules-pieces〕. Nothing in this repository makes DASM itself
    fail: searched 2026-09-30 for `ECHO`, `ERR` and `CHECK_PAGE` over the tree outside the vendored
    engines — one hit, a Go format string in `internal/emu/cbroll_test.go`. A crossing inside a timed
    loop is otherwise found when the image runs or is proved; this finds it when the source assembles.
    **Cited only, not verified** — the macro has not been assembled here.
- **div15's fine-movement range is implementation-dependent**: a naive div15 gives **−6..8px**; symmetrising with `eor #15` + `adc #((8+1)<<4)` gives **−7..8px**. Origin = Decuir / Video Olympics. Separate from HMOVE's raw hardware range (−8..+7) = it is a property of the routine. 〔mining 286698〕 (needs litmus backing)
  - **In Video Olympics the remainder reaches the high nibble through a `BRK` call.** clausb, on a
    6507 crossword that left `RTI` out: *"RTI is used with BRK for an often-called subroutine, saving
    2 bytes per call. Joe Decuir did this in Video Olympics for a nybble-swapper subroutine."*
    〔mining 193203 6507-opcodes-crossword-puzzle〕 One of its three calls is in the positioning
    routine at `$F445`: `SBC #$0F` in a loop counting Y, `EOR #$FF`, `SBC #$06`, `BRK` (the swap),
    `STY WSYNC`, a `DEY` wait, `STA RESP0,X`, `STA HMP0,X` (our reading of the ROM's bytes). The
    saving is 1 byte a call, not 2 — `integration-density-playbook.md` has the BRK-as-call measurement
    and the same three call sites. **Not verified** — the routine was not run here.
- **Moving an object relatively inside the kernel goes a few pixels at a time, and a paddle does not
  wait.** Thomas Jentzsch, 2025, on *StalaX*: *"Since the movement of the missiles is relative during
  the kernel, they can only move +/-7 pixel per missile. But the paddle can move much faster. So when
  the paddle is moved fast, the missiles will either lag behind (left diff = B) or they will be
  disabled until they have caught up (left diff = A)."* The day before: *"This would work easily with
  a joystick, but a paddle can move at infinite speed."* 〔mining 385470
  stalax-was-how-to-process-stalactites〕 The mismatch is not hidden; it is put on the left difficulty
  switch, so the player chooses lag or a gap (our reading). It may reach the rules too — his later
  note: *"If falling drops sit on the ground for a moment, this might have little impact. Because with
  the instant paddle movement, the player can "jump" over them. So maybe that won't work as
  expected."* His ±7 is
  his figure, beside the −8..+7 above. **Cited only, not verified** — his test ROM was not run.
- **For early-HMOVE (HMOVE before WSYNC), the "do not move" value is HMPx $80 (= 8), not $00**: with $00, an object spanning the same scanline drifts 8px. The idiom positions with a dedicated 15px × 11 kernel. 〔mining 169471〕 (needs litmus)
- **Striking HMOVE at cycle 73–74 suppresses the left-edge comb (black line)**: the known Cosmic Ark-family trick. 〔mining 165428, 183219, 319456 "HMOVE Shuffle"〕 **Measured 2026-09-03** — `roms/litmus/litmus_hmove_side.asm` band D is now graded: a late HMOVE adds **8 to the nibble** (HMP0 = $10 delivers nine clocks left, not one) and paints **no comb**, while a strobe right after WSYNC paints the comb with every HMxx at zero. `→ internal/emu/hmoveside_test.go`
  - **The same move, counted from the start of the instruction.** polygonpizza, 2015, to someone
    positioning a six-digit score: *"If you need to avoid black bars on the left of the screen, you can
    write #$90 into HMP1 and call HMOVE starting on cycle 70 or 71."* 〔mining 243197
    position-player-graphics-for-6-digit-score〕 Both numbers fit band D (our reading): `$90` is right
    7, and the late strobe's +8 makes it left 1 — what `$10`, his instruction for a normal HMOVE in the
    same post, gives — and a three-cycle `sta HMOVE` started on 70 or 71 completes on 73 or 74, where
    band D counts it. **Cited only, not verified** — a strobe started on 70 has not been run.
  - **The price: an emulating console may not honour it.** Karl G, 2020, from making a version of his
    game for the Atari Flashback Portable — a machine that runs games by emulation, which nukey-shay in
    the same thread calls *"a flaky emulator"* — says kernels that use an early HMOVE to hide the HMOVE
    bar break there; all three he names come from one kernel library for homebrew — a
    score routine (*"to avoid repeated digits in the score"* it needs a fix, the early HMOVE being
    something *"the AFP doesn't support"*), a 96-pixel title-screen kernel (*"will not work, since it
    relies on an early HMOVE"*), and a multisprite kernel, which *"could be modified to use standard
    HMOVEs"* 〔mining 300586 collection-of-fixes-and-best-practices-for-flashback-portable〕. Band D
    above is graded in the bundled engine only. **Cited only, not verified.**
  - **The opposite use: HMOVE on EVERY line turns the comb from a blemish into a border.** Dennis Debro,
    2002: *"I am also doing HMOVE on each scanline so I remove the comb effect"* 〔`200208/msg00235`〕.
    Glenn Saunders, 2001, answering Pete Holland's question of how Activision got the black border
    round its screens: *"They often used a wide-mode ball and set it to the side, or did an HMOVE on every
    scanline"* — and he disliked it, *"as it wastes potentially valuable screen resolution"*
    〔`200106/msg00092`〕. The per-line comb itself is band D's measurement above; **that
    Activision titles do this is cited only, not verified**.
- **Indirect-jump positioning: `JMP (ptr)` into a table of hardcoded positioning lines replaces the delay loop.** Each extra variant defers the RESPx strobe by 5 CPU cycles = **15 colour clocks**; the HMPx nibble loaded before the dispatch and applied by the HMOVE after it fills in between. **Nine variants reach 128 contiguous positions** — measured 136 — and they are contiguous *only* because the fine range (16 values, −7..+8) is at least the coarse step (15): the reachable intervals [15k−7, 15k+8] and [15(k+1)−7, 15(k+1)+8] meet exactly at their endpoint, so one fewer fine value puts holes in the set. Pays ROM for cycles, plus ≥ 2 bytes of RAM per object (pointer + HMOVE value). 〔Stella list, `star fire - return of the starfield ?!?`, 2002-07: Erik Mooney 200207/msg00330 + msg00334 for the mechanism, Manuel Polik 200207/msg00332 for the nine-parts arithmetic〕 `→ roms/litmus/litmus_jmpind_pos.asm` / `internal/emu/jmpindpos_test.go` (5 gradings, 2 negative controls). **Provenance correction:** this file previously credited the idea to Omegamatrix. That attribution is both later and a *different* shape — Omegamatrix folds the jump index into the HMPx low nibble, Erik's 2002 form dispatches through a plain pointer computed off-screen and keeps the nibble for movement.
  - **Erik Mooney's first form, in 2001, threaded the strobe through the playfield writes.** Eleven kernels `Kernel0`…`Kernel10`, each placing `STA RESP0` one step later among the line's
    six playfield writes, entered by `LDY SpritePosition` / `LDA BranchTable,Y` / `STA KernelBranch` /
    `JMP (KernelBranch)`, with the strobe positions *"equal to or less than 15 pixels"* apart and HMOVE
    finishing the job: *"So you've positioned your object by using lots and lots of ROM, but only 28
    cycles all told (including the last HMOVE).  Much less than using a full 76 cycle scanline."*
    〔`200108/msg00348`〕 Thomas Jentzsch: *"you will either need more than 11 routines for that or need
    some extra cycles, because the writes to RESP0 will not automatically fit into the other code"*, and
    *"It sounds possible to me, but not a simple task, because of the synchronization with the
    playfield writes. I haven't heard of that solution before"* 〔`200108/msg00374`〕. Mooney: more routines
    cost *"ROM, not any more cycles"*, and he asked whether an indirect jump across a page boundary
    needs care 〔`200108/msg00380`〕. Paul Slocum planned to try it under an asymmetric playfield in
    reflect mode, where *"the second write of PF2 has to land right on the exact cycle"*
    〔`200108/msg00353`〕. The page trap measured here for `JMP (ptr)` is a pointer stored at `$xxFF`,
    whose high byte is fetched from `$xx00` (`litmus_6502`, `fundamentals-audit.md`). The 28 is his
    count, not ours. **Cited only, not verified.**
- **A slide of `CMP #$C9` delays in single cycles, where indirect-jump positioning steps in fives.**
  kylearan: *"tricks like having several "cmp #$c9" instructions to use as a slide to jump into for
  cycle-exact kernel positioning, are also very similar to some obfuscation techniques, and in fact
  often confuse disassemblers in a similar way"* 〔mining 247638 obfuscated-software〕. How it works
  (arithmetic from the opcode table, not run here): in `C9 C9 C9 C9 C9 C5 EA`, every entry decodes as
  two-cycle `CMP #imm`s followed, by parity, either by `CMP $EA` (3 cycles) or by `NOP` (2), so entering
  at byte 0..6 costs 8, 7, 6, 5, 4, 3, 2 cycles. Reached with `JMP (ptr)`, that is a delay adjustable by
  one cycle — 3 colour clocks of `RESPx` placement. The flags are clobbered. It is probably what
  `techniques/multicolor48.md` calls a "clockslide" (our reading; the source does not use the word).
  The disassembler warning applies to `cmd/dissect` too: the
  bytes are code at every offset. **Not verified** — no ROM here uses one.
  - **A table read as code misleads the disassembler past the table.** nukey-shay, in a 2005–06
    thread, to someone whose two 4K halves of an 8K ROM would not fit back together: *"that's not the
    problem. The problem is that the 2 disassemblies were made without a configuration file that had the DATA SEPARATED FROM
    THE CODE correctly. If data tables are misinterpreted to be program lines, the values in those
    tables will be treated as code...and even worse, data misinterpreted as BRANCHES will cause the
    disassembler to misinter[pret more]"* — the bracket is how the distillation notes end the quote
    〔mining 66892 how-to-disassembler-a-8k-bin; only the notes are held here, not the thread〕. That a
    misread branch carries the error to the code it points at is our reading of that sentence. The `LAX`
    rule further down names the machinery here that separates code from data by following flow
    (`Gopher2600/disassembly`, `cmd/dissect`). **Cited only, not verified.**
- **48px** = NUSIZ $03 (3 copies) + P1 shifted 8px right + VDEL double-buffering to swap GRP with a time offset. Reuse score/bitmap48. 〔48px-positioning〕 `→ pkg/sprite.SplitWide/NUSIZ / design.MaxChars(Text48px)`
  - **When a picture's proportions do not fit the pixel, the technique and the layout are derived
    backwards from it.** Thomas Jentzsch, 2026, on drawing the American flag: *"For a reasonable star,
    you need at least 5 pixel. The distance between the stars should be 5 pixel then too. There are up
    to 6 stars per line. And these cannot be drawn at this distance, neither with repeated players,
    nor with RESPed ones. So what is left, is a simple 48 pixel display. Then you have only a 3 pixel
    gap between the stars. This results into a (pretty crowded) ~50 pixel wide starfield. The start
    field should be 40% of the total width, which means the flag should be ~50 / 40% = ~125 pixel
    wide. Which means you have to cover ~18 pixel left and right, e.g. using a black playfield. But
    then you have no 2nd background color in the starfield lines. So you have to change colors mid
    screen."* 〔mining 387341 great-britain-in-1k; "start field" is his spelling〕 Each step is forced
    by the one before: the star spacing removes two placement methods, the 48px display sets the width,
    the width forces a cover at the edges, and the cover takes the second background colour. A 5-pixel
    star and a 3-pixel gap are one 8-pixel copy (our arithmetic). **Cited only, not verified** — his
    ROM was not run.
- **Do not fix the picture first and then assign objects.** Order = colour budget → assignment table → negotiate any shortfall via "share colours / double up objects / change the layout".
  - **Finished games show the budget in their layout.** Magovinna, 2024, asked by LatchKeyKid why
    *Coarse Blade*'s tombstones sit below the characters rather than level with or over them: *"I had
    to move the tombstones underneath to remove the flickering and extra lines and also because of CPU
    cycle limitations"* 〔mining 345678 new-atari-2600-coarse-blade〕 (the enemies' two animation frames,
    in the same thread, are for memory — a different budget). nukey-shay, 2017, to doctorclu, who wanted
    to add a background to Joust: *"there is a reason the platforms are flat symmetrical with no
    detail...the program is (potentially) repositioning objects while displaying them. No cycle time to
    waste altering their appearance, just set it and forget it, and turn them off after HMOVE. Color
    shading the background might be barely possible with the existing kernel, tho."* 〔mining 258859
    bankswitching-playfields〕 Read a plain element of a shipped screen as a possible cost before
    reading it as a style. **Cited only, not verified** — neither says how many cycles were missing.
  - **Detail goes where the action is not.** CDS Games, 2018, on why *Double Dragon*'s backgrounds look
    so rich: *"The backgrounds are drawn in 3 main sections"* — the top windows and the garage-door
    band each *"two sprites, playfield, and background colors"*, the sidewalk band background colours
    only — *"Since the action doesn't take place up that high, the program can spend the available
    cycles just drawing each section in great detail. Then below the sidewalk where the actual game
    play takes place, the background switches to a single invariant color."* And, in the same post,
    *"It's just frustrating that the player characters looked shoddy in comparison."* 〔mining 278994
    the-backgrounds-in-double-dragon〕 The single colour is spent on the band that has the most else to
    draw (our reading). **Cited only, not verified** — his reading of the screen; the debug-colour
    screenshots in the thread were not looked at here.
  - **Static things go to the playfield, so the players are free for what moves.** splendidnut, 2023,
    on a Ghosts 'n Goblins-like game: *"I would probably do the gravestones as Playfield (and not worry
    about drawing the fence). This leaves the both player objects available for the player and
    enemies."* 〔mining 349459 is-a-game-like-ghosts-n-goblins-possible-on-the-2600〕 The Pizza Boy
    dissection in the rules of thumb below is the same split in a finished game (buildings = static PF,
    moving things = sprites). The assignment table can start from one question per element: does it
    move? **Cited only, not verified** — the copy here holds six of the thread's eight posts.
  - **Elements not yet added decide the resolution of the ones already drawn.** kamaleon70, porting
    *Destroyer*, 2025-12-21: *"so far I am using 1 line kernel for both the destroyer sprite and
    submarines (so better resolution), but I will see if I can keep it or I have to switch to 2 lines
    kernel (with lower res of the submarines) when adding depth charges (they will be two at the same
    time on screen using missile)."* Twelve days later, with the charges in: *"I managed to use
    one-line kernel for the destroyer and submarines so they can be decently defined (both destroyer
    and enemies are two players each so the resolution can be up about 16x16 for each, then I double
    the size so at the end they are ~ 32x16 but horizontally stretched)"* — and the same post still
    lists removing *"the "flickering" in the charges movements while falling down"*. 〔mining 386546
    destroyer-atari-2600-update-released-v-13〕 Budget the elements not yet drawn before promising a
    line resolution to the ones that are. **Cited only, not verified** — the ROM was not run here.
  - **The picture also grows as the kernel gets cheaper.** Dionoid, 2024, on his first game: *"For
    my first game, I think I've rewritten my kernel more than 20 times, just because every week I
    learned a new programming trick which saved some CPU cycles, and then I wanted to use those free
    cycles to add a new display feature."* In the same post: *"76 is the magic number here, which is
    the number of CPU cycles you can use on each scanline."* 〔mining 355941 am-i-too-old-for-asm〕
    He wanted the cycles each trick freed for a new display feature, so the picture was not settled
    before the kernel was (our reading) — one more reason for the order above. **Cited only, not verified** — his count is
    his own ("I think").
  - **A size drawn first can be changed by the code.** atari2600land, 2024, on *Up the Pineapples*:
    *"Unfortunately, I had to embiggen the ant due to coding stuff. It was hard to face this
    realization, but I needed to do it if I wanted to work on it any more. I think I'll worry about
    scanline adjustments more towards the end of coding this because things are constantly changing
    now."* He did come back to the line count: 258 (*"I know, it should be 262, so I think I'll
    wait until I'm done with everything else to bump it up to 262."*), then *"+ made scanline steady
    at 262."*, and in the last post held here: *"OK, I give up. This game is dead. If anyone wants to
    figure out how to make the scanline steady 262 when you press reset and are invisible, have at
    it."* — and in the same post, *"I'm giving up 2600. It's too hard."* 〔mining 373848
    up-the-pineapples〕 He does not say which code forced the larger ant. **Cited only, not verified**
    — the copy here holds 24 of the thread's 31 posts, so it may not be where the thread ends.
  - **Or change the subject.** atari2600land, 2022, on turning a hamburger game into one about potato
    chips: *"I needed to make the burger be one color since the missile would change colors. That and
    I decided to make the game easier for me to program. So I changed the burger into a potato chip."*
    〔mining 339833 potato-chips-en-masse〕 A missile takes its player's `COLUPx`
    (`techniques/missiles-bullets.md`), so a player recoloured line by line recolours its missile on
    those lines (our reading); a one-colour subject removed that, and he changed what the picture was
    rather than the objects or the layout. **Cited only, not verified.**
  - **Or pick the first game for the objects the machine has.** kisrael, on why his first game was
    *Joust Pong*: *"I chose it because I knew from prior experience it was pretty easy, and within the
    2600's 2-PLAYERS-AND-A-BALL capability."* cybergoth: *"Same here for choosing Gunfight as my first
    project."* 〔mining 65309 newbie-seeks-assistance; only the distillation notes are held here, not
    the thread〕 The order above works inside one screen; this is the same order one level up, at the
    choice of game, where a subject that fits two players and a ball has nothing to multiplex (our
    reading). **Cited only, not verified.**
  - **One object's job can take another object away.** Piero Cavina, 1998, assigning the objects for
    Ruffin Bailey's game: *"Sprite0 = player / Sprite1, reused, with varius NUSIZes = prizes (simple
    rectangular blocks, obviously) / Missile0 = first dot / Ball = second dot / Missile1 = probably
    useless in the kernel, because it's influenced by the prizes"* 〔`199803/msg00086`; one line per
    object in the post〕. A missile follows its player's NUSIZ copies (`techniques/sprite-placement.md`,
    rule 4) and draws in its player's `COLUPx` (`techniques/missiles-bullets.md`), which is presumably
    the influence (our reading). So an assignment table fills `Missile1` only after deciding what
    `Sprite1` does. **Cited only, not verified.**
- missile/ball = lines, edges, vertical frames; player = area via double width / multiple copies / 4x. Build one apparent shape by stacking several objects.
  - **The playfield can be the body and an object the outline.** Sohl, on a screen of his *Immunity*,
    where he *"used the playfield bitmap and color to fill in part of my viruses in a non-play portion
    of one screen to save a player graphic"*: *"the white virus bodies are playfield and square, but
    the overlaid spike proteins sort of hide that and make them seem more rounded, but I can more easily
    do this here because in this screen, the upper three viruses don't move"* 〔mining 337214
    using-ballmissile-sprites-for-added-color〕. The squareness is the 4-clock PF grid; the object on
    top breaks its corners. It suits shapes that stay put, since the body cannot move by less than a PF
    pixel (our reading). **Cited only, not verified** — the screenshot was not looked at here.
  - **A player can be a line too — here on one viewer's reading.** marcoj, on lisboa2k6's
    *Bloxudoku*: *"I like the way the squares are evenly divided using the sprites and missiles in
    single width."* bigmessowires, in the next post, read the board another way: *"I haven't looked at
    the disassembly, but I think the game board is playfield blocks? Then you're switching the
    background color between black, dark gray, and light gray at fixed positions, and using an
    asynchronous playfield to draw the wooden pieces at the desired spots?"* 〔mining 345854
    atari-2600-homebrew-wip-bloxudoku-v01-ntscpal-4k〕 The author's answer is not in the copy held
    here (10 of 14 posts). **Cited only, not verified** — the ROM was not looked at here.
- **A single irregular shape wider than 8px = "shape ONE player with a per-scanline NUSIZ + HMOVE table" — don't fall back on flicker**: keep GRP small and switch NUSIZ (size 1/2/4/8, copy count) and HMOVE on every scanline, and one player "stretches" into an irregular shape ~40 colour clocks wide (fish / shark / ship / wide creature). Accept a single colour. No extra object, no flicker. Confirmed on a live run. 〔Fishing Derby (David Crane / Debro disassembly) SharkTraveling*NUSIZValues = a shark made of per-line NUSIZ + HMOVE; ~40-clock width confirmed by running build/fishing_derby.bin〕 `→ casebook.md "large irregular shapes"`
- **A 1px line at an arbitrary slope = missile/ball + fractional-HMOVE accumulation (Bresenham in HMOVE)**: take an M/BL drawn vertically, `adc` the slope held as integer + fraction on every scanline, and on carry apply a ±1px HMOVE through `HMMx`/`HMBL` — that yields the **diagonal lines** of a fishing line / tether / rope / laser (drop the assumption that only vertical and horizontal are possible). 〔Fishing Derby fishingLineSlope (Integer/Fraction) + HMOVE; right line = BL, left line = M1; the right line's slope confirmed on a live run〕 `→ casebook.md "diagonal lines"`

## Playfield
- 40px across × 4 clocks per px. Expressive power is earned through vertical rhythm. 〔Davie S13〕
  - **Animation can cover the 4-pixel step.** Erik Mooney, 1997, on his playfield Space Invaders, told
    that the invaders march in place for two frames on every horizontal move: *"In the arcade, they
    alternate frames every time they move, but they move in single-pixel increments. I'm using
    playfield graphics, so they can only move in four-pixel increments, so I have them march in place
    to try and compensate."* 〔`199704/msg00197`〕 The frames between two coarse steps carry a motion of
    their own. The post he was answering gives the other choice, Atari's 2600 version, which changes
    shape only once per horizontal move, in time with the march sound. **Cited only, not verified.**
  - **A PF pixel itself cannot be narrower; colour timing or time can make it look finer.** random-terrain, 2021, asked whether
    playfield pixels can be 2 colour clocks wide. As the distillation notes record the thread, Thomas
    Jentzsch's answer was no, and his workaround is to change `COLUPF`/`COLUBK` part-way through a PF
    pixel, in steps of one CPU cycle (3 colour clocks, the "Two separate numbers" rule in the Colour
    section). mr-sql's second workaround, shown in *Fluid City*, is in time: a 30 Hz animation
    drawn at 60 Hz, with the PF pattern shifted on alternate frames, leaves a "phantom" pixel of half
    the width and half the brightness beside the real one — 8-clock tiles give 4-clock phantoms, and
    4-clock pixels would give 2. The notes record it as best on a CRT and gone when the B&W switch
    doubles the speed. 〔mining 322030 pf-2cc-wide〕 One emulator frame holds only one of the two
    patterns (our reading), as with flicker in the Multiplexing section. **Cited only, not verified** —
    only the distillation notes are held here, not the thread.
- **A still playfield can be stored as variable-length bands, not one entry per line.** 8bitworkshop's
  `complexscene` sample holds its picture as `.byte count, PF0, PF1, PF2` per band with a final
  `.byte 0` ending the table, reads it through a zero-page pointer (`lda (PFPtr),y`), loads the three
  PF bytes into X, a temporary and Y before `sta WSYNC` and stores them after it. In that two-line
  kernel a count is two scanlines (our reading of its loop: two `sta WSYNC` per `dex`), so its eight
  bands draw 94 two-line rows from 33 bytes. Read 2026-10-01 in the umbrella's
  `reference/docs_atari/8bitworkshop_samples/complexscene.asm`; a 2020 beginner's thread worked through
  the same format 〔mining 305503 reading-byte-tables; only the distillation notes are held here, not
  the thread〕. The sample assembles (`8bitworkshop-crosscheck.md`, 26/26); **Not verified** — the
  scanlines it draws were not measured here.
  - **The kernel can skip the writes too, on a symmetric playfield.** vdub_bobby, 2005, after a nested
    loop that draws 192 lines in blocks of 8: the best is to write the PF only when it changes, since
    unless told otherwise the TIA keeps drawing the same thing every line 〔mining 66378
    padawan-seeks-jedi-master; only the distillation notes are held here〕. That holds only while no
    register is rewritten mid-line: with an asymmetric PF each register ends the line holding the right
    half, and a skipped line inherits that. `cmd/framegen` drops a left-half write only together with
    its right-half one for this reason (its playfield-pruning comment). **Cited only, not verified** —
    the cycles freed were not counted. glurk, to a kernel that changes `COLUPF` along each line of a
    still playfield of coloured beans: *"you also do not need to set PF0 PF1 and PF2 on every line.
    Once you set them once, they stay set that way...  So you really only need to set them once at the
    top, and maybe clear them at the bottom."* and *"I didn't test it, but the extra cycles may allow
    you to do all the COLUPF changes on each line without using up a player sprite..."* (the asker had
    covered one bean with a player because the colour writes would not line up). The asker's reply puts
    the limit elsewhere: *"There's actually enough CPU time to make all ten COLUPF changes on each line,
    but my problem was how they lined up with the playfield beans."* 〔mining 346865
    help-with-optimizing-state-and-palette-lookup-code〕 So in that kernel the freed cycles did not
    buy the player back (our reading). **Cited only, not verified.**
- **A scrolling PF background is 3 layers — board RAM + display buffer + delta update** — plus tile-granular scrolling (avoids tearing). Iron rule = **keep the total scanline count constant from frame to frame** — the legal totals and the PAL parity constraint are the rule
  immediately below, stated once rather than twice. The scroll band is 10–16 lines top and bottom. 〔200972 tile-scrolling-engines, Boulder Dash style〕 `→ design.ScrollScanlinesConstant` **and `design.ScrollBackgroundFitsRAM`**. **Corrected 2026-09-03:** this line prescribed three layers and pointed only at the scanline check, which looks at line counts and PAL evenness and nothing else — so nothing asked whether the three fit. The same source says they usually do not: **a world you rewrite at run time needs SuperChip/CBS RAM, because the internal 128 bytes only hold a 120-byte-class malleable world** 〔200972:14〕. Budget the three layers plus the stack against `design.RAM2600` before choosing this structure. Found by auditing harness claims against the sources harness itself cites.
- **PAL frames must have an even scanline count** — an odd total loses the colour in that frame, so a
  kernel that varies its line count must vary it in twos. The count is the whole frame's, not only the
  visible lines. Eckhard Stolberg, 1999: *"The major difference between PAL and NTSC is the colour
  encoding and not the frequency. A VCS game has to display an even number of lines in every frame or
  it would loose the colour in the frame with the odd number of lines. That is the overall number of
  lines per frame and not just the visible ones."* 〔`199903/msg00086`〕 His sentence names no standard,
  but it answers a question about a PAL conversion, and in 2000 he gave the rule for PAL mode
  (`200006/msg00040`, in the NTSC/PAL rules); whether an odd NTSC frame loses colour is not settled
  here (our reading). **Cited only, not verified** — no odd frame was looked at on a set here.
  Safe zone 262 (NTSC) / 264 (PAL).
  `→ design.ScrollScanlinesConstant`, which carries **two** checks under one name: the count is
  constant frame to frame, **and** it is even when `pal` is set (`pkg/design/pf.go`, `ScrollScanlinesConstant`: `if pal && n%2 != 0 {`).
  ★**The rule says what is illegal and not what to do about it, so here is the fix, from the person
  who caught it happening.** Eckhard Stolberg, reviewing Andrew Davie's Qb in 2001: *"now you are
  doing **three lines less per frame** than you should, which is **an odd number and therefore results
  in the PAL colour loss**. You have to **reinsert them at some other space. Maybe just increase the
  VBLANK timer**"* 〔`200103/msg00173`〕. ★★So a kernel that loses lines does not have to give them
  back where it lost them — the frame only has to total right, and **VBLANK is the cheapest place to
  put them** because nothing is drawn there. ★★★Note the shape of the bug too: the author had just
  *fixed* something (a six-line VSYNC), and the fix is what unbalanced the frame. A line-count
  regression usually arrives attached to a correction.
  **Promoted to its own rule 2026-09-03.** It had been living as a parenthetical inside the
  scrolling-background bullet above, which is a different subject and carries a different name.
  The distillation measured the cost of that: **eight corpus references cite this claim and every
  one of them cites a line number that no longer holds it** — the worst rate of any claim in this
  file. A claim with no name of its own has no address, so nothing can point at it and survive an
  edit. That is the general lesson, not a fact about PAL. 〔200972 tile-scrolling-engines〕
- **Hold the board at the game's resolution, not the kernel's.** DaveM, 2022, planning ice blocks drawn
  in the playfield: five PF registers × 47 double-lines = 235 bytes of RAM. Pat Brady: *"Pengo's
  resolution (in terms of data storage for the stationary ice blocks) is 11x8. Conceptually that can fit
  in 11 bytes, though the actual game may use more RAM in some way dictated by the kernel"* — 88 cells
  at one bit each (our arithmetic), turned into PF writes while drawing. In the same thread he recalled
  a Primordial Ooze kernel whose 16 columns would have been a 384-byte bitmap and were instead *"a list
  of (row,column) pairs where transitions occurred, sorted by row, and a one-row bitmap"* — 34 bytes, on
  condition that *"no row had more than one transition, because the kernel didn't have time to check 2
  list entries"* 〔mining 326419 new-homebrew-kick-ice〕. The board-RAM layer of the scrolling-background
  rule above is where this choice is made. **Cited only, not verified.**
  - **A sparse playfield can cost a fixed RAM budget at any resolution.** Manuel Polik, 2001, for 32
    tombstones: two 32-byte RAM tables, one holding *"the absolute number of the PF value that contains
    one or more tombstones"* and one *"the actual PF value to set"*; the kernel counts the PF values it
    draws, and *"if the current location is next in the location tab -> ... draw the according value
    from the value tab -> and if no draw $00"* — *"You could do any resolution you want, without
    increasing the RAM usage of constant 64 bytes!"* The cost was left open: *"You'd need more counters
    in the kernel than you'd have as registers, but I'm sure that can be handled"* 〔`200108/msg00433`〕.
    The location table has to be in drawing order (our reading of "next"). **Not verified** — the
    kernel was proposed, not written, there or here.
  - **Or hold the board in the kernel's own bytes and edit it there.** Thomas Jentzsch, 2001, having
    just started disassembling *Surround*: *"I've just started disassembling Surround, which has a
    20x20 field: Alan uses 100 bytes for this, 5 bytes for each row (1*PF0, 2*PF1+PF2), which is the
    most compact format if you can't calculate the data during the display kernel. So ALL
    manipulations of the field are done directly on the data that is needed in the kernel."* And:
    *"I found a similar solution in River Raid, where Carol doesn't have any score variables, but is
    doing all calculations directly on the score _pointers_. But I'm not sure if she did this because
    of RAM (I doubt this) or cycle optimization (has to be compared)."* 〔`200108/msg00335`〕 Manuel
    Polik, for Arkanoid in the same exchange: *"you could do just 2*PF1+PF2 and you'd certainly not
    have 20 lines of bricks, but maybe only 8, reducing the data to 32 bytes. Decoding 4 different
    states of bricks would then cost 96 bytes, doing it the easy way."* 〔`200108/msg00336`〕 This is
    the other answer to the bullet's question — the board at the kernel's resolution, when the kernel
    has no time to convert — and a board that does not scroll then has one layer where the scrolling
    rule above has three (our reading). **Cited only, not verified** — his reading of a disassembly
    he had just begun.
- **For HUD/text the character COUNT picks the technique**: 48px = 12 characters / venetian blinds = 32 characters (but only at 3px width). Either split the HUD into its own screen mode, or isolate it in a zone and reuse the score area for several purposes. 〔197162 text-hud〕 ⚠ **The 32-at-3px figure is the source's, not this repository's, checked 2026-09-07.** `roms/techniques/venetian.asm` demonstrates the mechanism — one player drawing figure A on even rows and figure B on odd, zero flicker, half the vertical density — and it draws **shapes, not characters**: measured, an 8-px band at clock 80..87 with runs of 1, 2, 4, 6 and 8 px. **Nothing here has ever rendered 32 characters this way**, so 3 px is the width the technique would require rather than a width anything has been read at. ★For comparison the text kernels ARE measured: 4 px a character at every rung of their ladder 〔`techniques/text12.md`〕, so venetian's 3 px would be **narrower than the narrowest thing this repository has drawn a letter at** — worth knowing before designing to it. `→ design.MaxChars` `→ design.MaxChars/FitsText`
  - **The rungs around 12, from Robert M:** PF alone *"gives you 40 pixels / 4 per character giving you
    10 characters across"*; the 6-digit score trick, 48/4 = **12** at low res 3×5 or 3×7; Stellar
    Track's flicker, **12 hi-res** at 30 Hz; and **13 without flicker** — 3×5 letters in the PF with
    *"the missles, ball and sprites to superimpose lines between the characters which would otherwise be
    touching at their sides"* 〔mining 62722 couple-of-design-questions〕. A 2009 thread gives the same 10
    with the rows: for nukey-shay's asymmetric-PF kernel there, 24 rows of 8 scanlines (192 lines), the
    estimate recorded is 10 characters across (3 points wide plus a space) by 4 lines of 5-point-high
    letters, each line followed by a blank row 〔mining 148751 hello-world; only the distillation notes
    are held here, and they do not say who made the estimate〕 — 4 × (5 + 1) = 24 rows (our arithmetic).
    **Cited only, not verified** —
    the 10 and the 13 have not been built here.
    Two larger counts are named without a description: ZackAttack, sketching 8bitPoet's game, *"The
    score and lives remaining can use the 18 characters kernel (36 character with double width
    glyphs)"* 〔mining 344323 fluid-simulation-for-new-2600-game-concept〕. The post says neither how
    that kernel works nor what cartridge hardware it assumes. For scale, the widest line built here is
    24 characters at 50% flicker, two 12-character blocks on alternate frames (`techniques/text24.md`).
    **Cited only, not verified.**
  - **Six digits was the ceiling a player saw in 1997.** Erik Mooney, asking how high-resolution
    scores are drawn, guessed *"three copies of each player, rewriting GRP0 and GRP1 between display
    of each copy"*, and added: *"This would account for the six-digit limit on all 2600 games I've
    seen except Dark Cavern, which had three dummy zeroes anyway."* 〔`199703/msg00214`〕 The replies
    confirmed the mechanism with Defender's and Cheetah's code 〔`199703/msg00220`, `msg00219`〕 — the
    48px rule in the Sprites section. Dark Cavern's exception is itself a move: digits that never
    change need not be drawn by the six-copy line (our reading; he does not say how they were drawn).
    **Cited only, not verified** — the ceiling is one player's observation, not counted over the
    ROMs here.
- **An asymmetric PF is expensive** (PF0/1/2 written twice mid-scanline; the PF0 window is only ~20cy). Compromises = central 32px / every other line with double height / venetian / RAM self-modification. 〔Davie S17, castlevania-port〕 `→ design.AsymPFLineFits/AsymPFReachableX`
  - **The central-strip compromise buys two things at once.** tokumaru's 2010 Sonic mock-up: *"only the
    central portion of the screen is used, meaning that only 2 playfield registers are used. Also, the
    blanked area at the sides will conveniently hide portions of the sprites so that they can smoothly
    scroll in and out of view."* 〔mining 170135 sonic-the-hedgehog-on-the-2600〕 One loss of width pays
    for both the PF writes and the edge-entry problem. **Cited only, not verified** — a mock-up, not a
    running kernel.
  - **Keep PF0 as a fixed frame and the playfield becomes a board that fits in RAM.** kylearan,
    sketching a Space Taxi-like game: *"Restrict the playing area to PF1 and PF2 and use PF0 only for the
    enclosing border. That gives you 32 blocks = 128 pixels horizontally for level design of the interior
    area, and you'd only need to update PF1 and PF2 twice per scanline"*, and at ~20 blocks of 8 lines,
    *"an 80 bytes framebuffer to keep in RAM making dynamic levels possible, with gates
    appearing/disappearing, walls moving etc."* 〔mining 261054 wip-space-taxi〕 The 80 checks: 32
    columns are 4 bytes a row, × 20 rows. Giving up the edges buys both the write count and a board small
    enough to sit beside the game in 128 bytes. He reported later in the thread, *"I have a kernel
    more or less working now"*, at *"a resolution of 32x20"* (not looked at here). **Cited only, not
    verified.**
  - **The same RAM board could gate ROM art.** kylearan's next sentence: *"Not sure yet, but might be
    possible to use the framebuffer as an AND mask, allowing for higher resolution graphics stored in
    ROM where whole blocks can be switched on/off."* RAM would hold which blocks exist and ROM what they
    look like — the reverse of the masked-sprite rule in the Sprites section, where the mask is in ROM.
    Each masked write adds an `AND` of at least 3 cycles, and PF1 and PF2 are written twice a line
    (our arithmetic). His later report does not settle it: the higher-resolution option there is a
    separate 32x80 version that *"requires much more ROM per level"*, and whether it masks is not said.
    〔mining 261054〕 **Not verified** — an idea he was not yet sure of; the copy here holds 150 of the
    thread's 181 posts.
  - **Or pay in time: rewrite on some lines and rest on others.** Erik Mooney, 1997, in a thread
    about the HMOVE comb: *"try to have scanlines during which you don't need to rewrite the playfield -
    Centipede, Millipede, and my as yet unnamed program rewrite the playfield twice per line for four
    lines, then turn the playfield completely off for four lines, during which you can position objects
    and so on.  Pac-man only needs to rewrite the playfield twice in a line every eighth scanline - the
    rest of the time, it's a standard reflected playfield."* 〔`199703/msg00249`〕 Most of the compromises above
    give up width or height; this one gives up the asymmetric rewrite on some lines (our reading).
    **Cited only, not verified** — his account of the three commercial ROMs was not checked against
    them.
  - **Or run the line from RAM, where every load is an immediate.** Christopher Tumber, 2003: five
    table-fed TIA writes (`lda table,x` / `sta`) cost 35 cycles and the same five with `lda #` cost 25;
    the second form *"could be used now if you have enough ROM to predetermine all possible routines
    (like Galaxian). However, the number of combinations can quickly get out of hand unless you do it as
    a RAM routine and have your program just preset the values to be LDAed during the RAM routine."* In
    the 128 bytes this is already done, at a price: *"Space Instigators uses a RAM routine to draw the
    Instigators. However, since I can only have one copy of the routine in RAM, the kernal has to
    re-seed the RAM routine after each row of Instigators. This leads to a couple blank lines between
    rows of Instigators"*. Of a kernel built wholly in RAM: *"we could be writing to the TIA registers 15
    times per scanline"*, and lines chained by `JMP` cost *"only a sacrifice of 3 cycles per
    scanline"*, or by `JMP (table)` so that lines *"can be re-used"*. 〔`200305/msg00006`〕 15 writes at 5
    cycles are 75 of the line's 76, so the 15 and the 3-cycle `JMP` do not both fit one line (our
    arithmetic; his count adds that writes needing no load, such as `STA RESPn`, come on top). The
    whole-screen version he sized needs cartridge RAM, not the console's. **Cited only, not
    verified.**
- **Write deadlines for an asymmetric PF (measured cycles)**: when you display the left half and rewrite the right half on the same scanline, aim each write at the moment that PF is **no longer visible**. The classic kernel's actual values =
  first pass PF0[cy7] / PF1[cy14] / PF2[cy21] (for the left half — in time before it becomes visible) → then for the right half
  **PF0 rewritten at cy31 / PF1 at cy38 / PF2 at "exactly cy45"** (too early or too late and it breaks — adding a single nop destroys it).
  What remains, 76−47 ≈ **29cy per line, is the free budget for sprites and the like**. Judge whether a horizontally multi-coloured PF is feasible by asking "can we hit that single 45cy point without fail, and does the rest of the work fit in the remaining 29cy?". 〔Williams/Saunders "Asymmetric Reflected Playfield" tutorial〕
- **A free 2-colour PF = CTRLPF D1 (the score bit)**: set bit1 and **the left half of the PF takes COLUP0, the right half COLUP1**, independently coloured (no asymmetric write timing needed). The staple for score display, but also a cheap way to colour the left and right of a background differently. 〔w11/Asym2scrol〕 `→ design.ScoreModeTwoColor`
- **The saving trade of giving up PF0**: not drawing PF0 (on top platforms and the like) forces PF2 to be written at **exactly cycle 48** instead, but it frees **12cy per line + 18 bytes of RAM**, and lets the player fall off both edges of the screen (an advantage of the reflected PF). 〔mining blog SpiceWare Stay Frosty〕
  - **The PF0 trade is one rung of a ladder counted in registers rewritten per line.** SpiceWare,
    2015, on showing different images left and right: *"Takes a lot of cycles per scanline though ... In
    Stay Frosty I mirrored the screen and only updated PF1 and PF2. In Stay Frosty 2 I used the time
    saved by utilizing DPC+ to also update PF0."* 〔mining 242131
    simple-question-about-frogs-flies-and-atari-text〕 So the question is how many of the three
    registers a line can afford: two (Stay Frosty), three (Stay Frosty 2, with DPC+ paying), six (a
    full asymmetric line, above). In a 2018 thread he put dropping PF0 at *"10 cycles per scanline"*
    〔mining 276058〕, not the 12 above. **Not verified** — `fundamentals-audit.md` lists DPC+ as not
    verified here, and neither figure has been counted.
  - **What the two-register ceiling left out, and which resource ran out.** SpiceWare, asked whether an
    asymmetric reflected PF in PF1/PF2 plus a sprite and colour changes was possible, answered that Stay
    Frosty does it with two sprites (one reused and repositioned) and colour changes on a sprite and the
    PF; the melted trail is one missile, and the other missile and the ball are unused. Two things were
    dropped for different reasons: the snowman's carrot nose, planned for the other missile, for want of
    **CPU time**, and a level number because the snowman and fireball graphics used up the **ROM**
    〔mining 136387 the-2600-is-truly-challenging; only the distillation notes are held here〕.
    **Cited only, not verified.**
- **The wall can be the background.** iesposta, 2017, looking at Starpath's *Escape from the
  Mindmaster* in Stella's debug colours: *"Wow that means the background color is the walls and the
  Playfield is the ceiling, floor and side passages."* The day before he had described it as using
  *"sliding diagonal missiles and/or ball"* to *"smooth the tops and bottoms of the walls out"*. He
  asked gip-gip, whose pseudo-3D engine (VePseu) draws its walls in PF, to invert it; gip-gip: *"It would (in
  theory) be simpler, but it would also mean you would lose (color) shaded walls. Plus, it would look
  kinda weird with the fact that PF0 isn't colored in, and the last PF2 is also mostly empty."*
  iesposta: *"Background can change color every scan line just the same as playfield."* 〔mining 263329
  3d-engine-for-vanilla-cart〕 Inverted, the largest area costs one colour register and no PF bits, and
  the playfield draws the smaller shapes around it (our reading) — the background-colour move of "Painting a sprite
  the background colour" (Multiplexing section), used to draw rather than to hide. **Cited only, not
  verified** — Mindmaster's layout is one viewer's reading of the debug colours, not its author's
  statement, and the thread does not return to the inversion.
  - **The smoothing costs objects, and which ones depends on where the wall is.** Thomas Jentzsch,
    replying to gip-gip's plan to add wall smoothing to VePseu (the copy here holds his reply only as
    the next posts quote it): *"Smoothed walls are nice, that's for sure. But I agree, that not
    using the players will make the games much less restricted. And since players and missiles share
    the same color, the missiles cannot be used too (unless you want a monochrome look). So that only
    leaves the ball, but one single object is not sufficient or you have to use flicker."* iesposta:
    *"You are saying because this engine uses playfield for walls, the smoothing would have to be both
    missiles making invisible (black) P0 & P1?"* and *"Mindmaster is inverse, so the ball and one
    missile smooth leaving a free player and its missile?"* 〔mining 263329 3d-engine-for-vanilla-cart〕
    So there are two ways to smooth an edge: add to the wall in the wall's colour (Jentzsch's reading
    — for PF walls that is `COLUPF`, which only the ball has), or cut it away with objects in the
    background colour (iesposta's questions — missiles whose players are black). Which objects that
    costs depends on whether the wall is PF or background (our reading). `techniques/invisible-probe.md`
    hides an object the same way, for collisions. **Cited only, not verified.**
  - **A true 3D engine in 4K without ARM was designed; in the six posts held here it is not shown
    working.** bit-expander, 2014: rooms are convex zones joined by portals; the engine culls, rotates,
    translates, clips and projects into RAM, then draws *"up to 21 color-textured polygons on screen
    taken from RAM using the playfield"*, with *"the display is rotated 90 degrees to allow the colorful
    textures"* and *"16 colors are allowed in the textures as the engine uses 8 shades to perform
    Gouraud shading"*. *"63 bytes of RAM are used to store the 21 polygons, and this list is being
    refreshed every other frame (30Hz)"*, the blanking time going to the engine on alternate frames. His
    own status: *"Clipping to frustum is not working yet"*, and entering and exiting portals not yet
    handled; gemintronic's build gave *"a scrambled screen"*, which he put down to the clipping code
    *"messing the polygon list"*. 〔mining 222981 3d-fps-engine-for-the-2600; the copy here holds 6 of 13
    posts, so how the thread ends is not known〕 The figures are a design's budget for a ceiling, not a
    frame anyone in those six posts has seen. **Not verified.**
- **Vertically moving platforms use two zones of complementary height**: build the upper and lower band heights so that "when one grows the other shrinks by the same amount" and the total line count stays constant = a stable picture (mismatched, you get motion blur). 〔mining blog SpiceWare〕
- **Visible delay on a PF register write**: an `sta` to PF0/PF1/PF2 takes effect **2–3 colour clocks late**
  (colour registers are immediate). Complete the centre boundary of a reflected PF at **exactly cycle 48**
  (measured in mining 149228; clone consoles are +1cy) — consistent with the **"PF2 at exactly cy45"** deadline
  in the asymmetric-reflected-PF rule above, the 3-clock write delay being the difference. Do the timing
  arithmetic for a horizontally multi-coloured PF with this delay included. 〔mining 149228 PF write-timing table〕
  (The dangling "at line 38" this sentence used to carry is resolved 2026-08-06: it was a line number into an
  earlier revision of THIS file, and the rule it pointed at is the cy45 one now cited by name. Line numbers do
  not survive editing; a reference has to name the rule.)

## Multiplexing and flicker
- **★Flicker multiplexing DISABLES the TIA's hardware collision detection, and the reason is a miss rather than
  an inaccuracy**: two objects that are colliding may never be drawn on the SAME FRAME, so `CXPPMM` and friends
  simply never latch. The player sees "I hit it and nothing happened". Choosing flicker therefore decides the
  collision architecture too — it has to move into software. Cheapest first step, from the same source: test
  collisions **only for a sprite that MOVED this frame**, since most sprites are stationary; the cost is that
  an overlap present the moment a screen appears goes unnoticed until something moves. 〔blogs 8429 SpiceWare/Frantic〕 `→ design.HardwareCollisionUsable`
  **Flicker also makes a fast object's collision LATE, not only missed.** Erik Mooney, 1997, on his
  playfield Space Invaders, answering a report that a shot goes well into a shield before taking a piece
  out: *"the missiles move up four lines per frame and each missile is drawn only every other frame, so
  by the time I have a chance to check the collision, the missile is already sticking out of the top of
  the shield. 2600 SI handles this by enlarging its shields vertically and by slowing the missiles
  down."* 〔`199704/msg00197`〕 Four lines a frame, drawn one frame in two, is eight lines between chances
  to latch — that arithmetic is ours. The day before, he had declined to alternate frames on the
  shields: *"that'll really cause problems with the collision detection"* 〔`199704/msg00190`〕.
  **Cited only, not verified**, including what Atari's version did.
  - **A commercial collision handler: record in the kernel, decide outside it.** nukey-shay, 2019,
    reading *Defender II* for a hacker: *"When you collide with any sprite, it gets recorded in an ARRAY
    starting at ram $B0. This is later checked OUTSIDE THE KERNEL to decide what to do"*. In the routine
    he quotes, the code walks that array and compares each hit object's tag with the humanoid's: lower
    is an enemy, equal is a caught humanoid, and a second compare picks out the stargate (our reading of
    the quoted code) 〔mining 289892 defender-ii-hack-attempt-trying-to-find-2-portions-of-code; only the
    distillation notes are held here〕. **Cited only, not verified.**
- **The object budget is counted per scanline, not per screen.** Living Room Arcade (2024) asked how
  *Star Wars: The Arcade Game* is possible with *"two players, two missiles and a ball"*. Verdant: *"It's
  doesn't become simple, but it does become less difficult, when you stop thinking so much in two
  dimensions and keep in mind that you have all of those things (plus the playfield graphics) on every
  scanline. All the non-playfield elements can be moved slightly between every pair of lines, or moved
  completely arbitrarily after a single skipped line, without using any special trickery."* 〔mining
  361594 quick-questions-for-atari-2600-programmers-thread; "It's doesn't" is his spelling〕 "Five
  objects" counts a screen; the machine counts a line. The skipped line is `RepositionCostScanlines = 1`
  in the next rule. **Cited only, not verified.**
- **"Four objects on a line" is three different claims, and naming the games it beats says which.**
  Erik Mooney, 1997: *"The "never done before" part (to the best of our knowledge) is to have four
  independent detailed player-controlled flickerless objects on the same scanline. In single
  resolution. With color changes every scanline. We're using the missiles to display two player
  objects, and the players for the other two. Video Olympics had four such objects, but they were
  undetailed pong bars, and according to 'kickass', that doesn't count. :) Ditto for Super Challenge
  Football, which draws the non-player-object players with missiles... that has four independent
  detailed objects (double-resolution, though, and I've got mine running at single.) SC Football can
  display playfield and the ball on the same lines as the players, though, which I'm not doing."*
  〔`199709/msg00090`〕 Detail, resolution, and what else shares the line tell the three apart. His
  partner, the same day: *"obviously the missle objects aren't as detailed as the player objects, but
  they are easily identifiable."* 〔`199709/msg00092`; "missle" is his spelling〕. In 2013 Mooney
  recalled the four-object loop as working *"Not perfectly, since the register hits happen throughout
  the scanline so there'd be some graphical jitter"*, and the code he posted rolls in Z26 and Stella
  by his own account 〔mining 205205 old-tank-n-bomb-2600-game-erik-mooney-piero-cavina〕. **Cited
  only, not verified.**
- Beyond 2 objects, multiplex by Y band; a horizontal repositioning costs one scanline; **an empty Y lane is mandatory**; the price is 30Hz flicker. 〔Bumbershoot〕 `→ design.NeedsFlicker/NeedsEmptyYLane/RepositionCostScanlines`
  ★**"30 Hz" is the FIRST rung, not the price of multiplexing in general** — that predicate answers yes or no and gives the same answer for three objects and for twenty. `→ design.SubsetsFor` and `→ design.FlickerRateHz` give the ladder: 1–2 objects share nothing and run at the full 59.92 Hz, 3–4 need two subsets and land on the 30 Hz above, and it halves from there. Glenn Saunders, 1997: *"**It's never really necessary to drop below 30hz** and still manage to fill the screen with sprites"* 〔`199709/msg00139`〕; Piero Cavina five days later, on the other end: *"**'Adventure' must be the king of flicker**"* 〔`199709/msg00218`〕, and twenty-four objects in one room works out to **4.99 Hz**, which is the *"5hZ, maybe?"* he guessed. **Neither number was derived from the other.** Full table in `techniques/flicker-multiplexing.md`.
  **What that scanline looks like.** The same constraint was stated on the list in the form the
  person drawing the screen actually sees it — Piero Cavina, shipping a new build of *Look Mom No
  Flicker*: *"I've found an use for the ball :) Most of the changes aren't visible, but very
  important — now the 4 objects are repositioned before each floor (here's why the floors are
  thicker)"* 〔`199808/msg00051`, 1998-08-14〕. **The floors got thicker because each one now has to
  pay for a repositioning.** `RepositionCostScanlines = 1` and "the floors are thicker" are the same
  fact, and when the layout is being drawn rather than budgeted, the second one is the one that
  lands: a band that hosts a repositioning cannot also be the thinnest band in the picture.
  **When no line can be spared for the repositioning, the picture decides where it goes.** hammersdev on
  *Fate of a Bait* (2026), where the ball's motion value must be rewritten every line to draw a sloping
  fishing line or worm: *"the code needs a specific condition regarding the slope of the fishing line or
  worm body shape to start the reconfiguration for the next Objects area. In other words: It waits for a
  constellation where the pixel position of the fishing line does not change from line to line or the
  worm body stays the same."* That moment moves from frame to frame, so *"the intermediate areas must
  have a certain minimum height (9) to ensure that, at some point, all conditions for the
  reconfiguration are met"* 〔mining 386678 fate-of-a-bait-finished-homebrew〕. A gap band's height can
  be the guarantee that an opportunity occurs, not only the cost of one. **Cited only, not verified.**
  The same author had priced a third row in an earlier reply (2026-03-06): *"There won't be a third fish row without
  cutting down the worm's length or resolution (for RAM) and/or the sprite's height (for screen
  space)"*, having just said *"everything is huge (which I like)"*. Each way to buy the row is paid in
  the picture, so the drawing decides whether it is bought. **Cited only, not verified.**
- **Turn one sprite into many by rewriting GRP mid-scanline**: duplicate a single player with NUSIZ and re-`STA GRPx` just before each copy is drawn, and **every copy can be a different picture** (the shared basis of Space Invaders formations, 6-digit scores, and varied enemy rows). Keep `STA GRPx` strictly inside HBLANK. 〔mining 337131, 182923〕
- **A console switch can choose the TV standard, and the price is one switch.** A PAL frame needs
  more scanlines than an NTSC one, and that is a number the ROM decides — so a single build can serve
  both if the player tells it which television it is on. Piero Cavina listing what his 1997 demo does,
  crediting Nick Bensema: *"**pal/ntsc option using the bw/color switch** (**limited to screen size,
  not colors**)"* 〔`199703/msg00228`〕. ★★**The parenthesis is the whole caveat and it is in his own
  sentence**: the switch buys the line count, not the palette. A colour chosen for NTSC is still a
  different colour on a PAL console 〔see `internal/emu/palspec_test.go`〕, so this makes a ROM
  *displayable* on both, not *right* on both. ★★★What it costs is a switch the game can no longer use
  for anything else — and on this machine that is a real budget, since there are only three.
  - **The same switch is a different switch on a 7800.** Andrew Davie, posting a pause routine on the
    colour/B&W switch: *"On the 2600 it's a two-position switch so you choose COLOR or you choose B&W
    and it stays there. On the 7800 it's a momentary switch -- it's only 'pressed' while you're
    actually pressing it"*, so a 7800 pause has to toggle on each press; and *"on the 2600 you really
    don't want the game to pause when it starts up, so the code should detect a change in state, not
    an absolute position."* His routine tells the consoles apart by bytes the 7800 BIOS leaves at
    `$D0`/`$D1`. Later in the thread SpiceWare finds that this depends on the cartridge: on his original
    Harmony (BIOS 1.05) Stay Frosty 2 detects a 7800 as a 2600, on the Encore (1.06) correctly — and
    Dionoid's routine for *Lode Runner 2600* drops the detection: pause on any change of the switch (or
    the 7800's button), resume on the joystick or the fire button. 〔mining 194119
    26007800-pause-routine〕 Read the switch by its changes, not its position, if the ROM may run on a
    7800. **Cited only, not verified** — the 7800 is not modelled here.
  - **Of the two conversions, PAL to NTSC was called the harder.** phredreeke, 2011: *"IIRC PAL to
    NTSC is harder than the other way around, as PAL has a longer VBLANK period."* And: *"There's no
    region lockout though so a PAL game will work on an NTSC 2600, but chances are your TV wont be
    able to sync to it, and if it does the colors will be wrong."* 〔mining 190121
    rom-pal-to-ntsc-conversion-needed-or〕 By this repository's constants PAL's VBLANK is 45 lines to
    NTSC's 37 and its visible area 228 to 192 (`resources.md`), so work placed in PAL's longer blank
    may not fit NTSC's (our reading; the thread does not say which difference is the one that bites).
    Speed is a separate problem — the frame-counter rule in the craft section. **Cited only, not
    verified** — his own "IIRC", and no conversion was tried here.
  - **Thomas Jentzsch planned the one-binary version for *Thrust* and named what changes.** Eckhard
    Stolberg, 2000: *"If you still have plenty of space, maybe you could put a PAL mode into the same
    binary."* 〔`200006/msg00026`〕 Jentzsch: *"I thought about two different bins when i started developing Thrust. Your idea
    is much better, i think correcting the timings for VBLANK and overscan and some game-speed-parameters
    should do it."* 〔`200006/msg00030`〕 Stolberg: *"That would help, although most PAL TVs can handle
    60Hz displays just fine. The real difference is in the palette."*, and a PAL mode must always give
    an even number of scanlines 〔`200006/msg00040`〕. A plan, not a report of the result. Next to
    Cavina's switch, which buys only the line count, Jentzsch's list adds game speed, and Stolberg names
    the palette as the larger difference (our reading). **Cited only, not verified.**
  - **A kernel that reads a table per line pays for PAL's extra lines in ROM.** Eric Ball, 2002, on
    *Skeleton*: *"it is going to be difficult to accomodate the 20% increase in table size for the PAL
    version"* 〔`200209/msg00047`〕. Manuel Polik did not see why a conversion should cost space: *"Just
    adjust the number of WSYNCs for PAL to burn the additional lines away, that shouldn't cost a single
    byte."* 〔`200209/msg00080`〕 Ball: *"For a normal kernel, that may be true.  But the Skeleton kernel is
    based on very large look up tables and heavy use of LDA (ZP),Y.  For every line on the screen, I
    have 6 bytes of maze lookup.  And the skeleton player graphics are also done for each visible line.
    So for PAL, the active screen increases by 20% from 190 lines to 228 lines, with a similar increase
    in lookup table size."* 〔`200209/msg00085`〕 Polik's alternative was to draw the NTSC screen and add
    38 `WSYNC`s at the bottom 〔`200209/msg00088`〕; Ball wanted the full screen, with the skeletons 20%
    taller too, and had *"very nearly filled the entire 4K ROM, even with the NTSC version"*
    〔`200209/msg00090`, `msg00094`〕. **Cited only, not verified.**
- **Two different needs share the word "random", and only one of them is expensive.** A starfield or
  a terrain must be **reproducible** — Manuel Polik: *"Total randomness won't work, since you've to
  **REPEAT** what you're doing every frame"* — and that is what a fixed-seed LFSR is for
  (`roadmap.md` #7). A tetromino must only be **unpredictable**, and for that a counter is enough.
  Eckhard Stolberg, 1997: *"Having a **counter run from 0 to 6, that increments in every frame**, gave
  enough ramdomness for my Tetris version"* 〔`199706/msg00005`; `ramdomness` is his spelling〕 — seven
  values, exactly the number of tetrominoes, so not even a modulo.
  ★★**Measured** (`internal/emu/counterentropy_test.go`), sampling that counter on the frame the fire
  button goes down, over 140 frames:

  | presses | tally across the seven values |
  |---|---|
  | every 7 frames | `[0 0 0 0 0 20 0]` — every sample identical |
  | every 3 frames | `[7 7 6 7 7 7 6]` — all seven, evenly, and entirely predictable |
  | irregular gaps | `[2 4 4 4 3 1 3]` |

  ★★★**The counter carries no entropy of its own**: synchronise the presses with it and the output is
  a constant. What looks like randomness is *when a person pressed*. ★★★★**And the middle row is the
  one to remember — it is MORE uniform than the irregular case and completely deterministic.** A flat
  histogram is not evidence of unpredictability, and an auto-fire button is exactly a fixed period.
  - **A range that is not a power of two costs time somewhere; one draw per frame moves the cost into
    frames.** TROGDOR, 2010: for 1–6, draw 0–7 and redraw on 6 or 7, but *"it requires an indeterminate
    number of cycles"*, so *"adjust the game requirements so that only powers of two are needed"*.
    batari, answering: an LFSR's sequence is fixed, so the worst number of tries can be computed
    offline; or *"simply get one random number per frame and only use it if it's within a valid range.
    It might be perfectly fine for a certain action to be delayed a few frames."* groovybee: *"Or use a
    modulo look up table if you have the ROM space."* 〔mining 159268 random-numbers〕 **Cited only, not
    verified.**
  - **Re-randomising every frame can be the waste.** Manuel Polik, 2002, on a crash scene adapted from
    Battlezone's: *"I think it was a little over-randomized in the original and I didn't see the need of
    repositioning the *noise* every frame, as the positions turned out to be already pretty random
    anyway..."*. In the same post he had adapted the code *"in numerous ways"* and reduced *"the
    required ROM space to some 20%-30% of the original code"* 〔`200211/msg00018`〕; how much of that the
    skipped repositioning saved, he does not say. **Cited only, not verified.**
- **An odd width is a power of two PLUS ONE, and the join is free.** A ball is 1, 2, 4 or 8 pixels
  and nothing else, so a shape that must be an odd number of pixels wide has no ball that fits it.
  Thomas Jentzsch, 2001: *"all cursors are **7 pixels wide** (has to be an **odd number** to make the
  up/down arrows look nice) and so the 'hole' in the stop cursor is 5 pixels wide. That means, I can
  **only use a 4 pixel wide ball**"* 〔`200102/msg00234`〕. Andrew Davie: *"Instead of 7-wide, make the
  cursor **9 wide**. Use the **missile to give you the extra pixel** you need. (8 player + 1 missile)
  Then it is a simple-matter to use an **8-wide ball**"* 〔`200102/msg00238`〕.
  ★★**Measured** (`internal/emu/playermissile9_test.go`): player alone 8 px, player + missile **one
  contiguous run of 9 px**, missile alone 1 px on the clock immediately after the player's last.
  `RESM0` on the instruction after `RESP0` is three CPU cycles = nine colour clocks, and a player is
  eight wide, so **no fine motion is needed at all** — one extra store, and the missile carries
  `COLUP0` so the two read as one object. ★★★The lesson generalises past cursors: **when a size the
  hardware offers is one short of the size the drawing needs, add a missile rather than change the
  drawing.**
- **Painting a sprite the background colour is not removing it, and only one instrument here can
  tell.** Ruffin Bailey, 1998, on someone else's game: *"why can I see a **silhouetted tank** over on
  the far right side of the ground in one player games?"* Piero Cavina, who wrote it: *"it might be
  the second player's sprite, which is **always there, but painted in black** in 1-player games. But
  **I don't see it here**.."* 〔`199801/msg00038`, `msg00045`〕. ★★**Measured**
  (`internal/emu/paintasbackground_test.go`), three bands — a visible player, the same player in
  `COLUBK`'s colour, and no player at all:

  | band | pixels | elements |
  |---|---|---|
  | visible | BG ×129, WHITE ×8, BG ×23 | BG, P1, BG |
  | painted as background | **BG ×160** | **BG, P1, BG** |
  | not drawn | BG ×160 | BG |

  ★★★**The last two are pixel-identical and element-different.** Every colour comparison here —
  `vismatch`, a golden frame, `cmd/still`'s diff — reads "painted the background colour" as "not
  drawn"; `DecomposeRow` does not, and that is what it is for. **So painting it out frees nothing**:
  the object still costs its store, still holds a player slot, and still sets collision latches. ★And
  the third party who *did* see the tank is outside what any of this reaches — a television is not a
  pixel comparator, which is the frontier `known-traps.md` names.
- **Colour is a capacity tool, not only a look.** Two characters can share one set of graphics bytes
  and differ only in `COLUPx`. Andrew Davie, 2003, on a Mario demo: *"you will see **two Marios** -
  animating independently. The interesting thing here is that the Marios are **different colours - but
  use the same graphics in ROM**… in this case I am **switching Red and Green** for display of the 2nd
  Mario"* 〔`200303/msg00015`〕. He was solving a specific problem — *"this solves a problem I have with
  Fu Kung! in **differentiating the players**, yet using the same graphics in ROM for each"* — and the
  general form is that **a second character costs a colour register write instead of a sprite table**.
  The saving is the whole table: for a 16-line shape that is 16 bytes per character, before animation
  frames multiply it. The constraint is that the two must read as the same silhouette, which is a
  drawing decision, not a technical one. Nothing here said this until 2026-09-07; the repository had
  colour as an appearance axis only.
  - **Colour first, shape when colour is not enough — and the shape is paid in the kernel.** Lumi's
    paddle game *Drive!* (2016) told its power-ups apart by colour alone: *"Red: Gives you an extra
    life"*, *"Green: Makes you invincible for a few seconds"*, *"Purple: Allows you to jump as much as
    you want for a few seconds with no penalties"*. v1.1: *"Purple powerup has been changed to blue to
    make it more distinct."* Thomas Jentzsch, after v1.1: *"It would be nice if the power ups would be
    recognizable."* v1.2: *"Treasures now have defined graphics"* — coin, necklace, jar, statuette —
    and Lumi: *"I did have to sacrifice some features to get the treasure graphics working, but maybe
    you won't notice. (Unfortunately it's not totally stable, and the treasures have some issues with
    vertical placement, especially noticeable on the first speed. It's just a result of cramming way
    too much into the kernel.)"* 〔mining 249827 drive-wip-formerly-jet〕 The steps run cheap to
    dear: a colour value, a better-separated colour, then graphics bytes and the cycles to draw them. A
    colour-only distinction is also exactly what the B&W rule in the Colour section tests. **Cited
    only, not verified.**
- **Height is a capacity tool too: a stack can be counted by how tall it is.** freshbrood, 2024, on a
  solitaire layout: *"A "2" card sprite could actually be 6 pixels tall, while only drawing the number 2
  at the bottom 5 pixels and setting the height to 5. Then as you stack it on top of an A it becomes 6
  pixels tall and moves one y position down. Voila. Not fancy but now you can represent all 52 cards as
  only 4 stacks with varying heights. You only need to flicker twice"* 〔mining 361594
  quick-questions-for-atari-2600-programmers-thread〕. The count lives in each sprite's height and Y, so
  four objects carry 52 cards. His order of work, the same day: *"Don't worry about making graphics look
  good- just focus on making them functional and easy to understand first."* **Cited only, not
  verified** — nothing here has drawn it.
- **A piece can move with no moving object: overlap mapped to colour.** littaum's *Brik Boom* (2026)
  draws its 8×8 board as coloured cells, and each cell is one of four: *"black (no block or player
  shape present), a pre-defined color (a block is present but not a player shape), white (no block
  present but player shape is), or grey (block present and player shape present at the same time). I
  like that this color arrangement makes it look like you are moving an object around a grid even
  though it is just different colors in the grid."* Two 8-byte maps, the board and the piece, are
  overlaid to pick each cell's colour, and *"that function takes up most of the OverScan logic"*.
  〔mining 391158 brik-boom-new-puzzle-homebrew-for-atari-2600-complete〕 The piece costs no object;
  it is paid in CPU outside the kernel and in RAM. **Cited only, not verified.**
  - **When rebuilding the buffer every frame costs too much, there are two steps down.** Thomas
    Jentzsch, in the same thread, after littaum wrote that the grid kernel races the beam too closely
    to fetch colours from outside zero page: the colours are assigned outside the kernel — *"3
    bytes/row (=24 bytes, could be SC RAM) and 64 resulting bytes (ZP RAM) for the kernel"* — and
    *"instead of filling the whole grid each frame, you could update only the deltas. Which would
    result from removed rows or columns and moved tiles. The former are easy to handle and the latter
    form a rectangle (up to 4x4 = 16 I think)"*; or *"fill the grid over e.g. two frames. If that
    doesn't look good, you could buffer the result in SC RAM first and finally copy into ZP RAM."*
    Rebuild everything, rebuild what changed, or rebuild over time behind a buffer; the "delta update"
    layer of the scrolling-background rule in the Playfield section is the second step. **Cited only,
    not verified** — the 16 is his estimate ("I think").
- **Multi-kernel = reuse one object per region**: switch `REFP` / position / picture per Y band and reuse a single player for different purposes (Stay Frosty). Match a "never overlap on the same line" placement constraint with an AI that "never enters an occupied column" and flicker is zero. 〔mining 303364, 318140, 164247〕
- **The stack costs 4 bytes, not a policy.** Christopher Tumber, 2004: *"I pretty much try to avoid
  using JSR completely, and only do so when absolutely needed … **RAM management is really one of the
  keys**"* 〔`200401/msg00013`〕. Measured across every `.bin` in this tree and the works, 30 frames
  after a warmup: **281 ROMs use zero stack bytes, 89 use 1-8 (4 is by far the commonest — two levels
  of JSR), exactly one uses 9-16 (`rts_dispatch`, at 10), and nothing between 17 and 128.** The nine
  above that are the ROMs whose subject *is* the stack. So the direction of Tumber's instinct is
  right — every stack byte is a byte of the same 128 that holds game state — but the **size** is
  single digits. Do not contort a design to avoid subroutines; do count them if a kernel starts
  nesting. Guarded by `internal/emu/stackbudget_test.go` at 16 bytes.
  - **Three answers to one beginner, 2016, on calling at all.** gip-gip: *"If you know the amount of
    time a JMP or JSR will take, it's perfectly safe (and saves space)."* reveng: *"In the middle of
    the visible-display kernel, forget about it. Otherwise, when it results in overall ROM savings
    without excessive cycle penalty, sure."* SpiceWare: *"JSR and JMP are fine, though keep mind that
    the stack starts at the end of your 128 bytes of RAM so nested JSRs could collide with your RAM
    usage if you're not careful."* 〔mining 253253 begginer-questions-to-get-me-started〕 The first two
    differ on calls inside the kernel; the collision SpiceWare names is the quantity measured above.
    **Cited only, not verified.**
- **★Everything measured here is measured BEFORE the television.** `internal/emu` imports
  `hardware`, `cpu/instructions`, `cartridge/mapper` and `memorymap` — and nothing from the engine's
  GUI, where the CRT model lives (`gui/sdlimgui/gl32_crtseq_effects.go`, `preferences_crt.go`).
  Counted 2026-09-04: **zero references** from `internal`, `cmd` or `pkg`. So `vismatch`, `read_row`,
  `framesim`, `visual_ceiling` and `get_screen_annotated` all see **pixel-exact output that no console
  ever produced**, and `ingest.md` deliberately instructs turning TV effects *off*, which is right for
  measuring and wrong for judging.
  **This matters because the machine's visual style depends on the blur.** A 1997 post on the list:
  *"the TV screen seems to act as an **anti-aliasing device** … especially true for the 2600 because
  its games made a **massive use of colour-striping effects**, that look much better on TV"* — and
  then the question nobody here had answered: *"wouldn't it be possible for an emulator to emulate
  also the **good imperfections**?"* The engine can. We do not.
  Three pages already say a local version of this — `invisible-probe.md` (*"pixel equality is stricter
  than a CRT, so passing here is necessary and not sufficient"*), `text24.md` twice — **so the fact
  was known three times and generalised zero times.** As a principle: **pixel-exact agreement is
  necessary and never sufficient, and a picture judged only here is judged on a display sharper than
  any 2600 ever had.** The remedy is not to soften the measurements; it is to **look at the artwork
  once through a CRT model before calling it finished**, as a separate act from verifying it.
  ★**And name the missing leg correctly: this setup lacks a TEST path, not a DEVELOPMENT path.**
  Working on an emulator and reaching for hardware only at the end is not a compromise forced by not
  owning a console — it was already the standard practice on this list in 2000. Lee Krueger, asked
  about making cartridges: *"I think any **new game development would be done on emus** and using the
  cart for **real hardware testing**"* 〔`200002/msg00048`, 2000-02-22〕, summarised the same evening
  by Dan Iacovelli as *"most people are using the emulation programs to develop the games and using
  the system for testing"* 〔`200002/msg00049`〕. So the shape of this repository matches
  twenty-six-year-old practice and the gap is precisely the last step. Describing it the other way —
  as though authoring without hardware were itself the deficiency — **understates what the era
  actually did** and points the remedy at the wrong end of the process.
  〔stella-list, *pixel smoothing (slightly OT)*, 1997; found by the mailing-list distillation
  (helper-2)〕
  - **Emulators that disagree are the thing to fix first, and a pass on hardware does not clear
    them.** tokumaru, 2020, to someone about to try a ROM on a console: *"If you're doing everything by
    the book and yet the program behaves differently on different emulators, it's better to sort that
    out before testing on hardware. Testing on hardware is more like a validation step, it will do
    little to help you figure out what you're doing wrong."* And the other direction: *"Even if your
    code appears to work as expect on real hardware as is, the fact that some emulators have issues
    with it could mean that something is ever so slightly off that it could still fail on hardware
    under certain circumstances."* He exempts code *"intentionally doing something experimental that
    relies on undocumented behavior"*. 〔mining 300206 rom-to-test-on-real-hardware〕 The first half is
    what `cmd/oraclevote` (Gopher2600 + MAME) does for RAM; the second says the missing test leg above,
    once added, is one console under one set of conditions (our reading). **Cited only, not
    verified.**
- **Flicker is a last resort, and only for short-lived objects.** Never over a large area. Don't trust the emulator — verify by compositing several frames. 〔flicker-to-enhance-graphics〕
  **This is a POSITION, not a measurement, and the list held the opposite one.** It rested on a single
  source and stated no cost for following it. Glenn Saunders, then the list's administrator, 1997:
  *"Oystron and Rescue avoid flicker but constrain sprite placement and movement to do it. That's
  fine, but it narrows the horizon of what's possible on the 2600."* — *"flicker is another 2600
  programming strategy and it opens up a lot of territory."* His exemplars were **Solaris** and
  **Star Wars: The Arcade Game**, *"paragons of intelligent sprite flicker and reuse."*
  **The default here stays "last resort"**, because this project's own recorded preference is to cut
  rather than to add and not to let a technique show — a taste decision, made once, not re-argued per
  page. What changes is that the alternative is now written down with its advocate, and that **the
  cost of avoidance is named**: refusing to flicker forces objects apart in Y, and that is a
  measurable narrowing (`pkg/design/multiplex.go`'s `NeedsEmptyYLane` is the existing foothold —
  how many placements survive the constraint is a number, not an opinion).
  Two things remain matters for the eye and not for this file: whether a given flicker reads as
  motion or as damage, and how large an area is too large. Found by the mailing-list distillation
  (helper-1), who flagged it as belonging to the artist rather than to the harness.
- **Flickering more than 2 objects: list reordering REPLACED age-based, and it costs priority.**
  The older way is **age-based** — count how many times each object has been shown and display the
  oldest next. The newer one is **list reordering**: try each object in FLICKERLIST order, move the ones
  you drew to the end and the ones you could not to the front, so next frame's order falls out of this
  frame's. **It abolishes the age loop rather than sitting beside it** 〔blog SpiceWare 10777:8〕.
  **The price is that index order stops being priority, so the player's own ship starts flickering** —
  which is the thing an author notices last and minds most. **Put both players fully into the flicker
  pool and you reach ~24 objects** (Frantic is the real example). The design layer above
  `flicker_multiplex`/`dyn_multisprite`. **Corrected 2026-09-03:** this line offered the two as parallel
  choices and carried neither the supersession nor the cost; both were in the cited note.
  〔mining blog SpiceWare 10777:8, 11656〕
- **76cy per line is the ceiling.** Decide the line count first, then allocate features out of the remaining budget. 〔splendidnut〕 `→ design.LineBudget/RemainingCycles`
  - **Write the window next to the write.** SpiceWare, to tschak909 (the copy here carries no date): *"When you update the
    various TIA registers can be time critical, so you should get in the habit of adding cycle counts
    to your kernel."* His example from *Collect* ends each line with comments like `; 3 46 -`, and:
    *"The first number after each ; is the number of cycles that instruction will take. The second
    number is cumulative cycles for that line AFTER the instruction finishes. Some lines have an @xx-yy
    to denote the acceptable cycles for updating a time-critical TIA register. Additionally, branches
    show the timing for when the branch is not taken, followed by parentheses containing the timing if
    the branch is taken."* 〔mining 253087 simple-kernel-for-playfield〕 His `PF1` windows read `@66-28`
    and `@39-54`; the first appears to run across the line boundary (our reading, **Not verified**). Hand counts are what rule 2 of `CLAUDE.md`
    says not to trust alone — `prove_line_budget` proves the totals over all paths and `beam_intervals`
    where each write lands, and an `@xx-yy` window is the claim to hold their answers against (our
    reading). **Cited only, not verified.**
  - **In a hand count, `sta WSYNC` belongs to the line it ends.** wickeycolumbus, 2013, giving the same
    habit (a cycle count on every instruction, and *"the total cycle count for the line every few lines
    of code, especially where you store to TIA registers"*): *"when you hit WSYNC in your kernel (ie STA
    WSYNC), the 3 cycles from the STA (zeropage) count for the previous line, and the processor is
    halted until the beginning of the next line, cycle 0."* 〔mining 220213 how-do-you-keep-in-sync〕 The
    engine's per-instruction counts put the stall on the instruction AFTER the strobe
    (`integration-density-playbook.md`, the `sta WSYNC` note), so a figure read off them for that
    instruction is not its share of the new line (our reading). **Cited only, not verified.**
- **★RIOT 6532 timer wrap-around bug (the "Stella passes / real hardware rolls" trap)**: write `TIM64T`/`TIM1024T` on **exactly the cycle** the timer wraps around and the divider silently degenerates to **1T**, wrecking the frame length so the picture rolls on hardware. **The fix = a double write (double-write TIM64T).** Easy to miss because it is emulator-dependent = a direct hit on the harness's core mission (gap B). Diagnosed in that thread by Gopher2600's author (JetSetIlly). 〔mining 303277 "To Roll or not to Roll"〕 (harness-hardening candidate = an assert that detects a timer write on the wrap-around cycle)
  - **The longest the timer counts in one run is about a fifth of a second.** Erik Mooney, 2001, asked
    for a timer of roughly one second that does not depend on the frame rate: *"Can't really do that on
    the VCS.  The TIM1024T timer counts down every 1024 machine cycles, and can be initialized with a
    value up to 255.  So the longest hardware timer you can get is only about 0.2 seconds."*
    〔`200107/msg00035`〕 1024 × 255 = 261,120 cycles, 0.219 s at the NTSC CPU clock of 3579545/3 Hz
    (our arithmetic). **Cited only, not verified** — the largest interval was not run here.
- **Hard lower bounds on the vertical budget, and asymmetric failure modes**: given that the total scanline count is held constant, the lower bound of each region = VSYNC ≥ 3 / Overscan ≥ 3 / VBLANK ≥ 15 (even for PAL). **Why this is yours to do at all** (added 2026-09-04): the designers removed it on purpose —
  *"They also eliminated any provision for vertical synchronization and gave that task to the
  programmer."* Of a piece with the rest of the machine: *"making the software do as much of the
  work as possible, so that the hardware could be cheaper — silicon was very expensive in those
  days."* Their statement of the budget is *"must finish displaying a single frame in exactly the
  same time — 15.24 milliseconds"*; that is their round figure and not the 2600's NTSC frame
  (3579545/228/262 = 59.92 Hz = 16.69 ms), so do not carry it as a constant. 〔Perry & Wallich, IEEE Spectrum 1983-03〕 **Overstretching Overscan = no picture / overstretching VBLANK = jitter** — the failures show up differently, so do not absorb the surplus on the VBLANK side (a jitter source). 〔mining 171270〕
  (Ambiguity resolved 2026-08-06. The Japanese original's word order read as a contradiction; the sentence
  above is the only reading its own two failure modes support, so nothing is left to decide about the WORDING.
  **The CLAIM, though, is not verifiable here and is not treated as measured**: "no picture" and "jitter" are
  behaviours of a real television, and an emulator shows neither — Gopher2600 renders an over-long Overscan and
  an over-long VBLANK alike. What this harness can see is the frame's line count, which is a different
  quantity, gated by `frame_lines_stable` and `TestNoRomBreathesAcrossFrames`. Cited to 〔mining 171270〕 and
  left there.)
  - **"Overscan" and "VBLANK" are the programmers' names, not the television's.** seagtgruff, 2016:
    *"What Atari 2600 programmers refer to as the "overscan" is actually the "vertical front porch.""*,
    and "vblank", as the community uses it, is not technically the vertical blank, *"because
    technically it's the entire blanking period of the vertical cycle-- i.e., the vertical front porch
    (which Atarians call "overscan"), the vertical sync, and the vertical back porch (which Atarians
    call "vblank" or "vertical blank")."*
    〔mining 250661 what-is-a-cycle〕 He points to "overscan underscan" for the television sense, which
    is the sense the pixel-aspect note in the craft section means by "overscan assumptions". Read a
    video source's word by its definition, not by this repository's region names. **Cited only, not
    verified.**
  - **192 is a survey of television sets, not a constant of the chip.** Glenn Saunders, 1999,
    answering whether any commercial game shows more than *"Atari's suggested 192 lines"*: *"Atari chose 192
    based on average vertical overscan on 1970s TVs. I even have Larry Wagner's measurements from when
    they went out and checked various models. But that was then, and this is now. Even then, they were
    pretty conservative. It sure seems as if today's TVs tend to show more of the picture than they
    used to. People designing games would be pretty safe if their game had 200 or even 205 visible
    scanlines centered on the TV."* 〔`199904/msg00002`〕 Brad Mott, in the same thread: *"It wouldn't
    surprise me if there are more that display more than 192 lines than display 192 lines. I'm sure
    Pac-Man uses more than 192 lines and I think Combat does as well."* 〔`199904/msg00003`〕 The
    frame's total is held by the bounds above; how much of it is picture is a margin set on 1970s
    sets (our reading). 200 and 205 are two of the line counts in the pixel-aspect note in the craft
    section. **Cited only, not verified** — no ROM's visible line count was measured for this.
  - **The total has a range too.** NoLand's summary of a Digital Press table of measured line counts,
    as the distillation notes record it: NTSC standard 262, minimum 238, maximum 290, median 262; PAL
    standard 312, minimum 258, maximum 336, median 312 — most games keep the standard, some, popular ones
    among them, depart from it significantly, and televisions tolerate considerable variation. 〔mining 303750
    inconsistent-scanline; only the distillation notes are held here〕 The same thread's requirement is
    that the count be the same in every frame, which agrees with `frame_lines_stable` asking for one
    value rather than 262 (`internal/emu/framelines_corpus_test.go`). **Cited only, not verified** —
    the table was not seen here, and no television was tried.
- **WSYNC semantics**: `sta WSYNC` halts the CPU until **the start of the next HBLANK** (68 colour clocks = 22⅔ CPU cycles). Choose where to write with the register-update delays in mind (colour = immediate / PF = 2-3 clocks / VBLANK = +1 colour clock / note length = delayed). 〔mining 192183 register-update delay table〕 The VBLANK figure is seagtgruff's, also in an earlier thread: *"Changes to VBLANK are delayed by 1 color clock."* 〔mining 131319 asymmetric-reflected-playfield〕 The bundled Gopher2600 does the same: `tia.futureVblank.Schedule(1, reg.Value)`, so a mid-line write blanks the rest of that line, not the next one. On real hardware: **Cited only, not verified.**
  **`WSYNC` inside a kernel line is spent cycles, not alignment.** Verdant, 2024, to bkumanchik, whose
  kernel strobed `WSYNC` between computing the invaders and the missiles: *"strobing WSYNC literally
  throws away processor cycles so that our program can continue execution when the TV's electron beam
  is at a known point. When you're running out of cycles to get everything done, it seems silly to me
  to potentially waste them like this. At the moment you're getting away with it because you're getting
  the ENAMx registers updated before the beam reaches them"* 〔mining 371548
  missile-help-im-trying-to-keep-my-missile-4-scanlines-tall〕. A picture that looks right shows the
  writes landed in time on the paths that ran, not that they always will; `beam_intervals` gives where
  each write can land over all paths. **Cited only, not verified.**
- State = one GameState variable + a kernel per state. A title picture is padding top and bottom + a central PF table, clearing GRP/PF at the end. 〔title-to-game-transition〕
  - **Clearing at the end is for the lines after the loop, not for the loop.** Andrew Davie, 2019, asked
    whether PF must be cleared at the end of the loop: *"Just make sure the PF registers (PF0, PF1, PF2)
    ALWAYS have correct data in them before the TIA uses them. So no, you do not need to clear. But you
    do need to make sure the correct data is there for the NEXT line before the TIA "gets there"."*
    (his bold dropped). nukey-shay, on the end of the loop: if it covers the entire visible display
    there is no need, since VBLANK blanks the rest, but after an empty loop of WSYNCs *"the uncleared
    registers will continue to spill whatever values were last written"*, and so will the top of the
    next frame *"until it encounters PF writes to alter the values"*. 〔mining 295514
    loop-counter-basics〕 **Cited only, not verified.**
- Cycle saving = the unofficial ISC/ISB opcodes + borrowing SP as a line counter (needs litmus backing). 〔5cycle-color-cycling, illegal-opcodes〕
  - **`SAX` applies one mask to several stores.** walaber, 2015, to mikes360, whose kernel ANDed one
    mask into three registers: *"I think you could also use the SAX command, which ANDs A and X and
    then stores the result. That way you only load the mask once and reuse it multiple times"*
    (`LDA (maskPtr),Y` / `TAX`, then `LDA (gfxPtr),Y` / `SAX ENAM1` …). Kylearan, asked whether it is
    stable: *"Yes, it's stable"*. mikes360: *"I have just given the sax a whirl and it saves me a total
    of 5 cycles. I need to store x and load it back again as I use it as an index for playfield
    drawing."* The next day the routine moved on: with the rail's data packed into one byte, the mask
    is applied once with `AND` and `SAX` is gone from it. 〔mining 233300 tricky-graphics-suggestions〕
    X has to be free to hold the mask, and saving it there cost part of the gain (our reading). `SAX`
    is in the stability map below. **Cited only, not verified.**
- **Stability map for illegal (unofficial) opcodes**: **the ones that are stable on real hardware are the LAX/SAX/SBX/DCP family**. ★**"Real hardware" here means original NMOS silicon.** A 2600 cartridge also runs on machines that are not 6507s at all, and AtariAge `113732-clean-assembly` reports **`SBX` and `ARR` failing on the Flashback 2** — a chip-level reimplementation. ★★We cannot check that (no Flashback 2 here, and the engine models a 6507), so it is the source's claim, not ours; but the map should not be read as "safe everywhere". ★★★The `ASR` row in `known-traps.md` already carries this kind of scope (*late Taiwanese Atari Jr*) and this one did not. Found by the mailing-list distillation 2026-09-05, cross-checking the two corpora against each other for the first time. **LXA/XAA are unstable = do not use** (they depend on the individual chip and on temperature). **`ASR`/`ALR` is NOT in the stable set** — the very source this line cites reports it **failing on official hardware**: late Taiwanese-built Atari Jr units, with Thunderground's score corrupting, and a second independent report (omegamatrix, on real hardware) says the same. The one byte and two cycles it saves are not worth a unit-dependent failure. Gate opcode-level code generation on this allow/deny table. That `cmd/cpucheck` finds the engine and the perfect6502 netlist agreeing on `$AB` (LXA) is agreement with one netlist, not with every chip (`capability-gap-audit.md`, VV-7).
  **Extended 2026-09-04 — the map was right and incomplete.** `definitions.json` carries a
  `stability` field on exactly 8 of 256 opcodes (it is `omitempty`, so an absent field means
  stable): **magic** on `$8B ane` and `$AB lax #imm` — the two this line already names as XAA/LXA —
  and **unstable** on six more that the map never mentioned, all of them *stores*:
  **`$93 sha (zp),Y`, `$9B tas abs,Y`, `$9C shy abs,X`, `$9E shx abs,Y`, `$9F sha abs,Y`,
  `$BB las abs,Y`**. They AND the high byte of the target address into the value, so what they write
  depends on where they write; treat them as denied.
  One sharp edge inside the map's own wording: **`LAX` and `LXA` are the same mnemonic in this
  table.** Seven `lax` entries exist and six carry no stability field; the seventh is `$AB lax
  #imm`, which IS `LXA` and IS magic. "The LAX family is stable" therefore holds for every
  addressing mode **except immediate**, and a generator that reads the family name rather than the
  opcode will emit the one unstable member. `SAX` (4), `SBX` (1) and `DCP` (7) carry no stability
  field in any mode, so the rest of the line stands. Counted from
  `Gopher2600/hardware/cpu/instructions/definitions.json` by the mailing-list distillation
  (helper-1) and re-run here independently. 〔engine instruction table〕
  **A shipped homebrew used `LAX`, and which one is an open question this repository cannot close.**
  Andrew Davie's release note for Qb v0.04, quoted on the list (`200102/msg00205`): *"Mac users
  should recompile the source, exchanging all **"lax"** instructions with **"lda"** — this will give
  a buggered score display, but that's all that will be different."* So the instruction was load
  bearing in released code, and swapping it degraded exactly one thing. **If those were the
  addressed forms the map is confirmed by practice; if any was `$AB` (immediate) the map says magic
  and the note says it shipped anyway, which would be the more interesting outcome.** It cannot be
  settled here: the source was a list attachment (other people's commented assembly — outside the
  clean-room line, not opened) and **no Qb ROM exists in this tree** — checked, 319 `.bin` images
  under `reference/`, none of them Qb. A raw byte scan would not settle it — `$AB` as data
  is indistinguishable from `$AB` as an opcode — but **disassembling from the entry point would**, and
  this tree has the machinery: `Gopher2600/disassembly`'s `FromCartridge`/`bless` follow flow and
  separate code from data, and `cmd/dissect` drives them. **The method exists; the ROM does not.**
  (Corrected the same day: this line first said a byte scan settles nothing and stopped there, which
  understated what is available — helper-1 caught it.) With a `.bin`, this is a question that can be
  closed without opening anyone's source.
  Recorded as an open question with its falsifier named. Found by the distillation (helper-1), who
  declined to quote it as evidence for the same reason. 〔mining 168616 illegal-opcode stability (ASR caveat in the same note); 294471 §32 for the independent second report〕 **Corrected 2026-09-02**: this line previously listed ASR as stable and claimed it was "already used in 48px / dyn_multisprite". Both were wrong — those three ROMs use no illegal opcode at all, and no ROM in the corpus uses ASR/ALR (measured with two structurally different expressions, both exit 1). `scripts/check_traps.py:73` had already omitted ASR from what it recommends, so the docs were the outlier.
  - **The immediate form is said to be stable with one argument: zero.** omegamatrix, 2014, reviewing
    a start-up routine that wrote `LAX 0`: *"This translates to LAX CXM0P. There are a lot of undefined
    bits in there. It's better just to go with LDX #0, and TXA. There is another way to save that byte
    (LXA #0), but it LXA unstable."* enthusi, the next day: *"LXA/LAX ist 'stable' (from the result's
    point of view) when (and only then afaik) the argument is #00."* 〔mining 222981
    3d-fps-engine-for-the-2600; "ist" is his spelling〕 With the argument zero the AND leaves nothing
    for the chip-dependent part to change (our reading), but the engine's table marks `$AB` magic
    whatever the operand, and his "afaik" stands. omegamatrix's first point is a separate trap: without
    the `#`, `LAX 0` is a zero-page read of address `$00`, which on a read is the TIA's `CXM0P`.
    **Cited only, not verified.**
- **The resource triangle + a register convention**: RAM (128B) / CPU (76cy) / ROM are mutually exclusive = growing one shrinks the others (plus the human cost). The Thomas Jentzsch convention = inside the kernel, pin the roles to **Y = scanline and sprite index, X = PF, A = everything else** and it runs faster. Use subroutines for code reuse only (the call cost is high). 〔mining 146817〕 The same convention, older and with its reasons, from Thomas Jentzsch on the list in 2003: *"I usually use Y for graphics (lda (ind),y) and in parallel for scanline counting, X for the PF (normally also having a vertical lower resolution) or other things (like stack manipulation or to store temporary results, A is for multi purposes."* 〔`200303/msg00381`〕 Y is the register the graphics read needs — the 6502's post-indexed indirect mode exists only as `(zp),Y` — so it is also the line counter; X goes to the playfield because the playfield usually changes on fewer lines (our reading of his parenthesis). **Cited only, not verified.**
  - **The three sides are not equally for sale.** Kirk Israel, 2003: *"given my simple game concept, RAM and ROM are cheap compared to
    kernal time, but that might be true everywhere. (which is funny, because while a supercharger
    multiload might give you effectively more RAM and more ROM, I don't think anything ever gives you
    more time for your kernal.)"* 〔`200309/msg00067`; "kernal" is his spelling〕 The line stays 76
    cycles whatever the cartridge; what later hardware sells is fewer cycles per job inside it — the
    DoDraw → DPC → DPC+ → bus-stuffing ladder under Sprites. **Cited only, not verified.**
  - **Plan the cuts before the ROM runs out.** Andrew Davie, 2001, on *Qb*: conditional-compilation
    switches added for *"Feature-reduction (for later tweaking)"* — rising player, the player's hair,
    hat, score protection, attract mode — because *"I'm just about completely out of ROM. ... it will be
    a matter of picking features and deciding which need to stay and which go."* In the same build the
    player became coloured, and *"of course it came down to a single cycle being available"* 〔`200102/msg00235`〕.
    `integration-density-playbook.md` rejects compile-time "feature modules" because they yield N
    binaries; here the switches are a cut list for one build (our reading). **Cited only, not verified.**
  - **The bytes an optimisation pass finds are paid for in structure.** Davie's *Qb* post-mortem,
    2001: *"It constantly amazed me how, whenever I really needed to squeeze some space out of the code
    (which was just about every day for the last three weeks), I could find almost as much space as I
    needed.  I recall being jam-packed, then saving 70 bytes, then 'spending' it, then saving another
    190, then spending that, then finding another 100... etc.   I guess each iteration the code was
    becoming more and more esoteric and less structured."* And afterwards: *"I'd recommend saving 500
    bytes JUST for the finishing touches that everyone requests."* 〔`200103/msg00196`〕 The Combat rule
    near the end of this file says the same of RAM — packing *"spends clarity"* (our reading). **Cited
    only, not verified.**
  - **A macro is one line in the source and every instruction it expands to in the budget.** Manuel
    Rotschkar's 2003 `SET_POINTER pointer, address` stands for `LDA #<address` / `STA pointer` /
    `LDA #>address` / `STA pointer+1`; its header notes *"Alters the accumulator"* and gives no cycles
    or bytes 〔`200311/msg00089`〕. With a zero-page pointer that is 10 cycles and 8 bytes (our arithmetic:
    2+3+2+3 cycles, 2 bytes each). Count a kernel line from the expansion, not the source. **Cited
    only, not verified.**
  - **"RAM is faster than ROM" is true of one addressing mode.** Thomas Jentzsch, 2023: *"Loading from
    zeropage RAM is faster, not RAM in general. But only for non-indexed loads."* and *"There is no
    opcode for lda zp,y. Instead the assembler creates lda abs,y, which requires 1 extra byte. But
    there is an opcode for lda zp,x."* MarcoJ: *"loading with indexes is going to cost at least 4
    cycles. The only saving with zeropage RAM is a direct, non indexed load (3 cycles)."* 〔mining
    352846 faster-to-load-from-ram-or-rom〕 The asker settled on direct zero-page references, *"3 zp
    direct loads and 4 tia stores for 21 clocks"* (3×3 + 4×3). So copying a table into RAM buys
    nothing for an indexed read; it pays when the kernel can name each byte. One difference does remain
    for indexed reads (our note from the 6502's addressing, not the thread's): `zp,X` wraps inside page
    zero and never pays the page-cross cycle that `abs,X` can — the `abs,X` side is machine-locked by
    `TestPageCrossPenaltyRules`. **Cited only, not verified** for the zero-page counts.
    The store has the same gap, and Jentzsch gave it as a habit in 2001, reviewing *Qb*: *"the 6502 has
    no opcode for sta ZP,y, so the compiler uses sta ABS,y instead, which costs one byte. I'm always
    trying to use x for zero-page RAM access and y for reading ROM tables. Look at the DASM listing file
    (just search for 00 in the 3rd opcode column) and you might spot other places"* 〔`200103/msg00260`〕.
    The engine's instruction table agrees: `STA` has `$95` zero-page,X (2 bytes, 4 cycles) and only
    `$99` absolute,Y (3 bytes, 5 cycles); of the documented stores, only `STX` has a zero-page,Y form
    (`$96`), and the undocumented `SAX` has `$97` (`Gopher2600/hardware/cpu/instructions/definitions.json`,
    read 2026-10-02). It is the same
    division as the kernel roles above — Y for the graphics read, X elsewhere — with the byte as the
    reason (our reading). **Cited only, not verified** for the habit and the listing search.
  - **A table can be traded back.** David Galloway, at the turn of 2005, on Manuel Rotschkar's *Crazy
    Balloon*: *"when you are done and have any cycles plus a couple of bytes of RAM you can replace
    this `LDX divideby3,Y` with `LDX div3`"*, plus a down-counter that decrements `div3` on every third
    pass of the loop, and *"I know I'm going to get dinged by Thomas for optimizing too soon, but
    that 156 byte divideby3 (actually by 6) table was pretty tempting."* 〔`200501/msg00000`〕 Rotschkar: *"what you
    suggest is more or less where I came from"*, with his original counter, `INC`/`LDA`/`EOR #3`/`BNE`/
    `STA`/`DEX` 〔`200501/msg00001`〕. So the table had replaced a counter, and the counter was offered
    back for the table's 156 bytes. The positioning rule's *"turn the ÷15 into a lookup
    table"* is one direction of this trade; the price of the table is ROM (our reading). **Cited only,
    not verified** — no cycles or bytes were counted here.
- **Packing a table pays only when the table dominates.** Ed Fries fitted a 26-letter 3×4 font into 28
  bytes — letters overlapped vertically, three to a byte horizontally, unpacked by mask-and-shift — and
  answered his own question: *"Is it useful? Not really because you probably waste more code space
  dealing with the compactness of the font than you save in the storage of the font"* 〔mining 204475
  what-is-it〕. Count the decoder against the saving before compressing; for scale, the 4×5 font in
  `techniques/text12.md` is 200 bytes stored plainly. **Cited only, not verified.**
  - **Packing need not slow a lookup if the key is packed the same way.** Karl G, 2022, on the
    word dictionary of *2600 Words*: *"I was thinking that compressing the dictionary down to two bytes
    would mean slower word validation, since I'd have to perform a number of bit-masking and
    bit-shifting operations to compare with the word in RAM. It just occurred to me that I could
    instead compress the word to be checked down to two bytes, and then compare those bytes directly
    with the compressed words in ROM. Instead of slowing down the process, it will actually speed it
    up!"* 〔mining 331117 2600-words-wordle-clone〕 The two bytes hold a word's last three letters; the
    first two select a table (reveng counted 310 pairs in the 10k list, *"So no one table is larger
    than your bank size"*). When the encoding is a function, the lookup never runs the decoder above
    (our reading). **Cited only, not verified** — he reports no timing.
  - **One saving can foreclose the next.** Paul Slocum, 2001, to Erik Eid, out of ROM on *Euchre*:
    *"you can combine a few letter graphics like "U" and "H", or "I" and "T"."* Eid: *"I'm not sure I can
    use this since the offsets into the letter image table are calculated as multiples of six. I
    wouldn't be able to do this without creating a table of offsets. (I used to have one but took it out
    earlier to free up space. :) )"* 〔`200111/msg00344`〕 Overlapping glyphs need arbitrary start
    offsets; a fixed stride gives the offset table up, and with it the overlap. The exerciser's score
    font, `nibble×16` into a 16-byte-stride font (`exerciser.md`), is the same choice (our reading).
    **Cited only, not verified.**
  - **The range of the values can be read off the table once it is written.** walaber, on *Ninja Wall
    Jump*, out of ROM again, on movement tables indexed by a 0–3 frame counter: *"I arranged it in
    4-byte sections in order to understand visually how things will move. But looking at the data I
    realized I never move something more than 3 pixels in a given frame. This means I could combine all
    4 values for a single row into just 1 byte, by using 2 bits per value"*. *"I was worried that the
    additional code I'd need to unpack the data would offset the savings, but it was actually a new gain
    both on the data side and also on the code side."* *"In total this saved me ~50 bytes"* 〔mining
    232200 ninja-wall-jump-game-wip〕. Against Fries's font above, the decoder here was cheaper than
    the code it replaced; the width came from the largest value in the data, measured after the table
    was laid out for reading (our reading). **Cited only, not verified.**
- **Adventure's 255-room ceiling is the width of a byte, not of the machine.** On a 2011 thread about a
  multi-load RPG, syntaxerror999 took Adventure's rooms to be capped *"(255 I think)"*. e1will: *"the
  255-room limit in Adventure is fairly easy to overcome. I'm currently working on a 300+ room
  version"*; gemintronic got *"quite a bit more rooms than 200"* from pseudo-random generators; kiwi:
  *"RoomID = 0-255. By adding another byte would increase the number of room to 65536 different
  rooms."* 〔mining 178139 atari-rpg-idea〕 A second index byte costs one byte of RAM; the rooms
  themselves still cost ROM unless generated, reproducibly (the "random" rule in the Multiplexing
  section). **Cited only, not verified.**
  - **The ceiling can sit in the table that draws the value.** nukey-shay, 2012, on raising two
    games' starting lives from 3 to 9 by changing one ROM byte: in *Space Invaders*, *"This will glitch
    the # remaining display (when starting a wave or life), tho...since the lookup table only holds the
    gfx pointers for digits 0-3"*; in *Pac-Man*, *"Should be no problem here, since the game allows the
    reserve life counter to go up to 9."* 〔mining 197779 rom-hacking〕 The same 3→9 passes in one and
    breaks in the other: the variable could hold 9 and one digit table could not. Size a counter's
    display table with the counter. **Cited only, not verified** — his reading of the two ROMs;
    neither was looked at here.
  - **The ceiling can sit in how a table is laid out.** bogax, 2020, suggesting a shorter playfield
    routine for *Polarium*: *"this needs an extra byte per for the x 11 table and assumes col0 data is
    all in one page so 256/11 = 23 levels max"* 〔mining 307123
    need-some-help-with-getting-rid-of-excess-code; only the distillation notes are held here〕. 256/11
    is 23.3, so 23 (our arithmetic). The score kernels keep a font in one page for speed (no page-cross
    penalty, `techniques/score-kernel.md`); here the same choice sets how many levels the game can
    have. **Cited only, not verified.**
- **Bits outside the 128 bytes exist, and most cost more than they store.** A 2018 thread listed them
  after a claim that games kept data in offscreen scanlines — they cannot: the TIA's registers are
  either read-only or write-only (nanochess, Thomas Jentzsch), and nothing drawn comes back. reveng, to
  omegamatrix: *"I recall that you cleverly repurposed the 6507 interrupt flag for the star-map display
  state, with your Starmaster 2-button hack. So long as you stay away from adc/sbc you could do the same
  with the decimal mode flag."* Also *"the RIOT port bits"*, and the paddle capacitor as a flag, *"but
  without a resistor/paddle you wouldn't be able to unset it"*. nukey-shay: *"The stack pointer is
  another byte, and T1024T can be used as temporary storage of another byte if your program is quick
  about it"* — and *"None of it really matters, tho...since a cartridge can just include it's own Ram
  memory scheme"*. jeremiahk once set collision latches on purpose to save one byte in a score kernel:
  *"a major pain in the neck"*. 〔mining 279317 using-overscan-to-store-data〕 omegamatrix, 2012, of the
  same timer: *"using TIM1T as a storage container"*, a trick first used in the game his seven-digit
  score display came from 〔mining 198217 7-digit-full-sized-score-display〕. eshu, 2012: *"SEI and
  CLI gives you an extra bit of storage - things are that tight on the 2600 that it's actually
  handy!"* 〔mining 193203 6507-opcodes-crossword-puzzle〕 The I flag is free because the 6507 has no
  interrupt pins 〔mining 188134〕 and BRK ignores it (the IRQ/BRK rule in the Combat section). Reading
  either flag back takes a `PHP` and a test of bit 2 (I) or bit 3 (D) — the bits the engine's status
  register uses (`Gopher2600/hardware/cpu/registers/status.go`) — and while a bit is parked in D,
  `ADC`/`SBC` are off limits (reveng's condition). On a 7800, where the worry was that the I flag is
  fixed, batari reported using it to tell a cold start from a warm one in *Stella's Stocking* and that
  it worked on several 7800s, and bruce-tomlin that a 7800 has already locked out its own mode when a
  2600 game starts 〔mining 160610 making-use-of-the-status-register; only the distillation notes are
  held here〕. **Cited only, not verified** — none of these has been built here, and the 7800 is not
  modelled here.
  - **An upper bound, counted once and never built.** Thomas Jentzsch in the same thread: *"I wonder
    about the maximum storage in TIA"* — 20 bits of playfield, 32 from GRP0/1 with their shadow
    registers, 4 ENAM, 2 ENABL, 20 HM, ~37 from object positions, 4 CTRLPF (*"I don't think it is
    possible to check the score bit"*), 10 NUSIZ, 2 RESMP (he writes "RESPM"), 2 REFP, maybe 1 VSYNC and
    3 VBLANK: *"That's 137 bits in total. Minus the bits required for the object to check collisions
    (e.g. BL). So maybe 125 bits or 15.x bytes."* The terms do sum to 137 (our check). Reading any of it
    back goes through the collision latches, and DirtyHairy costed the playfield share: 20 bits read by
    all five objects spaced 16 pixels, in five lines, but *"you lose about 350 bytes of ROM for the
    readout code, and you cannot use playfield or ball while data is stored"*. 〔mining 279317〕 **Cited
    only, not verified** — a thought experiment; nobody in the thread built it.
  - **The address a routine is called from can carry a parameter.** alex_79, 2016: a 4K cartridge
    ignores A13–A15 (*"those pins aren't present at all on the 6507"*), so its ROM appears at eight
    places in the 16-bit space, and a 2K one also ignores A11, sixteen. Then: *"Jump to a mirror address
    according to the parameters that you need to pass before calling the subroutine and it can test the
    return address in the stack. (the 6507 is a full 6502 internally, so the program counter is 16 bit
    wide and all bits are stored in the stack when executing a jsr)"*. omegamatrix: *"The three highest
    bits of a pointer are also useful for temporary storage during the kernel since they will just
    access a mirrored address. They are especially great for branching with BIT tests (BMI, BPL, BVC,
    BVS)."* 〔mining 253796 reset-vector-with-2k-rom〕 ZackAttack's *"A 2k ROM mirrored to 4k could use
    bit 12 of PC"* 〔mining 279317〕 is the same idea; by alex_79's map the free line on a 2K image is
    A11 (A12 selects the cartridge), so his "bit 12" is that line counted from one (our reading). The
    2K mirror is the one the IRQ/BRK rule in the Combat section depends on. **Cited only, not
    verified.**
  - **A game with no zero-page RAM was called possible, and one was built.** omegamatrix, 2012:
    *"A game that uses no ram is possible. To be fair I would say no zero page ram"*, listing what is
    left — *"Accumulator / X register / Y register / Stack pointer / Timer (all modes) / SWCHB (up to
    eight bits) / the Interrupt bit in the status register / All collisions (15 bits) / REFP0 and REFP1
    (1 bit each) / the GRPx registers, ENxx registers"* 〔mining 197100 asm-or-bbasic; only the
    distillation notes are held here〕 — which `SWCHB` bits are safe is under *SWCHB's unused bits as
    storage* in `techniques/kernel-micro-idioms.md`. *Ramless Pong* (jeremiahk, 2019), as the notes record it, keeps
    its state in X and the stack pointer, parks one in `T1024T`/`INTIM` while the two are swapped twice
    a frame, and holds the ball's horizontal direction in the I flag, read back through a collision
    〔mining 291778 ramless-pong; only the distillation notes are held here〕. **Cited only, not verified**
    — none of it was built here.
- **Only the code that races the beam has to be assembly.** SplendidNut's 4K Frogger without flicker, 2024: *"Most of
  the project is written in C. Only the score kernel, the frogs-at-home kernel, and the repositioning
  routines are written in ASM. I'll probably convert the other kernels over to ASM so that I can utilize
  early-HMOVEs to hide those black bars on the far left."* 〔mining 364255 a-remake-of-frogger-in-4k〕 The
  split is by cycle-counting, not by importance — and a cosmetic (the comb, in the Sprites section) was
  enough to move more code across it. **Cited only, not verified** — 4K and "without flicker" are the
  author's.
- **The canonical kernel vocabulary** (Andrew Davie): "**N-scanline kernel**" (one picture row = N scanlines) plus 4 shape axes = sprite spacing / PF spacing / symmetry (sym/asym) / reflection (mirrored). Adopted as the harness's internal kernel vocabulary. 〔mining 320714〕
- **Movement = fixed-point subpixels**: hold position as 8.8 fixed point and add `vel` every frame → the carry moves the integer part = smooth slow motion, friction, gravity and wind in one framework. **A parabola = constant velocity in X × constant acceleration in Y** (no trigonometry). Enemy chasing = proportional homing from the sign-shift of `(target−pos)/16` (no division; 16 directions = octant + slope threshold — **but a signed shift is not free**: keeping the sign through `(target−pos)/16` costs a `cmp #$80` before each of the four `ror`s, so "no division" means four extra instructions, not none 〔107024:16〕). 〔mining 178177, 270373, 107024〕 (technique-candidate ㉕)
  - **Direction and speed in one byte, used as a table index rather than decoded.** SpiceWare, on
    *Medieval Mayhem*'s fireballs: *"I use a single byte to denote speed and direction, SSSDDDDD"* — 32
    directions, counted clockwise from up. Asked by gauauu whether that needs code to translate: *"No
    need to decode DirSpeed, it's used to index values in movement tables. The code is basically Y = Y +
    MoveTableY[DirSpeed] and X = X + MoveTableX[DirSpeed] for 16 bit values"*, which is *"4 tables with
    256 bytes in each table, so 1K of data in a 32K game"*. To keep a fireball out of a loop he added
    or subtracted 1 from D at random on a bounce 〔mining 254675
    doing-some-tank-pong-like-logic-for-ballmissile-help〕. The decoding is paid for in ROM rather than
    in code. **Cited only, not verified.**
  - **Fixed-point motion or a table of positions is a ROM question, and it can come out even.** Rob
    Kudla, 2000, on making his Boing demo's ball bounce: *"which would be more memory intensive - a
    vertical position table of say 64 bytes (read forward and reverse obviously for a total of 128
    frames between bounces …) or the code required to implement that algorithm?"* 〔`200001/msg00008`〕
    — the algorithm being Clay Halliwell's second byte of *"fractional accuracy"* with a velocity and a
    constant acceleration 〔`200001/msg00007`〕. Halliwell's answer: *"It would certainly be more than a
    64-byte table... in addition to the position, you need to store how long to hold the ball at each
    position"*, unless the table moves one line per entry with special-cased delays; *"You'll still
    have to generate this table programmatically though, or it'll look terrible. I agree that within
    the limited vertical space available, it'll probably come in pretty close between a table and an
    algorithm."* 〔`200001/msg00011`〕 Count both before choosing, as the packing rule above does for a
    font. **Cited only, not verified** — the thread does not say which he built.
  - **The same parabola can make the trigonometry instead of avoiding it.** Kylearan, 2017, to
    someone whose 255-entry sine table took *"a large chunk of my code"*: *"If you only need an
    approximation and can get away with a small deviation from a "real" sine function (I often can),
    you can also use parabolas to generate sine tables in assembly"*; and *"Of course RAM to store the
    table is a problem here, but you only need half a sine table (or even only a quarter) and can
    mirror the other half, and also a shorter table (128 or even 64 bytes for the full sine table)
    might be sufficient."* 〔mining 265153 using-sine-and-cosine〕 Above, a parabola is a path that
    needs no sine; here it is the sine, generated into RAM for some loss of accuracy (our reading).
    How much loss, for the textbook parabola `4x(π−x)/π²` over a half period (the thread does not say
    which parabola): at most 0.056 of the peak, near x = 0.15π — about 7 counts at amplitude 127 (our
    arithmetic, a million-step sweep).
    **Cited only, not verified** — no sine table is generated here.
- **×2^n on a small signed value = repeated `asl` (no multiply, sign preserved)**: in two's complement `asl` is exactly ×2, so a signed velocity such as BallDY becomes ×2^n with n `asl`s (e.g. the lookahead target = BallRow + 4×BallDY = two `asl`s + one `adc`). But (a) **the result's range widens → bit7 can no longer serve as the sign test** = do clamp/wrap tests on the value range instead (if the extrapolated target maxes out around ~190 the threshold is `cmp #220`; an application of known-traps' "bit7 clamping is not usable"), and (b) an input that overflows into bit7 during the shift (|value| × 2^n ≥ 128) destroys the sign = check the input range first. 〔in-house: PONG ai-variants v3 lookahead 2026-07〕
- **A BCD score can be compared with `cmp` without decoding**: for a valid packed BCD byte, binary ordering = decimal ordering (the upper nibble dominates) → both `cmp #$11` (first to 11 points) and a ScoreR vs ScoreL comparison are correct as written. But **a binary difference is not a decimal difference** (it inflates by +6 across a digit boundary: $10−$09 = 7) → when the difference is used as a QUANTITY, bucket it with saturation so the coarseness is harmless (v4 rubberband's score difference → error-width modulation). The bit7 sign of a subtraction is valid only while |binary difference| < 128. 〔in-house: PONG ai-variants v4 2026-07〕
  - **Read the encoding before applying a peephole: only `ADC`/`SBC` count in decimal.** Thomas
    Jentzsch, 2002, reviewing *Space Treat* for bytes, asked why `LDA LevelCounter+1` / `CLC` / `ADC #1` /
    `STA LevelCounter+1` did not use X or Y (*"That would save you two bytes."*). Manuel Polik: *"Uhm...
    maybe because it's BCD?"*, adding that from the snippet alone *"I'd otherwise even dare suggesting
    INC LevelCounter+1"* 〔`200212/msg00349`, `200212/msg00352`〕. On a BCD byte the rewrite is wrong: the
    bundled engine's `INX`/`INY`/`INC` are a plain binary add, while its `ADC` takes a decimal path when
    D is set (`Gopher2600/hardware/cpu/cpu.go`), so `$09` steps to `$0A` (our reading of the engine, not
    run). Here the guess was wrong — the author wrote that BCD math was something *"I'm currently not using"*
    〔`200212/msg00359`〕, and his code that Thomas quotes next resets the same counter at `CMP #10` —
    so the peephole would have been fine. **Cited only, not verified.**
- **★TIA revision differences are a trap when cross-checking against hardware**: HMOVE's "extra clock" effect (the Cosmic Ark stars) **reverses behaviour on post-1989 TIAs**, among others — **the same ROM produces a different picture per revision**. The harness's pixel comparison must **pin the TIA revision / emulator** before comparing (and record which revision the verification used). 〔mining 191061 Cosmic Ark stars〕
  - **The shift is 17 px a line, not the 15 usually quoted, and that is on an ordinary TIA.** Crispy,
    who re-implemented the TIA on an FPGA: *"the NTSC TIA produces 68 pixel clocks of horizontal
    blanking. This translates to 17 extra clocks per line, and so the shift is actually 17 pixels"* —
    one extra clock every four system clocks, over HBLANK's 68. He also shows the 1-1-2-0 width pattern
    breaking where a pixel straddles blanking and active video. 〔mining 261596
    cosmic-ark-star-field-revisited〕 This is the base effect, a different layer from the revision
    difference above. **Cited only, not verified.**
  - **The mechanism of the revision difference, as Eckhard Stolberg recalled it, and which objects it
    moves.** Stolberg, 2004, answering a test Thomas Jentzsch posted for "Kool-Aid-Man-compatible"
    consoles: *"It's about triggering HMOVE in such a way that some of the positioning clock pulses for
    the players get generated during the visible part of the screen. This creates the extra shifts for
    the player graphics on the China-TIA but not on the normal TIA."* *"I don't think that there were
    different designs for the TIA. It seems to be more of a manufacturing difference for a certain
    production run of the TIA chip that led to a slight timing difference on some of the clock lines.
    AFAIK only the very last 2600 Jrs and 7800s, that were produced in China after late 1989, are
    affected by this."* *"IIRC the problem is that some of the clock lines are used for different
    purposes during the visible part of the screen and during the horizontal blank. The HMOVE
    positioning clock pulses are supposed to happen only during the horizontal blank."* On a normal
    TIA the extra pulses have no effect, *"but on the China-TIA they seem to be delayed enough to
    overlap with the pixel counter pulses"*, the two pulses are seen as one, and *"therefore the players
    will be shifted to the right by one pixel"*. *"AFAIK only the player graphics are affected by this.
    The missiles and the ball are positioned the same on all TIAs."* 〔`200410/msg00080`〕 He describes a
    one-pixel shift of the players, narrower than the parent line's "reverses behaviour", and the
    one-pixel difference between consoles in `techniques/restrobe-copies.md` has the same size (our
    reading). **Cited only, not verified** — his own "IIRC" and "AFAIK"; nothing was run here.
- **Minimum-byte initialisation + hotspot placement**: in a tight 2K/4K, Omegamatrix's 8-byte self-modifying init (`bne .loop+1` jumps between operator and operand → `#$0A` executes as an ASL) yields A=0 / X=0 / SP=$FF / carry clear. Put bank hotspots **at the highest addresses (near the already-used interrupt vectors)** and the free chunk is maximised (a ZP hotspot = Tigervision 3F saves 1 ROM byte + 1cy per switch). 〔mining blog 12061, 11811〕
  - **The RIOT's `$02xx` addresses give the same trick a table.** omegamatrix, on AtariAge: *"The $2xx
    registers (TIM64T, INTIM, etc...) can be jumped between operator and operand in some cases for new
    instructions"* — the low address byte runs as an opcode and the `$02` after it as its operand:
    `SWCHA`/`SWCHB` → `nop #$02` (*"waste 2 cycles"*), `SWBCNT` → `sax (WSYNC,X)`, `INTIM` → `sty WSYNC`,
    `TIMINT` → `sta WSYNC`, `TIM1T` → `sty WSYNC,X`, `TIM8T` → `sta WSYNC,X`, `TIM64T` → `stx WSYNC,Y`,
    `T1024T` → `sax WSYNC,Y`; *"Very limited use, but interesting."* He also noted that `jmp (INTIM)`
    always lands in zero page or a mirror, since the high byte read is `TIMINT` — an idea he *"wanted to
    try at one point"* 〔mining 215833 the-mysterious-bit-instruction〕. Each decode matches the bundled engine's opcode
    table (`Gopher2600/hardware/cpu/instructions/definitions.json`: `$80`/`$82` nop immediate,
    `$83`/`$97` sax, `$84`/`$94` sty, `$85`/`$95` sta, `$96` stx,Y) against the register map in
    `cpubus.go` (our check, by reading). **Not verified** — none of these jumps was run.
- **Code that changes the memory map must run from memory that every map contains.** Atari's 7800 BIOS,
  in the routine that starts a 2600 cartridge, quoted on AtariAge by GroovyBee in 2011: *"This code must
  all run out of 6532 RAM since the memory map will change as we change mode. The 6532 RAM exists in all
  memory maps."* (a source comment; its `;` line markers dropped) 〔mining 184535
  curious-about-some-code-in-the-7800-bios〕 A bank-switched 2600 cartridge faces a comparable problem,
  and `techniques/bankswitching.md` answers it by putting the switching code at the same address in
  every bank, with the stack in RAM untouched; the comparison is ours, the source speaks only of the
  7800. **Cited only, not verified** — the 7800 is not modelled here.
- **★The "lodging" pattern for physics lines (sharing a WSYNC line between mutually exclusive paths)**: splitting the Overscan physics into "one concern = one WSYNC line" runs out of lines, but **paths that are mutually exclusive within the same frame (normal / hit / miss / frozen …) may use the same line for different purposes** —— each path strobes line N's WSYNC itself and only the contents of the line are swapped (e.g. line 3 = paddle input normally / english computation on a hit / serve handling on a miss). Work that gets skipped (a frame's worth of paddle input not being applied, say) merely means "drawn with a value one frame old" = an invisible compromise. Keep **the total line count identical on every path** (offset the variable part with the number of filler lines). When a feature addition inflates the budget, first ask "which path is it exclusive with", and consider lodging before adding a dedicated line. Housekeeping that must run every frame (LFSR / counters / note length / switch polling) is safest gathered on **a dedicated line where all paths converge**. 〔in-house: PONG pf2 physics-line architecture 2026-07-02–03 (serve lodging → generalised to hit/miss → new line 5)〕

- **★The simplicity of the RULES says nothing about the cost of the KERNEL, and the escape is to
  split by EVENT rather than by space.** Two messages from a 2002 thread on putting Battleship on the
  machine. Mark De Smet, on why a trivial game is not a trivial kernel: *"Don't forget that this means
  you have to draw a **10x10 grid with mulitple arbitrary objects/placement**. Not impossible (see
  video chess), but **more involved than it may appear given the simplicity of the game**"*
  〔`200203/msg00066`, `mulitple` is his spelling〕. ★★Glenn Saunders answered with the way out:
  *"I would **only draw the pegs, not the ships**. So that's **playfield only**. When a ship gets sunk
  I could **temporarily drop the playfield out to display just that ship**"* 〔`200203/msg00067`〕.
  ★★★**Two configurations, chosen by what is happening rather than by where on the screen you are.**
  The steady state draws the one thing the playfield is naturally good at — a coarse grid — and the
  rare event borrows the entire screen for a single object. `zone-multiplexing.md` divides a frame by
  **space**; this divides it by **time**, and the budget it buys is the whole line rather than a band
  of it. The price is that the two configurations must not both be needed at once, which is a rule
  about the game, decidable before any code exists.
- **★The lodging pattern has an OBJECT version, and the choice it forces is about feel, not bytes.**
  The same reasoning that shares one WSYNC line between mutually exclusive code paths shares **one
  object slot** between mutually exclusive events. Piero Cavina, 1997, on a game with a single
  explosion object: *"when something is hit, you remove it and add an \"explosion-object\"; since
  there can be **only one of these on the screen at once**, if you hit something else while the
  explosion is still around, it is **immediately replaced** by the new explosion"* 〔`199712/msg00039`〕.
  ★★He then names all three policies and rejects the one he shipped: *"I don't like this very much, I
  think it would be better to **always let the explosion finish**, and what is hit before the end of the
  explosion just disappears. Alternatively, you could handle an **array of explosions** - but this can
  be a real waste of RAM since you've to store all the information (position, status…) for **each**"*.
  ★★★So the slot is not a limitation to route around — it is a **design decision with three answers**:
  **newest wins** (the hit you just made always shows), **oldest wins** (an animation is never cut
  short), or **an array** (correct, and paid for in RAM per instance). The first two cost nothing and
  feel different; only the third costs bytes. Decide it deliberately rather than inheriting whichever
  one the obvious code produces.
- **★The lodging pattern has a RAM version: a graphics pointer is only a pointer while its band is
  drawn.** nukey-shay: *"you can usually repurpose those for temp Ram outside of the kernel...so long as
  you reset them before their display area(s) begin"* 〔mining 291287 indirect-addressing-question〕. The
  same exclusivity as the WSYNC-line version, applied to bytes across the frame: a kernel's 2-byte
  pointers are free from that kernel's end until they are set up again. `defuse` reports per region
  which bytes are read, which is the check before reusing one. **Cited only, not verified.**
  - **Across game states the same bytes can be different variables.** SpiceWare, 2019: *"Due to
    limited RAM in the 2600, the state also controls how RAM is used. In Medieval Mayhem the same memory
    that holds the state of the castle walls is used for drawing the main menu."* His listing declares
    eight 6-byte wall arrays (48 bytes) and `EQU`s the menu's variables — a `G48` buffer, pointers to
    each shown option and value, the top, selected and highlighted option — onto 31 of the first 36
    (the first three pairs; our count from the listing).
    〔mining 292204 states〕 The walls are never on screen with the menu, so the exclusivity is the game
    state itself, the widest form of the lodging above. **Cited only, not verified.**
- **★Placing a row of shapes and WRITING them are different limits, and the writes bind first.** A line's
  placement capacity is a search over strobe cycles (`plan_sprite_placement`); its write capacity is the
  graphics stores that must fit in the same 76 cycles (`prove_line_budget`). They are not the same number and
  the second is smaller, so "the row fits" answered from placement alone is answered from the wrong half.
  Measured on a ten-slot row at a uniform 16 px pitch: **one scanline can PLACE all ten and can WRITE only
  eight**, at both shape widths tried, best schedule ending at cycle **73 of 76**. That is what forces a second
  line — and a shape drawn on one of two lines is lit on every other scanline, so **a striped look is a
  consequence of the budget, not a style**. Two more in the same "cost, not taste" direction: the two lines
  afford **12 shape-draws at 7 px and 13 at 6 px, while one solid word costs 20**, so a row mixing two solid
  shapes with eight striped ones is not an arrangement the budget offers; and **the phase is free** — every
  arrangement that schedules at a given width ends its heavier line on the same cycle, whether the split is
  5/5 or 4/6. **Ask placement and cycles separately and report them separately: a combined "no" cannot tell
  you which of the two said it.** 〔measured 2026-08-26 in a piece in the private `roms/` repository;
  **not re-measured here** — the solvers behind it are bound to that work's own kernel, and the reusable form
  is on `techniques/roadmap.md` waiting for a second caller〕

## Rules of thumb for "good graphics"
- Visual impact ≈ number of colours × sprite density. More colours are bought by adding hardware (Pitfall II = DPC). 〔Demon Attack, Stay Frosty/Draconian〕
- The exemplars = the AtariAge Homebrew Awards "Best Graphics" category. **The strongest ground truth = the homebrew "Pizza Boy", every pixel of which the user drew personally** (designed in Photoshop; constraints confirmed with DaveC). More accurate than mining external threads = put the design questions (colour bands / NUSIZ / flicker tolerance) to the author directly.
- **Endorsement from a real production (the Pizza Boy dissection)**: professional-grade visuals were achieved **by craft on top of a STANDARD kernel** (batari Basic multisprite = 5 moving objects, P1 flickersort + P0 + M0/M1/BL + a 6-digit score). Not exotic code tricks — what works is **role separation (buildings = static asymmetric PF / moving things = sprites) + window rhythm (alternating solid and window across PF rows = vertical window texture) + colour and density design**. → a real production endorses TIA Studio's premise that "the designer composes the screen on top of a standard kernel". Details `reference/pizza-boy/dissection.ja.md` 〔Pizza Boy, bB multisprite kernel〕
  - **The other end of the same scale.** tokumaru, 2011: *"Since the amount of processing available is
    very limited, kernels have to be carefully tailored for each game in order to make full use of the
    hardware."* 〔mining 188783 tips-suggestions〕 This does not contradict the line above: Pizza Boy
    shows a standard kernel can carry professional-grade pictures, and this says full use of the machine
    is a kernel made for the game — a picture that needs the most needs a kernel written for it (our
    reading). **Cited only, not verified** — an opinion in a four-post thread.
  - **A regular grid can show spots that are not there.** Manuel Rotschkar, 2004, posted a kernel
    for a 7x7 board of squares, each with its own colour 〔`200404/msg00229`〕. Erik Mooney: *"It's got
    that optical-illusion thing going, where one's brain sees light spots at the intersection of the
    dark lines between the squares. :) Might be a bit visually distracting for an actual game."*
    〔`200404/msg00230`〕 (the Hermann grid, our naming). A window rhythm laid out as squares in a
    grid of dark lines would have the same shape; whether Pizza Boy's windows do was not looked at
    here (our reading). **Cited only, not verified** — one viewer, one board; the ROM was not run.
- Verify feasibility with a mockup before building (colour budget + scanline count + multiplexing on paper).
  **A mockup only checks the constraints you believed when you drew it.** Kurt Woloch, stella-list
  `199805/msg00187` era post in `new-members`, on the 2600 conversions he drew in 1984: *"I tried to
  figure out what the capabilities of graphics and sound were, roughly, and did some drawings …
  However, I did MISUNDERSTAND some of the constraints. I thought it would be allowed to have four
  colors on one scanline of playfield if you reduced the vertical resolution to double-scanline …
  which, I'm afraid, ISN'T POSSIBLE THIS WAY."* **The thing he got wrong was the colour budget** —
  the very item this line tells you to check — and the drawings looked good the whole way through.
  A mockup cannot catch a rule you do not know you are breaking, so the artist's constraints are only
  as good as whoever supplied them; get the budget from a measured page (`docs/techniques/`,
  `verified-coverage.md`), never from memory. *(And "trade vertical resolution for playfield colours"
  is not merely wrong here, it is plausible — swapping resolution for something else is true
  elsewhere on this machine. `known-traps.md` had nothing on it; see there.)*
- **Decide first whether it is a port or a de-make.** latchkeykid, 2023, asking whether a Ghosts 'n
  Goblins-style game is possible: *"I suppose I'm trying to find out if what I want is possible in
  full on the 2600 or only as a NES "de-make" to Atari style instead. The latter would definitely be
  both possible and likely easier relatively speaking but I'd prefer the former by a wide margin."*
  splendidnut put the same fork as screen-by-screen, *"capturing the essence of the game"*, against
  side-scrolling, *"more inline with other ports"* and apt to *"steer the project in the ARM-based
  direction (DPC+, CDFJ), but that's not a necessity"* 〔mining 349459
  is-a-game-like-ghosts-n-goblins-possible-on-the-2600〕. kylearan chose out loud for a Space
  Taxi-like game: *"my game will not be a true remake of Space Taxi, more a game heavily inspired by
  its game mechanics. I'd rather concentrate on the abilities of the VCS and try to make a game that
  looks and feels good on its own than trying to recreate a game faithfully while making too many
  compromises along the way."* (held here only as quoted in Thomas Jentzsch's reply) 〔mining 261054〕
  The two are different targets: a port is measured against the original and a de-make only against
  itself. `reproduce-loop.md`'s tools compare two 2600 ROMs, so they need a 2600 original and serve
  neither a port from another machine nor a de-make. **Cited only, not verified.**
- **A forum "cannot" is a forecast, and so is a "can".** On tokumaru's 2010 Sonic mock-up, cd-w:
  *"8-way scrolling on the 2600 is a tall order. I'd suggest revisiting the idea of a single screen
  Sonic, or restricting to vertical scrolling only."* Ed Fries, the next day: *"I've got a little demo
  with 8-way scrolling working using just the technique he mentions (p0 is the player and it remains
  relatively stationary in the center of the screen - only moves up and down), P1 and the missiles are
  used for everything else"*. Three weeks later tokumaru: *"while attempting to make the 8-way
  scrolling possible I had to give up on many features I planned on implementing, a fact that will
  cause the backgrounds to look a lot less interesting than I originally expected"*; ZackAttack, 2015,
  reported a 4K 8-way engine *"functional"*, with *"Scrolling platforms off the left and top"* still to
  do. 〔mining 170135 sonic-the-hedgehog-on-the-2600〕 What held was neither verdict: it could be done,
  and it cost the background. Budget either kind of claim before designing to it. **Cited only, not
  verified** — none of the demos was run here.
- **No framebuffer is a freedom and a bill on the same account.** tokumaru, 2018: *"the game program is
  responsible for drawing the entire picture every frame, meaning that with a few tweaks, a video frame
  can look completely different from the previous one. The NES, on the other hand, has a certain
  amount of video memory whose contents define what will be displayed on the screen, and changing large
  amounts of this memory from one frame to the next is usually not possible. All the dynamism of the
  2600 comes at a cost though: not only are the graphics more limited, but a huge portion of the CPU
  time is spent on video generation and can't be used for actual game logic"* 〔mining 281289
  question-for-1970s-1980s-vintage-console-computer-homebrew-developers〕. A frame that differs
  completely from the last costs the CPU no more than one that repeats it (our reading); both are paid
  from the kernel's share of every line. **Cited only, not verified.**

## Drawing craft (making the sprite/character pictures = the concrete rules of ⑥craft)
- **Start from thumbnail legibility**: verify **first** that it is still identifiable when shrunk to about one dot, then add detail. Shrink without interpolation (nearest, halving each step). 〔326595, 106110〕
- **A 2600 pixel is WIDE — one pixel covers about twice as much width as it does height, so a shape needs about HALF as many pixels across as it does down**: do not trust a square-dot preview. Decide letterforms and pictures at the real hardware aspect (player = thin out 1px horizontally, PF = 3–4× vertically to buy density). **→ draw previews with non-square pixels.** 〔326595〕 (there is no constant for this — see the note below)
  <!-- The Japanese original read 「横 ≒ 縦の約 1/2・≈2:1」 and looked self-contradictory in translation, because its two halves count different things: 「横 ≒ 縦の約 1/2」 is about how many PIXELS a shape needs across versus down, while 「≈2:1」 is the aspect of ONE pixel. Both say the same thing — the pixel is wide — and the English above now states it once. Resolved 2026-08-04. -->
  - **⚠★ WHY THE SOURCES DISAGREE, and what "measure it" can and cannot settle (2026-08-04).** The spread
    1.67–1.82 is not measurement noise, it is the question being underspecified. A 2600 pixel's aspect is
    `(visible width / visible height)` divided by the display's own `4:3`, and **the visible height is the
    free variable**: 192 lines of a 262-line frame is not the same picture as 210 or 228, and every source
    picked a different one. 5:3 (1.67), 12:7 (1.71) and 20:11 (1.82) are the same physics with three
    different overscan assumptions. **A fourth datum, 2026-09-04, falls BELOW the range and widens it to
    1.60–1.82** 〔stella-list `paint-tool-for-screen-mock-ups`, 2001-10, Erik Mooney〕: *"The 2600's
    160 x 192 is actually at an aspect ratio of 5:8 horz:vert (**an object 8 pixels high and 5 pixels wide
    will be visually square**), and the 40 x 192 is 5:32."* That is 8:5 = **1.60**, and it is the first of
    these to come from the mailing list rather than AtariAge — a fourth independent overscan assumption.
    **This strengthens the decision below rather than weakening it**: a spread that grows as sources
    accumulate is not converging, which is what "underspecified" predicts.
    **A fifth datum breaks the range at the TOP, on a different axis, 2026-09-04.** Eric Ball, 2004:
    *"For NTSC **160x200** is very close to 4:3, for PAL **160x240** is very close to 4:3."* PAL's
    240 gives **2.00** — above everything above — and the reason is not another overscan guess, it is
    **the television standard**. So the spread now has *two* free variables, not one.
    **And PAL immediately disagrees with itself, which is the point.** Atari's own table
    (`reference/docs_atari/stella_programmers_guide.html`, quoted in `fundamentals-audit`) gives PAL a
    **228-line kernel**, not 240 — so the vendor's recommendation is **1.90** while filling the frame
    is **2.00**. Same standard, same free variable, two answers, from the manufacturer and from a
    2004 practitioner. A 45° line reads as **27.8°** under one and **26.6°** under the other.
    **And all five values come out of one expression**, which is worth stating plainly because it
    makes the disagreement legible rather than mysterious:
    `pixel aspect = (4:3) ÷ (160 ÷ visible lines) = visible_lines / 120`.
    192 → 1.600, 200 → 1.667, 205 → 1.708, 218 → 1.817, 240 → 2.000 — reproducing 8:5, 5:3, 12:7 and
    20:11 to three places. Every source is the same physics; each picked a different line count, and
    PAL picks a different one again.
    **The consequence is not only shape, it is angle.** A line drawn at 45° on a square-pixel canvas
    reads as `atan(1/aspect)`: **32° at 1.60, 31° at 1.67, 27° at 2.00**. **Glenn Saunders** arrived from
    exactly that symptom — *"when I try doing 16 degrees of movement assuming 1 pixel per frame
    up/down and 1 pixel per frame left/right, **the 45' diagonals don't seem quite right**."*
    **Do not confuse this with the diagonal correction already in this file.** Combat's frame gating
    (`MPace & $03`, move on 3 of 4 frames) corrects **√2 — the distance travelled diagonally** — and
    is needed on any display. The aspect correction is about **the angle the eye sees**, and is
    needed because a 2600 pixel is wide. Two different corrections; this repository has the first and,
    within the searches run, not the second. Found by the mailing-list distillation (helper-2), who
    flagged that the 2004 quote is a description they had not verified and the arithmetic is theirs —
    re-run here and matching. Mooney's phrasing is also the
    most useful one for an artist — **8 tall × 5 wide reads as a square** — so it is the form to hand to
    whoever is drawing (see `docs/ingest.md`). **The sources name two more free variables this line left implicit** 〔found 2026-09-03〕: an NTSC display expects **227.5 colour clocks per line and the 2600 emits 228** 〔169128:12〕, so the horizontal scale is already off a standards-conforming set by half a clock per line; and the 2600 is **240p progressive — the even field of 480i, but at full refresh rather than half** 〔208810:9〕,
    **and TRUE interlace — two fields making one picture — is not in this repository at all**, which is
    worth saying because searching for it suggests otherwise: `interlac` matches **14 lines** here and
    **all fourteen mean line-interleaving of sprites**, the flicker/blinds sense. The count is right
    and "we have it" would be a lie.
    **The mechanism, from a 2012 build that started from Billy Eno's and Glenn Saunders' sync
    routines:** *"the sync for the second field starts half a line later, only that for PAL the sync is
    2.5 lines long while for NTSC it's 3 lines"*, and it buys vertical scrolling of *"1 scan line per
    field"* instead of 2 per frame. The author calls it *"a very subtle effect"* on a CRT, and guesses
    z26 *"sometimes can get confused when mapping a specific sync signal to an even or odd field"* — so
    an emulator picture does not judge it. 〔mining 200603 interlace-smooth-scrolling-demo〕 The ceiling
    it works against: *"If you count interlace, 525 is the maximum number of scanlines in NTSC … the
    maximum usable number of scanlines is 482, i.e. 241 noninterlaced"* (Glenn Saunders
    〔`200106/msg00092`〕). **Cited only, not verified** — nothing here emits a half line.
    People did attempt the real thing on this machine, and the way that
    thread went is the sharper illustration: the method was described on the list in **2000**; two
    other people built a working version in **2002** having *searched the archive first and missed
    it*; and the original author had to point them at his own post afterwards. Billy Eno, who built
    it, **Billy Eno**, replying to Erik Mooney: *"I searched the archives while I was doing this
    and **never saw your post**. Perhaps if the title had had something to do with interlacing :)"* 〔`200208/msg00131`, 2002-08-20〕
    **A subject line records where a conversation started, not what it produced** — which is also why
    the search above returns fourteen hits and none of them is this. Recorded 2026-09-04 by the mailing-list distillation
    (helper-2), who read all 96 hits across five layers before saying so; so "how many visible lines" asks about a signal with no interlaced partner to average against. Neither number lives anywhere else in this tree (227.5: five layers, zero hits; every 228 here is cycle budget, a different quantity). **So no single Stella measurement settles it either** — it settles what
    STELLA assumes, which is one more source, not an arbiter. (Checked 2026-08-04: Stella 7.0's `-help`
    offers no aspect-correction option at all — only `-tia.vsizeadjust <-5..5>` — so the "Stella 91%" figure
    cited above does not correspond to anything in the current build.)
    **HOW THIS WAS SETTLED (2026-08-04): the constant was DELETED, not corrected.** `pkg/design` carried
    `PixelAspectRatio = 2` and `ScanlinesForSquare(w) = w*2`, and 2.0 is above the entire 1.67–1.82 range, so
    it was wrong under every assumption. But it had **no caller anywhere** — not in the harness, not in the
    umbrella `sandbox/` tree holding the 54 authored PONG sources and the Pizza Boy reproduction. The only
    references were its own definition and a test asserting that definition. Dead code carrying a wrong
    constant is worse than no code, because the next reader trusts it.
    **The measurement is the part worth keeping, and it is the three derivations above.** The author draws in
    Photoshop on a 1:2 grid and has decided not to chase the remaining ~16%, which on a 2600 sprite is one dot
    either way — and real CRTs vary by more than the gap between 1.67 and 1.82.
  - **⚠ The precise value must be measured (the codified 2:1 is too large)**: several sources agree that "one 2600 px is wide" but **they disagree on the value** = 5:3 ≈ 1.67 (190154, 172161, 334673) / 12:7 ≈ 1.71 (169128) / 20:11 ≈ 1.82 (208810, Stella 91%). **The code used to carry `design.PixelAspectRatio = 2`, larger than every source and therefore too large under all of them; it was deleted on 2026-08-04 rather than corrected, because nothing called it** (see the note above). **The right answer splits over 1.67–1.82 depending on the display assumption**, and no single Stella measurement arbitrates that — it would settle what Stella assumes, which is one more source ([[feedback-verification-standard]]). Colour is likewise not RGB = the Stella palette is the comparison standard (306508, 300805). 〔mining 190154, 169128, 208810, 172161〕
- **★The canonical image→title route (a professional's real workflow)**: SpiceWare builds **the Photoshop mock FIRST and the kernel after it**. Logos and titles use a **flicker-free 2-colour 48px kernel** to turn "a designed 48px image" into "a stable on-screen display" (SF2 is the real example). = exactly this project's Photoshop→2600 path. `multicolor48`/`bitmap48` are its implementation basis. 〔mining blog SpiceWare 10640, 10515〕
  - **Mid-scanline colours sit on a 3CC grid, but a band is as wide as the STORE that paints it**:
    3 colour clocks is the CPU's granularity — one cycle — and it is not the band width. A band costs a
    whole store, so the floor is `writeCycles × 3` px: 9 px for `STA zp`, 12 for `STA abs`, 18 for a
    six-cycle write (`design.MinColorBandWidthPx`, and `color_test.go` pins all three). 160 ÷ 9 ≈ 18 is
    where the band count comes from — not 160 ÷ 3 ≈ 53. **Corrected 2026-09-03:** the sentence read as
    if the 3CC grid produced the ~18, which is a factor of three out; the code was right all along.
    Horizontal multi-colour tops out at ~18 bands / 3 colours (4 by borrowing SAX). Four arbitrary colours are impossible = substitute holes plus stacking. The SCORE bit (CTRLPF D1) splits the PF left/right. 〔mining 190154〕 `→ design.MinColorBandWidthPx, ScoreModeTwoColor`
  - **Picture first and fit after is also how a homebrewer opened a 4K game.** Magovinna, 2024,
    starting *JunglMaze*: *"So this is my Hero graphics for this new game:"*, then *"I have now created
    the map of the jungle."*, then *"Will be a fun challenge to cram this into 4k :-)"*; and later,
    *"There where not much space left in the cart :-)"* 〔mining 369164
    junglmaze-new-homebrew-what-would-a-combination-of-tonya8-and-adventure2600-look-like-on-the-2600-with-the-4k-limit〕.
    glurk, to someone whose mock-up was already drawn, put a check that the kernel is possible
    before the game logic: *"Probably, you should get your kernel figured out first and make sure it's going to be POSSIBLE to
    do what you envision, then work on the game logic afterwards."* 〔mining 346865
    help-with-optimizing-state-and-palette-lookup-code〕 The same Magovinna moved *Coarse Blade*'s
    tombstones for the budget (the layout rule in the Sprites section). **Cited only, not verified** —
    the order is read from the order of his posts.
- **Kill the misread letter pairs**: L/I/T · U/W · M/H/N · O/0/D. An author cannot notice their own misreadings → **verify with another person or by reading aloud**; the final adjustment is single-pixel. 〔294306, 326595 (confirmed twice = a strong principle)〕
- **At 4 px wide, seven letters are the hard ones: M, N, V, Q, Y, W, Z.** sheddy, on a 4×4 font called
  surprisingly clear: *"Not surprising as it's not all 4x4! Sure something passable can be done for M,
  N, V, Q, Y, W, Z. The others work well."* 〔mining 278281 160-pixel-display〕 A narrow font that reads
  well may be cheating on exactly those letters, so draw them first. The text kernels here are 4 px a
  character (`techniques/text12.md`). **Cited only, not verified.**
- **In 8px monochrome, spend the entire budget on the silhouette**: concentrate on the single most identifying part (hat, moustache, etc.). If that is not enough, buy density with double width + venetian stripes. 〔106110〕
- **A walk cycle needs a minimum of 2 frames at 50:50**: one bit of the frame counter (`and #2^n`) gives even spacing with no reset, and runs **only while moving**. 〔301861〕 `→ design.WalkFrame`
  - **The `and #2^n` schedule is what an accelerated frame counter must not skip.** Thomas Jentzsch,
    on converting games to PAL-50: *"a NTSC to PAL-50 conversion based on a frame counter will slow
    down by ~17%"*. His fix, in a Pitfall! hack, increments the counter twice every fifth frame, and
    *"these updates cannot be done in any frame, e.g. in Pitfall! (like in most other cases I have seen
    so far), the updates happen in frames 2^n (here every 2nd, 4th, 8th and 128th frame). So when
    increasing the frame counter a 2nd time, we must not skip these frames!"* What his code does not
    catch, by his list: *"updates which happen every frame"* and *"updates which don't happen at frame
    2^n"*. 〔mining 267100 pal-50-conversions〕 So a walk cycle keyed on a counter bit runs at the
    frame rate, and changes speed with the TV standard unless the counter is compensated (our
    reading). **Cited only, not verified.**
  - **A cycle that is not a power of two needs a reset or a table.** Manuel Polik's 2001 summary of a
    thread he had started on the list: for powers of two, `INC FrameCounter` / `LDA FrameCounter` /
    `AND` / `TAX`; for three values, `LDY frameCounter` / `LDX framecountertab,Y` / `STX frameCounter` with `framecountertab .byte
    $01,$02,$00`; for other lengths, `dec FrameCounter` / `bpl .skipReset` / `lda #MAXVAL` / `sta
    FrameCounter` / `.skipReset ldx FrameCounter`. Of the three-value table: *"You don't have to preload
    anything, when zeroing out the machine on startup anyway."* He also gave a form with no table,
    turning a shifting idea from Russ Perry Jr around: preload `x10x01x00`, then `ror` three times,
    store, `and #$03` — *"Which is 10 bytes and 16 cycles I think."* 〔`200103/msg00084`〕 The
    three-value table had come from Andrew Davie: *"That's just about the quickest I know...   10
    cycles"* 〔`200103/msg00071`〕. `design.WalkFrame` returns one counter bit (0 or 1), so it covers
    the first form only. **Cited only, not verified.**
- **Landscape gradients hold one hue and step only the luminance** (never mix hues). Depth from two layers: BG = far, PF = near. 〔160655〕 (consistent with the colour section's "high luminance → low saturation" rule)
  - **Steps of luminance can also draw relief.** MarcoJ, on *Bloxudoku*: *"I also like the subtle use
    of colour intensities to emboss the pieces when drawn on the board."* 〔mining 345854
    atari-2600-homebrew-wip-bloxudoku-v01-ntscpal-4k〕 With luminance effectively 8 steps (Colour
    section), the bevel is drawn in steps of one hue (our reading — he does not say which steps, or
    how far apart). **Cited only, not verified** — the screen was not looked at here.
- **Decide background art on 4 axes up front**: width (48/96px), colour count (1/2), PF mode (reflected/repeated — an asymmetric PF, rewritten mid-line, is a separate cost), row height (1–16 lines per row = detail vs load). **These ARE the input parameters of the background template (`design.BackgroundSpec`)**. 〔319884 atari-background-builder (= the tool the user used on Pizza Boy)〕 `→ design.BackgroundSpec.Feasible`

## Judgement rules no machine can decide (doc-only — deliberately not landed in `pkg/design`)
These cannot be quantified and need a judgement from Claude, a person, or an image, so they are deliberately left uncoded and collected here (= every rule is given a disposition, which is what guarantees coverage).
- **Thumbnail legibility**: whether it is still identifiable when shrunk = needs an image and a human eye. Judge from `get_screen_annotated`'s reduced preview. 〔326595, 106110〕
- **Whether the screen says what the rules are**: a capacity check measures that a picture is drawn,
  not that a player can read it. In one 2018 thread, a tester's questions on three unfinished games were
  mostly of that kind — what the area at the top of the screen is, whether the player holds a stick or a
  sword, whether the ghosts can be hit at all, why the score rises going down the screen and falls
  going up — and the author's answers explained intent that the screen did not show 〔mining 273513
  games-that-one-day-will-end; only the distillation notes are held here〕. One thread; a first-time
  player is the check. **Cited only, not verified.**
- **Misread letter pairs** (L/I/T · U/W · M/H/N · O/0/D): an author cannot notice their own misreadings = verify with another person or by reading aloud. Hard to mechanise. 〔294306, 326595〕
- **In 8px monochrome, spend the entire budget on the silhouette**: which part carries the identity is a subject-dependent aesthetic judgement. 〔106110〕
  - **A second answer: what the object does can matter more than whether it is recognisable.** Manuel Polik, 2001, had tried a
    train and a stampede for *Gunfight 2600* (*"it all either looked crap or wasn't playable"*,
    `200109/msg00393`) and could not draw *"some recognisable cattle"* — *"'brown something' blobs"*
    〔`200110/msg00005`〕. Glenn Saunders: *"the idea isn't to make it recognizable as much as presenting a
    different gameplay dynamic with some rationalized explanation to make it fit into the theme."* *"The
    arrows, for instance, are recognizable but they are so big they look like spears.  That's not a
    criticism, it's okay because we are talking about abstractions here."* *"It's the properties of the
    objects that mean more than the shape, I think."* 〔`200110/msg00007`〕 Chris Wilkson agreed, recalling
    *"the HMOVE bars in Soccer"* explained as referees: *"how many people think they look like referees?
    I don't.  But it doesn't matter.  It's a rational explanation."* 〔`200110/msg00012`〕 Whether to aim
    at recognition at all is part of the same judgement (our reading). **Cited only, not verified.**
- **The role split missile/ball = lines, player = areas**: which stack of objects builds one apparent shape is a composition judgement. 〔Davie〕
- **GameState = one variable + a kernel per state**: a structural pattern, not a numeric test. 〔title-to-game-transition〕
- **The ISC/ISB illegal opcodes + borrowing SP as a line counter**: a cycle-saving trick. Whether it is usable is backed by litmus measurement (guaranteed by verification, not by code). 〔illegal-opcodes〕
- **Symbolic naming / the two PAL-NTSC sets (N_xx/P_xx)**: a convention for how colour is held. `design.Hue/Luminance` can decompose the value, but the practice of "holding it under a symbolic name" is a convention, not something to check. 〔symbolic-color-names〕
- **Tool-implementation knowledge (the spritemate data model, implementing a per-scanline colour UI, and so on) is NOT absorbed**: it does not help authoring (writing asm). Preserving it in the frozen `tia-studio/` repo and the research notes is enough.

## Landing this in the implementation (`pkg/design` / the frozen TIA Studio)
- Feasibility = `pkg/design`'s static estimate plus live assert_line_budget / read_cycles / calibrate answers "does this layout fit inside 76cy?" immediately. Used as the gate before Claude writes asm.
- The defaults for the 4 feasibility axes (colour / scanlines / multiplexing / budget) are detailed at the end of `tools/research-w2-design.md`.
- The templates map onto the verified kernel techniques (zone_multiplex / dyn_multisprite / score6 / bitmap48 / two_line_kernel …).
- Note: TIA Studio (the canvas editor) is **frozen** ([[project-pivot-author-not-tool]]). These dimensions and judgements were originally aimed at its M4, but the main consumer now is Claude's authoring loop. The template set can be reused if it is revived.

## Structure & efficiency rules from the Combat (1977) disassembly comparison
Distilled from an efficiency/structure comparison of a self-authored Combat clone (`combat_mine`, 4K) vs the original Wagner 2K ROM (`sandbox/studies/combat/comparison-structure-vs-original.ja.md`, `diff-gaps.ja.md`). Clean-room: generalized prose + routine names only. These are **integration-under-budget** rules — how the original fits a whole 27-variant game in 2K.

- **Move ALL objects through ONE `,X`-indexed path over a bearings/state array — do NOT inline per object**: hold each object's dir/vel/pos in parallel arrays indexed by object (Combat's `DIRECTN[0..3]` drives both tanks AND both missiles down one `,X` loop with a 24-byte `MVtable`). The clone inlined friction+accel 4× (P0/P1 × X/Y ≈ +120–200 B of pure duplication). Decisive point: **movement runs in blanked overscan, so the 76cy/line budget does not apply — an index costs nothing off-beam, so the indexed loop is BOTH smaller AND free.** Before adding a per-object copy of any motion code, ask whether one indexed pass over an array does it. 〔Combat `DIRECTN`/`MVtable` — one `,X` path for 4 objects; comparison §2.4/§4/§7, diff-gaps GAP-3〕
- **Momentum = time-sliced increments, not a fractional velocity**: as an alternative to `pos += vel/frac`, dither the velocity across time. `FwdTimer` ($F0→$00, 16 steps) `ROL`s two 8-bit halves (`MVadjA`/`MVadjB`); the emerging bit nudges `XoffBase` by $10 for that one frame → faint analog acceleration over 16 frames, **no multiply**. Diagonal isotropy = **frame gating** (`MPace & $03` moves on 3 of 4 frames), not a √2 fraction (cheaper, VCS-idiomatic). A plain subpixel integrator moves correctly but can't reproduce that "faint inertia" texture — reach for time-slicing when the *feel* matters. 〔Combat `FwdTimer`/`MVadjA`/`MVadjB`/`MPace`; diff-gaps GAP-3, comparison §2.4〕
- **Rotation sprite = precompute the shape into a RAM buffer so the kernel reads a bare `LDA abs,Y` (zero per-line rotation math)**: store only **180° of shapes in ROM**; synthesize the other 180° as a **point-rotation = `REFP` hardware H-flip + a reverse-order byte copy (software V-flip)**, rendered in VBLANK into a RAM shape buffer; re-render only **one object per frame** (30 Hz each) to bound the VBLANK cost. General pattern: *don't compute in the kernel; stage the shape in VBLANK*; the table needs only 180° (symmetry supplies the rest). 〔Combat `ROT`/`SHAPES`+`REFP0/1`+reverse copy → 16B HIRES RAM; diff-gaps GAP-5, comparison §2.2〕
  - **How many shapes to store depends on where heading 0 points.** Roger Williams, 2002, on the
    Combat disassembly (16 headings, 0 = right, counter-clockwise): Combat stores eight bitmaps for
    headings `$0`–`$7`, indexes them with `angle AND 7`, and writes the angle itself to `REFPx`, whose
    reflect bit is D3, so headings `$8`–`$F` reflect with no logic (the engine tests `REFPx` against
    `0x08`, `Gopher2600/hardware/tia/video/masks.go`; the vertical flip is the reverse copy above). His keys: *"The RESPx flag can be indexed
    straight off the angle. There is no logic involved, very cheap."* (REFPx is meant) and *"No
    subtracts are necessary to invert the indexing."* Fewer bitmaps are possible — he offers four, with
    a backward index *"4-(angle and $3)"*, which is 4 at heading `$4`, so the table holds five (our
    arithmetic: straight up is no flip of `$0`–`$3`). And *"These compromises change a bit if you
    define direction 0 as vertical. With COMBAT's coordinate system, though, storing the rightmost 8
    shapes instead of the topmost 8 forces you to do a subtraction when you index the hidden side, and
    the logic cannot be sampled by simple bit-masking because it applies to angles $4 through $B."*
    〔`200203/msg00007`〕 Choose the heading origin with the flip bit in mind. **Cited only, not
    verified** — nothing here indexes a rotation table this way.
- **One interleaved HIRES buffer can feed BOTH players (P0 = even bytes / P1 = odd)**: a single 16-byte RAM buffer serves both sprites — pick a player's bytes with `AND #$FE` / `ORA #$01`, no shape math. Halves the RAM vs two separate buffers (~16 B) = a RAM-thrift move to hold in reserve for when 128 B is tight. 〔Combat shared 16B HIRES, P0/P1 interleaved; comparison §2.1/§2.2/§7〕
- **Fan one byte out to many duties, phase-locked, when RAM is tight**: `CLOCK` serves **5 roles** (frame timer / attract color / debounce pace / score-flash clock …) and `GameTimer` serves **3** (match clock + bit7 in-progress flag + attract period), sub-fields phase-locked so their uses never collide. Master-class RAM economy — but **only pay this when RAM is actually scarce**: packing with 43 B free just spends clarity for nothing (premature optimization). Know it; deploy it only under pressure. 〔Combat `CLOCK` (5-duty) / `GameTimer` (3-duty) / `VCNTRL`; comparison §2.7/§7〕
- **Load-level VBLANK with `TIM64T`/`INTIM` so the picture starts at a FIXED beam position — don't rely on a fixed WSYNC count + elastic filler**: arm a RIOT timer at VBLANK start, spin on `INTIM` until it expires, then begin the visible kernel = display-start **independent of how long the frame's logic ran**. A fixed WSYNC count + elastic `VBpad` tuned to today's code does NOT auto-absorb logic growth: add work and the picture dips (screen dip — the exact fragility the clone's positioner had to hand-engineer around). Prefer timer load-leveling when VBLANK work is variable or expected to grow. 〔Combat `VCNTRL`/`INTIM`/`TIM64T`; comparison §2.1/§6, diff-gaps (measure the VBLANK length with INTIM)〕 `→ techniques/sound-driver.md · game-states.md`
  - **A timer step is 64 cycles, so one target line can have two right values, and a coarser timer
    is set early.** Cye Freeman, 2004, timing his kernel with `TIM64T`, found his PAL value too high.
    Lee Fastenau does not time his kernel — he counts its lines (*"my answer is no, I've never used
    your method"*) — and his values are the timers of his own PAL support, tweaked until Z26 showed the
    right line count: *"Fortunately,
    I was able to narrow it down to a single working value, but because the timer resolution is 64
    cycles, it was certainly possible for two consecutive values to work (both ending up in the correct
    76-cycle scanline)."* 〔`200408/msg00055`〕 Alex Herbert timed it with `T1024T`: *"Over 13
    scanlines pass in each tick of the timer with divide by 1024, so it means setting the timer a few
    scalines early to get it to time-out on the correct scanline.  (I seem to remember it was 4
    scanlines for NTSC, 2 for PAL.)."* His code then strobes `WSYNC` four times on the NTSC path and
    twice on PAL, and he later dropped it to display the same number of lines in both modes
    〔`200408/msg00054`〕. Thomas Jentzsch: *"I always start with such a timer controlled kernel and
    remove the timer later."* 〔`200408/msg00056`〕 Freeman and Herbert time the visible kernel, not
    VBLANK; Lee's two-value remark is about his own PAL timers, and the 64-cycle step is the same for
    any `TIM64T` target (our reading). **Cited only, not verified.**
  - **Budget every routine at its worst case.** Robert Colbert, 1997, whose code to move and schedule
    five copies of player 0 fits in VBLANK, on why his sort no longer stops early when a pass makes no
    swaps — he took the flag out *"on purpose"*: *"with the 2600, you need to assume that every routine
    will execute the WORST-CASE SCENARIO every time, so checking for better than that actually is a
    waste of time!  If you do check for better than worst-case, you may be in for a surprise later when
    conditions are such that the worst-case occurs, or worse yet - two of your routines encounter the
    worst-case in the same frame..."* 〔`199710/msg00111`〕 An early exit shortens only frames that would
    have fitted anyway (our reading). **Cited only, not verified.**
  - **Logic that grows while a switch is held shows up as lines.** Kirk Israel, 2004: his game sat in
    *"a frame-free limbo as reset was being held"*; after looking at what *Joust* does he moved the reset
    check to the end of the game loop, *"that way the game displays but is frozen til reset is
    released. The problem is, because it's doing the init code over and over, it pops up to 267
    scanlines until Reset is released"*. His other options were to start on the press and ignore the
    hold, or to start a new game on release 〔`200403/msg00125`〕; Rob and Albert Yarusso preferred
    starting on release 〔`200403/msg00126`, `msg00127`〕. **Cited only, not verified.**
- **One wrap-around clear loop, reused with 4 seed values for 4 clear extents**: `ClearMem` is a single loop whose start index (X seed) is set 4 ways to wipe 4 regions — one routine, four callers, vs four clear loops. Cheap ROM-thrift for init/reset paths that wipe several ranges. 〔Combat `ClearMem`; comparison §2.8/§7〕
- **Audit your OWN hand-tuned code for cargo-cult — hand-tuned ≠ optimal, even in a 2K master ROM**: the annotated Combat disassembly honestly inventories its own cruft (a redundant double `STA GRP0`, a stray `WSYNC`, a self-flagged "why not `LDA MVtable+1,Y`?" 2-cycle miss). Model this: keep a written inventory of your ROM's own redundancy rather than assuming your tuned code is tight. (Applied to our clone, this surfaced ~250–400 B of recoverable duplication unrelated to its provability trade.) 〔Combat — Williams' annotations; comparison §7〕
  - **Shipped is not correct either: a real defect can sit where nothing visibly reads it.** ChildOfCv,
    2021, on a 6502 copy loop that steps `STA ($E1,X)` with `INX`: the pre-indexed mode adds X before
    the pointer is read, so each pass after the first reads a different pointer instead of advancing
    the destination (Thomas Jentzsch: *"The code might work if X would be increased by two"*). The loop
    is in a shipped NES game, *Mike Tyson's Punch-Out!!*; in a debugger, with a count of 2, *"The first
    byte went to the intended location (5C5), but the second was written to 405"* — yet the game is
    *"almost glitch-free"*, and his guess is *"It's likely that they never use this to copy more than a
    byte or 2"*. 〔mining 325492 this-is-a-bug-right〕 A wrong write that lands in RAM nothing visibly
    uses passes every look at the screen (our reading, **Not verified**); here `defuse`'s may-write set and `watch_ram` are the
    instruments that would show it. **Cited only, not verified** — the NES ROM was not looked at.

## Combat deep-read: design-intent, audio model & AI-nav primitives
A second pass over Combat (1977) through 5 lenses BEYOND round-1's efficiency/structure comparison — design intent, the audio channel model, and AI-nav primitives our own clone added (the original has no AI). Clean-room: generalized prose + labels only. Where round-1 gave **integration-under-budget** rules, these are the **why / feel / balance** rules the structural pass could not see. The AI-nav block is flagged **PONG-capstone material**.

- **Difficulty = a per-player self-handicap on the WINNER, not AI scaling — and the lever means something different per vehicle.** Each player reads his OWN difficulty switch. "Pro" nerfs the strong player two ways: (1) shorter missile RANGE (early-killed at a higher remaining-life threshold), and (2) a SLOWER vehicle (subtract the vehicle-index off velocity — a jet loses speed while a tank loses none; tanks only lose range). Intent = a self-selected handicap so parent/child or expert/novice play the same 2-player match evenly: nerf the strong player instead of buffing an enemy. 〔Combat `NoStir` (DIFSWCH ASL) / `ChkVM`·`MisEZ` (CMP #$1C early-kill) / `FwdPro` (SBC GAMSHP); manual p.71-77; deep-read harvest 2026-07-23〕
- **Orthogonal mechanic axes buy cheap combinatorial breadth — but CURATE the cross-product and RESKIN shared mechanics with new fiction.** 27 variations = ~6 independent bitfields, each driving one flag, so combos are nearly free. Yet the designers shipped a hand-picked 27, NOT all 2^N (many mechanically-legal combos dropped as unfun), and the SAME playfield bytes are reskinned as "maze/barriers" for tanks vs "clouds" for planes — identical rendering, different fiction and tactics. Content strategy, distinct from the round-1 VARMAP bit-packing fact. 〔Combat `VARMAP` / `InitPF` flag decode / `PLFPNT` maze-vs-cloud reuse; deep-read harvest 2026-07-23〕
  - **The same move scaled to a multicart was proposed, not built.** On a 2011 thread about a 52-game
    cartridge after the NES *Action 52*, Gemintronic: *"Whoever codes this thing will probably have to
    have generic gameplay and sprite routines they mix and match for most of the games. Jaguarmen being
    it's own deal on a separate bank."* bladejunker: *"no single engine will facilitate every game type
    but that doesn't mean most games can't be distilled into a few select game engine types"*, with
    scrolling (single screen, page flip, vertical, horizontal) as the first parameter. 〔mining 176572
    action-52on-the-2600〕 Shared routines for the many, a bank of its own for the headline game.
    **Cited only, not verified** — a proposal; the one game posted in the thread (scumsoft's, days
    earlier) stands alone, and nothing was built from the shared-routine idea.
- **Stealth where your own useful/risky verbs repaint you.** In invisible variants the tank is painted the background color, but firing, bumping a wall, and scoring/being-hit all RESTORE the visible color — each merely re-writes the color register those events already touch. The hidden-information layer is self-defeating by design: attacking or moving recklessly lights you up = attack-vs-hide tension for free. 〔Combat `ChkVM` (ColorBK paint) / `BumpTank` (XColor restore); manual "Invisible Tank"; deep-read harvest 2026-07-23〕
- **Force composition — how MANY and how BIG each side's units are — is a distinct handicap axis (~2 NUSIZ bytes).** A widths table turns the plane games into 1v1 / 2v2 / 1-vs-3 / one quad-width "Bomber" vs three planes almost for free (formation units all fire on one trigger). Quantity/size asymmetry is a curated difficulty knob separate from stat tuning. 〔Combat `WIDTHS` / `LDSTEL` NUSIZ setup; manual games 19-27; deep-read harvest 2026-07-23〕
- **Input deliberately throttled for "heft" — spend real effort on feel nobody consciously sees.** Three governors make vehicles feel weighty, not twitchy: a turn-rate governor (N frames between each 22.5° rotate), a whipsaw-reversal inhibitor (block instant flips), and forward-speed dithered over 16 frames so momentum is "just barely noticeable." Rate-limit raw input to express a vehicle's mass. (Round-1 has the FwdTimer momentum; new here: the rotational governor + whipsaw inhibitor + the invisible-polish philosophy.) 〔Combat `CHKSW` (TurnTimer/LastTurn) + `FwdTimer`; deep-read harvest 2026-07-23〕
- **A scoring event earns a consequence beat that ALSO resets board geometry (anti-camping).** A hit doesn't just respawn: the loser's tank spins, explosion volume ramps down, the loser is knocked to a NEW position (direction off the winner's missile bearing), and the winner's engine is silenced. Re-orienting and shoving the loser prevents play resuming in the same lethal geometry = anti-instant-re-hit fairness. Give scoring events a consequence beat that also resets state. 〔Combat `COLDET` / `CHKSW` stir branch / `RushTank`·`BumpTank`; deep-read harvest 2026-07-23〕
- **Overload one control with a contextual second meaning (control economy on a 1-button machine).** In guided-missile variants, rotating your body continuously copies your CURRENT bearing into the missile's — so after firing you steer the missile by continuing to turn, no separate control. Trades aim for vulnerability (the same stick turns your body). Depth without extra buttons. 〔Combat `ROT` (BIT GUIDED / STY DIRECTN+2,X); manual Fig E; deep-read harvest 2026-07-23〕
- **Fixed short match + a diegetic end-game telegraph rendered THROUGH the score itself — no separate UI.** A ~2-minute timer ticks ~1/sec; the last ~1/8 is telegraphed by BLINKING the score (no timer widget). Short fixed sessions keep 2-player play snappy; communicate urgent state by animating an element you already draw. 〔Combat `GSGRCK` (GameTimer / CMP #$F0 / CLOCK&$30 flash / KLskip=$0E); deep-read harvest 2026-07-23〕
- **Minimal-UI: attract == menu == play, and the score doubles as the variation selector.** No separate menu — in attract, Select increments the variation number straight into SCORE, shown by the normal score kernel (right score hidden so only the game number reads); the idle match-timer drives a color-cycle anti-burn-in. Reuse gameplay display elements as menu/attract UI. 〔Combat `SelGO` (STA SCORE / SHOWSCR) / `LDSTEL` color cycle; deep-read harvest 2026-07-23〕
- **Rule-layering & productive imprecision as design moves.** (a) Billiard adds a scoring PRECONDITION (must-bounce-first) over the unchanged bounce engine → a bank-shot game with a higher skill ceiling from the same physics. (b) The faked Pong reflection is imprecise ON PURPOSE (guesses the wall normal, jiggers +22.5°) so bounces are never perfectly axis-aligned → livelier, unsolvable. (c) Removing "reverse" from tanks is control-limitation-as-identity. New modes come from preconditions/constraints/omissions, not new systems. 〔Combat `Launch`/`COLIS` billiard gate / `COLMPF` reflection SM / `CTRLTBL` "No reverse"; deep-read harvest 2026-07-23〕
- **★SOUND PRIORITY = last-writer-wins on a 1-object-per-channel bus — arbitration is BRANCH ORDER, not a mixer.** The core 2600 audio mental model (only 2 channels). Each object owns one channel; precedence (explosion > shot-boom > engine > pong) is decided purely by which routine writes `AUDx0,X` LAST, via the branch order of the sound dispatch. A state flag can "steal" a channel (nonzero → emit the bounce tone INSTEAD of engine). Decide precedence by ORDERING writes, not comparing volumes — zero bytes of priority logic. 〔Combat MisLife dispatch (MisFly/MotMis/BoomSnd order) / `MOTORS` AltSnd hijack; deep-read harvest 2026-07-23〕
- **★Live game data overlaid on the CPU IRQ/BRK vector slot = a 2K→4K port booby-trap.** A 2-byte pitch table sits at the IRQ/BRK vector address because a 2K cart mirrors $F000-$F7FF into $F800-$FFFF, so the "vector" bytes ARE read as an ordinary data table (`LDA table,X`). It survives only because the code never takes BRK — `SEI` would not help, since BRK ignores the I flag. A 2K→4K port silently breaks. Never overlay meaningful data on $FFFA-$FFFF unless you fully model the bank mirror. (harness-warn candidate → capgap CMB-6.) 〔Combat ORG $F7FC / AudPitch $0F,$11 at $F7FE; deep-read harvest 2026-07-23〕
  - **The same slot from the other side: four of the six vector bytes are free if nothing needs
    them.** nukey-shay, 2009: *"AFAIK, the NMI vector (at $xFFA-B) is not used by the 2600. You can use
    these 2 bytes for whatever you want (or keep them unused along with the previous 2 bytes to
    provide unmodded Supercharger-compatability). If BRK interrupts are not used by the program, the
    interrupt vector ($xFFE-F) can also be used for something else if you wish (or kept unused for the
    SC-compatability). All a general program needs is the cold start vector ($xFFC-D) to lead it to
    the ram clear routine."* 〔mining 136813 help-debugging〕 Combat's pitch table is that use; the
    warning above is what the 2K mirror adds to it (our reading). **Cited only, not verified** — the
    Supercharger condition was not checked here.

**AI-nav primitives (PONG-capstone material — from our Combat clone `combat_mine.asm`; the original 2-player Combat has NO AI, so these are authored-new and emulator-verified):**
- **8-way octant seek from unsigned |dx|,|dy| + sign-first, with a >127px overflow guard.** Derive each axis's sign FIRST by unsigned CMP (a signed 8-bit subtract of two positions overflows once separation exceeds 127px: dx=-132 reads as +124), then form |dx|,|dy| by large-minus-small, then classify: 2·|dy|<|dx| → horizontal / 2·|dx|<|dy| → vertical / else diagonal (the ×2 via ASL+carry also handles 2·|d|>255). Picks one of 8 headings, division-free and overflow-proof on a 160px field. 〔clone `AiDxE`/`AiDyC`/`AiCls`; deep-read harvest 2026-07-23〕
  - **Twelve headings in thirteen table entries, method unpublished.** Thomas Jentzsch, 2002, on the
    tank AI of *Robot City*: *"There are 12 possible distance combinations (arranged like the hours on a
    clock) and each requires special handling.  After some brainstorming I found a surprisingly easy way
    to convert the distances into 13 indexes, which I could use for table access. That's the code I'm
    most proud of (only one byte wasted in table!)."* 〔`200208/msg00032`〕 The classifier above picks a
    heading by compares; his ends in a table read, and how the index is formed is not in the thread.
    **Cited only, not verified.**
- **Shortest-arc turn on a power-of-two direction ring.** To rotate toward a target heading the short way on a 16-step wrap ring: diff = (target − current) & $0F; if 0 done; CMP #9 → 1..8 turn CW (INC), 9..15 turn CCW (DEC). One compare picks the correct rotation sense across the wrap with no signed distance and no table. Generalizes via CMP #(N/2+1); rate-limit the turn. 〔clone `AiMove`/`AiCW`/`AiTe`; deep-read harvest 2026-07-23〕
- **Map-free navigation primitives (four independently-testable behaviors on a bare greedy seeker).** (1) Stall→180° reversal: every 32 frames sample horizontal headway; |Δ|<2px = wedged → about-face (gate OFF where an axis is intentionally frozen, else a legit vertical climb reads as "stuck"). (2) Reactive wall-slide: on wall-contact (CXP1FB) skip accel + rotate one notch + snap velocity to zero, so the heading sweeps off the wall — no normal, no map. (3) Ammo-gate fire-when-aligned: fire only when off-axis error < half a tank height; a hold sets a quick re-check WITHOUT charging the post-shot cooldown (hold ≠ fired). (4) Scatter-decoy target-swap: inside a concave pocket substitute hard-coded exit waypoints for the target (seek core unchanged) with the escape direction LATCHED against mid-corridor oscillation. Grow AI as named, separately-verifiable layers over "walk toward target." 〔clone `AiStk`/`P1Snap`/`AimOK`/`TgtEsc`; deep-read harvest 2026-07-23〕
