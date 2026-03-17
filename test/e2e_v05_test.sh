#!/usr/bin/env bash
# E2E tests for OPC Platform v0.5 API
# Requires: daemon running on 127.0.0.1:9527
# Usage: bash test/e2e_v05_test.sh

set -euo pipefail

BASE_URL="http://127.0.0.1:9527/api"
PASS=0
FAIL=0
TOTAL=0

# --- Helpers ---

green()  { printf "\033[32m%s\033[0m\n" "$*"; }
red()    { printf "\033[31m%s\033[0m\n" "$*"; }
yellow() { printf "\033[33m%s\033[0m\n" "$*"; }

assert_status() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"
    TOTAL=$((TOTAL + 1))
    if [ "$actual" = "$expected" ]; then
        green "  PASS: $test_name (HTTP $actual)"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $test_name (expected $expected, got $actual)"
        FAIL=$((FAIL + 1))
    fi
}

assert_json_field() {
    local test_name="$1"
    local body="$2"
    local field="$3"
    local expected="$4"
    TOTAL=$((TOTAL + 1))
    local actual
    actual=$(echo "$body" | jq -r "$field" 2>/dev/null || echo "PARSE_ERROR")
    if [ "$actual" = "$expected" ]; then
        green "  PASS: $test_name ($field = $actual)"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $test_name ($field expected '$expected', got '$actual')"
        FAIL=$((FAIL + 1))
    fi
}

assert_json_not_empty() {
    local test_name="$1"
    local body="$2"
    local field="$3"
    TOTAL=$((TOTAL + 1))
    local val
    val=$(echo "$body" | jq -r "$field" 2>/dev/null || echo "")
    if [ -n "$val" ] && [ "$val" != "null" ] && [ "$val" != "" ]; then
        green "  PASS: $test_name ($field is not empty)"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $test_name ($field is empty or null)"
        FAIL=$((FAIL + 1))
    fi
}

assert_json_array_min() {
    local test_name="$1"
    local body="$2"
    local field="$3"
    local min_len="$4"
    TOTAL=$((TOTAL + 1))
    local len
    len=$(echo "$body" | jq "$field | length" 2>/dev/null || echo "0")
    if [ "$len" -ge "$min_len" ]; then
        green "  PASS: $test_name ($field length=$len >= $min_len)"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $test_name ($field length=$len < $min_len)"
        FAIL=$((FAIL + 1))
    fi
}

# --- Pre-flight check ---

echo "============================================"
echo " OPC Platform v0.5 E2E Tests"
echo " Target: $BASE_URL"
echo "============================================"
echo ""

# ============================
# 1. Health Check
# ============================
echo "--- 1. Health Check ---"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
assert_status "GET /api/health" "200" "$STATUS"

BODY=$(curl -s "$BASE_URL/health")
assert_json_field "health status field" "$BODY" ".status" "ok"
echo ""

# ============================
# 2. Create Agent via Apply
# ============================
echo "--- 2. Create Agent via Apply ---"
AGENT_YAML='apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: e2e-test-agent
spec:
  type: claude-code
  runtime:
    model:
      name: sonnet-4
    timeout:
      task: 2m
  context:
    workdir: /tmp/opc/e2e-test'

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/apply" \
    -H "Content-Type: application/x-yaml" \
    -d "$AGENT_YAML")
assert_status "POST /api/apply (agent)" "200" "$STATUS"

# Verify agent exists
BODY=$(curl -s "$BASE_URL/agents/e2e-test-agent")
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/agents/e2e-test-agent")
assert_status "GET /api/agents/e2e-test-agent" "200" "$STATUS"
assert_json_field "agent name" "$BODY" ".name" "e2e-test-agent"
echo ""

# ============================
# 3. Goal Apply with Decomposition
# ============================
echo "--- 3. Goal CRUD ---"

# Create a goal
GOAL_BODY='{"name":"e2e-test-goal","description":"E2E test goal for v0.5","owner":"e2e-tester"}'
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/goals" \
    -H "Content-Type: application/json" \
    -d "$GOAL_BODY")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "POST /api/goals (create)" "200" "$STATUS"
GOAL_ID=$(echo "$BODY" | jq -r '.id // .ID // empty' 2>/dev/null || echo "")
if [ -n "$GOAL_ID" ]; then
    green "  INFO: created goal ID=$GOAL_ID"
else
    yellow "  WARN: could not extract goal ID from response"
    GOAL_ID="unknown"
fi

# List goals
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/goals")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "GET /api/goals (list)" "200" "$STATUS"

# Get goal by ID
if [ "$GOAL_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/goals/$GOAL_ID")
    assert_status "GET /api/goals/$GOAL_ID" "200" "$STATUS"
fi

# Delete goal
if [ "$GOAL_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/goals/$GOAL_ID")
    assert_status "DELETE /api/goals/$GOAL_ID" "200" "$STATUS"
fi
echo ""

# ============================
# 4. Workflow Operations
# ============================
echo "--- 4. Workflow Operations ---"

# Apply a workflow
WORKFLOW_YAML='apiVersion: opc/v1
kind: Workflow
metadata:
  name: e2e-test-workflow
spec:
  steps:
    - name: step-1
      agent: e2e-test-agent
      input:
        message: "Hello from E2E test"'

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/apply" \
    -H "Content-Type: application/x-yaml" \
    -d "$WORKFLOW_YAML")
assert_status "POST /api/apply (workflow)" "200" "$STATUS"

# List workflows
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/workflows")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "GET /api/workflows (list)" "200" "$STATUS"

# Toggle workflow
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/workflows/e2e-test-workflow/toggle")
assert_status "PUT /api/workflows/e2e-test-workflow/toggle" "200" "$STATUS"

# List workflow runs
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/workflows/e2e-test-workflow/runs")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "GET /api/workflows/e2e-test-workflow/runs" "200" "$STATUS"
echo ""

# ============================
# 5. Federation Companies CRUD
# ============================
echo "--- 5. Federation Companies ---"

# Register a company
COMPANY_BODY='{"name":"e2e-test-company","endpoint":"http://localhost:19999","type":"software"}'
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/federation/companies" \
    -H "Content-Type: application/json" \
    -d "$COMPANY_BODY")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "POST /api/federation/companies (register)" "200" "$STATUS"
COMPANY_ID=$(echo "$BODY" | jq -r '.id // .ID // empty' 2>/dev/null || echo "")
if [ -n "$COMPANY_ID" ]; then
    green "  INFO: registered company ID=$COMPANY_ID"
else
    yellow "  WARN: could not extract company ID"
    COMPANY_ID="unknown"
fi

# List companies
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/federation/companies")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "GET /api/federation/companies (list)" "200" "$STATUS"

# Get company
if [ "$COMPANY_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/federation/companies/$COMPANY_ID")
    assert_status "GET /api/federation/companies/$COMPANY_ID" "200" "$STATUS"
fi

# Delete company
if [ "$COMPANY_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/federation/companies/$COMPANY_ID")
    assert_status "DELETE /api/federation/companies/$COMPANY_ID" "200" "$STATUS"
fi
echo ""

# ============================
# 6. Logs Endpoint
# ============================
echo "--- 6. Logs ---"
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/logs")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "GET /api/logs" "200" "$STATUS"
echo ""

# ============================
# 7. Projects CRUD
# ============================
echo "--- 7. Projects CRUD ---"

# Create project
PROJECT_BODY='{"name":"e2e-test-project","description":"E2E test project","goalId":"test-goal-id"}'
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/projects" \
    -H "Content-Type: application/json" \
    -d "$PROJECT_BODY")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "POST /api/projects (create)" "200" "$STATUS"
PROJECT_ID=$(echo "$BODY" | jq -r '.id // .ID // empty' 2>/dev/null || echo "")
if [ -n "$PROJECT_ID" ]; then
    green "  INFO: created project ID=$PROJECT_ID"
else
    yellow "  WARN: could not extract project ID"
    PROJECT_ID="unknown"
fi

# List projects
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/projects")
assert_status "GET /api/projects (list)" "200" "$STATUS"

# Get project
if [ "$PROJECT_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/projects/$PROJECT_ID")
    assert_status "GET /api/projects/$PROJECT_ID" "200" "$STATUS"
fi

# Delete project
if [ "$PROJECT_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/projects/$PROJECT_ID")
    assert_status "DELETE /api/projects/$PROJECT_ID" "200" "$STATUS"
fi
echo ""

# ============================
# 8. Issues CRUD
# ============================
echo "--- 8. Issues CRUD ---"

# Create issue
ISSUE_BODY='{"name":"e2e-test-issue","description":"E2E test issue","projectId":"test-project-id"}'
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/issues" \
    -H "Content-Type: application/json" \
    -d "$ISSUE_BODY")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -1)
assert_status "POST /api/issues (create)" "200" "$STATUS"
ISSUE_ID=$(echo "$BODY" | jq -r '.id // .ID // empty' 2>/dev/null || echo "")
if [ -n "$ISSUE_ID" ]; then
    green "  INFO: created issue ID=$ISSUE_ID"
else
    yellow "  WARN: could not extract issue ID"
    ISSUE_ID="unknown"
fi

# List issues
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/issues")
assert_status "GET /api/issues (list)" "200" "$STATUS"

# Get issue
if [ "$ISSUE_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/issues/$ISSUE_ID")
    assert_status "GET /api/issues/$ISSUE_ID" "200" "$STATUS"
fi

# Delete issue
if [ "$ISSUE_ID" != "unknown" ]; then
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/issues/$ISSUE_ID")
    assert_status "DELETE /api/issues/$ISSUE_ID" "200" "$STATUS"
fi
echo ""

# ============================
# 9. Cleanup
# ============================
echo "--- 9. Cleanup ---"

# Delete test agent
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/agents/e2e-test-agent")
assert_status "DELETE /api/agents/e2e-test-agent (cleanup)" "200" "$STATUS"

# Delete test workflow
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/workflows/e2e-test-workflow")
assert_status "DELETE /api/workflows/e2e-test-workflow (cleanup)" "200" "$STATUS"
echo ""

# ============================
# Summary
# ============================
echo "============================================"
echo " E2E Test Results"
echo "============================================"
echo " Total:  $TOTAL"
green " Passed: $PASS"
if [ "$FAIL" -gt 0 ]; then
    red " Failed: $FAIL"
    echo "============================================"
    exit 1
else
    echo " Failed: 0"
    echo "============================================"
    green " All tests passed!"
    exit 0
fi
