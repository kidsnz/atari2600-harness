; shared_setxpos — position all 5 objects (P0,P1,M0,M1,BL) with ONE shared routine
; (technique #115690 "reposition all 5 objects in 1 shared code", LS_Dracon/robert-m 2007)
;
; Core idea: the TIA strobe registers RESP0..RESBL ($10..$14) and the motion registers
;   HMP0..HMBL ($20..$24) are *consecutive*. So a single SetXPos routine, indexed by an
;   object number X (0=P0,1=P1,2=M0,3=M1,4=BL), can coarse-position via RESP0,x and stage
;   the fine HMOVE nibble via HMP0,x — one shared code path for all five movable objects.
; The X coordinates are laid in RAM in RESxx strobe order, then a DEX/BPL loop walks them.
;   One final WSYNC+HMOVE applies all five fine adjustments at once.
;
; div15 positioning: sbc #$0f loop = 15px coarse step; the remainder is mapped to the
;   HMOVE nibble (eor #7, asl x4). Final verdict = tia.<obj>.hmoved_pixel.
        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
RESP0   = $10           ; RESP0/RESP1/RESM0/RESM1/RESBL = $10..$14 (consecutive)
GRP0    = $1B
GRP1    = $1C
ENAM0   = $1D
ENAM1   = $1E
ENABL   = $1F
HMP0    = $20           ; HMP0/HMP1/HMM0/HMM1/HMBL = $20..$24 (consecutive)
HMOVE   = $2A
HMCLR   = $2B

xpos    = $80           ; ×5: X coords in RESxx order (P0,P1,M0,M1,BL) @ $80..$84

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

        ; --- intended X coordinates, laid out in RESxx strobe order ---
        ;   P0=20  P1=55  M0=85  M1=110  BL=140
        lda #20
        sta xpos+0          ; P0
        lda #55
        sta xpos+1          ; P1
        lda #85
        sta xpos+2          ; M0
        lda #110
        sta xpos+3          ; M1
        lda #140
        sta xpos+4          ; BL

        ; --- appearance so all five are visible at distinct X ---
        lda #$1E            ; P0 yellow
        sta COLUP0
        lda #$46            ; P1 red
        sta COLUP1
        lda #$96            ; PF/ball cyan-ish (ball uses COLUPF)
        sta COLUPF
        lda #$00            ; black background
        sta COLUBK
        lda #%00110000      ; M0 width = 8 clocks (NUSIZ missile size bits)
        sta NUSIZ0
        lda #%00110000      ; M1 width = 8 clocks
        sta NUSIZ1
        lda #%00010000      ; ball width = 4 clocks (CTRLPF size bits)
        sta CTRLPF
        lda #$FF            ; players solid 8px
        sta GRP0
        sta GRP1
        lda #%00000010      ; enable M0
        sta ENAM0
        sta ENAM1           ; enable M1
        lda #%00000010      ; enable ball
        sta ENABL

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC          ; @lines 2 — vblank-top setup spans 2 scanlines; verified stable 262
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        sta HMCLR

        ; ========= the shared positioning loop =========
        jsr PositionAllObjects

        ; --- finish VBLANK (37 lines total). 3 used by VSYNC already. ---
        ldx #29
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- visible: 192 lines (objects keep their GRP/ENA state) ---
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis

        ; --- overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; ===========================================================================
; PositionAllObjects — the shared loop. Walks xpos[] in RESxx order and calls
;   the single SetXPos routine for each. One trailing WSYNC+HMOVE applies all
;   five fine adjustments at once.
PositionAllObjects:
        ldx #4              ; 4=BL down to 0=P0
PosLoop:
        lda xpos,x          ; next object's intended X
        jsr SetXPos         ; shared routine positions object X
        dex
        bpl PosLoop
        sta WSYNC
        sta HMOVE           ; apply all 5 fine adjustments simultaneously
        rts

; SetXPos — THE shared routine. A = target X, X = object number (0..4).
;   div15 coarse (RESP0,x) + remainder -> HMOVE nibble (HMP0,x). Indexed by X,
;   so one code path serves P0,P1,M0,M1,BL.
SetXPos:
        sec
        sta WSYNC           ; start of a fresh line (deterministic coarse timing)
WaitObj:
        sbc #15             ; 2 — subtract 15 (one coarse 15px step)
        bcs WaitObj         ; 2/3 — loop until borrow (time-kill = coarse position)
        eor #7              ; 2 — remainder -> HMOVE nibble correction
        asl                 ; 2 \
        asl                 ; 2  | shift low nibble to high nibble (HMxx uses upper)
        asl                 ; 2  |
        asl                 ; 2 /
        sta HMP0,x          ; 4 — stage fine adjustment for THIS object (indexed)
        sta.w RESP0,x       ; 5 — strobe RESxx (coarse). .w forces 5cy fixed timing
        rts

        org $FFFC
        .word Start
        .word Start
