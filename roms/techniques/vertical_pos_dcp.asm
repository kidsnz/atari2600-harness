; vertical_pos_dcp — skipDraw variant using the undocumented DCP (verified variant of technique #3)
; Produces the same look as the compare method (vertical_pos.asm), but on every line via
;   lda #H-1 / DCP sprDraw / bcs in-range
; — the classic idiom. DCP ($C7 zp) = DEC+CMP combined. sprDraw is initialized to sprY+H every frame,
; decremented every line = "draw only during the H lines where the countdown passes 0..H-1". Art is a reversed table.
; DASM does not accept undocumented mnemonics → encode directly as .byte \$C7.
; Horizontal: fixed to X=80 every frame after startup (divide-by-15 + HMOVE table, pos(v)=v calibrated; same shape as sprite_anim).
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A

SPRITE_H = 8
Y_MIN    = 4
Y_MAX    = 180
XPOS     = 80

sprY     = $80      ; visible line of the sprite's top edge (0-191)
vdir     = $81      ; 0=moving down / 1=moving up
sprDraw  = $82      ; skipDraw counter (init to sprY+H every frame; DCP-decremented every line)
sent     = $9E      ; execution sentinel

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
        lda #$86
        sta COLUP0          ; blue
        lda #Y_MIN
        sta sprY
        lda #$B4
        sta sent

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ; --- VB line 1: vertical movement (ping-pong) ---
        lda vdir
        bne MoveUp
        inc sprY            ; down
        lda sprY
        cmp #Y_MAX
        bcc MvDone
        lda #1
        sta vdir
        jmp MvDone
MoveUp: dec sprY            ; up
        lda sprY
        cmp #Y_MIN+1
        bcs MvDone
        lda #0
        sta vdir
MvDone: lda sprY
        clc
        adc #SPRITE_H
        sta sprDraw         ; skipDraw init (drops by 1 each visible line → in range only for the H lines 0..H-1)
        sta WSYNC           ; VB 1
        ; --- VB line 2: P0 to X=80 (coarse+fine; pos(v)=v calibrated) ---
        lda #XPOS
        clc
        adc #XCAL
        sec
Div:    sbc #15
        bcs Div
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 2
        sta HMOVE
        ldx #34             ; VBLANK remainder (1+1+34+1=37)
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → visible starts

        ; --- Visible 192 lines: skipDraw (DCP). In-range 20cy / out-of-range 17cy = lighter than the compare method ---
        ldy #0
Vis:    sta WSYNC
        lda #SPRITE_H-1     ; 2
        .byte $C7,sprDraw   ; 7   DCP sprDraw (M--; compare with A)
        bcs VDraw           ; 9/10 in range (sprDraw ≤ H-1)
        lda #0              ; 11
        beq VStore          ; 14 (always taken)
VDraw:  ldx sprDraw         ; 13
        lda ArtRev,x        ; 17  reversed table (sprDraw descends H-1→0)
VStore: sta GRP0            ; ~17-20
        iny
        cpy #192
        bne Vis

        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; explicitly own 3+37+192+30 = 262

XCAL = -5                   ; calibration is kernel-specific: this ROM uses lda #imm (2cy), so its prologue is
                            ; 1cy shorter than sprite_anim (lda zp=3cy) = lands 3px left → -8+3. Measured hmoved=80 confirms

ArtRev: ; 8×8 ball (own art; stored bottom→top, reversed for skipDraw)
        byte %00111100
        byte %01111110
        byte %11100111
        byte %11111111
        byte %11011011
        byte %11111111
        byte %01111110
        byte %00111100

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
