; litmus_switchdraw — a sprite kernel whose DRAW and SKIP paths cost the SAME, and what that costs.
;
; `fundamentals-audit.md` records the problem, measured 2026-09-03: the classic skipDraw idiom is
; **20 cycles on the lines that draw and 17 on the lines that skip**, so a kernel budgeted at a
; constant loses three cycles on exactly the tightest lines. `roms/techniques/vertical_pos_dcp.asm`:
;
;       lda #H-1            ; 2
;       .byte $C7,sprDraw   ; 7   DCP: sprDraw--, compare
;       bcs VDraw           ; 9/10
;       lda #0              ; 11        <- skip path, total 17
;       beq VStore          ; 14
;   VDraw: ldx sprDraw      ; 13        <- draw path
;       lda ArtRev,x        ; 17        total 20
;   VStore: sta GRP0
;
; The mailing list has the answer, 2005-02 (Thomas Jentzsch, "NOT skipdraw"): a replacement whose
; branches do not differ. The follow-up thread names it **SwitchDraw**. Nobody wrote down what it
; costs — the first post says only *"it has some disadvantages"* and the next two are a correction
; and an extra saving. **So this ROM measures both halves: that the paths agree, and the price.**
;
; The method here removes the branch entirely by making the TABLE absorb the range test. `sprDraw`
; is decremented every line and wraps through the whole byte range, so a table covering all 256
; indices — the art at 0..H-1, zero everywhere else — turns "am I in range" into an array lookup:
;
;       lda #H-1            ; 2
;       .byte $C7,sprDraw   ; 7   DCP: sprDraw--, sets C (unused now)
;       ldx sprDraw         ; 10
;       lda Art256,x        ; 14  <- zero unless in range
;       sta GRP0            ; 17  ALWAYS
;
; **17 cycles on every line, drawing or not** — the same as the old skip path, three cycles better
; than the old draw path, and, more importantly for beam racing, *the same number twice*.
;
; The price, which is the part the list left blank: **a 256-byte table instead of an 8-byte one**.
; On a 4K cartridge that is 6% of the ROM for one sprite, and it does not scale — two sprites with
; different art want two tables. That is the trade this file exists to state: **three cycles per
; line, bought with 248 bytes.** Whether that is worth it is a budget question, not a technique
; question, and it depends on which resource is binding — see `capability-gap-audit.md`.
;
; Bands: the sprite is drawn at a fixed Y so the picture is checkable, and the test times
; WSYNC->GRP0 on both a drawing line and a skipping line.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
COLUP0  = $06
GRP0    = $1B
RESP0   = $10
SPRITE_H = 8
SPRITE_Y = 40
sprDraw  = $82

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
        lda #$0E
        sta COLUP0
        lda #0
        sta COLUBK

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ; position the player in the visible area (a strobe in HBLANK clamps left)
        sta WSYNC
        ldy #8
DelayP: dey
        bne DelayP
        sta RESP0

        lda #SPRITE_Y+SPRITE_H
        sta sprDraw

        ldx #36             ; 1 (positioning line) + 36 = 37 VBLANK lines
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- 192 visible lines, BRANCHLESS: every line costs the same ---
        ldy #0
Vis:    sta WSYNC
        lda #SPRITE_H-1     ; 2
        .byte $C7,sprDraw   ; 7   DCP sprDraw (M--, compare; carry now unused)
        ldx sprDraw         ; 10
        lda Art256,x        ; 14  zero unless in range — the table IS the range test
        sta GRP0            ; 17  same cycle on every line
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
        jmp NextFrame

; The 256-byte table. Indices 0..SPRITE_H-1 carry the art (reversed, because sprDraw descends);
; every other index is 0, which is what removes the branch. This is the cost.
        align 256
Art256:
        .byte $3C,$7E,$FF,$FF,$FF,$FF,$7E,$3C     ; 8 rows of art
        ds 248, 0                                  ; <- the price of a constant-cost kernel

        org $FFFC
        .word Start
        .word Start
