; litmus_dag_region.asm — two BACKWARD branches and no loop.
;
; The prover decided what a loop was by ADDRESS ORDER: a branch counted as a back
; edge when its target sat at a lower address and was not a WSYNC sink. That is not
; reachability. A branch backwards into code that cannot reach the branch again is
; not an edge back at all, and a region whose graph is a perfectly good DAG was
; refused as "multiple back-edges (nested/complex loops)" for having no edges back.
;
; Measured on real cartridges: 4 regions across the corpus are refused that way and
; are acyclic — Seaquest $F1EC (59 proven, 53 measured), Chopper Command $FA78
; (74, 72) and $FAEC (103, 97 over two scanlines), Barnstorming $F3D4 (95). **Every
; one of them is a commercial image that is not part of this repository**, so the
; path that bounds them had no witness here at all. This ROM is that witness.
;
; THE SHAPE. `Pick` reads a flag and dispatches to one of two blocks placed BELOW
; it, so both branches point at lower addresses and both are counted as latches:
;
;       BlkA:  ...            <- lower address
;       BlkB:  ...            <- lower address
;       Pick:  beq BlkA       <- backward branch 1
;              bne BlkB       <- backward branch 2
;
; Neither block can reach `Pick` again: each falls into `Tail`, and `Tail` runs to
; the closing WSYNC. So the subgraph is a DAG with two backward branches in it, and
; its longest path is a bound over every reachable path — which is what the analysis
; now reports instead of a refusal.
;
; WHAT THIS DOES NOT SAY. The DAG answer is allowed to override exactly ONE refusal.
; foldLoops refuses for eight reasons and seven of them are about the loop BODY, not
; the graph — a WSYNC inside it, a branch inside it, a bank switch, an unknown trip
; count. Those stay. An earlier version ran the walk first and accepted whenever no
; cycle was met, which silently bypassed all seven: VideoOlympics $F5CA refuses for
; "WSYNC inside loop body", and that loop is INVISIBLE to the walk because the WSYNC
; is a sink and the walk stops there without ever traversing the edge back. It came
; back with 148 cycles for an interval the machine takes 163. The corpus gate caught
; it. A bound below the machine's is worse than no bound.
;
; The two branches are made to disagree at runtime by a flag in RAM that alternates
; each frame, so both arms are actually executed rather than merely decoded.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

flag    = $80

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

        inc flag        ; alternate, so both arms run across frames

; --- Visible: 192 lines ---
; One region is the dispatch below; the rest are a plain counted loop, so the ROM
; still holds a frame together.
        ldx #191
Fill:   sta WSYNC
        dex
        bne Fill
        jmp Pick        ; NOT fall-through: the blocks below are reachable only
                        ; from Pick's branches, or they are not backward edges at
                        ; all. The first version omitted this jmp, fell straight
                        ; into BlkA, and the region under test was never entered —
                        ; the fixture proved nothing and the report said so by
                        ; simply not listing it.

; The two blocks sit BELOW the branches that reach them.
BlkA:   lda #$0E
        sta COLUBK
        jmp Tail

BlkB:   lda #$44
        sta COLUBK
        jmp Tail

Pick:   sta WSYNC       ; <- the region under test opens here
        lda flag
        and #1
        beq BlkA        ; backward branch 1
        bne BlkB        ; backward branch 2

Tail:   lda #0
        sta COLUBK

; --- Overscan: 30 lines ---
        sta WSYNC       ; closes the dispatch region
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
