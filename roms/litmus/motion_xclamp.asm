; motion_xclamp.asm — the HORIZONTAL sibling of motion_glide, and the litmus for the
; trap a single late position read walks into.
;
; P0 glides RIGHT at a constant +2 px/frame from XSTART, CLAMPS at XMAX and holds there,
; and then the "round" ends: after ROUND frames the sprite is reset to XSTART and the
; whole cycle repeats. Its Y is CONSTANT (one fixed 40-line band) — the mirror image of
; motion_glide (Y moves, X fixed). The pair proves the two axes of `spritey` are
; independently live and that neither is being reported into the other.
;
; Why it exists (measured cost, 2026-07-30). Outlaw's horizontal clamp was measured by
; holding "right" for 700 frames and reading the position ONCE. That returned x=7 — near
; the LEFT edge — because by then the round had ended and the gunman was back at his
; start. No error, perfectly stable, plausible, wrong. The correct answer only appeared
; from a SAMPLED trajectory: 17 27 37 47 57 57 57 59 … settling at 57-59. This ROM
; reproduces that shape with known constants, so the claim "prefer the trajectory over a
; single late read" can be machine-checked instead of merely written in a description.
;
; Positioning uses the standard div-15 coarse + HMOVE-fine routine already used by
; roms/techniques/shared_setxpos.asm (technique #115690). The absolute X the routine
; lands on is kernel-specific (the offset includes this prologue's cycle count), so the
; test asserts the SHAPE — strictly increasing, then flat, then reset — against measured
; values, never against a hardcoded formula. (CLAUDE.md: verdict = measured HmovedPixel.)
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
CXCLR   = $2C

posX    = $80           ; the sprite's intended X, in the SetXPos routine's units
tick    = $81           ; frames elapsed in this "round"

XSTART  = 20            ; where every round starts
XMAX    = 100           ; the clamp: the sprite never goes past this
XSTEP   = 2             ; px per frame while gliding  => 40 frames to reach the clamp
ROUND   = 120           ; frames per round; at ROUND the sprite snaps back to XSTART

BANDTOP = 150           ; visible-loop counter bounds of the drawn band (counter runs
BANDBOT = 110           ; 192 down to 1, so this is 40 lines, 43..82 from the top)

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
        sta CXCLR
        sta HMCLR

        lda #$0E
        sta COLUP0      ; P0 white
        lda #$00
        sta COLUBK      ; black background — nothing else shares P0's colour
        lda #0
        sta NUSIZ0      ; one copy, normal width
        lda #XSTART
        sta posX

Frame:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines. 34 idle, then 3 spent positioning P0. ---
        ldx #34
VB:     sta WSYNC
        dex
        bne VB

        jsr UpdateX     ; pure logic, ~25 cycles — fits inside one line
        lda posX
        jsr SetXPos     ; consumes 1 line (its own WSYNC) and strobes RESP0
        sta WSYNC
        sta HMOVE       ; fine adjust applies on the line that follows — still blanked,
        sta WSYNC       ; so the 8px HMOVE comb never touches a visible line
        lda #0
        sta VBLANK

; --- Visible: 192 lines. GRP0 is on only inside the band, so y_top/y_bot are fixed. ---
        ldx #192
Vis:    sta WSYNC
        ldy #0
        cpx #BANDTOP
        bcs SetG        ; above the band
        cpx #BANDBOT
        bcc SetG        ; below the band
        ldy #$FF
SetG:   sty GRP0
        dex
        bne Vis

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Frame

; ---------------------------------------------------------------------------
; UpdateX — advance the sprite one frame: glide right, clamp, and end the round.
; The reset at the end of the round IS the trap: read the position once, late, and
; this is the state you measure.
UpdateX:
        inc tick
        lda tick
        cmp #ROUND
        bcc Glide
        lda #0          ; round over — back to the start, exactly like a game restart
        sta tick
        lda #XSTART
        sta posX
        rts
Glide:
        lda posX
        cmp #XMAX
        bcs Held        ; already at the clamp — hold, do not creep past it
        clc
        adc #XSTEP
        cmp #XMAX
        bcc Store
        lda #XMAX       ; land exactly on the clamp, never overshoot
Store:
        sta posX
Held:
        rts

; SetXPos — A = target X. div-15 coarse (RESP0) + remainder -> HMOVE nibble (HMP0).
SetXPos:
        sec
        sta WSYNC       ; start of a fresh line = deterministic coarse timing
Div15:  sbc #15         ; 2 — one 15px coarse step per iteration
        bcs Div15       ; 3 taken / 2 not
        eor #7          ; 2 — remainder -> HMOVE nibble
        asl             ; 2 \
        asl             ; 2  | the HMxx registers use the upper nibble
        asl             ; 2  |
        asl             ; 2 /
        sta HMP0        ; 3 — stage the fine adjustment
        sta RESP0       ; 3 — strobe the coarse position
        rts

        org $FFFC
        .word Reset
        .word Reset
