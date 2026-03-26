# OPC Platform — 产品需求文档 v0.7

> **"AI Agent 的 Kubernetes — 让一个人管理数百个 AI Agent"**

**版本**: v0.7
**日期**: 2026-03-26
**基线 commit**: `82e7ad5` (feature/prd-entropy-alignment)
**状态**: 对齐文档（代码领先 MD，本文是权威真相）

---

## 目录

1. [产品定位](#一产品定位)
2. [目标用户](#二目标用户)
3. [核心概念模型](#三核心概念模型)
4. [完整 Use Case 目录](#四完整-use-case-目录)
5. [当前实现状态（v0.6 基线）](#五当前实现状态v06-基线)
6. [v0.7 新增需求](#六v07-新增需求production-hardening--self-evolving-loop)
7. [系统架构](#七系统架构)
8. [API 参考](#八api-参考)
9. [数据模型](#九数据模型)
10. [风险登记](#十风险登记)
11. [里程碑](#十一里程碑)

---

## 一、产品定位

### 1.1 一句话定位

> **OPC Platform 是 AI Agent 的 Kubernetes** — 就像 K8s 让一个 SRE 管理数千个容器，OPC 让一个人管理数百个 AI Agent。

### 1.2 K8s 类比

| K8s 世界 | OPC 世界 |
|----------|----------|
| Docker 容器 | AI Agent（OpenClaw / Claude Code / Codex / Custom） |
| Kubernetes | OPC Platform |
| Pod | Agent Instance（运行中的实例） |
| Deployment | AgentSpec（声明式配置） |
| Service | Agent Endpoint |
| Ingress | Gateway（Telegram / Discord / CLI） |
| kubectl | opctl（命令行工具） |
| Namespace | Company（多租户隔离） |
| Job / CronJob | Task / Workflow |
| HPA | AutoScaler |

### 1.3 核心价值主张

```
用户只需声明目标（Goal）：
  "实现用户认证模块"

OPC 自动做：
  1. AI 分解 → Goal → Project → Task → Issue
  2. DAG 拓扑排序 → 分层并行调度 Agent
  3. 执行 → A2A 评审质量 → 不满意自动重试
  4. 全链路成本追踪（token / cost / 时间）
  5. 跨联邦分布式协同执行
  6. 自愈 → 指标采集 → AI 分析 → 闭环改进（v0.8）
```

### 1.4 差异化

| 竞品 | OPC 优势 |
|------|----------|
| LangChain / CrewAI | OPC 不绑定框架，管理任意外部 Agent 进程 |
| Kubernetes | OPC 理解 AI 语义（Goal 自动分解、A2A 评审） |
| n8n / Zapier | OPC 支持长时任务、代码执行、记忆恢复 |
| AutoGPT | OPC 有完整的成本控制、审计追溯、多租户 |

---

## 二、目标用户

### 2.1 用户画像

| 画像 | 描述 | 核心痛点 | 关键需求 |
|------|------|----------|----------|
| **Solo Founder** | 一人公司，用 AI 替代团队 | 同时管理 5-10 个 Agent，无法监控成本 | 声明式配置 + 成本预警 + 自动重启 |
| **AI Engineer** | 构建 Agent 工作流的开发者 | 编排复杂 DAG，需要可编程接口 | Workflow API + Federation + Tracing |
| **Tech Lead** | 小团队技术负责人 | 团队共享 Agent 资源，需要权限隔离 | 多租户 + 配额执行 + 审计日志 |
| **DevOps / Platform Eng** | 管理 AI 基础设施 | 无统一运维入口，Agent 崩溃无告警 | 健康检查 + 自动重启 + CI 集成 |

### 2.2 用户旅程

```
发现 OPC → 安装 opctl → 注册第一个 Agent → 执行第一个 Task
→ 创建第一个 Goal（AI 自动分解）→ 设置预算告警
→ 建立 Workflow（定时 / 触发）→ 接入 Telegram 频道
→ 查看成本报表 → 多节点联邦协同
→ 系统自主发现问题并改进（v0.8）
```

---

## 三、核心概念模型

### 3.1 实体层级

```
OPC Platform
│
├── Company（多租户单元）
│   ├── Agent（可执行 AI 任务的最小单元）
│   │   └── AgentSpec（声明式配置：类型 / 模型 / 副本 / 配额）
│   │
│   ├── Goal（战略目标）
│   │   └── Project（子项目）
│   │       └── Task（任务）
│   │           └── Issue（执行单元）
│   │
│   ├── Workflow（DAG 工作流）
│   │   └── WorkflowRun（执行实例）
│   │
│   ├── CostEvent（成本事件，task 级别）
│   └── AuditLog（审计日志）
│
└── Federation（跨节点协同）
    ├── FederatedGoalRun（跨节点 Goal 执行）
    └── FederatedProject（远程节点执行的 Project）
```

### 3.2 Agent 生命周期

```
Created ──► Starting ──► Running ──► Completing ──► Completed
                │            │
                ▼            ▼
             Failed ──► Retrying ──► Running
                │
                ▼
           Terminated
```

### 3.3 Goal 执行流程（完整）

```
用户创建 Goal（autoDecompose=true）
    │
    ▼
AI Decomposer（claude 模型）
    │  输出 JSON: {projects, tasks, issues, dependencies}
    ▼
DecompositionPlan 持久化
    │
    ▼  [autoApprove=false]     [autoApprove=true]
Plan 审查 ◄────────────────── or ──────────────► 直接执行
    │ opctl goal approve <id>                        │
    └──────────────────────────────────────────────►
                                                     ▼
                                            DAG 拓扑排序
                                                     │
                                            分层并行调度（Layer 0, 1, 2...）
                                                     │
                                     ┌───────────────┼───────────────┐
                                     ▼               ▼               ▼
                               Agent A           Agent B         Agent C
                               Execute()        Execute()       Execute()
                                     │               │               │
                                     └───────────────┼───────────────┘
                                                     ▼
                                            A2A 评审（Assessor）
                                         Satisfied? ──► 完成
                                         QualityIssue? ──► 生成 follow-up → 重试（max 3x）
                                         ExecutionError? ──► 重试（max 2x）
                                         EmptyResult? ──► 检查是否验证类 → pass / 重试 1x
                                                     │
                                                     ▼
                                            成本持久化（CostEvent）
                                                     │
                                                     ▼
                                            Goal 完成 / callback
```

### 3.4 联邦协同模型

```
Master OPC (:9527)
    │
    │ POST /api/goals/federated
    │ {projects: [{name, agent, company, dependencies}]}
    │
    ├──► Company A（本地）
    │        └── Agent: designer → 执行 ui-design
    │
    ├──► Company B（远程节点 :9528）
    │    │  HTTP dispatch + TraceContext 透传
    │    └── Agent: coder → 执行 api-spec
    │                │
    │                └── callback → Master（结果 + token 统计）
    │
    └──► Company C（远程节点 :9529）
         └── Agent: coder → 执行 frontend（依赖 api-spec）
```

---

## 四、完整 Use Case 目录

### UC-01：Agent 生命周期管理

**角色**: 任意用户
**前置条件**: opctl 已安装，opc serve 已启动

| # | 场景 | 命令 / API |
|---|------|-----------|
| 01.1 | 声明式创建 Agent | `opctl apply -f agent.yaml` |
| 01.2 | 查看所有 Agent | `opctl get agents` |
| 01.3 | 查看 Agent 详情 | `opctl describe agent <name>` |
| 01.4 | 热更新模型配置 | `opctl config set agent <name> runtime.model.name=claude-opus-4` |
| 01.5 | 手动重启 Agent | `opctl restart agent <name>` |
| 01.6 | 扩容 Agent 副本 | `opctl scale agent <name> --replicas=3` |
| 01.7 | 自动健康检查 + 重启 | 系统后台，interval 可配 |
| 01.8 | 删除 Agent | `opctl delete agent <name>` |

**主成功路径 (01.1)**:
1. 用户编写 `agent.yaml`（声明 type / model / replicas / resources）
2. `opctl apply -f agent.yaml` 提交到 OPC Server
3. Controller 解析 AgentSpec，调用对应 Adapter.Start()
4. Agent 状态变为 Running，写入 AuditLog
5. 健康检查循环启动（默认 30s interval）

**异常路径**:
- Agent 启动失败 → 指数退避重试（最多 maxRestarts 次）→ 超限写 Failed 事件
- 健康检查连续失败 3 次 → 触发自动重启 → 恢复 Running 状态的 Task

---

### UC-02：单 Agent 任务执行

**角色**: Solo Founder / AI Engineer

| # | 场景 | 命令 |
|---|------|------|
| 02.1 | 发送单次任务 | `opctl run --agent code-reviewer "review PR #123"` |
| 02.2 | 查看任务状态 | `opctl get tasks` |
| 02.3 | 实时查看日志 | `opctl logs <task-id>` |
| 02.4 | 智能调度（自动选 Agent） | `opctl run "写单元测试"` （Dispatcher 自动选最优 Agent） |

**主成功路径 (02.4 — 智能调度)**:
1. 用户发送任务，不指定 Agent
2. Dispatcher 分析任务内容 + Agent 当前负载 + 成本
3. 选择策略（round-robin / least-busy / cost-optimized）
4. Adapter.Execute() 执行，实时流式输出到终端
5. CostEvent 写入（tokens in/out / cost / duration）
6. 任务状态变为 Completed

**成本追踪**:
- OpenClaw: WebSocket RPC 获取 session snapshot → fallback 估算（标记 estimated: true）
- Claude Code: 解析 JSON 输出中的 usage 字段
- Custom: 用户自定义 token 上报

---

### UC-03：Goal 智能分解与自动执行

**角色**: Solo Founder / Tech Lead
**核心价值**: 用户只声明目标，系统完成从"要什么"到"怎么做"的全过程

| # | 场景 | 命令 / API |
|---|------|-----------|
| 03.1 | 创建 Goal（人工分解） | `opctl apply -f goal.yaml` |
| 03.2 | 创建 Goal（AI 自动分解） | `opctl apply -f goal.yaml` (autoDecompose: true) |
| 03.3 | 查看分解方案 | `opctl goal plan <id>` |
| 03.4 | 审批执行 | `opctl goal approve <id>` |
| 03.5 | 修改方案 | `opctl goal revise <id> --file plan.json` |
| 03.6 | 查看 Goal 进度 | `opctl get goals` / `opctl describe goal <name>` |
| 03.7 | 查看成本汇总 | `opctl cost report --by goal` |

**示例 Goal YAML**:
```yaml
apiVersion: opc/v1
kind: Goal
metadata:
  name: implement-auth
spec:
  description: "实现用户认证模块，包括 JWT、OAuth2 和权限控制"
  autoDecompose: true
  approval: manual          # manual | auto
  constraints:
    maxProjects: 5
    maxAgents: 3
    maxCostUSD: 20
  budget:
    total: $50
    alert: 80%
  deadline: 2026-04-15
```

**AI 分解输出示例**:
```json
{
  "projects": [
    {
      "name": "jwt-core",
      "description": "实现 JWT 签发与校验",
      "complexity": "medium",
      "agentSuggestion": "backend-coder",
      "estimatedCost": "$3"
    },
    {
      "name": "oauth2-integration",
      "description": "接入 Google / GitHub OAuth2",
      "complexity": "high",
      "dependsOn": ["jwt-core"],
      "agentSuggestion": "backend-coder",
      "estimatedCost": "$8"
    }
  ]
}
```

**Guardrails（安全阀）**:
- 超过 maxProjects / maxTasks / maxAgents → 自动降级为 manual 模式
- autoApprove 模式下带 guardrail 直接执行
- 分解成本独立追踪，不计入 Goal 预算

---

### UC-04：多 Agent Workflow 编排

**角色**: AI Engineer
**核心价值**: 定义 DAG，实现 Agent 之间的数据流和依赖执行

| # | 场景 | 命令 |
|---|------|------|
| 04.1 | 创建 Workflow | `opctl apply -f workflow.yaml` |
| 04.2 | 手动触发执行 | `opctl run workflow daily-research` |
| 04.3 | 查看执行历史 | `opctl get workflows` |
| 04.4 | 启用定时调度 | Workflow YAML 中配置 `schedule: "0 7 * * *"` |
| 04.5 | 暂停 / 恢复 | `opctl cron disable/enable daily-research` |

**Workflow YAML 示例（带上下文传递）**:
```yaml
apiVersion: opc/v1
kind: Workflow
metadata:
  name: daily-research
spec:
  schedule: "0 7 * * *"
  steps:
    - name: research
      agent: researcher
      input:
        message: "搜索今日 AI Agent 相关动态"
    - name: analyze
      agent: analyst
      dependsOn: [research]
      input:
        message: "分析以下研究结果"
        context:
          - ${{ steps.research.outputs.result }}
    - name: publish
      agent: writer
      dependsOn: [analyze]
      input:
        message: "生成每日简报并发送"
        context:
          - ${{ steps.analyze.outputs.result }}
      delivery:
        channel: telegram
```

---

### UC-05：联邦分布式协同

**角色**: AI Engineer / DevOps
**核心价值**: 多 OPC 节点组成集群，Master 统一调度，Worker 本地执行

| # | 场景 | 命令 / API |
|---|------|-----------|
| 05.1 | 注册远程公司 | `POST /api/companies` + heartbeat |
| 05.2 | 创建跨节点 Goal | `POST /api/goals/federated` |
| 05.3 | 查看联邦状态 | `opctl federation status` |
| 05.4 | 联邦 Goal 持久化 | 系统自动，重启后恢复 |
| 05.5 | 跨节点追踪 | TraceContext 透传，全链路可观测 |

**联邦 Goal API 示例**:
```bash
curl -X POST http://localhost:9527/api/goals/federated \
  -d '{
    "name": "implement-login",
    "projects": [
      { "name": "ui-design",  "agent": "designer",  "company": "local" },
      { "name": "api-spec",   "agent": "architect", "company": "remote-a", "dependencies": ["ui-design"] },
      { "name": "frontend",   "agent": "fe-coder",  "company": "remote-b", "dependencies": ["api-spec"] },
      { "name": "backend",    "agent": "be-coder",  "company": "remote-c", "dependencies": ["api-spec"] },
      { "name": "testing",    "agent": "qa-coder",  "company": "local",    "dependencies": ["frontend","backend"] }
    ]
  }'
```

**重启恢复**:
1. Server 启动时 `ListActiveFederatedGoalRuns()` 加载未完成 run
2. Pending → 重新 dispatch；Running → 等待 callback 超时后重试；Completed → 跳过
3. 恢复过程写审计日志

---

### UC-06：成本控制与预算管理

**角色**: Solo Founder / Tech Lead
**核心价值**: Token 消耗透明，超支自动熔断，成本可按层级分析

| # | 场景 | 命令 |
|---|------|------|
| 06.1 | 查看实时成本 | `opctl cost report` |
| 06.2 | 按 Goal 统计 | `opctl cost report --by goal` |
| 06.3 | 按 Agent 统计 | `opctl cost report --by agent` |
| 06.4 | 设置每日预算 | `opctl budget set --daily $10` |
| 06.5 | 设置每月预算 | `opctl budget set --monthly $200` |
| 06.6 | 导出 CSV | `opctl cost export --csv --period 30d` |
| 06.7 | 查看配额状态 | `GET /api/quota/status` |

**配额策略（OnExceed）**:
| 策略 | 行为 |
|------|------|
| `pause` | 暂停 Agent，等待人工恢复 |
| `alert` | 仅告警，继续执行 |
| `reject` | 拒绝新任务，已在执行的继续 |

**配额触发层级**:
- `perTask`：单任务 token / cost 上限
- `perHour`：每小时 token 上限
- `perDay`：每日 token / cost 上限
- `perMonth`：每月 cost 上限
- `Goal 级`：Goal 总预算超限 → 暂停整个 Goal

---

### UC-07：审计追溯

**角色**: Tech Lead / 合规需求
**核心价值**: 所有操作可追溯，完整链路：Goal → Project → Task → Issue → Agent → CostEvent

| # | 场景 | 命令 |
|---|------|------|
| 07.1 | 查看 Goal 审计 | `opctl audit goal <name>` |
| 07.2 | 追溯 Issue 完整链路 | `opctl audit trace issue <name>` |
| 07.3 | 导出审计日志 | `opctl audit export --format json` |
| 07.4 | 查看 Agent 崩溃历史 | `opctl crashes agent <name>` |
| 07.5 | 查看配置变更历史 | `opctl config history agent <name>` |

**审计追溯输出示例**:
```
Goal: implement-auth
  └── Project: jwt-core
      └── Task: implement-jwt-signing
          └── Issue: write-sign-function
              ├── Agent: backend-coder
              ├── Started: 2026-03-26 10:00:00
              ├── Completed: 2026-03-26 10:02:15
              ├── Tokens: 4,230 (in: 2,100 / out: 2,130)
              ├── Cost: $0.048
              └── Retries: 1 (QualityIssue → re-reviewed)
```

---

### UC-08：Gateway 多频道接入

**角色**: Solo Founder（Telegram 控制 Agent）/ 团队用户（Discord）

| # | 场景 | 配置 |
|---|------|------|
| 08.1 | Telegram 下发任务 | Bot Token 配置，消息路由到 Dispatcher |
| 08.2 | 结果自动回传 Telegram | Agent 完成后 callback 到 Bot |
| 08.3 | Discord 频道接入 | 同 Telegram |
| 08.4 | 路由规则配置 | `/code` → code-reviewer；`/research` → researcher |

---

### UC-09：记忆恢复与崩溃续接

**角色**: 任意用户

| # | 场景 | 命令 |
|---|------|------|
| 09.1 | 查看 Agent Checkpoint | `opctl checkpoints list agent <name>` |
| 09.2 | 手动触发恢复 | `opctl recovery agent <name>` |
| 09.3 | 崩溃后自动恢复 | 系统自动（maxRestarts 内） |

**恢复策略优先级**:
1. Checkpoint（精确恢复）
2. Memory 文件（上下文重建）
3. 审计日志（lookback 1h 重放）

---

### UC-10：Dashboard 可视化

**角色**: 所有用户
**入口**: `http://localhost:4000`（Next.js + React）

| 页面 | 功能 |
|------|------|
| 总览 | Agent 状态、运行中任务数、今日成本、健康率 |
| Agent 列表 | 每个 Agent 状态 / 副本 / 负载 / 成本 |
| Goal 进度 | Goal → Project 树形进度 + 分解可视化 |
| Workflow | Workflow 列表 + 执行历史 + 运行详情 |
| 成本报表 | 按 Goal / Agent / 时间的成本分析图表 |
| 联邦状态 | 节点健康心跳 + 跨节点任务进度 |
| 审计日志 | 全量操作记录，可过滤搜索 |

---

### UC-11：多租户与权限管理

**角色**: Tech Lead
**状态**: 基础框架已有（pkg/tenant, pkg/auth），完整 RBAC 待 v0.8

| # | 场景 |
|---|------|
| 11.1 | 公司（Company）隔离：每个公司独立 Agent 空间 |
| 11.2 | HMAC-SHA256 API Key 认证（Federation 节点间） |
| 11.3 | 注册时生成 APIKey，后续请求携带验证 |

---

### UC-12：CI/CD 集成（GitHub Actions）

**角色**: DevOps / AI Engineer

| # | 场景 |
|---|------|
| 12.1 | PR 提交触发 integration test（6 scenarios） |
| 12.2 | Mock OpenClaw Gateway（轻量 WS echo server） |
| 12.3 | Mock Claude Code（shell script 模拟 JSON 输出） |
| 12.4 | 失败时上传日志 artifact + 阻断 merge |

---

## 五、当前实现状态（v0.6 基线）

> 基于 commit `82e7ad5` 的代码真实状态（非 TaskManager 显示状态）

| Phase | 功能 | 代码状态 | 关键包 |
|-------|------|----------|--------|
| 1 | CLI + OpenClaw 适配器 + SQLite | ✅ 已实现 | `cmd/opctl`, `pkg/adapter/openclaw`, `pkg/storage/sqlite` |
| 2 | Claude Code / Codex 适配器 + 生命周期控制器 + 记忆恢复 | ✅ 已实现 | `pkg/adapter/claudecode`, `pkg/controller` |
| 3 | Goal 层级 + Workflow DAG + Cron + Dispatcher | ✅ 已实现 | `pkg/workflow`, `pkg/dispatcher`, `pkg/audit` |
| 4 | 成本控制 + Gateway + Dashboard + 扩缩容 | ✅ 已实现 | `pkg/cost`, `pkg/gateway`, `dashboard/` |
| 5 | AI Goal 分解 + DAG 执行 + A2A 评审 + Guardrails | ✅ 已实现 | `pkg/goal/ai_decomposer.go`, `pkg/goal/assessor.go` |
| 5b | Federation Auth + Retry Queue + 跨节点协同 | ✅ 已实现 | `pkg/federation/auth.go`, `pkg/federation/retry.go` |
| 6 | 联邦 Goal 持久化 + 配额执行 + 智能重试 + 健康检查 + CI | ✅ 已实现 | `pkg/cost/enforcer.go`, `.github/workflows/` |

### 已知技术债

| 包 | 覆盖率 | 目标 | 状态 |
|----|--------|------|------|
| `pkg/adapter/claudecode` | **96.6%** | 80% | ✅ 已达标 |
| `pkg/storage/sqlite` | **88.5%** | 80% | ✅ 已达标 |
| `pkg/controller` | **88.7%** | 80% | ✅ 已达标 |
| `pkg/server` | **80.6%** | 80% | ✅ 已达标 |
| `pkg/a2a` | **85.5%** | 80% | ✅ 新增包 |
| `pkg/trace` | 43.3% | 80% | P1 |
| `internal/config` | 46.4% | 80% | P1 |
| `pkg/federation` | 71.5% | 80% | P1 |

---

## 六、v0.7 新增需求：Production Hardening 补全 + Self-Evolving Loop

### 6.1 TaskManager 熵对齐（P0，非代码工作）

- Phase 5 所有任务标记为 🟢（ai_decomposer.go, prompt.go, assessor.go 已存在）
- Phase 5b 所有任务标记为 🟢（federation/auth.go, federation/retry.go 已存在）
- Phase 6 所有任务标记为 🟢（代码全部落地，CI 已通过）

### 6.2 测试覆盖率补全（P0）

优先修复 `pkg/controller`、`pkg/server`、`pkg/adapter/claudecode` 覆盖率到 80%+。

### 6.3 Phase 7: Self-Evolving Loop（v0.8 目标，v0.7 预研）

**核心思路**：系统像 K8s 的 HPA 一样，自动观察指标 → 分析异常 → 生成改进提案 → 人工审批 → 自动执行改进 → 验证效果

#### UC-13：系统自主演进

| # | 场景 | 组件 |
|---|------|------|
| 13.1 | 系统每小时采集核心指标 | `pkg/evolve/metrics.go` + MetricsCollector |
| 13.2 | AI 分析异常模式 | `pkg/evolve/analyzer.go` |
| 13.3 | 自动生成 RFC 改进提案 | RFC = 问题 + 方案 + 预期收益 + 风险 |
| 13.4 | 人工审批 RFC | `opctl rfc list` + `opctl rfc approve <id>` |
| 13.5 | RFC → Meta-Goal 自动执行 | autoDecompose=true + Guardrails |
| 13.6 | 指标验证闭环 | 执行后检查指标是否改善 |

**采集指标**:
```
SuccessRate        任务成功率（目标 >95%）
AvgLatency         平均执行时长
CostPerGoal        每个 Goal 平均成本
RetryRate          重试率（高重试 = 质量问题）
CoverageGap        测试覆盖率缺口
ErrorPatterns      高频错误模式
```

**安全阀（v0.7 严格模式）**:
- Meta-Goal 只允许修改：`test/`, `docs/`, `config/`
- 禁止修改：`pkg/`, `cmd/`, `api/`（核心代码）
- v0.8 放开 + 更细粒度 Guardrails

**OODA 循环**:
```
Observe（采集指标）
  → Orient（AI 分析 + RFC 生成）
    → Decide（人工审批）
      → Act（Meta-Goal 执行）
        → 回到 Observe（验证效果）
```

---

## 七、系统架构

### 7.1 组件图

```
                    ┌──────────────────────┐
                    │  Dashboard :4000      │
                    │  Next.js + React      │
                    └──────────┬───────────┘
                               │ REST API / WebSocket
   ┌──────────────────────────────────────────────────────┐
   │  Communication Layer                                  │
   │  REST API (:9527) — Dashboard / opctl CLI            │
   │  gRPC (:9528) — A2A AgentService + FederationService │
   └──────────────────────────────────────────────────────┘
                               │
              ┌────────────────▼────────────────────────┐
              │         OPC Server (:9527)               │
              │                                          │
              │  ┌────────┐  ┌──────────┐  ┌──────────┐ │
              │  │ Goal   │  │Federation│  │ Cost     │ │
              │  │ Engine │  │Controller│  │ Tracker  │ │
              │  │ + DAG  │  │ +Auth    │  │+Enforcer │ │
              │  └────────┘  └──────────┘  └──────────┘ │
              │  ┌────────┐  ┌──────────┐  ┌──────────┐ │
              │  │Workflow│  │Dispatcher│  │ Audit    │ │
              │  │ + Cron │  │          │  │ Logger   │ │
              │  └────────┘  └──────────┘  └──────────┘ │
              │  ┌─────────────────────────────────────┐ │
              │  │    Controller（生命周期管理）         │ │
              │  │  Apply → Start → Health → Restart   │ │
              │  └─────────────────┬───────────────────┘ │
              └────────────────────┼─────────────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                    │
   ┌──────────▼──────┐   ┌─────────▼──────┐  ┌────────▼──────┐
   │ Claude Code     │   │ OpenClaw       │  │ Custom Agent  │
   │ Adapter         │   │ Adapter        │  │ Adapter       │
   │ • claude CLI    │   │ • WS RPC v3    │  │ • stdin/stdout│
   │ • JSON output   │   │ • ed25519      │  │ • JSONL       │
   └─────────────────┘   └────────────────┘  └───────────────┘

   ┌──────────────────────────────────────────────────────┐
   │  Storage Layer                                        │
   │  SQLite（默认）+ PostgreSQL（可选）                    │
   │  Tables: agents, tasks, goals, projects, issues,     │
   │          cost_events, audit_logs, workflows,         │
   │          federated_goal_runs, federation_retry_queue │
   └──────────────────────────────────────────────────────┘
```

### 7.2 数据流（任务执行）

```
opctl run --agent X "message"
  → HTTP POST /api/agents/X/tasks
    → Server: QuotaEnforcer.Check()  ← 超限则 reject/pause
      → Dispatcher.Route()            ← 选择最优 Agent
        → Controller.Execute()
          → Adapter.Execute()         ← 实际调用 claude/openclaw/custom
            → 流式输出 → Response
          → CostEvent.Record()        ← token + cost 持久化
          → AuditLog.Write()          ← 操作记录
```

---

## 八、API 参考

### 8.1 Agent API

```
POST   /api/agents              创建/更新 Agent（apply）
GET    /api/agents              列出所有 Agent
GET    /api/agents/:name        Agent 详情
DELETE /api/agents/:name        删除 Agent
POST   /api/agents/:name/restart 重启
POST   /api/agents/:name/scale  扩缩容
GET    /api/agents/:name/config 获取配置
PUT    /api/agents/:name/config 热更新配置
GET    /api/agents/:name/tasks  Agent 任务列表
POST   /api/agents/:name/tasks  执行任务
```

### 8.2 Goal API

```
POST   /api/goals               创建 Goal
GET    /api/goals               列出 Goals
GET    /api/goals/:id           Goal 详情 + 进度树
GET    /api/goals/:id/plan      查看 AI 分解方案
POST   /api/goals/:id/approve   审批执行
POST   /api/goals/:id/revise    修改方案
POST   /api/goals/federated     创建联邦 Goal（跨节点）
```

### 8.3 Workflow API

```
POST   /api/workflows           创建/更新 Workflow
GET    /api/workflows           列出 Workflows
POST   /api/workflows/:name/run 手动触发
PUT    /api/workflows/:name/toggle 启用/禁用
GET    /api/workflows/:name/runs          执行历史
GET    /api/workflows/:name/runs/:id      执行详情
```

### 8.4 Cost & Quota API

```
GET    /api/costs               成本报表
GET    /api/costs?group=goal    按 Goal 聚合
GET    /api/quota/status        当前配额使用情况
POST   /api/budget              设置预算
```

### 8.5 System API（v0.7 新增）

```
GET    /api/system/metrics      系统运行指标（时序）
GET    /api/system/rfcs         RFC 提案列表
GET    /api/system/rfcs/:id     RFC 详情
POST   /api/system/rfcs/:id/approve  审批 RFC
POST   /api/system/rfcs/:id/reject   拒绝 RFC
```

---

## 九、数据模型

### 9.1 核心表

| 表名 | 描述 |
|------|------|
| `agents` | Agent 配置与状态 |
| `tasks` | 任务执行记录 |
| `goals` | Goal 定义 + 分解方案（JSON） |
| `projects` | Project 记录 |
| `issues` | Issue 执行单元 |
| `workflows` | Workflow 定义 |
| `workflow_runs` | Workflow 执行历史 |
| `cost_events` | Token / Cost 明细（task 级） |
| `audit_logs` | 全量操作审计 |
| `checkpoints` | Agent 内存快照 |
| `federated_goal_runs` | 联邦 Goal 执行状态 |
| `federated_goal_projects` | 联邦 Project 状态 |
| `federation_retry_queue` | 联邦 callback 重试队列 |
| `system_metrics` | 系统指标时序（v0.7） |
| `rfcs` | RFC 改进提案（v0.7） |

### 9.2 AgentSpec YAML 规范（精简）

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: code-reviewer
spec:
  type: claude-code          # claude-code | openclaw | codex | custom
  replicas: 2
  runtime:
    model:
      name: claude-sonnet-4-6
      fallback: claude-haiku-4-5
    timeout:
      task: 300s
      idle: 600s
  resources:
    tokenBudget:
      perTask: 100000
      perDay: 2000000
    costLimit:
      perTask: $1
      perDay: $20
    onExceed: pause            # pause | alert | reject
  healthCheck:
    interval: 30s
    consecutiveFailuresThreshold: 3
  recovery:
    maxRestarts: 5
    backoff: exponential
```

---

## 十、风险登记

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| Claude API 变更/速率限制 | 中 | 高 | Adapter 抽象 + fallback 模型 |
| AI 分解质量不稳定 | 高 | 高 | Few-shot + JSON schema 校验 + Plan 模式 |
| 分解成本失控 | 中 | 中 | 独立预算 + 便宜模型优先（haiku） |
| 联邦节点断线 | 中 | 高 | RetryQueue + 指数退避 + 超时重试 |
| 测试覆盖不足导致回归 | 高 | 中 | CI 门禁 + 覆盖率 badge |
| Self-Evolving 误改核心代码 | 中 | 极高 | v0.7 严格安全阀（只允许 test/docs/config） |
| SQLite 并发瓶颈 | 中 | 中 | 读写分离 + PostgreSQL 迁移路径 |

---

## 十一、里程碑

| 里程碑 | 版本 | 状态 | 核心交付 |
|--------|------|------|----------|
| M1: Foundation | v0.1 | ✅ | opctl CLI + OpenClaw + SQLite |
| M2: Multi-Agent | v0.2 | ✅ | Claude Code / Codex + 生命周期 |
| M3: Orchestration | v0.3 | ✅ | Workflow + Dispatcher + Audit |
| M4: Production | v0.4 | ✅ | 成本控制 + Gateway + Dashboard |
| M5: Federation | v0.5 | ✅ | 联邦协同 + Trace 传播 |
| M6: Multi-Adapter | **v0.6** | ✅ **当前** | AI Goal 分解 + A2A 评审 + 配额执行 + CI |
| M7: Hardening 补全 | **v0.7** | 🟡 进行中 | 测试覆盖率 80%+ ✅ + TaskManager 对齐 ✅ + A2A protobuf ✅ + Dashboard 升级 ✅ + RFC 预研 ✅ |
| M8: Self-Evolving | **v0.8** | 🔲 规划中 | 指标采集 + AI 分析 + RFC 闭环 + 安全阀 |

### v0.7 具体交付（优先级排序）

- [x] **P0** TaskManager 全面更新（Phase 5 / 5b / 6 标记 🟢）
- [x] **P0** `pkg/controller` 覆盖率 55% → 88.7% ✅
- [x] **P0** `pkg/server` 覆盖率 55% → 80.6% ✅
- [x] **P0** `pkg/adapter/claudecode` 覆盖率 30% → 96.6% ✅
- [x] **P1** `pkg/storage/sqlite` 覆盖率 38% → 88.5% ✅
- [x] **P1** pkg/evolve 预研：MetricsCollector 骨架 ✅
- [x] **P2** Dashboard Goals 分解树形可视化 ✅
- [x] **NEW** A2A protobuf 通信协议（pkg/a2a/ + proto/）✅
- [x] **NEW** gRPC AgentService + FederationService ✅
- [x] **NEW** 无代码 Agent 创建向导 ✅
- [x] **NEW** 模型注册表 API ✅
- [x] **NEW** RFC 审批 + 系统指标 Dashboard ✅
- [ ] **P1** Phase 5b: OpenClaw 密钥持久化 + 自动读 Token（5.5.1）
- [ ] **P1** Phase 5b: Goal 分解持久化 + 成本追踪（5.5.2）
- [ ] **P1** Workflow 执行详情 API（5.5.4）
- [ ] **P2** README + CI badge

---

*文档版本*: v0.7
*最后更新*: 2026-03-26
*权威来源*: 代码（MD 与代码不一致时，以代码为准）
*下次 Review*: 每周一 10:00
