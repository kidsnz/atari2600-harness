# Technique — the invisible probe (a hidden missile or ball as a hit box)

**Goal:** get a hit test the hardware computes for you, on a region the hardware has no register
for. Park a missile or the ball on the region you want to test, read the collision latch, and hide
the object so the player never sees the instrument.

Demo: TODO — no standalone demo.
CI: TODO — no gate yet. Proposed litmus under "How to verify".
Hardware basis: **the hiding is not measured here.** The priority chain below is read from the
vendored engine's source, not from a litmus; whether a probe is *actually* invisible in a given
kernel is a per-kernel question and nothing in this repository answers it yet.

## Why it works

The 2600 gives you fifteen collision pairs and no way to ask "did the fist reach the head". But
missiles and the ball are **positionable one-clock objects that no colour register of their own**:

| object | takes its colour from | so it disappears over |
|---|---|---|
| `M0` | `COLUP0` (player 0) | player 0, and anything else drawn in `COLUP0` |
| `M1` | `COLUP1` (player 1) | player 1 |
| `BL` | `COLUPF` (playfield) | lit playfield, and the background if `COLUBK == COLUPF` |

〔`Gopher2600/hardware/tia/video/video.go:344` "priority 2 (missile 0 is same color as player 0)";
`:336` "priority 1 (ball is same color as playfield)"〕

So a missile riding on its own player is invisible **by construction** — there is no register to
set wrong. That is the whole trick.

## The three ways to hide a probe, and what each costs

**1 — Same colour (a missile on its own player).** Free: `M0` cannot be a different colour from
`P0`. *Cost:* the missile is only hidden where `COLUP0` is what is being drawn. Over the **other**
player, or over a playfield of a different hue, it shows as a one-to-eight-clock dot. And that
missile is now spent — it cannot also be a bullet.

**2 — Priority (the ball under a player).** Also free, because it is the default. The engine's
normal chain is

```
P0 > M0 > P1 > M1 > BL > PF > BG        〔video.go, the "normal priority" branch〕
```

so a ball underneath either player is covered by it. *Cost:* **you give up `CTRLPF` D2.** Setting
the playfield-priority bit reorders the chain to `PF/BL > P0/M0 > P1/M1 > BG` — the ball rises
**above both players** and your instrument becomes a visible dot on everyone's chest. A technique
that hides by priority and a kernel that wants PF priority cannot share a screen.

**3 — `COLUPF == COLUBK`.** The ball shares the playfield colour, so making the playfield the same
colour as the background hides it everywhere. *Cost:* this is the expensive one — **the playfield
becomes invisible too**, everywhere on the screen, for every line where it holds. Worth it for a
game whose background is a flat colour; ruinous for one that draws with the playfield.

**A fourth, implicit in the source: enable the probe only on the lines it is testing.** A hit box
is a few scanlines tall, so `ENAM0`/`ENABL` is set for those lines and clear for the rest. This is
not really a hiding method — it is what makes the other three cheap, because the exposure is a
handful of pixels rather than a whole sprite.

## The same technique has different requirements in a litmus and in a game

This repository uses the identical idiom for a different purpose, and the requirements barely
overlap:

| | in a **litmus** | in a **game** |
|---|---|---|
| must the probe be invisible? | **no** — nobody is looking at the picture | **yes**, or it is a visible bug |
| must its position be exact? | **yes** — a probe one column off measures the wrong thing | roughly — a hit box is a design choice |
| must its response be calibrated? | **yes, in both directions** (1 when it should be, **0 when it should not**) | rarely — a wrong hit box is a gameplay complaint, not a wrong number |

**Both halves are load-bearing in their own context and dead weight in the other**, which is why
the technique reads as two different techniques depending on who is writing. `roms/litmus/litmus_pf0_reflect.asm`
carries a probe that is deliberately visible and calibrated in both directions before it is trusted
(its band 0); a game wants the opposite trade.

The calibration point is the one worth carrying across: **nothing in this repository measures a
collision field returning 0 when the objects do not overlap.** `roms/litmus/scenarios/collide_all.json`
asserts all fifteen pairs `== 1` with everything overlapped at the left edge and has no `== 0`
assert at all, so a probe's negative direction is unverified. A game can live with that; a
measurement cannot.

## How to verify (proposed — not yet run)

1. **Litmus for the hiding, not the sensing.** Place `M0` on `P0` and `BL` under `P1`, run a frame,
   and assert with `read_row` that the probe columns are **identical to the same columns with the
   probes disabled**. That is the claim "invisible" actually makes, and it is checkable.
2. **Negative control for each hiding method.** (a) move `M0` off `P0` → the row must differ;
   (b) set `CTRLPF` D2 → the ball must appear; (c) set `COLUPF != COLUBK` → the ball must appear.
   Three controls for three methods; a method whose control does not fire was never doing anything.
3. **What this cannot settle.** Whether a probe is invisible *on a television* — the emulator's
   pixel equality is stricter than a CRT, so passing here is necessary and not sufficient.

## Sources

- **Chris Cracknell, Stella mailing list, 11 Nov 1998** (`reference/stella-list/199811/msg00041.html`),
  on Eckhard Stolberg's fighting-game demo:

  > "maybe you could put a missile graphic in the player's fist and a ball graphic in the three areas
  > of the opponent's body that would score a hit. A missle/ball collision would count as a hit. The
  > missle graphic would be the same colour as the player so it wouldn't show up, and if the priority
  > of the ball was under the player and the PF and BG were set to the same colour it wouldn't show
  > up either."

  〔distilled at `reference/stella-list/threads/beat-em-up-08-0c15/notes.ja.md`〕

- The priority chain and the shared colour registers are read from
  `Gopher2600/hardware/tia/video/video.go`; the costs above are derived from that chain, not measured.
