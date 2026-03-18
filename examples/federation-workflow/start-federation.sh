#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="${HOME}/.opc-federation-demo"
OPCTL="${OPCTL:-opctl}"

echo "=== OPC Federation Workflow Demo ==="
echo "Starting 4 OPC instances..."

rm -rf "${BASE_DIR}"
mkdir -p "${BASE_DIR}"/{master,design,frontend,backend}/state

$OPCTL serve --port 9527 --host 0.0.0.0 --state-dir "${BASE_DIR}/master/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-master &
echo "  Master  → :9527 (PID $!)"

$OPCTL serve --port 9528 --host 0.0.0.0 --state-dir "${BASE_DIR}/design/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-design &
echo "  Design  → :9528 (PID $!)"

$OPCTL serve --port 9529 --host 0.0.0.0 --state-dir "${BASE_DIR}/frontend/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-frontend &
echo "  Frontend → :9529 (PID $!)"

$OPCTL serve --port 9530 --host 0.0.0.0 --state-dir "${BASE_DIR}/backend/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-backend &
echo "  Backend  → :9530 (PID $!)"

echo ""
echo "Waiting for health checks..."
sleep 3

for port in 9527 9528 9529 9530; do
  if curl -sf "http://localhost:${port}/api/health" > /dev/null 2>&1; then
    echo "  :${port} ✓"
  else
    echo "  :${port} ✗ (may need more time)"
  fi
done

echo ""
echo "Next steps:"
echo "  1. Run: bash register-companies.sh"
echo "  2. Run: bash dispatch-goal.sh"
echo "  3. Open Jaeger UI: http://localhost:16686"
echo ""
echo "To stop: bash stop-federation.sh"
