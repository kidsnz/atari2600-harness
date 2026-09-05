; litmus_lsradcror — what the LSR/ADC/ROR sequence in an unnamed 2002 disassembly actually is.
;
; stella-list, 200201/msg00015. Manuel Polik posts five instructions pulled from a disassembly and
; asks what they are for:
;
;       LDA $82 / LSR / ADC #$00 / LSR / ROR $82
;
; and adds "**The carry state on entry of this part may vary.** $82 starts with 0 in the first
; frame. **Is this some sort of a random number/sequence generator, or what is in $82?**"
;
; **Nobody answered.** The thread is two messages long. It is the kind of question the archive is
; full of and the kind this harness can close, because the answer is a property of the 6507 and not
; of anyone's opinion: enumerate the map and look at its orbits.
;
; The claim under test came from the distillation (helper-3), by enumerating the map in Python:
;
;   - the map is a **bijection** on 0..255 (nothing is lost)
;   - its orbits are: one **fixed point** ($00), one cycle of length **3**, and **four** cycles of
;     length **63**  (1 + 3 + 63*4 = 256)
;   - so from almost any seed the sequence **returns exactly to its start after 63 steps** — at
;     60 Hz that is **1.05 seconds**, which is short enough to see
;   - the **entry carry does not matter**, because the first `LSR` overwrites it
;
; Every one of those is arithmetic done off the machine, which is exactly the kind of claim that
; needs a second implementation before it is written down. This ROM runs the real instructions on
; the real CPU and records:
;
;       $80  period from seed $01           (steps until the value returns to $01, 0 = did not)
;       $81  period from seed $02
;       $82  the working register (the sequence's own variable, as in the original)
;       $83  period from seed $03
;       $84  1 if the orbit from $01 and the orbit from $01 with the entry carry SET agree,
;            0 if they differ  — the answer to Polik's own worry
;       $85  the fixed point check: value after stepping $00 once (want $00)
;       $88  how many of the 256 seeds have period 1   (fixed points)
;       $89  how many have period 3
;       $8A  how many have period 63
;       $8B  how many have any other period, including "did not close within 255"
;       $90.. the first 16 values of the orbit from seed $01, in order
;
; Note the ROM deliberately keeps the sequence's variable at $82, the address the 2002 post used,
; so the listing can be read against the post without translation.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

per1    = $80
per2    = $81
seq     = $82      ; ★the sequence's own variable — the same address as the 2002 post
per3    = $83
carryOK = $84
fixed   = $85
count   = $86
seed    = $87
tmp     = $8F      ; ★$88-$8B are the census counters
orbit   = $90      ; $90..$9F: the first 16 values from seed $01

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

        ; --- period from each of three seeds -------------------------------------------
        lda #$01
        sta seed
        jsr Period
        lda count
        sta per1

        lda #$02
        sta seed
        jsr Period
        lda count
        sta per2

        lda #$03
        sta seed
        jsr Period
        lda count
        sta per3

        ; --- the fixed point: stepping $00 must give $00 --------------------------------
        lda #0
        sta seq
        jsr Step
        lda seq
        sta fixed

        ; --- does the entry carry change anything? --------------------------------------
        ; Run one step from $01 with carry CLEAR, keep it; run again with carry SET; compare.
        lda #$01
        sta seq
        clc
        jsr Step
        lda seq
        sta tmp
        lda #$01
        sta seq
        sec
        jsr Step
        lda seq
        cmp tmp
        bne CarryDiffers
        lda #1
        sta carryOK
        jmp CensusInit
CarryDiffers:
        lda #0
        sta carryOK

CensusInit:
        ; --- the FULL orbit census: every one of the 256 seeds, counted by period ------
        ; ★This is the part the arithmetic cannot be trusted for. Three seeds agreeing is three
        ; ★samples; the claim is about the whole map, so the machine walks the whole map.
        ;   $88 seeds whose period is 1   (fixed points)
        ;   $89 seeds whose period is 3
        ;   $8A seeds whose period is 63
        ;   $8B seeds with any other period, INCLUDING "did not close within 255"
        lda #0
        sta $88
        sta $89
        sta $8A
        sta $8B
        lda #0
        sta seed
Census:
        jsr Period
        lda count
        cmp #1
        bne C3
        inc $88
        jmp CNext
C3:     cmp #3
        bne C63
        inc $89
        jmp CNext
C63:    cmp #63
        bne COther
        inc $8A
        jmp CNext
COther: inc $8B
CNext:
        inc seed
        bne Census          ; wraps to 0 after $FF -> all 256 seeds visited

        ; --- the first sixteen values from seed $01 -------------------------------------
Orbit16:
        lda #$01
        sta seq
        ldx #0
O16:    jsr Step
        lda seq
        sta orbit,x
        inx
        cpx #16
        bne O16

        ; --- picture (the values live in RAM; the frame just has to be legal) -----------
NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        lda per1
        sta COLUBK          ; the measured period, shown as a colour
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; --- Step: one application of the 2002 sequence, on `seq` ---------------------------
; LDA $82 / LSR / ADC #$00 / LSR / ROR $82 — written exactly as posted.
Step:
        lda seq
        lsr
        adc #$00
        lsr
        ror seq
        rts

; --- Period: how many Steps until `seq` comes back to `seed`. 0 means "more than 255". ---
Period:
        lda seed
        sta seq
        lda #0
        sta count
PLoop:
        jsr Step
        inc count
        lda seq
        cmp seed
        beq PDone
        lda count
        cmp #$FF
        bne PLoop
        lda #0
        sta count           ; did not close within 255 steps
PDone:
        rts

        org $FFFC
        .word Start
        .word Start
