; litmus_tim64t_zero — how many cycles after `STA TIM64T` does INTIM reach 0?
;
; Two answers are in circulation and they differ by a scanline.
;
;   harness `docs/capability-gap-audit.md`   43 * 64          = 2752
;   Erik Mooney, stella-list 2004-05         1 + 42 * 64      = 2689
;
;   2752 / 76 = 36.2  -> 37 lines if the wait is spent on WSYNC
;   2689 / 76 = 35.4  -> 36 lines
;
; Mooney's reasoning is that the first decrement is immediate: write 43, read 42 straight
; away, then 64 cycles per step down to 0. That matches what fundamentals-audit.md already
; measures for a different value (write 20, read 19 next instruction).
;
; This ROM does not count cycles itself -- it parks the answer where the harness can read it.
; It writes N to TIM64T, spins on INTIM until it reads zero, and stores a marker. The cycle
; count comes from the emulator, which can count exactly; a counting loop inside the ROM
; would only measure its own loop.
;
;   $80  = 0 before the write, $01 once INTIM has been seen at zero
;   $81  = the value INTIM returns on the instruction immediately after the write
;
        processor 6502
INTIM   = $284
TIM64T  = $296
WSYNC   = $02
VBLANK  = $01

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
        lda #2
        sta VBLANK

Again:
        lda #0
        sta $80
        sta $81
        lda #43
        sta TIM64T          ; <- the emulator's cycle counter is read here
        lda INTIM           ; the very next instruction: Mooney says 42, not 43
        sta $81
Spin:
        lda INTIM
        bne Spin
        lda #1
        sta $80             ; <- and here
        jmp Again

        org $FFFC
        .word Start
        .word Start
