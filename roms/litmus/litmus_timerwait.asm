; litmus_timerwait — a loop that waits on the RIOT timer is not an unbounded loop,
; it is a loop that is not measured in cycles at all.
;
; `determineBound` needs a counted `dex`/`dey` or the `sbc` divide idiom. Measured
; across the sixteen-cartridge corpus, of the loops it refuses for having neither:
;
;       12   lda $0284 / bne                     <- INTIM, polled until zero
;        7   ldy $0284 / bne                     <- the same, through Y
;        1   sta $002A / lda $0284 / bne         <- the same, with a store in the body
;        1   sta $xxxx,X / inx / bne             <- a genuine count, upwards
;
; **Twenty of twenty-one are the same idiom.** `$0284` is INTIM, the RIOT's interval
; timer read port. The standard 2600 frame does
;
;       lda #43
;       sta TIM64T          ; 43 x 64 = 2752 cycles from now
;       ...work...
;   W:  lda INTIM
;       bne W               ; spin until the timer reaches zero
;
; and the trip count is not a property of any register the analysis tracks — it is
; whatever the hardware has left to count down. Reporting "loop bound unknown (need a
; counted dex/dey or sbc-divide idiom)" is true and useless: the loop is not missing a
; counter, it is waiting on a clock, and no counter will ever appear.
;
; **The refusal is right and the reason is wrong**, so this fixture is about the
; MESSAGE and the CLASSIFICATION rather than about proving anything new. A builder
; reading "this region cannot be proven" should be told that the region is a timer
; spin, because that is the difference between "the analysis is not strong enough" and
; "there is nothing here to measure in cycles".
;
; THE ROWS:
;
;   TimerRow    the idiom itself, on a short TIM1T wait so it fits a scanline. It must
;               be refused, and refused AS A TIMER WAIT.
;
; THE CONTROLS:
;
;   NotTimerRow the identical shape reading an ordinary RAM address instead of INTIM.
;               Nothing about it is a timer, and it must keep the generic refusal —
;               otherwise the detector is keying on `lda abs / bne` and would rename
;               every polling loop in the corpus.
;   CountedCtl  an ordinary `dex`/`bne`. It must still be BOUNDED at its old value, so
;               the detector is measured not to intercept loops that determineBound
;               can already handle.
;
; TIM1T is used rather than TIM64T so the wait is a few cycles instead of thousands and
; the row stays inside one scanline; the idiom being detected is identical.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INTIM   = $0284
TIM1T   = $0294

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

; --- Visible ---
; THE ROW: the timer spin. Refused, and the reason must say so.
TimerRow:
        sta WSYNC
        lda #8
        sta TIM1T       ; 8 cycles from now, INTIM reaches zero
TWait:  lda INTIM
        bne TWait

; CONTROL 1 — the same shape on a plain RAM address. Not a timer, and the detector
; must not claim it: keying on `lda abs / bne` alone would rename every polling loop
; in the corpus.
NotTimerRow:
        sta WSYNC
        lda #0
        sta $A0
NWait:  lda $A0
        bne NWait

; CONTROL 2 — INTIM IS READ, BUT THE BRANCH IS NOT ABOUT IT. The `ldx` overwrites Z
; before the latch sees it, so the loop spins on $A0 and the timer read is incidental.
; A detector that only asks "does the body contain `lda INTIM`?" calls this a timer
; wait and is wrong about what the ROM does.
FlagRow:
        sta WSYNC
        lda #8
        sta TIM1T
FWait:  lda INTIM       ; read, but its Z is about to be replaced
        ldx $A0         ; THIS is what the latch tests
        bne FWait

; CONTROL 3 — a counted loop determineBound already handles. It must stay bounded.
CountedCtl:
        sta WSYNC
        ldx #5
CtlL:   dex
        bne CtlL

        lda #0
        sta COLUBK
        ldx #187
Fill:   sta WSYNC
        dex
        bne Fill

; --- Overscan: 30 lines ---
        sta WSYNC
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
