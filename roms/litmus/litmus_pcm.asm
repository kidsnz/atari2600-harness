; litmus_pcm — 4-bit digitised-speech PCM stream, ONE sample per scanline.
;
; What this fixture is FOR. The mined speech technique (AtariAge topic/234209,
; "Doctor Who Berzerk w/speech", iesposta + spiceware) plays digitised voice on a
; stock 2600 with no extra chip: AUDC is parked so the tone generator contributes
; nothing, and the 4-bit VOLUME register AUDV0 is written as a DAC — one sample per
; fixed time slot. Samples are the top nibble of each 8-bit source sample, packed
; TWO PER BYTE, and the thread's own warning is that the packer and the player must
; agree on nibble order (iesposta packs low-first, spiceware high-first; a mismatch
; degrades the voice). The thread's other warning is about TIME, not values: the old
; Berzerk speech hack made the TV roll and lose sync during playback, and this ROM's
; whole reason to exist is to be a stream whose slot timing can be MEASURED, so a
; kernel that loses cycles is caught by the clock rather than by ear.
;
; Contract this ROM promises (the checker in internal/pcm grades against it):
;   - AUDC0 = 0 and AUDF0 = 0 for the whole run: AUDV0 IS the amplitude.
;   - 144 samples per frame, 4-bit, HIGH NIBBLE FIRST, from the table below.
;   - exactly ONE sample per scanline, 144 consecutive scanlines.
;   - the first sample lands on the first visible line = 3 VSYNC + 37 VBLANK = 40.
;   - every write lands at the same beam clock, on both line types (the low-nibble
;     line is padded with three NOPs so it matches the high-nibble line's cycle
;     count before the store). A drifting kernel therefore shows up twice: in the
;     scanline slot and in the intra-line clock.
;   - 262 scanlines: 3 VSYNC + 37 VBLANK + 192 visible (144 PCM + 48 plain) + 30 OS.
;
; Rate: one write per scanline = 3579545/228 = 15699.9 Hz, ~4x the 3900-4000 Hz the
; thread recommends for speech, so a real voice would hold each sample for 4 lines.
; One-per-line is used here because it is the tightest slot grid the technique has,
; and the tightest grid is the one a lost cycle breaks first.
;
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
AUDC0   = $15
AUDF0   = $17
AUDV0   = $19

PACKED  = 72       ; bytes of packed sample data (2 samples each = 144 samples)

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        ; --- PCM setup: park the tone generator so AUDV0 is raw amplitude ---
        lda #0
        sta AUDC0
        sta AUDF0
        sta AUDV0

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC          ; VSYNC line 0
        sta WSYNC          ; VSYNC line 1
        sta WSYNC          ; VSYNC line 2
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC          ; VBLANK lines 3..39
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- 144 PCM lines: scanlines 40..183, one AUDV0 write each ---
        ; X is the byte index for the whole loop (nothing here clobbers it), so both
        ; line types reach their store after the SAME number of cycles from WSYNC:
        ;   high line: LDA 4 + LSR*4 8 + STA 3 = 15cy
        ;   low  line: LDA 4 + AND 2 + NOP*3 6 + STA 3 = 15cy
        ; 15 cycles = 45 colour clocks from the HBLANK start (-68) -> clock -23.
        ldx #0
PcmLoop:
        sta WSYNC          ; high-nibble line
        lda PcmData,x      ; 4cy  (table is page-aligned: no page-cross penalty)
        lsr                ; 2cy  \
        lsr                ; 2cy   |  A = high nibble = sample 2n
        lsr                ; 2cy   |
        lsr                ; 2cy  /
        sta AUDV0          ; 3cy
        stx COLUBK         ; visible feedback: background walks with the sample index

        sta WSYNC          ; low-nibble line
        lda PcmData,x      ; 4cy
        and #$0F           ; 2cy   A = low nibble = sample 2n+1
        nop                ; 2cy  \
        nop                ; 2cy   |  pad to the high-nibble line's cycle count
        nop                ; 2cy  /
        sta AUDV0          ; 3cy
        inx
        cpx #PACKED
        bne PcmLoop

        ; --- 48 plain visible lines, keeping the visible field at 192 ---
        ; AUDV0 is deliberately NOT written again: with AUDC0=0 the channel emits a
        ; constant, so the last sample just holds a DC level (inaudible), and the
        ; frame therefore contains EXACTLY 144 AUDV0 writes — the denominator stays
        ; the stream itself instead of "the stream plus the kernel's housekeeping".
        lda #0
        sta COLUBK
        ldx #48
Vis2:   sta WSYNC
        dex
        bne Vis2

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC          ; overscan lines 232..261
        dex
        bne OS
        jmp NextFrame

; ---- the intended waveform ----
; 144 4-bit samples packed two per byte, HIGH NIBBLE FIRST. The block between the
; two markers is parsed by internal/pcm as the intended stream, so this table is the
; single source of truth for both the player above and the grader: a typo in either
; one shows up as a mismatch instead of cancelling out.
; Regions, in order: ramp up 0-15 (16) / ramp down 15-0 (16) / square burst 15,0 x8
; (16) / staircase (16) / triangle 0-15-0 (32) / plateaus 8,3,13,8 (32) / ramp up
; 0-15 (16). Deliberately NOT periodic at the frame level, so a stream that is late
; by a whole number of slots cannot alias onto itself.
; The table is page-aligned so `lda PcmData,x` never crosses a page: a crossing read
; costs +1 cycle and would move the store 3 colour clocks later on part of the frame.
        align 256
; PCM_TABLE_BEGIN
PcmData:
        byte $01,$23,$45,$67,$89,$AB,$CD,$EF
        byte $FE,$DC,$BA,$98,$76,$54,$32,$10
        byte $F0,$F0,$F0,$F0,$F0,$F0,$F0,$F0
        byte $00,$44,$88,$CC,$FF,$CC,$88,$44
        byte $01,$23,$45,$67,$89,$AB,$CD,$EF
        byte $FE,$DC,$BA,$98,$76,$54,$32,$10
        byte $88,$88,$88,$88,$33,$33,$33,$33
        byte $DD,$DD,$DD,$DD,$88,$88,$88,$88
        byte $01,$23,$45,$67,$89,$AB,$CD,$EF
; PCM_TABLE_END

        org $FFFC
        .word Start
        .word Start
