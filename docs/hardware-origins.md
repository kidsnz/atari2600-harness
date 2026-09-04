# Hardware origins — why the machine is shaped this way

Two primary sources, acquired 2026-09-04. Everything else in `docs/` says *what* the hardware does;
this file records *why the people who built it made it do that*, in their own words. Both were absent
from every layer before today — `rg` for `24-pin`, `polynomial counter`, `15.24`, `Perry`, `Decuir`
across `docs/ pkg/ internal/ roms/` returned nothing.

Neither source is a measurement. They are testimony and legal description, and where they touch
something we have measured, the measurement stays authoritative. What they add is **cause**: the
constraints below explain shapes we had been treating as brute facts.

---

## 1. Perry & Wallich, "Design case history: the Atari Video Computer System"

IEEE Spectrum, March 1983, pp. 45-51. Interviews with the designers, Jay Miner and Joe Decuir among
them. Transcribed at `atariage.com/2600/archives/design_case.html`.

**The design philosophy, stated plainly:**

> The key to the design was simplicity: making the software do as much of the work as possible, so
> that the hardware could be cheaper — silicon was very expensive in those days.

That sentence is the reason for most of what this repository spends its time proving. A machine that
pushed work into software is a machine whose behaviour has to be *measured* rather than looked up.

**Why there is no frame buffer:**

> The microprocessor was synchronized to the television scan rate and created the display one or two
> lines at a time.

**Why the playfield is coarse and the players are not:**

> display the background of the screen at relatively low resolution, while displaying moving objects
> with higher resolution — low-resolution playfield, high-resolution players.

**Why VSYNC is the programmer's job — a deliberate omission, not an oversight:**

> They also eliminated any provision for vertical synchronization and gave that task to the programmer.

**The frame budget as the designers stated it:**

> A VCS kernel must count the number of lines displayed on the television screen and must finish
> displaying a single frame in exactly the same time — 15.24 milliseconds

**Why the TIA counts with polynomials.** This is the one that explains a mechanism we have measured
repeatedly without ever asking where it came from:

> A polynomial counter occupies one-fourth the silicon area of an equivalent binary counter, but,
> unlike a binary counter, it does not count in any simple order.

The LFSRs in the audio dividers, in Pitfall's world generator, in `eor #$B4` — all of them are the
same trick, and the trick is in the chip because a polynomial counter was four times cheaper in
silicon. "Does not count in any simple order" is the cost, paid by every programmer since.

**Why the cartridge port is the shape it is — and the designers' own verdict on it:**

> Atari limited the cartridge connector to 24 pins, omitting read-write and clock lines for RAM, as
> well as lines for addresses greater than 4096.

> Mr. Miner and Mr. Decuir agreed in retrospect that this decision was a mistake, since a 30-pin
> connector would have cost only 50 cents for each VCS and 10 cents a cartridge.

Bank-switching hotspots, cartridge RAM's split read/write ports, the 4K ceiling: all of it descends
from twenty-four pins. The people responsible thought it was worth fifty cents to avoid.

---

## 2. US Patent 4,623,147 — "Process for displaying a plurality of objects on a video screen"

Mark S. Ackerman and Glenn Parker, assigned to GCC Technologies (General Computer Corporation).
Filed 1983-09-20, granted 1986-11-18.

**This is the primary source for the re-strobe technique.** `docs/techniques/restrobe-copies.md`
credits a 2011 AtariAge post; the patent describes the same method **twenty-eight years earlier**, by
the people who invented it, in a document written to be precise about a mechanism.

> Each player position counter has three decodes in addition to the zero crossing decode. These
> decodes are controlled by data bits D0, D1, and D2 of the 8-bit number/spacing control registers.

> [the counters are] clocked continuously during the unblanked portion of every horizontal line

The claimed process is to write a new NUSIZ value **during the active scan line, after a reset**, so
copies appear beyond the three the register nominally offers — and, by loading GRP0 and GRP1
consecutively with timed resets, to put up to four *graphically distinct* high-resolution objects on
one line.

Nothing here contradicts what we measured. It supplies provenance and, in "clocked continuously
during the unblanked portion", a one-line statement of the invariant that `sprite-placement.md`'s rule
table exists to pin down.

---

## What these do not settle

Testimony is not measurement, and a patent claims a method rather than reporting an experiment. Where
this file and a `verified-coverage.md` row disagree, the row wins. The 15.24 ms figure in particular
is the designers' round number and is not the same quantity as our measured `RefreshRate`
(NTSC = 15734.26 / 262 = 60.0544 Hz, so 16.65 ms) — **they are not in conflict; they are different
things, and the difference is exactly the sort of thing that gets miscopied.**
