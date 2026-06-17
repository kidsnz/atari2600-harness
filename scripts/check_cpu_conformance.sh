#!/bin/bash
# check_cpu_conformance.sh — certify the embedded Gopher2600 CPU core against EXTERNAL authoritative suites.
# VV-1 (docs/capability-gap-audit.md). The harness's every numeric verdict assumes the embedded CPU is
# bit/cycle-correct; this gate proves it against two suites the harness does NOT author:
#   - Klaus Dormann 6502 functional + decimal test  (GPL; https://github.com/Klaus2m5/6502_65C02_functional_tests)
#       embedded as .bin inside the Gopher2600 clone (committed upstream) -> always runs, no provisioning.
#   - Tom Harte / SingleStepTests 65x02             (MIT; https://github.com/SingleStepTests/65x02)
#       per-opcode, ~10k cases each, CYCLE-BY-CYCLE bus (addr/data/read|write) + final state + cycle count.
#       The ~1GB JSON corpus is NOT in the Gopher2600 repo (its .gitignore excludes it); fetched on demand,
#       never vendored into this repo (no redistribution).
#
# Run from harness/ (or anywhere — it cd's to the harness root). Usage:
#   scripts/check_cpu_conformance.sh             # Klaus + Harte smoke subset (12 opcodes; fetch if missing)
#   scripts/check_cpu_conformance.sh full        # Klaus + Harte ALL 256 (needs the local 1GB corpus present)
#   scripts/check_cpu_conformance.sh --selftest  # planted-discrepancy: prove the gate REJECTS a bad expectation
set -euo pipefail
cd "$(dirname "$0")/.."
export CGO_ENABLED=0

KLAUS="github.com/jetsetilly/gopher2600/hardware/cpu/tests/klaus2m5/..."
HARTE="github.com/jetsetilly/gopher2600/hardware/cpu/tests/thomharte/..."
V1DIR="Gopher2600/hardware/cpu/tests/thomharte/6502/v1"
RAW="https://raw.githubusercontent.com/SingleStepTests/65x02/main/6502/v1"
# Representative smoke subset: imm / ADC-SBC flags / branch+page / JMP / JMP(ind) $xxFF bug / JSR /
# (ind),Y read+page / abs,X store fixed-5 / abs,X RMW fixed-7 / BRK / implied.
SUBSET="a9 69 e9 d0 4c 6c 20 b1 9d fe 00 ca"

fetch_subset() {
  mkdir -p "$V1DIR"
  for op in $SUBSET; do
    if [ ! -f "$V1DIR/$op.json" ]; then
      echo "  fetch $op.json"
      curl -fsSL "$RAW/$op.json" -o "$V1DIR/$op.json" \
        || { echo "FETCH FAILED for $op.json (network?) — not silently passing"; exit 1; }
    fi
  done
}

subset_env() { echo "$SUBSET" | tr ' ' ','; }

# Restore the corpus file the selftest corrupts. Globals + ${:-} guards so it is safe under `set -u`
# whether reached via the EXIT/INT/TERM trap (corrupt window) or called inline on the success path.
_ST_F=""; _ST_BAK=""
_st_restore() {
  if [ -n "${_ST_BAK:-}" ] && [ -f "${_ST_BAK:-}" ]; then
    cp "$_ST_BAK" "$_ST_F"; rm -f "$_ST_BAK"; _ST_BAK=""
  fi
}

selftest() {
  # The gate is worthless unless an injected wrong expectation is caught. Corrupt one expected final
  # register in one Harte case and require `go test` to FAIL; restore the file no matter what.
  fetch_subset
  _ST_F="$V1DIR/a9.json"
  _ST_BAK="$(mktemp)"; cp "$_ST_F" "$_ST_BAK"
  trap _st_restore EXIT INT TERM
  python3 - "$_ST_F" <<'PY'
import json, sys
p = sys.argv[1]
d = json.load(open(p))
d[0]['final']['a'] = (d[0]['final']['a'] + 1) & 0xff   # deliberately wrong expectation
json.dump(d, open(p, 'w'))
PY
  echo "selftest: Harte opcode a9 with a planted-wrong expected final A (must FAIL)..."
  local rc=0
  GOPHER2600_SINGLESTEP_TEST=a9 go test "$HARTE" >/dev/null 2>&1 || rc=$?
  _st_restore                 # restore inline on the success path
  trap - EXIT INT TERM
  if [ "$rc" -eq 0 ]; then
    echo "SELFTEST FAILED: the conformance gate PASSED a corrupted expectation = vacuous gate"
    exit 1
  fi
  echo "selftest OK: corrupted expectation correctly rejected (gate is live, not vacuous)"
}

case "${1:-}" in
  --selftest) selftest; exit 0 ;;
  full)       RANGE="00-ff" ;;
  "")         RANGE="" ;;
  *)          echo "usage: $0 [full|--selftest]"; exit 2 ;;
esac

echo "== Klaus 6502 functional + decimal (embedded, always-on) =="
go test "$KLAUS"

echo "== Tom Harte / SingleStepTests 65x02 (per-cycle bus) =="
if [ "$RANGE" = "00-ff" ]; then
  echo "  full 256-opcode run (local 1GB corpus)"
  GOPHER2600_SINGLESTEP_TEST=00-ff go test "$HARTE"
else
  fetch_subset
  echo "  smoke subset: $SUBSET"
  GOPHER2600_SINGLESTEP_TEST="$(subset_env)" go test "$HARTE"
fi
echo "cpu conformance OK"
