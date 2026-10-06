# Technique #10b — Dynamic multi-sprite kernel (the full form)

**Source:** clean-room synthesis of standard 2600 multi-sprite / flicker-sort practice (AtariAge topic 107063 *interlacing-multi-sprites*; bB multisprite kernel as seen in Pizza Boy) extending technique #10; hardware-grounded + CI-locked by `scenarios/dyn_multisprite.json`.

**Goal:** N objects at arbitrary, *crossing* vertical positions through 2 players — the general
engine behind games with free-moving object sets. Extends the verified flicker-pairs core (#10)
with the three missing pieces: per-frame **Y-sort**, **dynamic 2-of-N slot allocation**, and
**mid-screen repositioning** of a player after its previous object ends.

Demo: `roms/techniques/dyn_multisprite.asm` — 5 color-coded objects bouncing at different rates
(orders cross constantly), CI-locked by `scenarios/dyn_multisprite.json` + an ingest-based color
proof (`TestDynMultisprite`).

## The architecture (what real engines do)

- **Sorting network, not insertion sort.** 9 fixed compare-swaps sort 5 objects with
  deterministic cycles — kernels and VBLANK budgets hate data-dependent timing.
- **Slot queues.** Sorted objects assign alternately to P0/P1 (alternation start flips each
  frame = fairness); an object only joins a slot if it starts ≥ previous-end + 2 pairs (the
  repositioning gap); fallback to the other slot, else dropped this frame. Queues end with a
  **0 sentinel** — the kernel's wait-state compares against it harmlessly (cheaper than a
  bounds check by 5 cycles, which mattered).
- **2-line kernel state machine.** Line A = P0's slot, line B = P1's: WAIT (stage the next
  object's color; compare trigger pair) → POSITION (timed RESP via per-object delay constants —
  X = 33+15d (slot A) / 36+15d (slot B), measured; no HMOVE needed) → DRAW (art rows) → next queue entry.
- **TIM64T VBLANK.** Sort + assignment cost varies by path (~60–160 cycles); padding WSYNCs
  can't equalize that. The timer absorbs it — the real-game idiom, now verified here.

## The same choices on the stella-list (2001–2002)

None of the programs below has been run here.

- **What gets sorted.** The network permutes `sortIdx`, five bytes of object numbers, and never moves
  `ys` or `dirs` (macro `CMPSW` in the demo), so index *k* of every per-object array stays object *k*.
  Manuel Polik's engine (November 2002) Y-sorted the object data itself — five arrays of ten bytes — so one object's
  bytes could sit at index 3 one frame and 7 the next, and his object handler had to test each
  object's type: *"no other choice. Except sacrificing another 10 bytes RAM for indexing the object
  types"* 〔stella-list `200211/msg00073`〕. In March 2002 he had expected even a bubble sort to *"sort only
  links, otherwise I'd have to swap Y-Pos/X-Pos/Size/Color/Y-Movment & X-Movement values on every
  occuring swap"*; Thomas Jentzsch: *"Yes, that's a disadvantage"* 〔`200203/msg00096`,
  `200203/msg00097`〕. (`flicker-multiplexing.md` has Roger Williams's indirect sort.) **Cited only, not verified.**
- **Is the input arbitrary?** The demo resets `sortIdx` to 0–4 every frame, so it does not reuse the
  last frame's order. Jentzsch, answering Polik's linked-list sort 〔`200203/msg00094`〕: a tree sort is
  *"WAY faster than bubble-sort for large and UNsorted lists"*, *"BUT, in my (and your?) case, the list
  is not unsorted, but imperfect sorted. From frame to frame, the elements move only slightly (e.g.
  +/-1)"*; bubble sort corrects that *"very soon AND the algorithm can recognize perfect sorting (no
  swaps) and then stop very early. And bubble-sort saves memory, because you don't need the
  link-table"* 〔`200203/msg00095`〕. Polik disputed the premise for his game: objects *"appear/disappear
  any given time, any place"*, which *"might (depending on the sort order) result in a worst-case Bubble Sort, from one frame to
  another"*, and since he must cope with a worst case at any time, an early bail-out *"would only make
  the situation worse"* 〔`200203/msg00096`〕.
  Jentzsch: with the inner loop running backwards, 0 and 7 added to the end of 1–6 sort in one
  iteration, *"I can't think of real worst-case scenarios"*, and the stop's overhead is *"very marginal,
  because it's only checked in the outer loop"* 〔`200203/msg00097`〕; Polik asked whether a 7 added at the *beginning* would be the worst case
  (*"would be worst case, or?"*, adding *"maybe I'm just on a very wrong path now"*) 〔`200203/msg00098`〕, and Jentzsch that he always adds at the end 〔`200203/msg00105`〕. The early stop
  makes the number of passes data-dependent; this page fixes the compare count instead (a swap still
  costs extra cycles in `CMPSW`), and under TIM64T what must fit is the worst case; the "deterministic
  cycles" above holds for the compare count, not for the cycles (our reading). **Cited only, not verified.**
- **Count the compares, not the swaps.** Julian Squires put the bubble-sort worst case at reverse order
  (*"n^2 swaps"*), with only n swaps for a 7 ahead of an otherwise sorted list 〔`200203/msg00099`〕.
  Polik: *"The number of swaps is only n, agreed"*, but the 7 moves one place per iteration — he asked,
  *"so you'd have to do both loops completely, with n^2 compares at least, right?"* 〔`200203/msg00100`〕. Erik Mooney:
  *"Classic sorting algorithms are usually defined as speed being the number of swaps required. In
  tightly timed 6502 assembly language, the comparisons and looping take a quite significant amount of
  time already"*, and on Polik's example: in 7, 1, 2, 3, 4, 5, 6 *"you require n^2 comparisons (actually
  (n-1)^2) to get the 7 to the end of the list"* 〔`200203/msg00102`〕. Greg Miller, replying to that
  example, said a classic bubble sort needs six compares to place the 7, then one more pass to notice
  the list is sorted 〔`200203/msg00103`〕; Polik answered him that *"that depends on the sort order"* — his
  example was for the backwards inner loop Jentzsch had used, and the other way round the worst case
  is a zero inserted at the end 〔`200203/msg00106`〕; Mooney: *"Actually, you're right. But in the reverse
  case - 2, 3, 4, 5, 6, 7, 1 - it would take n^2 comparisons although only n swaps"*
  〔`200203/msg00107`〕. The network above makes 9 compares for five objects whatever the order. **Cited only, not verified.**
- **Code size and storage.** Squires, in his own words: quicksort is *"probably not suitable for this
  application"* — with *"the limited stack space"* an iterative version is *"far more complex than a
  simple sort, without giving you any real gains"*, and it *"works best on unsorted lists"*. From the
  MIXAL in Knuth's *Sorting and Searching* (TAOCP vol. 3): quicksort is *"about 63 lines"* and uses a
  stack; bubble sort is *"16 lines and appears to use only one extra word of storage"*; insertion sort *"only 12 lines, uses no additional
  storage, and might perform slightly better than bubble sort in your cases"* 〔`200203/msg00099`〕. He
  later posted both MIX programs with per-line execution counts 〔`200203/msg00138`〕. Andrew Davie, quoting
  Knuth: *"Compared to straight insertion (Algorithm 5.2.1S), bubble sorting requires a more complicated
  program and takes more than twice as long!"* 〔`200203/msg00137`〕. These are MIX line counts, not 6502 bytes or cycles. **Cited only, not verified.**
- **When the simple sort is the right one.** Jentzsch, answering Davie: what Knuth does not write about
  are *"the special cases we are talking about"* — *"small Ns (<20)"*, *"nearly sorted values"*, *"little
  extra memory needed"*, *"small code size"*; bubble sort is generally *"even worse than other simple O(n^2) sorting algorithms"*,
  *"but it is IMO the perfect choice for our special problem"* 〔`200203/msg00139`〕. Squires replied that
  Knuth does treat storage and cycles 〔`200203/msg00140`〕, and Jentzsch: *"Yes, you're right"* — he had
  meant only the passage Davie quoted first, *"doesn't write about *here*"* 〔`200203/msg00142`〕. This page has N = 5 and chose on
  another axis, a fixed compare count. **Cited only, not verified.**
- **Walking the result.** Polik's routine keeps a link table (one byte per object plus a head node, our
  reading of the posted code) and walks it once in sorted order 〔`200203/msg00094`〕. Jentzsch:
  *"Iterating through a sorted list without the additional table is even faster. That helps inside a
  tight kernel"* 〔`200203/msg00095`〕; Polik: the links could be used *"only temporary for the sorting,
  then doing all necessary swaps _once_"* — *"Agreed"* 〔`200203/msg00096`, `200203/msg00097`〕. This kernel
  walks neither: VBLANK copies each placed object's trigger pair and number, in `sortIdx` order, into the two
  slot queues, and the kernel steps through those by index. **Cited only, not verified.**
- **A missile across the repositioning line.** Discussing a layout turned 90° into five
  horizontal segments, with repositioning at each segment border, Polik asked how to show the bullets
  when they cross a border 〔stella-list `200102/msg00326`〕; Glenn Saunders suggested
  disassembling Air Sea Battle or Canyon Bomber 〔`200102/msg00329`〕. Polik: *"The bullet from Air Sea
  battle *jumps* in a certain patern and it's three or four pixels wide, so ENAMx is just hit right
  before & after the repositioning"* — not usable for him, *"since I'd have two sprites to reposition
  every segment, needing at least four complete lines to reposition"* 〔`200102/msg00330`〕. His
  description; the cartridge was not traced here. **Cited only, not verified.**

## Cycle war stories (all measured, all CI-guarded now)
- The assignment's worst path (double slot fallback) is ~160 cycles — uncountable in per-line
  WSYNC budgeting; this alone forced the TIM64T design.
- The B-line POSITION path landed on **exactly 76 cycles** — the closing WSYNC itself crossed
  the boundary. Moving the POSITION block to fall through to the loop tail (deleting one `jmp`,
  −3 cycles) fixed it. Enumerated-spill probing (every interval >76 with its PC) found both.
- Queue-advance was the other spiller until exhaustion checks moved into the sentinel.

## Verified
- 262 lines every frame, **zero visible-region budget spills over 10 frames** (instruction-level
  interval enumeration), all 5 object colors render across frames (multi-frame ingest proof),
  golden + RAM asserts in CI.
