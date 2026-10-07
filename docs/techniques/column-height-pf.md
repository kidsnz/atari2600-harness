# Technique — column-height playfield (a height per column, drawn as liquid or terrain)

**Goal:** draw a liquid surface or a terrain profile by keeping one height per playfield column in
RAM and having the kernel build each scanline's playfield bits from that array of heights.

**Status:** the only source is our summary notes on one AtariAge thread. This harness has no ROM and
no scenario for it, and nothing on this page has been measured. Every claim taken from them below is
what the notes say, and every one is **Cited only, not verified**.
**Source:** AtariAge `topic/33432` (titled Design #1: Primordial Ooze, as our notes give it), a design
thread opened by Andrew Davie (our notes date it 2003-2009), about a game where falling slime is shot back. Only our notes on
the thread are held here, not its text, so nothing below is a quotation. The names in the account of
the thread are the ones the notes give; the names in the last section come from the other pages cited
there.

## The data model
- **One byte per column.** The notes describe the screen as 40 vertical columns, each one playfield
  pixel wide, holding for each column one byte: the scanline where the slime ends at the bottom. Keeping
  both a top and a bottom edge makes it 80 bytes. Two of the kernels below use 32 or 16 columns.
  **Cited only, not verified.**
- **Simple rules make it look liquid.** As the notes summarise the thread's rules, slime at the top
  moves to a neighbouring column that is lower (a drip), and slime at the bottom moves to a neighbouring
  column that is lower (spreading flat): plain diffusion of heights. **Cited only, not verified.**

## Three kernels the thread weighed
- **Direct, 40 columns (ericball's estimate).** A height becomes a PF bit with `cpy height` / `ror`,
  the compare's carry shifted in, at 5 cycles a bit, so 40 bits take 200 cycles. With an asymmetric
  playfield needing at least 36 cycles a line, his count came to a six-line kernel with practically no
  time left for sprite updates. The notes give double buffering (two sets of PF shadow bytes, one
  updated while the other is shown) as a precondition. Done directly, it is heavy. **Cited only, not
  verified.**
- **Bitplanes, 32 columns.** Two-line vertical resolution in an eight-line kernel. The upper bits of
  each height sit in 32 bytes, one per column; the low two bits are held in bitplane form so that
  eight columns are handled in parallel. A setup step first copies the current byte to a saved one
  (`lda` / `sta`), then runs eight `cpy` / `rol` pairs that fold into one byte whether each of eight
  columns is lit on this line; the notes put the whole setup at about 49 cycles (more if registers have
  to be saved); each line then combines bytes with `ora` and `and` and stores the result to a temporary and
  to `PF1`. The notes count 49 + 84 cycles for each quarter of the work, 532 of the 608 cycles in total:
  thin, but it fits. Half the lines only load PF from the temporary, so that the stores land at the
  right screen position, and the four groups of columns can show the same length at
  different apparent lengths, which the setup corrects. The notes do not name who proposed it.
  **Cited only, not verified.**
- **Sorted, 16 columns, one-line kernel (bizarrostormy's implementation, `ooze.zip`, attached to the
  thread and not opened here).** The columns are kept sorted by length, and drawing only drops masks
  in order from the shortest. **The price: two columns cannot have the same length.** In exchange the
  columns are fully independent and 16 of them fit a one-line kernel. The tips wobble a little, which
  the notes say is not noticeable in play. Our notes call it the most practical outcome. **Cited only, not
  verified.**

## Price
Each figure is what the notes give; a blank in the notes is marked so.

| kernel | RAM | cycles | lines |
|---|---|---|---|
| direct, 40 columns | 40 bytes (80 with top and bottom), plus the PF shadows, not counted in the notes | 5 a bit, 200 for 40 bits | six-line kernel |
| bitplanes, 32 columns | 32 bytes of upper bits; the low two bits not counted in the notes (32 × 2 ÷ 8 = 8 bytes, our arithmetic) | about 49 for setup; 532 of 608 in all | eight-line kernel, two-line resolution |
| sorted, 16 columns | not in the notes | not in the notes | one-line kernel |

ROM is not priced in the notes for any of the three. **Not measured here:** no row of this table has been assembled or counted in this harness.

## How it sits next to the other pages
- **`procedural.md` generates terrain; this page stores it.** There, terrain comes from an LFSR stepped
  per row or cell, so a seed reproduces it and no map is stored in ROM. Here the profile is state: a byte
  per column in RAM, changed by the drip and spread rules above, and the kernel's cost is turning heights into
  bits on every line. That page's *What generating does not save* already reaches the same storage from
  the other side: for a fill function, Thomas Jentzsch would still need each column's height (AtariAge
  `topic/385470`). Cited only, not verified. Setting the two side by side is our reading.
- **`pf-modes.md` is about CTRLPF bits, which the notes do not mention.** What the notes do name for 40
  columns is an asymmetric playfield, which that page lists as verified elsewhere. The measurement behind it,
  `litmus_pf_async`, holds two `PF1` write points in repeat mode (`docs/verified-coverage.md`); it says
  nothing about building the bits, and the notes' 36-cycle floor is not measured here. A measured game
  kernel that rewrites PF mid-line is in `asymmetric-pf-score.md`.
- **Another account of how a 16-column Primordial Ooze kernel stored its columns** — transitions sorted
  by row, on condition that no row has more than one — is recalled by Pat Brady in another thread and quoted in
  `docs/design-principles.md` (*Hold the board at the game's resolution*). Our reading: it may be the same
  kernel as the sorted one above described another way, and his condition and the notes' condition that
  no two columns share a length may be one limit seen from two sides. **Not verified.**

## Not verified
- Every cycle count above (5 a bit, 200, 36, about 49, 532 of 608), and the six- and eight-line kernels.
- That the sorted 16-column kernel fits one line, and that its wobble is not noticeable in play.
- That the drip and spread rules look like liquid.
- The RAM of the bitplane low bits and of the double-buffered shadows; the RAM and cycles of the sorted
  kernel; the ROM of all three.
