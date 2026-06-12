# RAM maps — auto-extracted audit (V2-18, overnight M-J)

Zero-page equates per ROM (auto-generated; regenerate with the snippet in this file's footer).
Overlaps within one ROM are design (scene overlays); the audit confirms every used address has a
named equate and no ROM strays outside $80-$FF user RAM.

## roms/exerciser/exerciser.asm
- $80 `scene`
- $81 `prevFire`
- $82 `frameCt`
- $83 `lastScene`
- $84 `m0idx`
- $85 `m0dur`
- $86 `m1idx`
- $87 `m1dur`
- $90 `score0`
- $91 `score1`
- $92 `score2`
- $9E `sent0`
- $9F `sent1`
- $A0 `row`
- $A1 `t3`
- $A2 `t4`
- $A3 `t5`
- $A4 `t0`
- $A4 `zx0`
- $A4 `px`
- $A5 `t1`
- $A5 `py`
- $A6 `t2`
- $A6 `mx`
- $A7 `mAct`
- $A8 `p0`
- $A9 `hitCol`
- $AA `p1`
- $AA `zx1`
- $AC `p2`
- $AE `p3`
- $B0 `p4`
- $B2 `p5`
- $B4 `lineCt`
- $B6 `pdlCnt`
- $B7 `pdlPos`
- $B8 `pdlDone`
- $B9 `sfxTmr`
- $BA `worldSeed`
- $BB `rnd`
- $BC `t7`
- $C0 `mPF0`
- $CA `mPF1`
- $D4 `mPF2`
- $DE `mrnd`
- $DF `t6`

## roms/techniques/dyn_multisprite.asm
- $80 `ys`
- $85 `dirs`
- $8A `sortIdx`
- $8F `frameCt`
- $90 `q0y`
- $94 `q0o`
- $97 `q0n`
- $98 `q1y`
- $9B `q1o`
- $9D `q1n`
- $9E `sent`
- $A0 `p0st`
- $A1 `p0row`
- $A2 `p0qi`
- $A3 `p1st`
- $A4 `p1row`
- $A5 `p1qi`
- $A6 `pair`
- $A7 `tmp`
- $A8 `tIA`
- $A9 `tIB`
- $AA `minS0`
- $AB `minS1`
- $AC `firstS`
- $AD `aIdx`

## roms/techniques/flicker_multiplex.asm
- $80 `oy0`
- $81 `oy1`
- $82 `ox2`
- $83 `ox3`
- $84 `d0`
- $85 `d1`
- $86 `d2`
- $87 `d3`
- $88 `frameCt`
- $89 `sy0`
- $8A `sy1`
- $8B `px0`
- $8C `px1`
- $9E `sent`

## roms/techniques/pf_modes.asm
- $9E `sent`

## roms/techniques/sprite_anim.asm
- $80 `phase`
- $81 `animTmr`
- $82 `xpos`
- $83 `dir`
- $84 `frameBase`
- $9E `sent`

## roms/techniques/two_line_kernel.asm
- $80 `y0`
- $81 `y1`
- $82 `d0`
- $83 `d1`
- $9E `sent`

## roms/techniques/two_line_vdel.asm
- $80 `y0`
- $81 `y1`
- $82 `d0`
- $83 `d1`
- $84 `y0p`
- $9E `sent`

## roms/techniques/venetian.asm
- $9E `sent`

## roms/techniques/vertical_pos.asm
- $80 `sprY`
- $81 `vdir`
- $9E `sent`

## roms/techniques/vertical_pos_dcp.asm
- $80 `sprY`
- $81 `vdir`
- $82 `sprDraw`
- $9E `sent`

## roms/techniques/zone_multiplex.asm
- $80 `zx0`
- $86 `zx1`

---
Regenerate: see scripts in git history (M-J). Source of truth = the equates in each .asm.
