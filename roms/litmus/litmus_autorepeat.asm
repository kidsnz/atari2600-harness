; litmus_autorepeat — the third use of the same input machinery, next to the two harness has.
;
; Glenn Saunders, 1996, on a Tetris in progress:
;   "When moving the joystick left and right, when you HOLD the joystick, it should keep moving
;    the piece.  It should not require extra taps.
;    Perhaps some extra 'grace time' when a piece is lying flat so you can 'slide' L-shaped
;    pieces into place after they are on the ground.  That's how the arcade one work[s]"
;   〔stella-list 199612/msg00012〕
;
; game-states.md records edge detection with "hold-to-repeat bugs gone" -- a record of REMOVING
; repetition. design-principles.md records deliberate throttling (a turn-rate governor). Neither
; is deliberate repetition, and that is what a held direction needs.
;
; Two counters run side by side off the SAME button, so the difference is the policy and nothing
; else:
;
;   $80  edge only        -- one step per press, however long it is held
;   $81  auto-repeat      -- one step at once, then one every REPEAT frames after a DELAY
;   $82  frames held      -- so a reader can see how long the input actually lasted
;
; DELAY and REPEAT are the two numbers a designer picks. They are here so a scenario can assert
; them rather than infer them from how the game feels.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INPT4   = $3C
edgeN   = $80
repN    = $81
heldF   = $82
wasDown = $83
timer   = $84

DELAY   = 16      ; frames before the repeat starts
REPEAT  = 8       ; frames between repeats once it has

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

        lda INPT4
        bmi Released        ; D7 high = not pressed

        ; --- held this frame ---
        inc heldF
        lda wasDown
        bne StillDown

        ; the press edge: both policies step once
        inc edgeN
        inc repN
        lda #DELAY
        sta timer
        lda #1
        sta wasDown
        jmp Done

StillDown:
        dec timer
        bne Done
        inc repN            ; only the auto-repeat policy steps again
        lda #REPEAT
        sta timer
        jmp Done

Released:
        lda #0
        sta wasDown
Done:
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
