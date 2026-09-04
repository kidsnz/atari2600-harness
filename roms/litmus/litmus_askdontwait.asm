; litmus_askdontwait — polling the RIOT timer WITHOUT blocking on it, and what the poll costs.
;
; Every use of INTIM in this repository waits: `lda INTIM / bne loop` in `timerwrap_clean.asm`,
; `sound_driver.asm`, and three more. All of them ask "has the interval finished?" and stand still
; until it has. stella-list 2002 (Roger Williams) uses the timer the other way round — **ask whether
; there is room for one more unit of work, and if not, drop the work and carry on**:
;
;       lda #$FC        ; 2
;       and INTIM       ; 4   (INTIM is $0284: absolute, four cycles)
;       bne HaveTime    ; 2 not taken / 3 taken
;       ...             ; out of time -> skip the unit
;
; Two things are worth measuring and neither was written down.
;
; **1. The poll is not constant.** `bne` costs 3 taken and 2 not taken, so the question costs **8
; cycles when the answer is "no time" and 9 when it is "time remains"** — the same one-branch
; asymmetry that made skipDraw 17/20, one third the size. For a poll run once per line that is 192
; cycles a frame of jitter, which is why it belongs on the same shelf as `litmus_switchdraw`.
;
; **2. The mask sets the granularity, and the granularity is coarse.** `AND #$FC` ignores the low two
; bits, so the poll cannot see the last 3 units of the interval. Under `TIM64T` one unit is 64 cycles,
; so the answer "no time left" can arrive with up to **4 x 64 = 256 cycles** still on the clock —
; three quarters of a scanline of headroom given away to make the test cheap. A wider mask sees more
; and costs the same; the choice is a resolution/waste trade, not a speed one.
;
; The ROM records, per frame: the poll's own cycle cost on each path (timed by the test), how many
; work units completed, and the INTIM value the loop stopped at.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
TIM64T  = $0296
INTIM   = $0284
units    = $80      ; how many units of work fitted
stopAt   = $81      ; INTIM when the loop gave up
mask     = $82      ; the mask under test (the test pokes this)

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
        lda #$FC
        sta mask            ; default mask; the test varies it

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ; --- the measured region: one timer interval, filled by asking without waiting ---
        lda #0
        sta units
        lda #20
        sta TIM64T          ; 20 units x 64 = 1280 cycles of budget
        lda INTIM
        sta $83             ; ★INTIM の初期値（推論せず記録する）

Ask:    lda mask            ; 3   the question
        and INTIM           ; 7   (abs)
        beq NoTime          ; 9 / 10
        inc units           ; 14  one unit of "work"
        nop                 ; 16
        nop                 ; 18
        jmp Ask             ; 21
NoTime: lda INTIM
        sta stopAt          ; what was still on the clock when we gave up
        lda units
        sta $84             ; ★完了値を別番地へ。$80 は毎フレーム 0 に戻るので、
                            ; ★★フレーム境界で読むと【途中の数】が返る（私が一度そう読んだ）
        ; The poll region's LENGTH depends on the mask, so drain the interval and resync: the frame
        ; must be 262 lines whatever the mask is, or the measurement changes the thing measured.
TDrain: lda INTIM
        bne TDrain
        sta WSYNC

        ; make the count visible so the picture is not blank
        lda units
        sta COLUBK

        ldx #21             ; 3 VSYNC + 17 (the drained interval) + 20 = 40
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
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
