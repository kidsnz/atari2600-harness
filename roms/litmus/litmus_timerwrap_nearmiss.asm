; litmus_timerwrap_nearmiss — writes a timer divider JUST AFTER the underflow, not
; on it. Named for what it was MEASURED to be, not for what it was built to be.
;
; On the real RIOT the divider internally drops to 1T when INTIM underflows. A
; write to TIM8T/TIM64T/T1024T on that exact cycle loses the race: the requested
; divider does not take, the timer keeps running at 1T, and an interval the ROM
; intended to last hundreds of scanlines ends almost at once — the frame comes out
; short and the picture rolls.
;
; Gopher2600 was first READ as not reproducing it: its `Timer.Update` assigns the
; requested divider unconditionally and resets ticksRemaining to 0. Both are true
; and the conclusion was not. `Update` also keeps the expired flag when the write
; lands exactly one CPU cycle after the counter goes $00 -> $FF, and the engine then
; counts one per cycle whatever the divider — measured one cycle at a time in
; internal/emu/timerwrap_state_test.go, where every other offset from -6 to +6
; takes the requested interval. This ROM's store is not on that cycle and, measured
; in the same file, takes its interval: it runs correctly under emulation. It was
; BUILT as the "passes in emulation, fails on hardware" shape the harness exists to
; catch, where what a detector can report is the HAZARD, not the failure.
;
; It was built to land the store ON the underflow. It does not, and the measurement
; says so: at the store the timer reads INTIM=255, ticksRemaining=0, divider=8 with
; the expired flag set — it has wrapped and is counting one per cycle, so the next
; underflow is 255+1 = 256 cycles away against a 4-cycle store (this line said
; 255*8 = 2040 until 2026-10-13, as the detector's own distance did). The polling
; loop's exit plus the intervening `lda` take longer than the ticks that remained,
; so the counter has already wrapped by the time the write happens.
;
; That makes this the NEGATIVE control, and a useful one: a divider write shortly
; after a wrap is the ordinary, safe shape, and a detector that flagged it would be
; crying wolf on every timer-driven kernel in the corpus. The positive case — a
; store whose own cycles straddle the wrap — is litmus_timerwrap_hit.
;
;   lda #1 / sta TIM64T     arm a short interval
;   wait   INTIM != 0       poll until the counter reaches 0
;   sta    T1024T           <-- built to be the hazardous write; measured 256
;                               cycles before the next underflow
;
; A ROM that instead waits for the timer to expire and THEN writes is safe, and the
; detector must stay silent on that shape — litmus_timerwrap_clean is that control.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09
INTIM  = $0284
TIM8T  = $0295
TIM64T = $0296
T1024T = $0297

    org $F000

Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clear:
    sta $80,x
    dex
    bne Clear

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
    ldx #37
VB:
    sta WSYNC
    dex
    bne VB

    lda #0
    sta VBLANK
    lda #$2C
    sta COLUBK
    ldx #192
Vis:
    sta WSYNC
    dex
    bne Vis

    lda #2
    sta VBLANK

    ; ---- the trap -----------------------------------------------------------
    ; Arm a short interval, then poll INTIM down to zero. Leaving the loop the
    ; instant INTIM reads 0 puts the CPU on the cycle the counter underflows.
    lda #1
    sta TIM8T
Poll:
    lda INTIM
    bne Poll
    ; INTIM is 0 and its next decrement is due: on hardware the divider is
    ; dropping to 1T right now, and this store loses the race.
    lda #40
    sta T1024T                ; <-- the hazardous write
    ; ---- end of the trap ----------------------------------------------------

    ldx #26
OS:
    sta WSYNC
    dex
    bne OS
    jmp Frame

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
