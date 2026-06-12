#!/bin/bash
# check_gopher_pin.sh — ローカルの Gopher2600 クローンが CI のピンと一致するか確認（F-2）
# 使い方: scripts/check_gopher_pin.sh   （harness/ から実行）
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
