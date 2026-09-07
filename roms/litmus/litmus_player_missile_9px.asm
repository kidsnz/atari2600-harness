; litmus_player_missile_9px — can a player and a missile make ONE 9-pixel shape?
;
; Thomas Jentzsch, 2001, describing a shape the hardware would not give him:
;   "all cursors are 7 pixels wide (has to be an odd number to make the up/down arrows look
;    nice) and so the 'hole' in the stop cursor is 5 pixels wide.  That means, I can only use a
;    4 pixel wide ball"  〔stella-list 200102/msg00234〕
;
; Andrew Davie's answer:
;   "Instead of 7-wide, make the cursor 9 wide.  Use the missile to give you the extra pixel you
;    need.  (8 player + 1 missile)  Then it is a simple-matter to use an 8-wide ball to provide
;    the white area you need."  〔200102/msg00238〕
;
; The point is that a ball is 1, 2, 4 or 8 pixels and nothing else, so a shape whose width must be
; ODD is built as a power of two PLUS one. The missile carries the player's colour (M0 uses
; COLUP0), so the join is invisible if the two are adjacent.
;
; Three bands, so the join can be seen to be a join and not a coincidence:
;
;   A   player alone, all eight bits set          -> one run of 8
;   B   player + missile placed to abut it        -> one run of 9
;   C   missile alone                             -> one run of 1
;
; B is the measurement; A and C are what make "9" mean "8 and 1 touching" rather than "some run".
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
RESM0   = $12
GRP0    = $1B
ENAM0   = $1D
HMCLR   = $2B

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
        sta HMCLR
        lda #$0E
        sta COLUP0
        lda #$00
        sta COLUBK
        sta NUSIZ0          ; one copy, missile width 1

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

        ; Place P0, then strobe RESM0 on the very next instruction. Three CPU cycles is nine
        ; colour clocks, and a player is eight wide, so the missile lands on the clock
        ; immediately after the player's last -- no fine motion needed.
        ;
        ; That is not what this litmus was first written to do: it applied HMM0=$1 to pull the
        ; missile one clock left, on the reasoning that nine clocks would leave a gap. Measured,
        ; the shift put the missile ON the player's last pixel and band B stayed eight wide. The
        ; arithmetic was wrong by one and the picture said so.
        sta WSYNC
        ldx #10
Pa:     dex
        bne Pa
        sta RESP0           ; P0 starts here
        sta RESM0           ; M0 three CPU cycles later = nine colour clocks
        sta WSYNC
        sta HMCLR

        ldx #28
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ---- band A: player alone ----
        lda #$FF
        sta GRP0
        lda #0
        sta ENAM0
        ldy #20
Arow:   sta WSYNC
        dey
        bne Arow

        ; ---- band B: player and missile together ----
        lda #2
        sta ENAM0
        ldy #20
Brow:   sta WSYNC
        dey
        bne Brow

        ; ---- band C: missile alone ----
        lda #0
        sta GRP0
        ldy #20
Crow:   sta WSYNC
        dey
        bne Crow

        lda #0
        sta ENAM0
        ldx #120
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
