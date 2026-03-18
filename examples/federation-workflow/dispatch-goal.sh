#!/usr/bin/env bash
set -euo pipefail

MASTER="http://localhost:9527"

echo "=== Fetching company IDs ==="
COMPANIES=$(curl -sf "${MASTER}/api/federation/companies")

DESIGN_ID=$(echo "$COMPANIES" | jq -r '.[] | select(.name=="design-team") | .id')
FRONTEND_ID=$(echo "$COMPANIES" | jq -r '.[] | select(.name=="frontend-team") | .id')
BACKEND_ID=$(echo "$COMPANIES" | jq -r '.[] | select(.name=="backend-team") | .id')

echo "  Design:   ${DESIGN_ID}"
echo "  Frontend: ${FRONTEND_ID}"
echo "  Backend:  ${BACKEND_ID}"

echo ""
echo "=== Dispatching federated goal ==="

GOAL=$(cat goal-login-feature.json \
  | sed "s/<DESIGN_COMPANY_ID>/${DESIGN_ID}/g" \
  | sed "s/<FRONTEND_COMPANY_ID>/${FRONTEND_ID}/g" \
  | sed "s/<BACKEND_COMPANY_ID>/${BACKEND_ID}/g")

curl -sf -X POST "${MASTER}/api/goals/federated" \
  -H "Content-Type: application/json" \
  -d "$GOAL" | jq .

echo ""
echo "Goal dispatched! Monitor progress:"
echo "  Jaeger UI:     http://localhost:16686"
echo "  Master API:    curl ${MASTER}/api/goals | jq"
echo "  Design node:   curl http://localhost:9528/api/tasks | jq"
echo "  Frontend node: curl http://localhost:9529/api/tasks | jq"
echo "  Backend node:  curl http://localhost:9530/api/tasks | jq"
