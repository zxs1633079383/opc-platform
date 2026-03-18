#!/usr/bin/env bash
set -euo pipefail

echo "=== Stopping OPC Federation Demo ==="

for port in 9527 9528 9529 9530; do
  pid=$(lsof -ti :"${port}" 2>/dev/null || true)
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null && echo "  Stopped :${port} (PID ${pid})" || true
  fi
done

echo "All instances stopped."
