#!/bin/bash
# check_gopher_pin.sh — check that the local Gopher2600 clone matches the CI pin (F-2)
# Usage: scripts/check_gopher_pin.sh   (run from harness/)
set -eu
cd "$(dirname "$0")/.."
PIN=$(grep -o 'checkout [0-9a-f]\{40\}' .github/workflows/*.yml | head -1 | awk '{print $2}')
LOCAL=$(git -C Gopher2600 rev-parse HEAD)
if [ "$PIN" = "$LOCAL" ]; then
  echo "OK: local Gopher2600 == CI pin ($PIN)"
else
  echo "MISMATCH: local=$LOCAL ci_pin=$PIN"
  echo "  → ローカルを進めたら .github/workflows の checkout も同じ SHA に更新すること"
  exit 1
fi
