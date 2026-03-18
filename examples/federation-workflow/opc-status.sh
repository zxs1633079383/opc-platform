#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# OPC Federation Status — 查看联邦全局状态
#
# 用法:
#   bash opc-status.sh                  # 总览：公司 + 各节点 task
#   bash opc-status.sh tasks            # 聚合所有节点的 tasks
#   bash opc-status.sh goal <goalId>    # 查看某个 goal 的完整执行链
#   bash opc-status.sh trace <taskId> <port>  # 查看某个 task 的血缘链
###############################################################################

MASTER="${OPC_MASTER:-http://localhost:9527}"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

# ─── helpers ──────────────────────────────────────────────────────────────────
py() { python3 -c "$1" 2>/dev/null; }

# ─── overview ─────────────────────────────────────────────────────────────────
show_overview() {
    echo -e "${BOLD}=== OPC 联邦状态总览 ===${NC}"
    echo ""

    # Companies
    COMPANIES=$(curl -s "${MASTER}/api/federation/companies")
    if [ -z "$COMPANIES" ] || [ "$COMPANIES" = "null" ]; then
        echo -e "${RED}无法连接 Master ${MASTER}${NC}"
        exit 1
    fi

    echo -e "${BOLD}联邦公司:${NC}"
    echo "$COMPANIES" | py "
import sys, json
companies = json.load(sys.stdin)
for c in companies:
    status_icon = '🟢' if c['status'] == 'Online' else '🔴'
    agents = ', '.join(c.get('agents', []) or ['(none)'])
    print(f\"  {status_icon} {c['name']:20s} {c['endpoint']:30s} agents: [{agents}]\")
print(f\"\n  共 {len(companies)} 个节点\")
"

    # Aggregated metrics
    echo ""
    echo -e "${BOLD}联邦聚合指标:${NC}"
    METRICS=$(curl -s "${MASTER}/api/federation/aggregate/metrics" 2>/dev/null || echo "{}")
    echo "$METRICS" | py "
import sys, json
m = json.load(sys.stdin)
print(f\"  Agents: {m.get('totalAgents',0)} (运行中: {m.get('runningAgents',0)})\")
print(f\"  Tasks:  {m.get('totalTasks',0)} (完成: {m.get('completedTasks',0)}, 失败: {m.get('failedTasks',0)})\")
print(f\"  Cost:   \${m.get('totalCost',0):.4f}\")
print(f\"  节点数: {m.get('companyCount',0)}\")
"

    # Per-company tasks
    echo ""
    echo -e "${BOLD}各节点 Tasks:${NC}"
    echo "$COMPANIES" | py "
import sys, json, urllib.request
companies = json.load(sys.stdin)
for c in companies:
    cid = c['id']
    name = c['name']
    try:
        resp = urllib.request.urlopen(f\"${MASTER}/api/federation/companies/{cid}/tasks\", timeout=3)
        tasks = json.loads(resp.read())
        if not tasks:
            print(f\"  {name:20s} (无 tasks)\")
            continue
        print(f\"  {name}:\")
        for t in (tasks if isinstance(tasks, list) else []):
            status_icon = {'Completed': '✅', 'Failed': '❌', 'Running': '🔄', 'Pending': '⏳'}.get(t.get('status','?'), '❓')
            goal_id = t.get('goalId', '-')[:12]
            proj_id = t.get('projectId', '-')[:20]
            print(f\"    {status_icon} {t['id'][:16]:16s}  agent={t.get('agentName','?'):12s}  goal={goal_id}  project={proj_id}  {t.get('status','?')}\")
    except Exception as e:
        print(f\"  {name:20s} (无法连接: {e})\")
"

    # Federated goal runs (from master's local state via goals endpoint)
    echo ""
    echo -e "${BOLD}Master Goals:${NC}"
    GOALS=$(curl -s "${MASTER}/api/goals" 2>/dev/null || echo "[]")
    echo "$GOALS" | py "
import sys, json
goals = json.load(sys.stdin)
if not goals:
    print('  (无 goals)')
else:
    for g in goals:
        print(f\"  {g['id'][:16]:16s}  {g.get('name','?'):30s}  status={g.get('status','?')}  phase={g.get('phase','?')}\")
"
}

# ─── tasks (aggregated) ──────────────────────────────────────────────────────
show_tasks() {
    echo -e "${BOLD}=== 联邦 Tasks 汇总 ===${NC}"
    echo ""

    COMPANIES=$(curl -s "${MASTER}/api/federation/companies")
    echo "$COMPANIES" | py "
import sys, json, urllib.request

companies = json.load(sys.stdin)
all_tasks = []

for c in companies:
    cid, name, endpoint = c['id'], c['name'], c['endpoint']
    try:
        resp = urllib.request.urlopen(f\"${MASTER}/api/federation/companies/{cid}/tasks\", timeout=3)
        tasks = json.loads(resp.read())
        if isinstance(tasks, list):
            for t in tasks:
                t['_node'] = name
                t['_endpoint'] = endpoint
                all_tasks.append(t)
    except:
        pass

if not all_tasks:
    print('  (无 tasks)')
    sys.exit(0)

# Sort by createdAt
all_tasks.sort(key=lambda t: t.get('createdAt', ''))

# Group by goal
by_goal = {}
for t in all_tasks:
    gid = t.get('goalId', 'no-goal')
    by_goal.setdefault(gid, []).append(t)

for goal_id, tasks in by_goal.items():
    print(f'Goal: {goal_id}')
    for t in tasks:
        status_icon = {'Completed': '✅', 'Failed': '❌', 'Running': '🔄', 'Pending': '⏳'}.get(t.get('status','?'), '❓')
        node = t.get('_node', '?')
        agent = t.get('agentName', '?')
        proj = t.get('projectId', '-')
        # Extract project name from projectId (format: proj-goalXXX-projectName)
        proj_short = proj.split('-', 2)[-1] if '-' in proj else proj
        lineage = t.get('lineage', '[]')
        has_lineage = lineage not in ('', '[]', None)
        lineage_mark = ' 🔗' if has_lineage else ''
        print(f'  {status_icon} [{node:15s}] task={t[\"id\"][:16]:16s} agent={agent:12s} project={proj_short:20s}{lineage_mark}')
        if t.get('error'):
            print(f'     ↳ error: {t[\"error\"][:80]}')
    print()
"
}

# ─── goal detail ──────────────────────────────────────────────────────────────
show_goal() {
    local goal_id="${1:-}"
    if [ -z "$goal_id" ]; then
        echo "用法: bash opc-status.sh goal <goalId>"
        exit 1
    fi

    echo -e "${BOLD}=== Goal 详情: ${goal_id} ===${NC}"
    echo ""

    # Collect all tasks across nodes for this goal
    COMPANIES=$(curl -s "${MASTER}/api/federation/companies")
    echo "$COMPANIES" | py "
import sys, json, urllib.request

goal_id = '${goal_id}'
companies = json.load(sys.stdin)
all_tasks = []

for c in companies:
    cid, name = c['id'], c['name']
    try:
        resp = urllib.request.urlopen(f\"${MASTER}/api/federation/companies/{cid}/tasks\", timeout=3)
        tasks = json.loads(resp.read())
        if isinstance(tasks, list):
            for t in tasks:
                if t.get('goalId', '') == goal_id or goal_id in t.get('goalId', ''):
                    t['_node'] = name
                    all_tasks.append(t)
    except:
        pass

if not all_tasks:
    print(f'  Goal {goal_id} 无关联 tasks（可能 ID 不完整，试试部分匹配）')
    sys.exit(0)

# Group by project
by_project = {}
for t in all_tasks:
    pid = t.get('projectId', 'unknown')
    by_project.setdefault(pid, []).append(t)

print(f'Goal ID:    {goal_id}')
print(f'Projects:   {len(by_project)}')
print(f'Tasks:      {len(all_tasks)}')
print()

for proj_id, tasks in by_project.items():
    proj_name = proj_id.split('-', 2)[-1] if '-' in proj_id else proj_id
    print(f'📦 Project: {proj_name}')
    print(f'   ID: {proj_id}')
    for t in tasks:
        status_icon = {'Completed': '✅', 'Failed': '❌', 'Running': '🔄', 'Pending': '⏳'}.get(t.get('status','?'), '❓')
        node = t.get('_node', '?')
        print(f'   {status_icon} Task {t[\"id\"]}')
        print(f'      Node:    {node}')
        print(f'      Agent:   {t.get(\"agentName\", \"?\")}')
        print(f'      Status:  {t.get(\"status\", \"?\")}')
        if t.get('error'):
            print(f'      Error:   {t[\"error\"][:100]}')
        lineage = t.get('lineage', '[]')
        if lineage and lineage not in ('[]', ''):
            print(f'      Lineage: {lineage}')
        if t.get('result'):
            result_preview = t['result'][:100].replace(chr(10), ' ')
            print(f'      Result:  {result_preview}...')
    print()
"
}

# ─── trace ────────────────────────────────────────────────────────────────────
show_trace() {
    local task_id="${1:-}"
    local port="${2:-9528}"
    if [ -z "$task_id" ]; then
        echo "用法: bash opc-status.sh trace <taskId> [port]"
        echo "  port 默认 9528 (design 节点)"
        exit 1
    fi

    echo -e "${BOLD}=== Task 血缘追踪: ${task_id} ===${NC}"
    echo ""

    TASK=$(curl -s "http://localhost:${port}/api/tasks/${task_id}")
    echo "$TASK" | py "
import sys, json
t = json.load(sys.stdin)

print(f'Task ID:    {t.get(\"id\", \"?\")}')
print(f'Agent:      {t.get(\"agentName\", \"?\")}')
print(f'Status:     {t.get(\"status\", \"?\")}')
print(f'Goal ID:    {t.get(\"goalId\", \"-\")}')
print(f'Project ID: {t.get(\"projectId\", \"-\")}')
print()

lineage_raw = t.get('lineage', '[]')
if lineage_raw and lineage_raw not in ('[]', ''):
    try:
        lineage = json.loads(lineage_raw) if isinstance(lineage_raw, str) else lineage_raw
        print('🔗 血缘链 (Lineage):')
        for i, ref in enumerate(lineage):
            prefix = '  └─' if i == len(lineage) - 1 else '  ├─'
            print(f'{prefix} [{ref.get(\"opcNode\",\"?\")}] {ref.get(\"projectName\",\"?\")} → {ref.get(\"label\",\"?\")}')
            print(f'     goalId={ref.get(\"goalId\",\"-\")}  issueId={ref.get(\"issueId\",\"-\")}')
    except:
        print(f'Lineage (raw): {lineage_raw}')
else:
    print('🔗 血缘链: (根节点，无上游依赖)')

if t.get('error'):
    print(f'\n❌ Error: {t[\"error\"]}')
if t.get('result'):
    print(f'\n📄 Result (前200字):')
    print(f'  {t[\"result\"][:200]}')
"
}

# ─── main ─────────────────────────────────────────────────────────────────────
case "${1:-}" in
    tasks)   show_tasks ;;
    goal)    show_goal "${2:-}" ;;
    trace)   show_trace "${2:-}" "${3:-9528}" ;;
    *)       show_overview ;;
esac
