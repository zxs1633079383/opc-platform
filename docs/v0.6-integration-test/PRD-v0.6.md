# OPC Platform PRD v0.6 — 多 Adapter 统一调度 & 联邦协同

**版本**: v0.6
**日期**: 2026-03-19
**状态**: 已验证 (ALL 6 SCENARIOS PASSED)
**前置版本**: v0.5 (Federation + Trace)

---

## 一、版本定位

> v0.6 是 OPC Platform 从 "概念验证" 到 "真实可用" 的关键版本。
> 核心交付：**统一调度 Claude Code + OpenClaw，跨联邦 DAG 协同，任务级成本追踪。**

### 1.1 v0.6 新增能力

| 能力 | 描述 | 验证状态 |
|------|------|----------|
| 多 Adapter 统一调度 | 同一 API 管理 Claude Code / OpenClaw / 自定义 Agent | ✅ 已验证 |
| OpenClaw WebSocket RPC v3 | 完整握手 + 异步执行 + 结果提取 | ✅ 已验证 |
| 混合编排 | 同一 DAG 中混合使用不同类型 Adapter | ✅ 已验证 |
| 联邦 DAG 自动分层 | 拓扑排序 → 分层 → 依赖驱动执行 | ✅ 已验证 |
| A2A 结果评审 | Goal Driver 自动评审执行质量，不满意重试 | ✅ 已验证 |
| 任务级成本持久化 | token/cost 写入 DB，Goal/Project/Task 全链路 | ✅ 已验证 |
| 并发 Goal 隔离 | 多个联邦 Goal 同时执行互不干扰 | ✅ 已验证 |

### 1.2 里程碑进度

```
Phase 1: Foundation (CLI/Storage/Controller)          ✅ v0.1
Phase 2: Workflow + Cost + Audit                      ✅ v0.2
Phase 3: Dashboard + Multi-Company                    ✅ v0.3
Phase 4: Federation + Trace Propagation               ✅ v0.5
Phase 5: Multi-Adapter + Real Integration Test   ←── ✅ v0.6 (本版本)
Phase 6: Production Hardening                         🔲 v0.7 (下一步)
```

---

## 二、架构概览

### 2.1 系统架构

```
                          ┌──────────────────────┐
                          │    Dashboard :4000    │
                          │  (Next.js + React)    │
                          └──────────┬───────────┘
                                     │ REST API
              ┌──────────────────────▼───────────────────────┐
              │              OPC Master (:9527)                │
              │  ┌─────────┐  ┌──────────┐  ┌─────────────┐  │
              │  │ Goal     │  │Federation│  │ Cost        │  │
              │  │ Engine   │  │Controller│  │ Tracker     │  │
              │  │ (DAG)    │  │ (Heartbeat│  │ (per-task)  │  │
              │  └─────────┘  └──────────┘  └─────────────┘  │
              │  ┌─────────────────────────────────────────┐  │
              │  │         Controller (Lifecycle)           │  │
              │  │  Apply → Start → Execute → Stop → Delete │  │
              │  └──────────────┬──────────────────────────┘  │
              └─────────────────┼──────────────────────────────┘
                                │
               ┌────────────────┼────────────────┐
               │                │                │
    ┌──────────▼─────┐  ┌──────▼──────┐  ┌──────▼──────┐
    │ Claude Code    │  │ OpenClaw    │  │ Custom      │
    │ Adapter        │  │ Adapter     │  │ Adapter     │
    │                │  │             │  │             │
    │ • claude CLI   │  │ • WS RPC v3 │  │ • stdin/out │
    │ • JSON output  │  │ • ed25519   │  │ • JSONL     │
    │ • per-process  │  │ • persistent│  │ • per-proc  │
    └───────┬────────┘  └──────┬──────┘  └──────┬──────┘
            │                  │                │
    ┌───────▼────────┐  ┌──────▼──────┐  ┌──────▼──────┐
    │ Claude API     │  │ OpenClaw GW │  │ Any Process │
    │ (Anthropic)    │  │ (:18789)    │  │             │
    └────────────────┘  └─────────────┘  └─────────────┘
```

### 2.2 联邦架构

```
                    ┌──────────────────────────┐
                    │     Master OPC (:9527)    │
                    │  • 联邦注册中心            │
                    │  • Goal DAG 编排           │
                    │  • A2A 评审 (Goal Driver)  │
                    │  • Callback 接收           │
                    └───────┬──────────┬────────┘
                            │          │
            Heartbeat ◄─────┤          ├─────► Heartbeat
                            │          │
                 ┌──────────▼──┐  ┌────▼──────────┐
                 │ Worker1     │  │ Worker2        │
                 │ :9528       │  │ :9529          │
                 │ CC + OC     │  │ CC + OC        │
                 │ Agents      │  │ Agents         │
                 └─────────────┘  └────────────────┘
```

### 2.3 数据模型

```
Goal (战略目标)
 ├── status: active → in_progress → completed/failed
 ├── phase: GoalPhase 枚举
 ├── tokens_in / tokens_out / cost  ← v0.6 新增持久化
 │
 └── Project (项目)
      ├── company_id (联邦: 目标公司)
      ├── dependencies: ["other-project"]  ← DAG 依赖
      │
      └── Issue (工单)
           └── Task (执行单元)
                ├── agent_name
                ├── status: Pending → Running → Completed/Failed
                ├── tokens_in / tokens_out / cost
                ├── lineage_json  ← 追溯链
                └── result / error
```

---

## 三、核心功能详述

### 3.1 多 Adapter 统一调度

**问题**: 不同 AI Agent 有不同的通信协议和生命周期。

**解决方案**: Adapter 接口抽象 + 注册表模式。

```go
type Adapter interface {
    Type() AgentType
    Start(ctx, spec) error
    Stop(ctx) error
    Execute(ctx, task) (ExecuteResult, error)
    Stream(ctx, task) (<-chan Chunk, error)
    Health() HealthStatus
    Status() AgentPhase
    Metrics() AgentMetrics
}
```

**已实现 Adapter**:

| Adapter | 通信方式 | 连接模型 | Token 追踪 |
|---------|---------|---------|-----------|
| claude-code | CLI 进程 spawn | 非持久 (per-task) | ✅ 完整 |
| openclaw | WebSocket RPC v3 | 持久连接 | ⚠️ 部分 |
| codex | CLI 进程 spawn | 非持久 | 🔲 待实现 |
| custom | stdin/stdout | 非持久 | 🔲 待实现 |

**API — 注册 Agent**:

```bash
# Claude Code Agent
opctl apply -f agent-claude-code.yaml

# OpenClaw Agent
opctl apply -f agent-openclaw.yaml

# 或 REST API
POST /api/apply
Content-Type: application/x-yaml
---
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: my-coder
spec:
  type: openclaw  # 或 claude-code
  runtime:
    model: { provider: anthropic, name: claude-sonnet-4 }
    timeout: { task: 300s }
```

### 3.2 Goal 自动分解与执行

**问题**: 用户只想描述目标，不想手动拆分任务。

**解决方案**: `autoDecompose` 模式自动创建 Project → Issue → Task → 调度执行。

```bash
# 创建目标，系统自动拆分并执行
POST /api/goals
{
  "name": "实现用户认证模块",
  "description": "设计并实现 JWT 认证，包括登录/注册/刷新 Token",
  "autoDecompose": true
}

# 返回 202 Accepted
# 系统自动: 创建 Project → 创建 Issue → 创建 Task → 启动 Agent → 执行
```

**Goal 生命周期**:
```
active → in_progress → completed
                    → failed
```

### 3.3 联邦 DAG 编排

**问题**: 复杂任务需要多个团队/Agent 按依赖顺序协作。

**解决方案**: 联邦公司注册 + DAG 拓扑排序 + 层级并行执行。

```bash
# 1. 注册联邦公司
POST /api/federation/companies
{ "name": "design-team", "endpoint": "http://worker1:9528", "agents": ["designer"] }
{ "name": "dev-team",    "endpoint": "http://worker2:9529", "agents": ["coder"] }

# 2. 创建联邦 Goal (带 DAG 依赖)
POST /api/goals/federated
{
  "name": "实现登录功能",
  "projects": [
    { "name": "ui-design",  "companyId": "xxx", "agent": "designer" },
    { "name": "api-spec",   "companyId": "yyy", "agent": "coder",
      "dependencies": ["ui-design"] },
    { "name": "frontend",   "companyId": "yyy", "agent": "coder",
      "dependencies": ["ui-design", "api-spec"] },
    { "name": "backend",    "companyId": "yyy", "agent": "coder",
      "dependencies": ["api-spec"] }
  ]
}

# DAG 自动分层:
# Layer 0: ui-design                ← 立即执行
# Layer 1: api-spec                 ← 等 ui-design 完成
# Layer 2: frontend + backend       ← 并行执行
```

**DAG 执行引擎**:
- `ValidateProjectDAG()`: 检测循环依赖
- `BuildProjectLayers()`: 拓扑排序 → 分层
- 每层完成后自动触发下一层
- 上游结果自动注入下游 Agent 的 Prompt

### 3.4 A2A 结果评审 (Goal Driver)

**问题**: Agent 可能返回空结果或低质量输出。

**解决方案**: Goal Driver 自动评审每个 Project 的执行结果，不满意时自动重试。

```
Project 执行完成
     ↓
Goal Driver 评审 (Claude Code)
     ↓
满意 → 标记完成, 触发下一层
不满意 → 生成 follow-up, 重新 dispatch (max 3轮)
     ↓
超过 3 轮 → 接受当前结果, 继续执行
```

**评审日志**:
```json
{
  "goalId": "goal-xxx",
  "project": "api-spec",
  "round": 2,
  "reason": "result is empty — agent produced no output",
  "action": "re-dispatch"
}
```

### 3.5 成本追踪 (全链路)

**问题**: 多 Agent 协作时成本不透明。

**解决方案**: Task 级 token/cost 记录 → Project 聚合 → Goal 聚合。

```
Task 执行完成
  → 记录 tokens_in, tokens_out, cost
  → 生成 CostEvent (持久化到磁盘)
  → 聚合到 Goal.tokens_in/tokens_out/cost

# 查询 API
GET /api/goals/:id         → 包含 tokensIn, tokensOut, cost
GET /api/goals/:id/stats   → 完整成本统计
GET /api/costs/events      → 明细事件列表
GET /api/costs/daily       → 日报
```

### 3.6 审计追溯

**问题**: 需要知道每个任务的执行链路和结果。

**已实现的审计点**:

| 审计点 | 记录内容 |
|--------|---------|
| Agent 生命周期 | apply, start, stop, delete, failed |
| Task 执行 | taskId, agentName, duration, tokens, cost, result |
| Goal 状态 | goalId, phase 流转, 完成时间 |
| 联邦 Callback | goalId, projectId, taskId, status, tokens, cost |
| A2A 评审 | round, reason, re-dispatch/accept |
| Cost Event | agent, totalCost, totalTokens |

---

## 四、API 参考 (v0.6 新增/修改)

### 4.1 Agent 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/apply` | 声明式注册 Agent (YAML) |
| POST | `/api/agents/:name/start` | 启动 Agent |
| POST | `/api/agents/:name/stop` | 停止 Agent |
| DELETE | `/api/agents/:name` | 删除 Agent |
| GET | `/api/agents` | 列出所有 Agent |

### 4.2 Goal 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/goals` | 创建 Goal (支持 autoDecompose) |
| GET | `/api/goals/:id` | 查询 Goal (含 status, tokens, cost) |
| POST | `/api/goals/federated` | 创建联邦 Goal (DAG) |
| GET | `/api/goals/:id/stats` | Goal 成本统计 |

### 4.3 联邦

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/federation/companies` | 注册联邦公司 |
| GET | `/api/federation/companies` | 列出公司 |
| POST | `/api/federation/callback` | 接收执行结果回调 |
| GET | `/api/federation/aggregate/metrics` | 聚合指标 |

### 4.4 成本

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/costs/events` | 成本事件明细 |
| GET | `/api/costs/daily` | 每日成本报告 |
| GET | `/api/metrics` | 集群指标 |

---

## 五、已知限制与 v0.7 规划

### 5.1 当前限制

| # | 限制 | 优先级 | 计划版本 |
|---|------|--------|---------|
| 1 | OpenClaw `agent.wait` 不返回 token 统计 | P1 | v0.7 |
| 2 | 联邦 Goal 状态仅内存, 重启后丢失 | P1 | v0.7 |
| 3 | A2A 评审对空结果反复重试浪费 token | P2 | v0.7 |
| 4 | 单 OpenClaw Gateway 无连接池 | P2 | v0.8 |
| 5 | 无 Agent 级别的权限控制 | P2 | v0.8 |
| 6 | 缺少 Webhook/Slack 通知 | P3 | v0.8 |

### 5.2 v0.7 规划: Production Hardening

| 特性 | 说明 |
|------|------|
| 联邦 Goal 持久化 | FederatedGoalRun 写入 DB, 支持重启恢复 |
| OpenClaw token 补全 | 从 session 日志或 snapshot 提取 token 数据 |
| 智能重试策略 | 区分空结果/执行错误, 空结果直接通过不浪费评审 |
| Agent 健康检查增强 | 定期探测, 不健康自动重启 |
| 配额执行 | tokenBudget/costLimit 真实生效, 超限暂停/告警 |
| E2E 测试 CI | integration_multi_adapter.sh 集成到 CI pipeline |

---

## 六、竞品对比

| 特性 | OPC Platform v0.6 | CrewAI | AutoGen | LangGraph |
|------|-------------------|--------|---------|-----------|
| 多 Adapter 统一管理 | ✅ CC+OC+Custom | ❌ 自有 Agent | ❌ 自有 Agent | ❌ 自有 Agent |
| 声明式 YAML 配置 | ✅ K8s 风格 | ❌ Python 代码 | ❌ Python 代码 | ❌ Python 代码 |
| 联邦跨实例协同 | ✅ 多 OPC 节点 | ❌ 单进程 | ❌ 单进程 | ❌ 单进程 |
| DAG 依赖编排 | ✅ 拓扑排序 | ✅ Sequential/Parallel | ⚠️ 基础 | ✅ 图结构 |
| 任务级成本追踪 | ✅ token+cost/task | ⚠️ 有限 | ❌ | ❌ |
| CLI 原生 | ✅ opctl | ❌ | ❌ | ❌ |
| A2A 自动评审 | ✅ Goal Driver | ❌ | ❌ | ❌ |
| Dashboard | ✅ Web UI | ❌ | ❌ | ❌ |
| 分布式 Trace | ✅ OpenTelemetry | ❌ | ❌ | ❌ |

**OPC Platform 的独特定位**: 不是另一个 Agent Framework，而是管理**任意 AI Agent** 的基础设施层 — Agent 的 Kubernetes。

---

## 七、版本发布检查清单

- [x] S1: Local Goal × Claude Code — 通过
- [x] S2: Local Goal × OpenClaw — 通过
- [x] S3: Federated Goal × 全 CC (3层 DAG) — 通过
- [x] S4: Federated Goal × 全 OC (3层 DAG) — 通过
- [x] S5: Federated Goal × 混合 CC+OC (3层 DAG) — 通过
- [x] S6: 并发 Federated Goal × 2 — 通过
- [x] UpdateGoal SQL 修复 (status/tokens/cost 持久化)
- [x] GetGoal/ListGoals SQL 修复 (读取全字段)
- [x] OpenClaw adapter payload 字段修复
- [x] Dashboard 端口迁移 (3000 → 4000)
- [ ] CI 集成 (后续)
- [ ] Tag v0.6 (待确认)
