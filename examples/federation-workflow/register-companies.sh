#!/usr/bin/env bash
set -euo pipefail

MASTER="http://localhost:9527"

echo "=== Registering federation companies ==="

curl -sf -X POST "${MASTER}/api/federation/companies" \
  -H "Content-Type: application/json" \
  -d '{"name":"design-team","endpoint":"http://localhost:9528","type":"software","agents":["designer"]}' | jq .

curl -sf -X POST "${MASTER}/api/federation/companies" \
  -H "Content-Type: application/json" \
  -d '{"name":"frontend-team","endpoint":"http://localhost:9529","type":"software","agents":["coder"]}' | jq .

curl -sf -X POST "${MASTER}/api/federation/companies" \
  -H "Content-Type: application/json" \
  -d '{"name":"backend-team","endpoint":"http://localhost:9530","type":"software","agents":["coder"]}' | jq .

echo ""
echo "Federation ready. Companies:"
curl -sf "${MASTER}/api/federation/companies" | jq '.[] | {id, name, endpoint, status}'
