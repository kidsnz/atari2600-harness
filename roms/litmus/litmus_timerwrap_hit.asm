; litmus_timerwrap_hit — the POSITIVE witness the timer-divider detector never had.
;
; `emu.WatchTimerDividerHazard` reports a write to TIM1T/TIM8T/TIM64T/T1024T whose
; own cycles straddle the counter's underflow. On real hardware the RIOT's divider
; internally drops to 1T at that moment, so the requested divider is lost and the
; interval comes out ~64x or ~1024x short — the ROM rolls on hardware while every
; emulator passes. Gopher2600 does not reproduce the race (its `Update` assigns the
; divider unconditionally), which is exactly why a DETECTOR is needed rather than a
; test: the consequence is invisible here, only the hazard can be reported.
;
; The detector's condition is
;
;       TicksRemaining + INTIM * Divider  <=  the store's own cycle count
;
; and until now NOTHING in the repository satisfied it. `litmus_timerwrap_nearmiss`
; is the negative control — it writes JUST AFTER a wrap, the ordinary safe shape, and
; measures 0 hazards. The audit says so in as many words: *"the positive case is
; untested ... this detector's silence must not be read as 'no hazard here'."*
;
; A detector that has never been seen to FIRE is not evidence, and the whole point of
; this one is to fire on a shape a builder must not ship.
;
; HOW THE SHAPE IS BUILT. TIM1T counts one tick per CPU cycle, so after `sta TIM1T`
; with A = N the underflow is about N cycles away. `sta T1024T` in absolute mode
; spans 4 cycles. Putting a small, known number of cycles between the two therefore
; places the underflow INSIDE the second store. Each row below uses a different N so
; that at least one lands in the window whatever the off-by-one turns out to be — the
; arming store's own cycles and the RIOT's tick phase both shift it, and neither is
; worth deriving when a sweep settles it. The test names WHICH rows fired.
;
; Rows are one per scanline so a fired hazard's reported scanline identifies it.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INTIM   = $0284
TIM1T   = $0294
TIM8T   = $0295
TIM64T  = $0296
T1024T  = $0297

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

; --- Visible: the sweep -----------------------------------------------------
; Each row arms TIM1T with a different N and then stores a divider a fixed, small
; number of cycles later. `lda #imm` is 2 cycles; `sta abs` is 4.

N1:     sta WSYNC
        lda #1
        sta TIM1T       ; underflow ~1 cycle away
        lda #40         ; 2
        sta T1024T      ; 4  <- candidate

N2:     sta WSYNC
        lda #2
        sta TIM1T
        lda #40
        sta T1024T

N3:     sta WSYNC
        lda #3
        sta TIM1T
        lda #40
        sta T1024T

N4:     sta WSYNC
        lda #4
        sta TIM1T
        lda #40
        sta T1024T

N5:     sta WSYNC
        lda #5
        sta TIM1T
        lda #40
        sta T1024T

N6:     sta WSYNC
        lda #6
        sta TIM1T
        lda #40
        sta T1024T

N7:     sta WSYNC
        lda #7
        sta TIM1T
        lda #40
        sta T1024T

N8:     sta WSYNC
        lda #8
        sta TIM1T
        lda #40
        sta T1024T

; A wider gap: two extra `nop`s (4 cycles) before the store, so a larger N can also
; land in the window. Together the two groups cover N = 1..12 in effect.
N9:     sta WSYNC
        lda #9
        sta TIM1T
        nop
        nop
        lda #40
        sta T1024T

N10:    sta WSYNC
        lda #10
        sta TIM1T
        nop
        nop
        lda #40
        sta T1024T

N11:    sta WSYNC
        lda #11
        sta TIM1T
        nop
        nop
        lda #40
        sta T1024T

N12:    sta WSYNC
        lda #12
        sta TIM1T
        nop
        nop
        lda #40
        sta T1024T

        lda #0
        sta COLUBK
        ldx #180
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
