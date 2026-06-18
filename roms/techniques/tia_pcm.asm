; tia_pcm — digitized sample playback via TIA volume (technique: tia-pcm)
; Plays a digitized waveform WITHOUT TIA's tone generators: AUDC0=AUDC1=0 makes
; each channel emit a steady "1" stream, so AUDV (0-15) IS the instantaneous
; amplitude = raw 4-bit PCM. Summing both channels gives a pseudo-5-bit DAC
; (AUDV0+AUDV1 = 0..30 ~= log2(31)=4.95 bit). seagtgruff/Tjoppen, AA #184034.
;
; Source = a small 1-bit ADPCM decoder. A fixed, looped bitstream drives a
; compact LUT-based state machine: each bit walks a 16-entry state index, and
; the state maps to a 0..30 level. One sample is decoded per frame (overscan),
; so output is fully deterministic and the AUDV pair for any frame is fixed.
; The level is split across the two AUDV registers with seagtgruff's 12-cycle
; branchless trick (LSR/STA/ADC/STA). Background color = level (audible AND
; visible feedback). AUDC0/AUDC1 stay 0 the whole run = pure PCM.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
AUDC0   = $15
AUDC1   = $16
AUDF0   = $17
AUDF1   = $18
AUDV0   = $19
AUDV1   = $1A

BITS    = 32        ; bitstream length (bits), looped

state   = $80       ; ADPCM state index 0..15
bitpos  = $81       ; current bit index 0..BITS-1
level   = $82       ; decoded amplitude 0..30 (for COLUBK feedback)

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        ; --- PCM setup: silence tone generators so AUDV = raw amplitude ---
        lda #0
        sta AUDC0
        sta AUDC1
        sta AUDV0
        sta AUDV1
        lda #8              ; start state mid-scale
        sta state

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
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- visible kernel: paint background = current decoded level ---
        lda level
        asl                 ; level<<3 -> spread 0..30 across luminance
        asl
        asl
        ora #$10            ; hue so it's never pure black
        sta COLUBK
        ldx #192
Vis:    sta WSYNC          ; @lines 2 — last visible line + overscan ADPCM-decode span 2 scanlines; verified stable 262
        dex
        bne Vis
        lda #2
        sta VBLANK

        ; --- overscan: decode ONE 1-bit ADPCM sample, drive the DAC ---
        ; fetch current bit of the looped stream
        ldx bitpos
        lda BitMask,x       ; mask isolating bit within its byte
        ldy ByteIdx,x       ; which stream byte
        and Stream,y
        beq Bit0
        lda #1              ; bit = 1
        .byte $2C           ; BIT abs = skip the next 2-byte LDA #0
Bit0:   lda #0              ; bit = 0
        ; index = (state<<1) | bit, with A = bit here
        sta level           ; stash bit (level reused as scratch)
        lda state
        asl                 ; A = state<<1
        ora level           ; A = (state<<1) | bit
        tax
        lda StateLUT,x      ; nextState = StateLUT[index]
        sta state           ; advance state
        tax
        lda LevelLUT,x      ; level = amplitude 0..30 for this state
        sta level

        ; --- split level across both AUDV (pseudo-5-bit DAC), 12cy branchless ---
        lda level
        lsr                 ; A = level/2, carry = low bit
        sta AUDV0
        adc #0              ; add carry back
        sta AUDV1

        ; advance looped bit position
        inc bitpos
        lda bitpos
        cmp #BITS
        bcc OSwait
        lda #0
        sta bitpos
OSwait:
        ldx #28
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; ---- 1-bit ADPCM state machine ----
; StateLUT[(state<<1)|bit] -> next state (0..15). bit=0 steps down, bit=1 up,
; with clamping at the ends. 16 states * 2 = 32 entries.
StateLUT:
        byte  0, 1      ; s0:  down->0, up->1
        byte  0, 2      ; s1
        byte  1, 3      ; s2
        byte  2, 4      ; s3
        byte  3, 5      ; s4
        byte  4, 6      ; s5
        byte  5, 7      ; s6
        byte  6, 8      ; s7
        byte  7, 9      ; s8
        byte  8,10      ; s9
        byte  9,11      ; s10
        byte 10,12      ; s11
        byte 11,13      ; s12
        byte 12,14      ; s13
        byte 13,15      ; s14
        byte 14,15      ; s15: down->14, up->15 (clamp)

; LevelLUT[state] -> amplitude 0..30 (pseudo-5-bit). Roughly linear ramp.
LevelLUT:
        byte  0, 2, 4, 6, 8,10,12,14
        byte 16,18,20,22,24,26,28,30

; ---- fixed, looped 1-bit ADPCM bitstream (BITS bits = 4 bytes) ----
; Chosen so the decoded waveform rises then falls = an audible, time-varying
; envelope (a triangle-ish sweep), not a constant. MSB-first within each byte.
Stream:
        byte %11111111      ; bits 0..7  : climb
        byte %11110000      ; bits 8..15 : peak then drop
        byte %00000000      ; bits 16..23: fall to floor
        byte %00001111      ; bits 24..31: rise again -> loop continuity

; BitMask[bitpos] = mask of the bit within its byte (MSB-first)
BitMask:
        byte $80,$40,$20,$10,$08,$04,$02,$01
        byte $80,$40,$20,$10,$08,$04,$02,$01
        byte $80,$40,$20,$10,$08,$04,$02,$01
        byte $80,$40,$20,$10,$08,$04,$02,$01

; ByteIdx[bitpos] = which Stream byte
ByteIdx:
        byte 0,0,0,0,0,0,0,0
        byte 1,1,1,1,1,1,1,1
        byte 2,2,2,2,2,2,2,2
        byte 3,3,3,3,3,3,3,3

        org $FFFC
        .word Start
        .word Start
