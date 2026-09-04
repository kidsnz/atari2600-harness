# Technique — blank a whole frame (the third answer to a setup that will not fit)

**Goal:** when a *one-off* setup routine — respawn, level load, state transition — cannot finish
inside VBLANK plus overscan, do not trim it and do not let it run over. Set VBLANK for an entire
frame, do the work with the screen off, and give the frame back.

Demo: TODO — no standalone demo yet.
CI: TODO — no gate yet. Proposed litmus + negative control under "How to verify".
Hardware basis: **not measured here.** The only evidence in this repository is a 1999 report from
real hardware, quoted under "Sources": a PAL 7800 showed the picture jump for one frame at respawn,
and the fix proposed by the person who saw it is the technique above.

## Why this is a third answer, not a worse first one

This catalogue already answers "the work does not fit" twice, and both answers assume the work
happens **every frame**:

| answer | what it changes | fits when |
|---|---|---|
| trim the work | fewer objects, cheaper maths | the cost is per-frame and can be reduced |
| spend more lines | 2-line kernel, band splitting | the cost is per-frame and can be spread |
| **blank a frame** | nothing about the kernel | **the cost is per-event, not per-frame** |

A respawn routine runs once per death. Reducing it costs design everywhere in exchange for a
budget that is only tight for one frame in several hundred. Blanking is the answer that scales with
how often the event actually happens.

## What overrunning actually costs

The failure is not "the frame is slow". It is that **the frame is the wrong length**:

- On hardware the TV loses vertical lock for that frame, which is what the report below describes
  ("the screen jumps a bit as if it were out of sync for one frame").
- In this repository the same fault is a *single* frame of the wrong line count. That matters for
  how it is caught: a scenario that reads `ntsc_frame_lines` samples **one** frame, so a one-off
  overrun at frame 40 is invisible to an assert at frame 4. `frame_lines_stable` is what sees it.
  Use both — one catches a frame that varies, the other catches a whole ROM that is stably wrong.

## The shape

```
; one-off event (respawn / level load / mode change)
        lda #2
        sta VBLANK          ; screen off for the WHOLE frame, not just the top
        ; --- the long routine runs here, across what would have been the visible region ---
        ...
        ; end the frame normally: VSYNC, then
        lda #0
        sta VBLANK          ; picture returns next frame
```

**The cost is one blank frame — 1/60 s — and it is paid only on the event.** The alternative
costs a rolling picture at the exact moment the player is looking for their new ship.

## The hazard

**A blank frame is visible.** One is a blink; a chain of them is a fault the player reads as the
game hanging. If the routine needs more than one frame, that is a signal to split it across
frames with the picture on, not to blank several in a row.

**And the blanked frame still has to be a frame.** VSYNC, the line count and the timer discipline
are unchanged — blanking removes the *picture*, not the obligation to emit 262 lines. A routine
that runs long enough to eat the VSYNC as well produces exactly the fault it was meant to avoid.

## How to verify (proposed — not yet run)

1. **Litmus, positive.** A ROM whose per-event routine deliberately overruns VBLANK+overscan on one
   frame in ten. Assert `frame_lines_stable` **fails** and `ntsc_frame_lines == 262` sampled at an
   unaffected frame **passes** — that pair is the point: the second check alone would call the ROM
   green.
2. **Litmus, the technique.** The same routine wrapped in a full-frame VBLANK. Assert
   `frame_lines_stable` passes and every frame is 262.
3. **Negative control.** Remove the `sta VBLANK` and keep the long routine: check 2 must fail.
4. **What this cannot settle.** Whether a real television relocks in one frame or several is not
   measurable here; the emulator has no vertical-hold model. The line count is ours to check, the
   picture rolling is not.

## Sources

- **Eckhard Stolberg, Stella mailing list, 18 Nov 1999** (`reference/stella-list/199911/msg00035.html`),
  testing Thomas Jentzsch's *Thrust* on a PAL 7800:

  > "The only problem is that when a life is lost and a new ship appears, the screen jumps a bit as
  > if it were out of sync for one frame. **If the setup routine for a new ship takes longer to
  > execute than you have time in the VBLANK and overscan part of the screen, it might be better to
  > blank the screen for a full frame instead.**"

  〔distilled at `reference/stella-list/threads/thrust-4aef/notes.ja.md`〕

- The observation is a **report from hardware**, not a measurement of ours, and the technique is his
  proposed remedy rather than something he showed working. Both halves are stated so the next reader
  does not promote either one.
