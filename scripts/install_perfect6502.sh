#!/bin/bash
# install_perfect6502.sh — provision the perfect6502 silicon CPU oracle for VV-7.
#
# perfect6502 (MIT; https://github.com/mist64/perfect6502) is a transistor-level
# netlist simulator of the 6502. We use it as a hardware-grade DIFFERENTIAL oracle
# for the embedded Gopher2600 CPU core (internal/cpudiff, cmd/cpucheck) — see
# docs/capability-gap-audit.md VV-7. It is a CPU-only model (no TIA/RIOT), so it
# is NOT a member of the full-system RAM vote (cmd/oraclevote); it cross-checks at
# the instruction layer, covering undocumented opcodes / decimal edges that the
# Tom Harte corpus (VV-1) excludes.
#
# Like the Gopher2600 upstream clone, the perfect6502 source is NOT vendored into
# this repo (gitignored); it is fetched on demand and pinned to a known commit.
# This script clones it, then compiles our first-party harness
# internal/cpudiff/p6502step/p6502step.c against it into bin/p6502step.
#
# Idempotent. Run from anywhere; it cd's to the harness root. Usage:
#   scripts/install_perfect6502.sh          # clone (if needed) + build
#   scripts/install_perfect6502.sh --check   # print whether bin/p6502step is present
set -euo pipefail
cd "$(dirname "$0")/.."

PIN="09fc542877a84318291aa42dab143a3e2c3db974"   # mist64/perfect6502 pinned commit
SRC="third_party/perfect6502"
HARNESS="internal/cpudiff/p6502step/p6502step.c"
OUT="bin/p6502step"

if [ "${1:-}" = "--check" ]; then
  if [ -x "$OUT" ]; then echo "present: $OUT"; exit 0; else echo "absent: $OUT (run scripts/install_perfect6502.sh)"; exit 1; fi
fi

if [ ! -d "$SRC/.git" ]; then
  echo "== cloning perfect6502 (pinned $PIN) =="
  rm -rf "$SRC"
  git clone --quiet https://github.com/mist64/perfect6502.git "$SRC"
fi
( cd "$SRC" && git fetch --quiet --depth 1 origin "$PIN" 2>/dev/null || true
  git checkout --quiet "$PIN" 2>/dev/null || { echo "WARNING: could not pin to $PIN; using cloned HEAD"; } )

echo "== building $OUT =="
mkdir -p bin
CC="${CC:-cc}"
"$CC" -O2 -I "$SRC" \
  "$SRC/perfect6502.c" "$SRC/netlist_sim.c" "$HARNESS" \
  -o "$OUT"
echo "built $OUT"
"$OUT" --help >/dev/null 2>&1 || true
echo "ok"
