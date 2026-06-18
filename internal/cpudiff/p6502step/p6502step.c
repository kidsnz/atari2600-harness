/*
 * p6502step — single-instruction execution harness over perfect6502 (the
 * transistor-level 6502 netlist simulator). Reads test vectors on stdin, runs
 * exactly one instruction each on the silicon-accurate netlist, and writes the
 * resulting CPU/memory state on stdout. Used by internal/cpudiff (VV-7) as a
 * hardware-grade differential oracle against the embedded Gopher2600 CPU core.
 *
 * This file is FIRST-PARTY (atari2600-harness). It links against the vendored,
 * pinned perfect6502 clone (MIT; mist64/perfect6502 @ 09fc542) which is fetched
 * and built by scripts/install_perfect6502.sh — see that script for provenance.
 *
 * Technique (register injection) follows perfect6502's own measure.c: the reset
 * vector points at a short prologue (LDX/TXS, LDA/PHA, LDA, LDX, LDY, PLP, JMP)
 * whose immediate operands are patched to inject S/P/A/X/Y, then it JMPs to the
 * instruction under test at INSTRUCTION_ADDR. perfect6502 exposes no register
 * writers, so this is the supported way to set arbitrary CPU state.
 *
 * Instruction-boundary + counting were pinned EMPIRICALLY (not assumed): the
 * boundary runs exactly one opcode-fetch into the following instruction, so the
 * raw span overshoots by one full cycle and PC is one fetch ahead — both are
 * corrected by -1. Writes are captured as the net memory diff vs a snapshot
 * taken after the prologue (so prologue stack traffic is excluded).
 *
 * Vector line (stdin), all tokens hex, whitespace-separated:
 *   A X Y S P N  addr0 val0  addr1 val1  ... addr(N-1) val(N-1)
 * where the N (addr,val) pokes include the instruction bytes at 0xF800.. and
 * any data the instruction reads. Blank lines and lines starting with '#' skip.
 *
 * Result line (stdout):
 *   STATUS A X Y S P PC CYCLES M  waddr0 wval0 ... waddr(M-1) wval(M-1)
 * STATUS: ok | badfetch | overrun. A/X/Y/S/P/PC/CYCLES/wval are hex.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include "types.h"
#include "perfect6502.h"
#include "netlist_sim.h"   /* isNodeHigh — read the SYNC line directly */

#define SETUP_ADDR       0xF400
#define INSTRUCTION_ADDR 0xF800
#define BRK_VECTOR       0xFC00
#define SYNC_NODE        539  /* 6502 SYNC: high during every opcode fetch (visual6502 node) */
#define MAX_INSTR_CYCLES 40   /* generous guard; longest real 6502 insn is 7 */
#define MAX_WRITES       32

extern uint8_t memory[65536];
extern unsigned long cycle;

static void setup_memory(uint8_t a, uint8_t x, uint8_t y, uint8_t s, uint8_t p,
                         uint8_t sentinel) {
    memset(memory, sentinel, 65536);
    memory[0xFFFC] = SETUP_ADDR & 0xFF;
    memory[0xFFFD] = SETUP_ADDR >> 8;
    uint16_t addr = SETUP_ADDR;
    memory[addr++] = 0xA2; memory[addr++] = s; /* LDX #S */
    memory[addr++] = 0x9A;                     /* TXS    */
    memory[addr++] = 0xA9; memory[addr++] = p; /* LDA #P */
    memory[addr++] = 0x48;                     /* PHA    */
    memory[addr++] = 0xA9; memory[addr++] = a; /* LDA #A */
    memory[addr++] = 0xA2; memory[addr++] = x; /* LDX #X */
    memory[addr++] = 0xA0; memory[addr++] = y; /* LDY #Y */
    memory[addr++] = 0x28;                     /* PLP    */
    memory[addr++] = 0x4C;                     /* JMP    */
    memory[addr++] = INSTRUCTION_ADDR & 0xFF;
    memory[addr++] = INSTRUCTION_ADDR >> 8;
    memory[0xFFFE] = BRK_VECTOR & 0xFF;
    memory[0xFFFF] = BRK_VECTOR >> 8;
    memory[BRK_VECTOR] = sentinel; /* BRK/IRQ landing pad: non-BRK so the boundary resolves */
}

static uint8_t pre[65536];

struct result {
    const char *status;
    uint8_t a, x, y, s, p;
    uint16_t pc;
    int cycles;
    int nwr;
    uint16_t wa[MAX_WRITES];
    uint8_t wv[MAX_WRITES];
};

/* Run exactly one instruction at INSTRUCTION_ADDR with the given injected
 * registers; the caller must already have placed instruction+data bytes via the
 * poke list AFTER setup_memory. */
static struct result run_one(uint8_t a, uint8_t x, uint8_t y, uint8_t s, uint8_t p,
                             const uint16_t *poke_a, const uint8_t *poke_v, int npoke) {
    struct result r;
    memset(&r, 0, sizeof r);

    uint8_t test_op_probe = 0; /* opcode = byte poked at INSTRUCTION_ADDR */
    /* choose a sentinel distinct from the test opcode so the next opcode fetch
     * (wherever it lands) reliably changes IR. */
    for (int i = 0; i < npoke; i++)
        if (poke_a[i] == INSTRUCTION_ADDR) test_op_probe = poke_v[i];
    uint8_t sentinel = (test_op_probe == 0xEA) ? 0x00 : 0xEA;

    setup_memory(a, x, y, s, p, sentinel);
    for (int i = 0; i < npoke; i++) memory[poke_a[i]] = poke_v[i];
    uint8_t test_op = memory[INSTRUCTION_ADDR];

    (void)test_op;
    void *st = initAndResetChip();

    /* advance to the opcode fetch of the test instruction: SYNC high while the
     * CPU reads its opcode from INSTRUCTION_ADDR. SYNC (not the IR value) marks
     * fetches, so the boundary is robust even when control flow returns to the
     * instruction (e.g. a branch with offset -2). */
    int guard = 0;
    while (!(isNodeHigh(st, SYNC_NODE) && readAddressBus(st) == INSTRUCTION_ADDR && readRW(st))) {
        step(st);
        if (++guard > 600) { r.status = "badfetch"; destroyChip(st); return r; }
    }
    long cyc_fetch = cycle;
    memcpy(pre, memory, 65536); /* post-prologue snapshot for the write diff */

    /* leave this fetch (SYNC returns low during the instruction body) */
    guard = 0;
    while (isNodeHigh(st, SYNC_NODE)) { step(st); if (++guard > 8) break; }

    /* run until SYNC rises again = the NEXT opcode fetch = our boundary */
    guard = 0;
    while (!isNodeHigh(st, SYNC_NODE)) {
        step(st);
        if (++guard > MAX_INSTR_CYCLES * 2) { r.status = "overrun"; destroyChip(st); return r; }
    }
    long cyc_next = cycle;

    r.status = "ok";
    r.cycles = (int)((cyc_next - cyc_fetch) / 2); /* exact: span between two fetches */
    r.pc = readPC(st);                            /* PC = address of the next opcode fetch */

    /* The test instruction's result registers — in particular the status flags —
     * settle as the next fetch completes. Step through that fetch (SYNC high ->
     * low) so flags are fully retired; the next instruction is still in decode
     * and has not altered any register yet. */
    guard = 0;
    while (isNodeHigh(st, SYNC_NODE)) { step(st); if (++guard > 8) break; }
    r.a = readA(st); r.x = readX(st); r.y = readY(st);
    r.s = readSP(st); r.p = readP(st);

    if (memcmp(pre, memory, 65536) != 0) {
        for (int addr = 0; addr < 65536 && r.nwr < MAX_WRITES; addr++) {
            if (memory[addr] != pre[addr]) {
                r.wa[r.nwr] = (uint16_t)addr;
                r.wv[r.nwr] = memory[addr];
                r.nwr++;
            }
        }
    }
    destroyChip(st);
    return r;
}

int main(void) {
    char line[8192];
    while (fgets(line, sizeof line, stdin)) {
        if (line[0] == '#' || line[0] == '\n') continue;
        char *p = line;
        long tok[6];
        int ok = 1;
        for (int i = 0; i < 6; i++) {
            char *end;
            tok[i] = strtol(p, &end, 16);
            if (end == p) { ok = 0; break; }
            p = end;
        }
        if (!ok) continue;
        uint8_t a = tok[0], x = tok[1], y = tok[2], s = tok[3], pp = tok[4];
        int npoke = (int)tok[5];
        if (npoke < 0 || npoke > 3000) continue;
        static uint16_t pa[3000];
        static uint8_t pv[3000];
        ok = 1;
        for (int i = 0; i < npoke; i++) {
            char *end;
            long av = strtol(p, &end, 16); if (end == p) { ok = 0; break; } p = end;
            long vv = strtol(p, &end, 16); if (end == p) { ok = 0; break; } p = end;
            pa[i] = (uint16_t)av;
            pv[i] = (uint8_t)vv;
        }
        if (!ok) continue;

        struct result r = run_one(a, x, y, s, pp, pa, pv, npoke);
        printf("%s %02X %02X %02X %02X %02X %04X %d %d",
               r.status, r.a, r.x, r.y, r.s, r.p, r.pc, r.cycles, r.nwr);
        for (int i = 0; i < r.nwr; i++) printf(" %04X %02X", r.wa[i], r.wv[i]);
        printf("\n");
        fflush(stdout);
    }
    return 0;
}
