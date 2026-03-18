#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# OPC Federation Workflow — 一键启动演示
#
# 用法:
#   bash run-demo.sh          # 启动全部（Jaeger + 4 OPC 节点 + 注册 + 下发 Goal）
#   bash run-demo.sh stop     # 停止全部
###############################################################################

BASE_DIR="${HOME}/.opc-federation-demo"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OPCTL="${PROJECT_ROOT}/opctl"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[OPC]${NC} $*"; }
warn() { echo -e "${YELLOW}[OPC]${NC} $*"; }
err()  { echo -e "${RED}[OPC]${NC} $*"; }

# ─── stop ────────────────────────────────────────────────────────────────────
stop_all() {
    log "停止所有 OPC 实例和 Dashboard..."
    for port in 9527 9528 9529 9530 3000 3001 3002 3003; do
        pid=$(lsof -ti :"${port}" 2>/dev/null || true)
        if [ -n "$pid" ]; then
            kill "$pid" 2>/dev/null && log "  :${port} 已停止 (PID ${pid})" || true
        fi
    done
    pkill -f "opctl serve" 2>/dev/null || true
    # Kill dashboard on port 3000
    pid_3000=$(lsof -ti :3000 2>/dev/null || true)
    [ -n "$pid_3000" ] && kill "$pid_3000" 2>/dev/null || true
    sleep 1
    log "全部停止"
}

if [ "${1:-}" = "stop" ]; then
    stop_all
    exit 0
fi

# ─── 编译检查 ─────────────────────────────────────────────────────────────────
log "=== OPC 联邦编排演示 ==="
echo ""

if [ ! -f "$OPCTL" ]; then
    log "编译 opctl..."
    (cd "$PROJECT_ROOT" && go build -o opctl ./cmd/opctl)
fi

# 验证二进制有新 flags
if ! "$OPCTL" serve --help 2>&1 | grep -q "state-dir"; then
    warn "opctl 二进制过旧，重新编译..."
    (cd "$PROJECT_ROOT" && go build -o opctl ./cmd/opctl)
fi

# ─── 清理 ─────────────────────────────────────────────────────────────────────
stop_all 2>/dev/null || true

log "清理旧状态..."
rm -rf "${BASE_DIR}"
mkdir -p "${BASE_DIR}"/{master,design,frontend,backend}/state

# ─── Jaeger ───────────────────────────────────────────────────────────────────
if ! docker ps 2>/dev/null | grep -q jaeger; then
    log "启动 Jaeger..."
    docker rm -f jaeger 2>/dev/null || true
    docker run -d --name jaeger \
        -p 16686:16686 \
        -p 4318:4318 \
        -e COLLECTOR_OTLP_ENABLED=true \
        jaegertracing/all-in-one:1.54 > /dev/null
    sleep 2
    log "  Jaeger UI: http://localhost:16686"
else
    log "  Jaeger 已在运行"
fi

# ─── 启动 4 个 OPC 实例 ───────────────────────────────────────────────────────
log "启动 OPC 实例..."

"$OPCTL" serve --port 9527 --host 0.0.0.0 \
    --state-dir "${BASE_DIR}/master/state" \
    --otel --otel-endpoint localhost:4318 --otel-service opc-master \
    > "${BASE_DIR}/master/stdout.log" 2>&1 &
log "  Master   → :9527 (PID $!)"

"$OPCTL" serve --port 9528 --host 0.0.0.0 \
    --state-dir "${BASE_DIR}/design/state" \
    --otel --otel-endpoint localhost:4318 --otel-service opc-design \
    > "${BASE_DIR}/design/stdout.log" 2>&1 &
log "  Design   → :9528 (PID $!)"

"$OPCTL" serve --port 9529 --host 0.0.0.0 \
    --state-dir "${BASE_DIR}/frontend/state" \
    --otel --otel-endpoint localhost:4318 --otel-service opc-frontend \
    > "${BASE_DIR}/frontend/stdout.log" 2>&1 &
log "  Frontend → :9529 (PID $!)"

"$OPCTL" serve --port 9530 --host 0.0.0.0 \
    --state-dir "${BASE_DIR}/backend/state" \
    --otel --otel-endpoint localhost:4318 --otel-service opc-backend \
    > "${BASE_DIR}/backend/stdout.log" 2>&1 &
log "  Backend  → :9530 (PID $!)"

# ─── 等待健康检查 ─────────────────────────────────────────────────────────────
log "等待实例就绪..."
for i in $(seq 1 10); do
    all_up=true
    for port in 9527 9528 9529 9530; do
        if ! curl -sf "http://localhost:${port}/api/health" > /dev/null 2>&1; then
            all_up=false
        fi
    done
    if $all_up; then break; fi
    sleep 1
done

echo ""
for port in 9527 9528 9529 9530; do
    if curl -sf "http://localhost:${port}/api/health" > /dev/null 2>&1; then
        log "  :${port} ${GREEN}✓${NC}"
    else
        err "  :${port} ✗"
    fi
done

# ─── 启动 Master Dashboard ────────────────────────────────────────────────────
DASHBOARD_DIR="${PROJECT_ROOT}/dashboard"
if [ -d "$DASHBOARD_DIR" ] && [ -d "${DASHBOARD_DIR}/.next" ]; then
    log "启动 Master Dashboard..."
    (cd "$DASHBOARD_DIR" && npx next start -p 3000 \
        > "${BASE_DIR}/master/dashboard.log" 2>&1) &
    log "  Dashboard → :3000（Federation 页面可查看所有节点）"
    sleep 2
else
    warn "Dashboard 未构建，跳过（cd dashboard && npm run build）"
fi

# ─── 注册联邦 ─────────────────────────────────────────────────────────────────
echo ""
log "注册联邦公司..."
MASTER="http://localhost:9527"

DESIGN=$(curl -s -X POST "${MASTER}/api/federation/companies" \
    -H "Content-Type: application/json" \
    -d '{"name":"design-team","endpoint":"http://localhost:9528","type":"software","agents":["designer"]}')
DESIGN_ID=$(echo "$DESIGN" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "FAIL")
DESIGN_STATUS=$(echo "$DESIGN" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "?")
log "  design-team   → ID: ${DESIGN_ID}  Status: ${DESIGN_STATUS}"

FRONTEND=$(curl -s -X POST "${MASTER}/api/federation/companies" \
    -H "Content-Type: application/json" \
    -d '{"name":"frontend-team","endpoint":"http://localhost:9529","type":"software","agents":["coder"]}')
FRONTEND_ID=$(echo "$FRONTEND" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "FAIL")
FRONTEND_STATUS=$(echo "$FRONTEND" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "?")
log "  frontend-team → ID: ${FRONTEND_ID}  Status: ${FRONTEND_STATUS}"

BACKEND=$(curl -s -X POST "${MASTER}/api/federation/companies" \
    -H "Content-Type: application/json" \
    -d '{"name":"backend-team","endpoint":"http://localhost:9530","type":"software","agents":["coder"]}')
BACKEND_ID=$(echo "$BACKEND" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "FAIL")
BACKEND_STATUS=$(echo "$BACKEND" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "?")
log "  backend-team  → ID: ${BACKEND_ID}  Status: ${BACKEND_STATUS}"

if [ "$DESIGN_ID" = "FAIL" ] || [ "$FRONTEND_ID" = "FAIL" ] || [ "$BACKEND_ID" = "FAIL" ]; then
    err "联邦注册失败，请检查日志: ${BASE_DIR}/master/stdout.log"
    exit 1
fi

# ─── 下发 Goal ────────────────────────────────────────────────────────────────
echo ""
log "下发联邦 Goal（带依赖编排）..."

GOAL_RESPONSE=$(curl -s -X POST "${MASTER}/api/goals/federated" \
    -H "Content-Type: application/json" \
    -d "{
  \"name\": \"实现用户登录功能\",
  \"description\": \"完整登录功能：UI 设计 → API 接口定义 → 前后端并行开发\",
  \"projects\": [
    {
      \"name\": \"ui-design\",
      \"companyId\": \"${DESIGN_ID}\",
      \"agent\": \"designer\",
      \"description\": \"设计登录页面，输出高保真设计稿和交互标注\"
    },
    {
      \"name\": \"api-spec\",
      \"companyId\": \"${BACKEND_ID}\",
      \"agent\": \"coder\",
      \"description\": \"定义 REST API 接口文档: POST /api/auth/login, POST /api/auth/register\",
      \"dependencies\": [\"ui-design\"]
    },
    {
      \"name\": \"frontend-dev\",
      \"companyId\": \"${FRONTEND_ID}\",
      \"agent\": \"coder\",
      \"description\": \"根据 UI 设计稿和 API 文档实现前端登录页\",
      \"dependencies\": [\"ui-design\", \"api-spec\"]
    },
    {
      \"name\": \"backend-dev\",
      \"companyId\": \"${BACKEND_ID}\",
      \"agent\": \"coder\",
      \"description\": \"根据 API 文档实现后端登录接口\",
      \"dependencies\": [\"api-spec\"]
    }
  ]
}")

GOAL_ID=$(echo "$GOAL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('goalId','?'))" 2>/dev/null || echo "?")
LAYERS=$(echo "$GOAL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('layers','?'))" 2>/dev/null || echo "?")

log "  Goal ID: ${GOAL_ID}"
log "  DAG 层数: ${LAYERS}"
log "  编排顺序:"
log "    Layer 0: ui-design (design-team)          ← 立即执行"
log "    Layer 1: api-spec (backend-team)           ← 等 ui-design 完成"
log "    Layer 2: frontend-dev + backend-dev (并行)  ← 等 api-spec 完成"

# ─── 完成 ─────────────────────────────────────────────────────────────────────
echo ""
log "=========================================="
log "  演示已启动！"
log "=========================================="
echo ""
log "  ┌─────────────┬────────────────────────────────┐"
log "  │ 节点        │ API                            │"
log "  ├─────────────┼────────────────────────────────┤"
log "  │ Master      │ http://localhost:9527/api       │"
log "  │ Design      │ http://localhost:9528/api       │"
log "  │ Frontend    │ http://localhost:9529/api       │"
log "  │ Backend     │ http://localhost:9530/api       │"
log "  └─────────────┴────────────────────────────────┘"
echo ""
log "  Dashboard:     http://localhost:3000"
log "  联邦聚合视图:  http://localhost:3000/federation"
log "  Jaeger UI:     http://localhost:16686"
echo ""
log "  Federation 页面里展开 Company 卡片即可查看各节点的 Agents / Tasks / Metrics"
echo ""
log "  查看日志:      tail -f ${BASE_DIR}/master/stdout.log"
log "  停止:          bash run-demo.sh stop"
