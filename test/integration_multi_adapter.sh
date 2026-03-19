#!/usr/bin/env bash
###############################################################################
# OPC Platform — Multi-Adapter Integration Test (方案 C: 场景驱动)
#
# 架构: 1 Master(:9527) + 2 Worker(:9528, :9529)，场景内动态注册 Agent
#
# 覆盖场景:
#   Scenario 1: Local Goal × Claude Code
#   Scenario 2: Local Goal × OpenClaw
#   Scenario 3: Federated Goal × 全 Claude Code (3层 DAG)
#   Scenario 4: Federated Goal × 全 OpenClaw (3层 DAG)
#   Scenario 5: Federated Goal × 混合 CC + OC (3层 DAG)
#   Scenario 6: 并发 — 同时下发 2 个 Federated Goal
#
# 前提:
#   - OpenClaw Gateway 运行在 http://127.0.0.1:18789
#   - claude CLI 可用 (which claude)
#
# 用法:
#   bash test/integration_multi_adapter.sh              # 运行全部
#   bash test/integration_multi_adapter.sh --scenario 3 # 只跑 Scenario 3
#   bash test/integration_multi_adapter.sh --skip-build # 跳过编译
#   bash test/integration_multi_adapter.sh stop         # 停止所有实例
###############################################################################

set -euo pipefail

# ─── 配置 ────────────────────────────────────────────────────────────────────
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OPCTL="${PROJECT_ROOT}/opctl"
BASE_DIR="${HOME}/.opc-integration-test"
LOG_DIR="${BASE_DIR}/logs"

MASTER_PORT=9527
WORKER1_PORT=9528
WORKER2_PORT=9529
MASTER_URL="http://127.0.0.1:${MASTER_PORT}/api"
WORKER1_URL="http://127.0.0.1:${WORKER1_PORT}/api"
WORKER2_URL="http://127.0.0.1:${WORKER2_PORT}/api"

OPENCLAW_GW_URL="ws://127.0.0.1:18789"
OPENCLAW_HTTP_URL="http://127.0.0.1:18789"

# 超时 (秒)
GOAL_TIMEOUT=300        # 单 Goal 最长等待
HEALTH_TIMEOUT=15       # 实例启动等待
FEDERATED_TIMEOUT=600   # 联邦 Goal 最长等待

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# 计数器
PASS=0
FAIL=0
SKIP=0
TOTAL=0
SCENARIO_RESULTS=()

# 参数
ONLY_SCENARIO=""
SKIP_BUILD=false

# ─── 解析参数 ─────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        stop)
            # 提前定义 stop，解析后立即执行
            shift
            for port in ${MASTER_PORT} ${WORKER1_PORT} ${WORKER2_PORT}; do
                pids=$(lsof -ti :"${port}" 2>/dev/null || true)
                if [ -n "$pids" ]; then
                    echo "$pids" | xargs kill -9 2>/dev/null || true
                    echo -e "${GREEN}[STOP]${NC} :${port} 已停止 (PIDs: $(echo $pids | tr '\n' ' '))"
                fi
            done
            pkill -9 -f "opctl serve" 2>/dev/null || true
            echo -e "${GREEN}[STOP]${NC} 全部停止"
            exit 0
            ;;
        --scenario)
            ONLY_SCENARIO="$2"; shift 2 ;;
        --skip-build)
            SKIP_BUILD=true; shift ;;
        *)
            echo "未知参数: $1"; exit 1 ;;
    esac
done

# ─── 工具函数 ─────────────────────────────────────────────────────────────────
log()      { echo -e "${GREEN}[OPC]${NC} $*"; }
warn()     { echo -e "${YELLOW}[WARN]${NC} $*" >&2; }
err()      { echo -e "${RED}[ERR]${NC} $*" >&2; }
info()     { echo -e "${BLUE}[INFO]${NC} $*" >&2; }
scenario() { echo -e "\n${BOLD}${CYAN}═══════════════════════════════════════════${NC}"; \
             echo -e "${BOLD}${CYAN}  Scenario $1: $2${NC}"; \
             echo -e "${BOLD}${CYAN}═══════════════════════════════════════════${NC}"; }

assert_ok() {
    local name="$1" actual="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$actual" -ge 200 ] && [ "$actual" -lt 300 ]; then
        echo -e "  ${GREEN}✓ PASS${NC}: $name (HTTP $actual)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗ FAIL${NC}: $name (HTTP $actual)"
        # Show response body on failure for debugging
        [ -n "$RESP_BODY" ] && echo -e "  ${RED}  ↳ ${RESP_BODY}${NC}" >&2
        FAIL=$((FAIL + 1))
    fi
}

assert_eq() {
    local name="$1" expected="$2" actual="$3"
    TOTAL=$((TOTAL + 1))
    if [ "$actual" = "$expected" ]; then
        echo -e "  ${GREEN}✓ PASS${NC}: $name ($actual)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗ FAIL${NC}: $name (expected '$expected', got '$actual')"
        FAIL=$((FAIL + 1))
    fi
}

assert_not_empty() {
    local name="$1" value="$2"
    TOTAL=$((TOTAL + 1))
    if [ -n "$value" ] && [ "$value" != "null" ] && [ "$value" != "" ]; then
        echo -e "  ${GREEN}✓ PASS${NC}: $name (non-empty)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗ FAIL${NC}: $name (empty or null)"
        FAIL=$((FAIL + 1))
    fi
}

assert_gt() {
    local name="$1" threshold="$2" actual="$3"
    TOTAL=$((TOTAL + 1))
    if [ "$actual" -gt "$threshold" ] 2>/dev/null; then
        echo -e "  ${GREEN}✓ PASS${NC}: $name ($actual > $threshold)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗ FAIL${NC}: $name ($actual <= $threshold)"
        FAIL=$((FAIL + 1))
    fi
}

# HTTP 请求封装
http_post() {
    local url="$1" data="$2"
    # Use temp file to avoid bash quoting issues with large JSON payloads
    local tmpfile
    tmpfile=$(mktemp)
    printf '%s' "$data" > "$tmpfile"
    curl -s -w "\n%{http_code}" -X POST "$url" \
        -H "Content-Type: application/json" -d @"$tmpfile" 2>/dev/null || echo -e "\n000"
    rm -f "$tmpfile"
}

http_post_yaml() {
    local url="$1" data="$2"
    curl -s -w "\n%{http_code}" -X POST "$url" \
        -H "Content-Type: application/x-yaml" -d "$data" 2>/dev/null || echo -e "\n000"
}

http_get() {
    local url="$1"
    curl -s -w "\n%{http_code}" "$url" 2>/dev/null || echo -e "\n000"
}

http_delete() {
    local url="$1"
    curl -s -w "\n%{http_code}" -X DELETE "$url" 2>/dev/null || echo -e "\n000"
}

# 从 curl 输出分离 body 和 status code
split_response() {
    local response="$1"
    RESP_BODY=$(echo "$response" | sed '$d')
    RESP_CODE=$(echo "$response" | tail -1)
}

# JSON 提取
jq_field() {
    echo "$1" | python3 -c "import sys,json; print(json.load(sys.stdin)$2)" 2>/dev/null || echo ""
}

# ─── 前置检查 ─────────────────────────────────────────────────────────────────
preflight_check() {
    log "前置检查..."

    # jq / python3
    if ! command -v python3 &>/dev/null; then
        err "python3 未安装"; exit 1
    fi

    # Claude CLI
    if command -v claude &>/dev/null; then
        log "  claude CLI: $(which claude) ${GREEN}✓${NC}"
        HAS_CLAUDE=true
    else
        warn "  claude CLI 未找到，CC 相关场景将跳过"
        HAS_CLAUDE=false
    fi

    # OpenClaw Gateway
    if curl -sf --max-time 3 "${OPENCLAW_HTTP_URL}" >/dev/null 2>&1 || \
       curl -sf --max-time 3 "${OPENCLAW_HTTP_URL}/health" >/dev/null 2>&1; then
        log "  OpenClaw Gateway: ${OPENCLAW_HTTP_URL} ${GREEN}✓${NC}"
        HAS_OPENCLAW=true
    else
        warn "  OpenClaw Gateway (${OPENCLAW_HTTP_URL}) 不可达，OC 相关场景将跳过"
        HAS_OPENCLAW=false
    fi

    echo ""
}

# ─── 编译 ─────────────────────────────────────────────────────────────────────
build_opctl() {
    if [ "$SKIP_BUILD" = true ] && [ -f "$OPCTL" ]; then
        log "跳过编译 (--skip-build)"
        return
    fi
    log "编译 opctl..."
    (cd "$PROJECT_ROOT" && go build -o opctl ./cmd/opctl)
    log "  编译完成"
}

# ─── 启动/停止 OPC 实例 ───────────────────────────────────────────────────────
stop_all() {
    for port in ${MASTER_PORT} ${WORKER1_PORT} ${WORKER2_PORT}; do
        # lsof may return multiple PIDs; kill all of them
        lsof -ti :"${port}" 2>/dev/null | xargs kill -9 2>/dev/null || true
    done
    pkill -9 -f "opctl serve" 2>/dev/null || true
    sleep 1
}

start_instances() {
    log "清理旧进程..."
    stop_all 2>/dev/null || true

    log "清理旧状态..."
    rm -rf "${BASE_DIR}"
    mkdir -p "${LOG_DIR}"
    mkdir -p "${BASE_DIR}"/{master,worker1,worker2}/state

    log "启动 OPC 实例..."

    # Master
    OPENCLAW_GATEWAY_URL="${OPENCLAW_GW_URL}" \
    "$OPCTL" serve --port ${MASTER_PORT} --host 127.0.0.1 \
        --state-dir "${BASE_DIR}/master/state" \
        > "${LOG_DIR}/master.log" 2>&1 &
    log "  Master   → :${MASTER_PORT} (PID $!)"

    # Worker 1
    OPENCLAW_GATEWAY_URL="${OPENCLAW_GW_URL}" \
    "$OPCTL" serve --port ${WORKER1_PORT} --host 127.0.0.1 \
        --state-dir "${BASE_DIR}/worker1/state" \
        > "${LOG_DIR}/worker1.log" 2>&1 &
    log "  Worker1  → :${WORKER1_PORT} (PID $!)"

    # Worker 2
    OPENCLAW_GATEWAY_URL="${OPENCLAW_GW_URL}" \
    "$OPCTL" serve --port ${WORKER2_PORT} --host 127.0.0.1 \
        --state-dir "${BASE_DIR}/worker2/state" \
        > "${LOG_DIR}/worker2.log" 2>&1 &
    log "  Worker2  → :${WORKER2_PORT} (PID $!)"

    # 等待健康检查
    log "等待实例就绪..."
    local deadline=$((SECONDS + HEALTH_TIMEOUT))
    while [ $SECONDS -lt $deadline ]; do
        all_up=true
        for port in ${MASTER_PORT} ${WORKER1_PORT} ${WORKER2_PORT}; do
            if ! curl -sf "http://127.0.0.1:${port}/api/health" >/dev/null 2>&1; then
                all_up=false
            fi
        done
        if $all_up; then break; fi
        sleep 1
    done

    echo ""
    for port in ${MASTER_PORT} ${WORKER1_PORT} ${WORKER2_PORT}; do
        if curl -sf "http://127.0.0.1:${port}/api/health" >/dev/null 2>&1; then
            log "  :${port} ${GREEN}✓ Ready${NC}"
        else
            err "  :${port} ✗ 未就绪"
            err "  查看日志: tail -f ${LOG_DIR}/*.log"
            exit 1
        fi
    done
    echo ""
}

# ─── Agent 管理 ───────────────────────────────────────────────────────────────

# 通过 /api/apply 注册 Agent (YAML)
apply_agent() {
    local api_url="$1" name="$2" type="$3" workdir="${4:-/tmp/opc}"
    local yaml_body

    if [ "$type" = "claude-code" ]; then
        yaml_body="apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ${name}
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: 300s
  context:
    workdir: ${workdir}
  recovery:
    enabled: true
    maxRestarts: 2"
    elif [ "$type" = "openclaw" ]; then
        yaml_body="apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ${name}
spec:
  type: openclaw
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: 300s
  context:
    workdir: ${workdir}
  recovery:
    enabled: true
    maxRestarts: 2
  env:
    OPENCLAW_GATEWAY_URL: ${OPENCLAW_GW_URL}"
    else
        err "未知 agent type: $type"; return 1
    fi

    split_response "$(http_post_yaml "${api_url}/apply" "$yaml_body")"
    info "apply agent ${name}(${type}) → HTTP ${RESP_CODE}"

    # Apply only creates the agent; must start it explicitly.
    sleep 1
    split_response "$(http_post "${api_url}/agents/${name}/start" '{}')"
    info "start agent ${name} → HTTP ${RESP_CODE}"
}

# 删除 Agent
delete_agent() {
    local api_url="$1" name="$2"
    split_response "$(http_delete "${api_url}/agents/${name}")"
}

# 注册联邦公司
register_company() {
    local name="$1" endpoint="$2" agent_type="$3" agents="$4"
    local payload="{\"name\":\"${name}\",\"endpoint\":\"${endpoint}\",\"type\":\"software\",\"agents\":[\"${agents}\"]}"
    split_response "$(http_post "${MASTER_URL}/federation/companies" "$payload")"
    local company_id
    company_id=$(jq_field "$RESP_BODY" ".get('id','')")
    if [ -z "$company_id" ] || [ "$company_id" = "null" ]; then
        info "register_company ${name} failed: ${RESP_BODY}" >&2
    else
        info "register_company ${name} → ${company_id}" >&2
    fi
    echo "$company_id"
}

# 清理联邦公司
cleanup_companies() {
    local response
    split_response "$(http_get "${MASTER_URL}/federation/companies")"
    local ids
    ids=$(echo "$RESP_BODY" | python3 -c "
import sys,json
try:
    data=json.load(sys.stdin)
    if isinstance(data, list):
        for c in data: print(c.get('id',''))
except: pass
" 2>/dev/null || true)
    for id in $ids; do
        [ -n "$id" ] && http_delete "${MASTER_URL}/federation/companies/${id}" >/dev/null 2>&1
    done
}

# ─── Goal 轮询 ───────────────────────────────────────────────────────────────

# 等待 Local Goal 完成
wait_local_goal() {
    local goal_id="$1" timeout="${2:-$GOAL_TIMEOUT}"
    local deadline=$((SECONDS + timeout))
    local status=""

    info "轮询 Goal ${goal_id} (超时 ${timeout}s)..."

    while [ $SECONDS -lt $deadline ]; do
        split_response "$(http_get "${MASTER_URL}/goals/${goal_id}")"
        status=$(jq_field "$RESP_BODY" ".get('status','')")
        local phase
        phase=$(jq_field "$RESP_BODY" ".get('phase','')")

        # Check both status and phase (UpdateGoal bug: status may not persist)
        if [ "$status" = "completed" ] || [ "$phase" = "completed" ]; then
            info "  Goal ${goal_id} → completed ✓ (status=${status} phase=${phase})"
            echo "$RESP_BODY"
            return 0
        elif [ "$status" = "failed" ] || [ "$phase" = "failed" ]; then
            warn "  Goal ${goal_id} → failed ✗ (status=${status} phase=${phase})"
            echo "$RESP_BODY"
            return 1
        fi

        info "  ... status=${status} phase=${phase} (elapsed $((SECONDS - deadline + timeout))s)"
        sleep 3
    done

    err "  Goal ${goal_id} 超时 (${timeout}s), last status=${status} phase=${phase}"
    echo "$RESP_BODY"
    return 1
}

# 等待联邦 Goal 完成 (通过 /api/goals/:id 状态)
wait_federated_goal() {
    local goal_id="$1" timeout="${2:-$FEDERATED_TIMEOUT}"
    local deadline=$((SECONDS + timeout))
    local status=""

    info "轮询联邦 Goal ${goal_id} (超时 ${timeout}s)..."

    while [ $SECONDS -lt $deadline ]; do
        split_response "$(http_get "${MASTER_URL}/goals/${goal_id}")"
        status=$(jq_field "$RESP_BODY" ".get('status','')")
        local phase
        phase=$(jq_field "$RESP_BODY" ".get('phase','')")
        local cost
        cost=$(jq_field "$RESP_BODY" ".get('cost', 0)")

        if [ "$status" = "completed" ] || [ "$phase" = "completed" ]; then
            info "  联邦 Goal ${goal_id} → completed ✓ (cost=${cost})"
            echo "$RESP_BODY"
            return 0
        elif [ "$status" = "failed" ] || [ "$phase" = "failed" ]; then
            warn "  联邦 Goal ${goal_id} → failed ✗"
            echo "$RESP_BODY"
            return 1
        fi

        info "  ... status=${status} phase=${phase} cost=${cost} (elapsed $((SECONDS - deadline + timeout))s)"
        sleep 5
    done

    err "  联邦 Goal ${goal_id} 超时 (${timeout}s), last status=${status} phase=${phase}"
    echo "$RESP_BODY"
    return 1
}

# ─── 场景函数 ─────────────────────────────────────────────────────────────────

should_run() {
    [ -z "$ONLY_SCENARIO" ] || [ "$ONLY_SCENARIO" = "$1" ]
}

record_scenario() {
    local num="$1" name="$2" result="$3"
    if [ "$result" = "PASS" ]; then
        SCENARIO_RESULTS+=("${GREEN}✓ S${num}${NC}: ${name}")
    elif [ "$result" = "SKIP" ]; then
        SCENARIO_RESULTS+=("${YELLOW}○ S${num}${NC}: ${name} (SKIPPED)")
    else
        SCENARIO_RESULTS+=("${RED}✗ S${num}${NC}: ${name}")
    fi
}

# ── Scenario 1: Local Goal × Claude Code ─────────────────────────────────────
run_scenario_1() {
    scenario "1" "Local Goal × Claude Code"

    if [ "$HAS_CLAUDE" != true ]; then
        warn "claude CLI 不可用，跳过"; SKIP=$((SKIP + 1))
        record_scenario 1 "Local Goal × Claude Code" "SKIP"
        return
    fi

    local s1_pass=$PASS s1_fail=$FAIL

    # autoDecompose 使用固定 agent 名称 "coder"，预先 apply 为 CC 类型
    # 这样 ensureAgent 发现已存在就不会覆盖
    apply_agent "$MASTER_URL" "coder" "claude-code" "/tmp/opc-s1"
    assert_ok "Apply CC agent (coder)" "$RESP_CODE"

    # 创建 Local Goal (autoDecompose=true → 使用 "coder" agent)
    split_response "$(http_post "${MASTER_URL}/goals" '{
        "name": "S1-CC-Local-Goal",
        "description": "请用一句话解释什么是 Kubernetes。只需回答，不需要写代码。",
        "autoDecompose": true
    }')"
    assert_ok "Create local goal" "$RESP_CODE"
    local goal_id
    goal_id=$(jq_field "$RESP_BODY" ".get('id','')")
    assert_not_empty "Goal ID returned" "$goal_id"

    # 等待完成
    local goal_result
    if goal_result=$(wait_local_goal "$goal_id" "$GOAL_TIMEOUT"); then
        assert_eq "Goal status" "completed" "$(jq_field "$goal_result" ".get('status','')")"

        # 验证 token 消耗
        local tokens_in
        tokens_in=$(jq_field "$goal_result" ".get('tokensIn', 0)")
        assert_gt "TokensIn > 0" 0 "${tokens_in:-0}"
    else
        TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
        echo -e "  ${RED}✗ FAIL${NC}: Goal did not complete"
    fi

    # 清理 (删除 coder agent 以便 S2 可以用 OC 类型重建)
    delete_agent "$MASTER_URL" "coder"

    if [ $FAIL -eq $s1_fail ]; then
        record_scenario 1 "Local Goal × Claude Code" "PASS"
    else
        record_scenario 1 "Local Goal × Claude Code" "FAIL"
    fi
}

# ── Scenario 2: Local Goal × OpenClaw ────────────────────────────────────────
run_scenario_2() {
    scenario "2" "Local Goal × OpenClaw"

    if [ "$HAS_OPENCLAW" != true ]; then
        warn "OpenClaw Gateway 不可用，跳过"; SKIP=$((SKIP + 1))
        record_scenario 2 "Local Goal × OpenClaw" "SKIP"
        return
    fi

    local s2_pass=$PASS s2_fail=$FAIL

    # autoDecompose 使用固定 agent 名称 "coder"，预先 apply 为 OC 类型
    # S1 已删除 "coder"，这里重新创建为 openclaw 类型
    apply_agent "$MASTER_URL" "coder" "openclaw" "/tmp/opc-s2"
    assert_ok "Apply OC agent (coder)" "$RESP_CODE"

    # 创建 Local Goal (autoDecompose=true → 使用 "coder" agent，此时为 OC 类型)
    split_response "$(http_post "${MASTER_URL}/goals" '{
        "name": "S2-OC-Local-Goal",
        "description": "请用一句话解释什么是微服务架构。只需回答，不需要写代码。",
        "autoDecompose": true
    }')"
    assert_ok "Create local goal" "$RESP_CODE"
    local goal_id
    goal_id=$(jq_field "$RESP_BODY" ".get('id','')")
    assert_not_empty "Goal ID returned" "$goal_id"

    # 等待完成
    local goal_result
    if goal_result=$(wait_local_goal "$goal_id" "$GOAL_TIMEOUT"); then
        assert_eq "Goal status" "completed" "$(jq_field "$goal_result" ".get('status','')")"
        # OpenClaw agent.wait doesn't return token counts — skip assertion
        local tokens_in
        tokens_in=$(jq_field "$goal_result" ".get('tokensIn', 0)")
        if [ "${tokens_in:-0}" -gt 0 ] 2>/dev/null; then
            echo -e "  ${GREEN}✓ PASS${NC}: TokensIn > 0 (${tokens_in})"
            TOTAL=$((TOTAL + 1)); PASS=$((PASS + 1))
        else
            echo -e "  ${YELLOW}○ SKIP${NC}: TokensIn = 0 (OpenClaw agent.wait 不返回 token 统计)"
        fi
    else
        TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
        echo -e "  ${RED}✗ FAIL${NC}: Goal did not complete"
    fi

    # 清理
    delete_agent "$MASTER_URL" "coder"

    if [ $FAIL -eq $s2_fail ]; then
        record_scenario 2 "Local Goal × OpenClaw" "PASS"
    else
        record_scenario 2 "Local Goal × OpenClaw" "FAIL"
    fi
}

# ── Scenario 3: Federated Goal × 全 Claude Code (3层 DAG) ────────────────────
run_scenario_3() {
    scenario "3" "Federated Goal × All Claude Code (3-layer DAG)"

    if [ "$HAS_CLAUDE" != true ]; then
        warn "claude CLI 不可用，跳过"; SKIP=$((SKIP + 1))
        record_scenario 3 "Federated Goal × All CC" "SKIP"
        return
    fi

    local s3_pass=$PASS s3_fail=$FAIL

    # 1. 在 Worker 上预注册 Agent
    apply_agent "$WORKER1_URL" "designer" "claude-code" "/tmp/opc-s3-w1"
    apply_agent "$WORKER1_URL" "coder" "claude-code" "/tmp/opc-s3-w1"
    apply_agent "$WORKER2_URL" "coder" "claude-code" "/tmp/opc-s3-w2"

    # 2. 注册联邦公司
    cleanup_companies
    local design_id frontend_id backend_id
    design_id=$(register_company "design-team" "http://127.0.0.1:${WORKER1_PORT}" "claude-code" "designer")
    frontend_id=$(register_company "frontend-team" "http://127.0.0.1:${WORKER1_PORT}" "claude-code" "coder")
    backend_id=$(register_company "backend-team" "http://127.0.0.1:${WORKER2_PORT}" "claude-code" "coder")

    assert_not_empty "Design company ID" "$design_id"
    assert_not_empty "Frontend company ID" "$frontend_id"
    assert_not_empty "Backend company ID" "$backend_id"

    # 3. 创建联邦 Goal (3层 DAG)
    local fed_payload
    fed_payload=$(cat <<EOF
{"name":"S3-全CC联邦Goal","description":"设计并实现一个简单的 Hello API","projects":[{"name":"api-design","companyId":"${design_id}","agent":"designer","description":"设计一个 Hello API 的接口规范。只输出文字描述即可。"},{"name":"backend-impl","companyId":"${backend_id}","agent":"coder","description":"根据上游的 API 设计，用伪代码描述后端实现。只输出文字即可。","dependencies":["api-design"]},{"name":"integration-test","companyId":"${frontend_id}","agent":"coder","description":"根据上游设计和后端实现，描述集成测试方案。只输出文字即可。","dependencies":["api-design","backend-impl"]}]}
EOF
    )
    split_response "$(http_post "${MASTER_URL}/goals/federated" "$fed_payload")"
    assert_ok "Create federated goal" "$RESP_CODE"
    local goal_id layers
    goal_id=$(jq_field "$RESP_BODY" ".get('goalId','')")
    layers=$(jq_field "$RESP_BODY" ".get('layers','')")
    assert_not_empty "Federated Goal ID" "$goal_id"

    if [ -z "$goal_id" ] || [ "$goal_id" = "null" ]; then
        err "联邦 Goal 创建失败，跳过等待。响应: ${RESP_BODY}"
    else
        assert_eq "DAG layers = 3" "3" "$layers"

        info "Goal ${goal_id}: DAG 层数=${layers}"
        info "  Layer 0: api-design (design-team/CC)"
        info "  Layer 1: backend-impl (backend-team/CC)"
        info "  Layer 2: integration-test (frontend-team/CC)"

        # 4. 等待完成
        local goal_result
        if goal_result=$(wait_federated_goal "$goal_id" "$FEDERATED_TIMEOUT"); then
            assert_eq "Federated goal status" "completed" "$(jq_field "$goal_result" ".get('status','')")"
        else
            TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
            echo -e "  ${RED}✗ FAIL${NC}: Federated goal did not complete"
        fi
    fi

    # 5. 清理
    cleanup_companies
    delete_agent "$WORKER1_URL" "designer"
    delete_agent "$WORKER1_URL" "coder"
    delete_agent "$WORKER2_URL" "coder"

    if [ $FAIL -eq $s3_fail ]; then
        record_scenario 3 "Federated Goal × All CC" "PASS"
    else
        record_scenario 3 "Federated Goal × All CC" "FAIL"
    fi
}

# ── Scenario 4: Federated Goal × 全 OpenClaw (3层 DAG) ───────────────────────
run_scenario_4() {
    scenario "4" "Federated Goal × All OpenClaw (3-layer DAG)"

    if [ "$HAS_OPENCLAW" != true ]; then
        warn "OpenClaw Gateway 不可用，跳过"; SKIP=$((SKIP + 1))
        record_scenario 4 "Federated Goal × All OC" "SKIP"
        return
    fi

    local s4_pass=$PASS s4_fail=$FAIL

    # 1. 在 Worker 上预注册 Agent
    apply_agent "$WORKER1_URL" "oc-designer" "openclaw" "/tmp/opc-s4-w1"
    apply_agent "$WORKER1_URL" "oc-coder" "openclaw" "/tmp/opc-s4-w1"
    apply_agent "$WORKER2_URL" "oc-coder" "openclaw" "/tmp/opc-s4-w2"

    # 2. 注册联邦公司
    cleanup_companies
    local design_id frontend_id backend_id
    design_id=$(register_company "oc-design-team" "http://127.0.0.1:${WORKER1_PORT}" "openclaw" "oc-designer")
    frontend_id=$(register_company "oc-frontend-team" "http://127.0.0.1:${WORKER1_PORT}" "openclaw" "oc-coder")
    backend_id=$(register_company "oc-backend-team" "http://127.0.0.1:${WORKER2_PORT}" "openclaw" "oc-coder")

    assert_not_empty "Design company ID" "$design_id"
    assert_not_empty "Frontend company ID" "$frontend_id"
    assert_not_empty "Backend company ID" "$backend_id"

    # 3. 创建联邦 Goal (3层 DAG)
    local fed_payload
    fed_payload=$(cat <<EOF
{"name":"S4-全OC联邦Goal","description":"设计并实现一个用户注册接口","projects":[{"name":"api-design","companyId":"${design_id}","agent":"oc-designer","description":"设计一个用户注册 API 的接口规范。只输出文字描述即可。"},{"name":"backend-impl","companyId":"${backend_id}","agent":"oc-coder","description":"根据上游设计，用伪代码描述后端实现。只输出文字即可。","dependencies":["api-design"]},{"name":"frontend-impl","companyId":"${frontend_id}","agent":"oc-coder","description":"根据上游设计和后端实现，描述前端调用方式。只输出文字即可。","dependencies":["api-design","backend-impl"]}]}
EOF
    )
    split_response "$(http_post "${MASTER_URL}/goals/federated" "$fed_payload")"
    assert_ok "Create federated goal" "$RESP_CODE"
    local goal_id layers
    goal_id=$(jq_field "$RESP_BODY" ".get('goalId','')")
    layers=$(jq_field "$RESP_BODY" ".get('layers','')")
    assert_not_empty "Federated Goal ID" "$goal_id"

    if [ -z "$goal_id" ] || [ "$goal_id" = "null" ]; then
        err "联邦 Goal 创建失败，跳过等待。响应: ${RESP_BODY}"
    else
        assert_eq "DAG layers = 3" "3" "$layers"

        info "Goal ${goal_id}: DAG 层数=${layers}"
        info "  Layer 0: api-design (oc-design-team/OC)"
        info "  Layer 1: backend-impl (oc-backend-team/OC)"
        info "  Layer 2: frontend-impl (oc-frontend-team/OC)"

        local goal_result
        if goal_result=$(wait_federated_goal "$goal_id" "$FEDERATED_TIMEOUT"); then
            assert_eq "Federated goal status" "completed" "$(jq_field "$goal_result" ".get('status','')")"
        else
            TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
            echo -e "  ${RED}✗ FAIL${NC}: Federated goal did not complete"
        fi
    fi

    # 5. 清理
    cleanup_companies
    delete_agent "$WORKER1_URL" "oc-designer"
    delete_agent "$WORKER1_URL" "oc-coder"
    delete_agent "$WORKER2_URL" "oc-coder"

    if [ $FAIL -eq $s4_fail ]; then
        record_scenario 4 "Federated Goal × All OC" "PASS"
    else
        record_scenario 4 "Federated Goal × All OC" "FAIL"
    fi
}

# ── Scenario 5: Federated Goal × 混合 CC + OC (3层 DAG) ──────────────────────
run_scenario_5() {
    scenario "5" "Federated Goal × Mixed CC + OpenClaw (3-layer DAG)"

    if [ "$HAS_CLAUDE" != true ] || [ "$HAS_OPENCLAW" != true ]; then
        warn "需要 claude CLI + OpenClaw Gateway，跳过"; SKIP=$((SKIP + 1))
        record_scenario 5 "Federated Goal × Mixed" "SKIP"
        return
    fi

    local s5_pass=$PASS s5_fail=$FAIL

    # 混合布局:
    #   Worker1 → CC (designer + cc-coder)
    #   Worker2 → OC (oc-coder)
    # DAG:
    #   Layer 0: design (CC)
    #   Layer 1: api-spec (OC)
    #   Layer 2: frontend(OC) + backend(CC) 并行

    apply_agent "$WORKER1_URL" "mix-designer" "claude-code" "/tmp/opc-s5-w1"
    apply_agent "$WORKER1_URL" "mix-cc-coder" "claude-code" "/tmp/opc-s5-w1"
    apply_agent "$WORKER2_URL" "mix-oc-coder" "openclaw" "/tmp/opc-s5-w2"

    cleanup_companies
    local cc_company_id oc_company_id
    cc_company_id=$(register_company "mix-cc-team" "http://127.0.0.1:${WORKER1_PORT}" "claude-code" "mix-designer")
    oc_company_id=$(register_company "mix-oc-team" "http://127.0.0.1:${WORKER2_PORT}" "openclaw" "mix-oc-coder")

    assert_not_empty "CC company ID" "$cc_company_id"
    assert_not_empty "OC company ID" "$oc_company_id"

    # 联邦 Goal: 4 个 project, 3 层
    local fed_payload
    fed_payload=$(cat <<EOF
{"name":"S5-混合联邦Goal","description":"设计并实现一个登录功能（混合 CC + OpenClaw）","projects":[{"name":"ui-design","companyId":"${cc_company_id}","agent":"mix-designer","description":"设计登录页面的 UI 布局和交互流程。只输出文字描述即可。"},{"name":"api-spec","companyId":"${oc_company_id}","agent":"mix-oc-coder","description":"根据上游 UI 设计，定义登录 REST API 接口规范。只输出文字即可。","dependencies":["ui-design"]},{"name":"frontend-dev","companyId":"${oc_company_id}","agent":"mix-oc-coder","description":"根据 UI 设计和 API 规范，描述前端实现方案。只输出文字即可。","dependencies":["ui-design","api-spec"]},{"name":"backend-dev","companyId":"${cc_company_id}","agent":"mix-cc-coder","description":"根据 API 规范，描述后端登录接口实现方案。只输出文字即可。","dependencies":["api-spec"]}]}
EOF
    )
    split_response "$(http_post "${MASTER_URL}/goals/federated" "$fed_payload")"
    assert_ok "Create mixed federated goal" "$RESP_CODE"
    local goal_id layers
    goal_id=$(jq_field "$RESP_BODY" ".get('goalId','')")
    layers=$(jq_field "$RESP_BODY" ".get('layers','')")
    assert_not_empty "Federated Goal ID" "$goal_id"

    if [ -z "$goal_id" ] || [ "$goal_id" = "null" ]; then
        err "联邦 Goal 创建失败，跳过等待。响应: ${RESP_BODY}"
    else
        assert_eq "DAG layers = 3" "3" "$layers"

        info "Goal ${goal_id}: DAG 层数=${layers}"
        info "  Layer 0: ui-design (CC)"
        info "  Layer 1: api-spec (OC)"
        info "  Layer 2: frontend-dev(OC) + backend-dev(CC) ← 并行"

        local goal_result
        if goal_result=$(wait_federated_goal "$goal_id" "$FEDERATED_TIMEOUT"); then
            assert_eq "Mixed federated goal status" "completed" "$(jq_field "$goal_result" ".get('status','')")"
            local total_cost
            total_cost=$(jq_field "$goal_result" ".get('cost', 0)")
            info "  总 cost = ${total_cost}"
        else
            TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
            echo -e "  ${RED}✗ FAIL${NC}: Mixed federated goal did not complete"
        fi
    fi

    # 清理
    cleanup_companies
    delete_agent "$WORKER1_URL" "mix-designer"
    delete_agent "$WORKER1_URL" "mix-cc-coder"
    delete_agent "$WORKER2_URL" "mix-oc-coder"

    if [ $FAIL -eq $s5_fail ]; then
        record_scenario 5 "Federated Goal × Mixed" "PASS"
    else
        record_scenario 5 "Federated Goal × Mixed" "FAIL"
    fi
}

# ── Scenario 6: 并发 — 同时下发 2 个 Federated Goal ──────────────────────────
run_scenario_6() {
    scenario "6" "Concurrent Federated Goals"

    # 使用可用的 adapter (优先 OC，回退 CC)
    local agent_type=""
    if [ "$HAS_OPENCLAW" = true ]; then
        agent_type="openclaw"
    elif [ "$HAS_CLAUDE" = true ]; then
        agent_type="claude-code"
    else
        warn "无可用 adapter，跳过"; SKIP=$((SKIP + 1))
        record_scenario 6 "Concurrent Goals" "SKIP"
        return
    fi

    local s6_pass=$PASS s6_fail=$FAIL

    info "使用 adapter: ${agent_type}"

    # 注册 Agent
    apply_agent "$WORKER1_URL" "concurrent-a" "$agent_type" "/tmp/opc-s6-w1"
    apply_agent "$WORKER2_URL" "concurrent-b" "$agent_type" "/tmp/opc-s6-w2"

    cleanup_companies
    local company_a company_b
    company_a=$(register_company "concurrent-team-a" "http://127.0.0.1:${WORKER1_PORT}" "$agent_type" "concurrent-a")
    company_b=$(register_company "concurrent-team-b" "http://127.0.0.1:${WORKER2_PORT}" "$agent_type" "concurrent-b")

    assert_not_empty "Company A ID" "$company_a"
    assert_not_empty "Company B ID" "$company_b"

    # 同时下发 2 个 Goal
    info "并发下发 2 个联邦 Goal..."

    local fed_alpha
    fed_alpha=$(cat <<EOF
{"name":"S6-并发Goal-Alpha","description":"并发测试 Alpha","projects":[{"name":"task-alpha-1","companyId":"${company_a}","agent":"concurrent-a","description":"请回答: 1+1 等于几？只回答数字。"},{"name":"task-alpha-2","companyId":"${company_b}","agent":"concurrent-b","description":"请回答: 2+2 等于几？只回答数字。","dependencies":["task-alpha-1"]}]}
EOF
    )
    split_response "$(http_post "${MASTER_URL}/goals/federated" "$fed_alpha")"
    local goal_alpha
    goal_alpha=$(jq_field "$RESP_BODY" ".get('goalId','')")
    assert_not_empty "Goal Alpha ID" "$goal_alpha"

    local fed_beta
    fed_beta=$(cat <<EOF
{"name":"S6-并发Goal-Beta","description":"并发测试 Beta","projects":[{"name":"task-beta-1","companyId":"${company_b}","agent":"concurrent-b","description":"请回答: 3+3 等于几？只回答数字。"},{"name":"task-beta-2","companyId":"${company_a}","agent":"concurrent-a","description":"请回答: 4+4 等于几？只回答数字。","dependencies":["task-beta-1"]}]}
EOF
    )
    split_response "$(http_post "${MASTER_URL}/goals/federated" "$fed_beta")"
    local goal_beta
    goal_beta=$(jq_field "$RESP_BODY" ".get('goalId','')")
    assert_not_empty "Goal Beta ID" "$goal_beta"

    info "  Alpha: ${goal_alpha}"
    info "  Beta:  ${goal_beta}"

    # 并行等待两个 Goal
    local alpha_ok=false beta_ok=false
    local deadline=$((SECONDS + FEDERATED_TIMEOUT))

    while [ $SECONDS -lt $deadline ]; do
        if [ "$alpha_ok" != true ]; then
            split_response "$(http_get "${MASTER_URL}/goals/${goal_alpha}")"
            local a_status a_phase
            a_status=$(jq_field "$RESP_BODY" ".get('status','')")
            a_phase=$(jq_field "$RESP_BODY" ".get('phase','')")
            if [ "$a_status" = "completed" ] || [ "$a_phase" = "completed" ]; then
                alpha_ok=true; info "  Alpha → completed ✓"
            elif [ "$a_status" = "failed" ] || [ "$a_phase" = "failed" ]; then
                info "  Alpha → failed ✗"; break
            fi
        fi

        if [ "$beta_ok" != true ]; then
            split_response "$(http_get "${MASTER_URL}/goals/${goal_beta}")"
            local b_status b_phase
            b_status=$(jq_field "$RESP_BODY" ".get('status','')")
            b_phase=$(jq_field "$RESP_BODY" ".get('phase','')")
            if [ "$b_status" = "completed" ] || [ "$b_phase" = "completed" ]; then
                beta_ok=true; info "  Beta  → completed ✓"
            elif [ "$b_status" = "failed" ] || [ "$b_phase" = "failed" ]; then
                info "  Beta  → failed ✗"; break
            fi
        fi

        if [ "$alpha_ok" = true ] && [ "$beta_ok" = true ]; then break; fi

        if (( SECONDS % 15 == 0 )); then
            info "  ... alpha=${alpha_ok} beta=${beta_ok} (${SECONDS}s)"
        fi
        sleep 5
    done

    TOTAL=$((TOTAL + 1))
    if [ "$alpha_ok" = true ] && [ "$beta_ok" = true ]; then
        echo -e "  ${GREEN}✓ PASS${NC}: Both concurrent goals completed"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗ FAIL${NC}: Concurrent goals did not both complete (alpha=${alpha_ok} beta=${beta_ok})"
        FAIL=$((FAIL + 1))
    fi

    # 验证：两个 Goal 的 task 没有交叉污染
    split_response "$(http_get "${MASTER_URL}/goals/${goal_alpha}")"
    local alpha_name
    alpha_name=$(jq_field "$RESP_BODY" ".get('name','')")
    assert_eq "Alpha goal name intact" "S6-并发Goal-Alpha" "$alpha_name"

    split_response "$(http_get "${MASTER_URL}/goals/${goal_beta}")"
    local beta_name
    beta_name=$(jq_field "$RESP_BODY" ".get('name','')")
    assert_eq "Beta goal name intact" "S6-并发Goal-Beta" "$beta_name"

    # 清理
    cleanup_companies
    delete_agent "$WORKER1_URL" "concurrent-a"
    delete_agent "$WORKER2_URL" "concurrent-b"

    if [ $FAIL -eq $s6_fail ]; then
        record_scenario 6 "Concurrent Goals" "PASS"
    else
        record_scenario 6 "Concurrent Goals" "FAIL"
    fi
}

# ─── 测试报告 ─────────────────────────────────────────────────────────────────
print_report() {
    echo ""
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║         Integration Test Report               ║${NC}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════╝${NC}"
    echo ""

    for result in "${SCENARIO_RESULTS[@]}"; do
        echo -e "  $result"
    done

    echo ""
    echo -e "  ──────────────────────────────"
    echo -e "  ${GREEN}PASS${NC}: ${PASS}    ${RED}FAIL${NC}: ${FAIL}    ${YELLOW}SKIP${NC}: ${SKIP}    Total: ${TOTAL}"
    echo ""

    if [ $FAIL -eq 0 ]; then
        echo -e "  ${GREEN}${BOLD}ALL TESTS PASSED ✓${NC}"
    else
        echo -e "  ${RED}${BOLD}SOME TESTS FAILED ✗${NC}"
    fi

    echo ""
    echo -e "  日志目录: ${LOG_DIR}"
    echo -e "  Master:   tail -f ${LOG_DIR}/master.log"
    echo -e "  Worker1:  tail -f ${LOG_DIR}/worker1.log"
    echo -e "  Worker2:  tail -f ${LOG_DIR}/worker2.log"
    echo ""
}

# ─── 主流程 ───────────────────────────────────────────────────────────────────
main() {
    echo ""
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║  OPC Multi-Adapter Integration Test          ║${NC}"
    echo -e "${BOLD}${CYAN}║  方案 C: 场景驱动 (6 Scenarios)               ║${NC}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════╝${NC}"
    echo ""

    preflight_check
    build_opctl
    start_instances

    local start_time=$SECONDS

    # 执行场景
    should_run 1 && run_scenario_1
    should_run 2 && run_scenario_2
    should_run 3 && run_scenario_3
    should_run 4 && run_scenario_4
    should_run 5 && run_scenario_5
    should_run 6 && run_scenario_6

    local elapsed=$((SECONDS - start_time))
    echo ""
    log "全部场景执行完毕 (耗时 ${elapsed}s)"

    # 停止实例
    log "停止所有 OPC 实例..."
    stop_all

    print_report

    # 退出码
    if [ $FAIL -gt 0 ]; then exit 1; fi
}

main
