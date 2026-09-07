; litmus_counter_entropy — where the randomness in a free-running counter actually comes from.
;
; Eckhard Stolberg, 1997, telling someone their generator was too regular:
;   "Having a counter run from 0 to 6, that increments in every frame, gave enough ramdomness
;    for my Tetris version."  〔stella-list 199706/msg00005; `ramdomness` is his spelling〕
;
; Seven values is exactly the number of tetrominoes, so there is no modulo either. The obvious
; objection is that a counter is not random at all — and that is the point. Manuel Polik, on the
; other half of the same problem:
;
;   "Total randomness won't work, since you've to *REPEAT* what you're doing every frame"
;
; So there are two needs and this repository only had the expensive one. A starfield or a terrain
; must be REPRODUCIBLE, and that is what an LFSR with a fixed seed is for. A tetromino must only be
; UNPREDICTABLE, and for that the counter is enough — because the entropy is not in the counter, it
; is in **when a person pressed the button**.
;
; This ROM makes that testable. $80 runs 0..6, once per frame, forever. When the fire button is
; down, the current value is latched into $81 and a tally at $90+value is bumped. Drive it with
; irregular presses and the tally spreads; drive it on a fixed period and it does not.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INPT4   = $3C
cnt     = $80     ; free-running 0..6
last    = $81     ; the value latched at the most recent press
held    = $82     ; was the button already down last frame
tally   = $90     ; $90..$96 — how often each value was sampled

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Frame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK

        ; --- the counter: 0..6, one step per frame ---
        inc cnt
        lda cnt
        cmp #7
        bcc NoWrap
        lda #0
        sta cnt
NoWrap:

        ; --- sample it on the FRAME THE BUTTON GOES DOWN (edge, not level) ---
        lda INPT4
        bmi NotDown         ; D7 high = not pressed
        lda held
        bne Same            ; already counted this press
        lda cnt
        sta last
        ldx cnt
        inc tally,x
Same:
        lda #1
        sta held
        jmp Done
NotDown:
        lda #0
        sta held
Done:
        ; ★No colour is driven from `cnt`. An earlier version set COLUBK to `cnt << 4` "to make the
        ; ROM watchable", and the Stella oracle caught it: the background then depends on WHICH
        ; FRAME is sampled, so harness read $50 where Stella read $40 and the two emulators were
        ; being compared on a phase rather than on a behaviour. Nothing here was ever measured from
        ; the picture, so the line bought nothing and cost the ROM its place as an oracle fixture.

        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
