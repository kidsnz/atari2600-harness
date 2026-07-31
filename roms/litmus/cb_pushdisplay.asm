; cb_pushdisplay — the fixture for `pushMissesDisplay`'s "SP can reach the display"
; refusal, which had run ZERO times across the whole corpus.
;
; A PHA writes to $0100|SP, and page 1 mirrors the addresses the console decodes, so
; a program that points SP at the bottom of the stack turns a push into a write to
; VSYNC ($0100) or VBLANK ($0101). That is the Stack Trick. It is the entire reason
; the prover does not simply treat every push as display-touching — it tracks SP and
; asks whether the range can get there.
;
; Nothing exercised the branch where the answer is YES. Measured 2026-07-30 over 129
; ROMs: `pushMissesDisplay` is reached by ONE ROM (rts_dispatch), which takes the
; "proved to miss" path, and litmus_stack_trick — the fixture written for this very
; hazard — never reaches the predicate at all. The hazard had a fixture and the
; predicate had a witness, and they were not the same ROM.
;
; Here the overscan's last region is opened by a WSYNC, has VBLANK already on, and
; contains nothing that stores to the display — so it would be classified BLANK and
; its cost skipped — except for a PHA taken with SP = 1, which lands on $0101. The
; prover must therefore report that region as "visible", not "blank".
;
; Pairs with cb_pushsafe.asm, identical but for the SP value, whose push lands at
; $01FF (ordinary stack RAM) and whose region stays blank. The two differ by one
; immediate operand, so the reclassification is attributable to the SP range and to
; nothing else.
;
; The pushed value is $02 — VBLANK's "on" bit — so the write is a no-op at runtime
; and the frame stays a clean 262 lines. What is being tested is the prover's
; classification, not a visual effect.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

STACKTOP = 1            ; <- the whole difference from cb_pushsafe: $0100|1 = VBLANK

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible: 192 lines ---
        ldx #192
Vis:    sta WSYNC
        stx COLUBK
        dex
        bne Vis

; --- Overscan: 30 lines, the last of which carries the push ---
        lda #2
        sta VBLANK
        ldx #28
OS:     sta WSYNC
        dex
        bne OS

        sta WSYNC       ; opens the region under test: VBLANK on, no display stores
        ldx #STACKTOP
        txs             ; SP is now provably {STACKTOP}
        lda #2
        pha             ; $0100|SP — the write the prover has to notice
        ldx #$FF
        txs             ; hand the stack back before the next frame
        sta WSYNC       ; CLOSES it here. Without this the region ran on through
                        ; `jmp Main` into the next frame's `sta VSYNC`, which is a
                        ; display store, so it was classified visible in BOTH twins
                        ; and the fixture proved nothing (measured before shipping).
        jmp Main

        org $FFFC
        .word Reset
        .word Reset
