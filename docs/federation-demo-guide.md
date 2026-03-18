# OPC Platform 联邦编排 Demo 指南

## 概述

本文档演示 OPC Platform 的联邦编排能力：一个 Master 节点调度多个 Team 节点，按依赖关系自动编排任务执行，全链路可追踪。

### 架构

```
                          ┌──────────────┐
                          │   Jaeger     │
                          │   :16686     │
                          │  (全链路追踪) │
                          └──────┬───────┘
                                 │ OTLP (:4318)
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌────────▼─────────┐   ┌────────▼─────────┐   ┌────────▼─────────┐
│  Master (:9527)  │   │  Design (:9528)  │   │ Frontend (:9529) │
│                  │   │                  │   │                  │
│  - 联邦调度中心   │   │  - 设计团队 OPC   │   │  - 前端团队 OPC   │
│  - DAG 依赖编排   │──►│  - 自动创建 agent │   │  - 自动创建 agent │
│  - Goal 持久化    │   │  - 执行 + 回调    │   │  - 执行 + 回调    │
│  - 回调接收+推进  │   └──────────────────┘   └──────────────────┘
│  - Dashboard :3000│
└────────┬─────────┘            ┌──────────────────┐
         │                      │ Backend (:9530)  │
         └─────────────────────►│                  │
                                │  - 后端团队 OPC   │
                                │  - 自动创建 agent │
                                │  - 执行 + 回调    │
                                └──────────────────┘
```

## 快速开始

### 前置条件

```bash
# 1. Go 编译环境
go version   # 需要 1.23+

# 2. Docker（用于 Jaeger）
docker --version

# 3. Anthropic API Key（agent 执行需要）
export ANTHROPIC_API_KEY="sk-ant-..."

# 4. Dashboard 构建（可选，用于 Web UI）
cd dashboard && npm install && npm run build && cd ..
```

### 一键启动

```bash
cd examples/federation-workflow

# 启动全部：Jaeger + 4 OPC 实例 + 注册联邦 + 下发 Goal + Dashboard
export ANTHROPIC_API_KEY="sk-ant-..."
bash run-demo.sh

# 停止全部
bash run-demo.sh stop
```

### 脚本做了什么

`run-demo.sh` 依次执行以下步骤：

| 步骤 | 动作 | 说明 |
|------|------|------|
| 1 | 编译 `opctl` | 自动检测二进制是否最新 |
| 2 | 清理 | 杀残留进程 + 删旧状态目录 |
| 3 | 启动 Jaeger | Docker 容器，OTLP :4318 + UI :16686 |
| 4 | 启动 4 个 OPC | 各自隔离的 `--state-dir`，各自上报 OTel |
| 5 | 健康检查 | 等待所有节点 `/api/health` 返回 200 |
| 6 | 注册联邦 | 向 Master 注册 3 个 Company（自动 ping 设 Online） |
| 7 | 下发 Goal | 带项目依赖的联邦 Goal |
| 8 | 启动 Dashboard | standalone 模式，:3000 |

---

## 联邦编排流程详解

### 1. Goal 下发（Master）

```bash
POST /api/goals/federated
```

```json
{
  "name": "实现用户登录功能",
  "description": "完整登录功能：UI 设计 → API 接口定义 → 前后端并行开发",
  "projects": [
    {
      "name": "ui-design",
      "companyId": "<design-team-id>",
      "agent": "designer",
      "description": "设计登录页面，输出高保真设计稿和交互标注"
    },
    {
      "name": "api-spec",
      "companyId": "<backend-team-id>",
      "agent": "coder",
      "description": "定义 REST API 接口文档",
      "dependencies": ["ui-design"]
    },
    {
      "name": "frontend-dev",
      "companyId": "<frontend-team-id>",
      "agent": "coder",
      "description": "根据 UI 设计稿和 API 文档实现前端登录页",
      "dependencies": ["ui-design", "api-spec"]
    },
    {
      "name": "backend-dev",
      "companyId": "<backend-team-id>",
      "agent": "coder",
      "description": "根据 API 文档实现后端登录接口",
      "dependencies": ["api-spec"]
    }
  ]
}
```

### 2. DAG 拓扑排序（Master 自动完成）

Master 对 projects 做 Kahn 算法拓扑排序，分成执行层：

```
Layer 0:  ui-design (design-team)              ← 无依赖，立即 dispatch
            ↓ callback 完成后
Layer 1:  api-spec (backend-team)              ← 依赖 ui-design
            ↓ callback 完成后
Layer 2:  frontend-dev (frontend-team)         ← 依赖 ui-design + api-spec
          backend-dev (backend-team)           ← 依赖 api-spec
          （并行执行）
```

### 3. Dispatch 到远端节点

Master 向目标节点发送 HTTP 请求：

```
POST http://design-team:9528/api/run
{
  "agent": "designer",
  "message": "[Federated Goal: 登录功能]\n\n设计登录页...\n\n## Your Task\n设计登录页面...",
  "callbackURL": "http://master:9527/api/federation/callback",
  "goalId": "goal-xxx",
  "projectId": "proj-goal-xxx-ui-design",
  "lineage": "[]"
}
```

关键特性：
- **上下文注入**：下游 project 的 message 自动包含上游的 result
- **血缘传递**：`lineage` 字段携带完整的上游引用链
- **回调地址**：远端完成后回调 Master 推进下一层

### 4. 远端节点接收并执行

远端 OPC 收到 `/api/run` 后：

1. **自动创建 Agent** — `ensureAgent()` 检查 agent 是否存在，不存在则自动创建并启动
2. **创建 Task** — 记录 goalId、projectId、lineage 到数据库
3. **异步执行** — Agent（Claude Code）执行任务
4. **回调 Master** — 执行完成后发送 `FederationCallback`

```json
// 回调 payload
{
  "goalId": "goal-xxx",
  "projectId": "proj-goal-xxx-ui-design",
  "taskId": "task-xxx",
  "status": "completed",
  "result": "设计稿产出...",
  "tokensIn": 1200,
  "tokensOut": 3500,
  "cost": 0.02,
  "lineage": "[...]"
}
```

### 5. Master 推进下一层

`advanceFederatedGoal()` 收到 callback 后：

1. **标记 project 完成** — 更新 `FederatedGoalRun` 中的 project 状态
2. **检查依赖** — 扫描所有 Pending 的 project，看依赖是否全部满足
3. **Dispatch 就绪项目** — 满足条件的 project 立即 dispatch，注入上游 result
4. **级联失败** — 如果上游 project 失败，依赖它的下游自动标记为 Failed
5. **Goal 终态** — 所有 project 完成或失败时，更新 Goal 状态到数据库

### 6. 上下文注入（跨节点传递）

当 Layer 1 的 `api-spec` 被 dispatch 时，message 自动包含 Layer 0 的产出：

```
[Federated Goal: 登录功能]

完整登录功能描述...

## Your Task
定义 REST API 接口文档

## Upstream Output from [ui-design]
（这里是 design-team 的 agent 产出的设计稿内容）
```

---

## 追踪与可观测

### 血缘链（Lineage）

每个 Task 携带 `lineage` 字段，记录完整的上游引用链：

```json
[
  {
    "goalId": "goal-xxx",
    "projectName": "ui-design",
    "issueId": "proj-goal-xxx-ui-design",
    "opcNode": "design-team-id",
    "label": "ui-design"
  },
  {
    "goalId": "goal-xxx",
    "projectName": "api-spec",
    "issueId": "proj-goal-xxx-api-spec",
    "opcNode": "backend-team-id",
    "label": "api-spec"
  }
]
```

查询方式：
```bash
# 查看某个节点的 task 及其血缘
curl http://localhost:9529/api/tasks | python3 -m json.tool
```

### OpenTelemetry Trace

每个关键操作生成 OTel span：

| Span 名称 | 触发时机 | 属性 |
|-----------|---------|------|
| `createFederatedGoal` | Master 创建联邦 Goal | goal.id, goal.name, projects.count |
| `runTask` | 远端节点执行 Task | task.id, agent, goal.id, project.id |
| `federationCallback` | Master 收到回调 | goal.id, project.id, task.id, status |

查看方式：打开 Jaeger UI `http://localhost:16686`，选择 service `opc-master`。

### 日志

```bash
# Master 日志
tail -f ~/.opc-federation-demo/master/stdout.log

# Design 节点日志
tail -f ~/.opc-federation-demo/design/stdout.log

# 各节点支持 --log-level 配置
opctl serve --log-level debug   # debug | info | warn | error
```

---

## Web UI 查看

### Dashboard（:3000）

| 页面 | 路径 | 看什么 |
|------|------|--------|
| Goals | `/goals` | 联邦 Goal 列表、状态（in_progress/completed/failed） |
| Tasks | `/tasks` | Master 本地 task（联邦 task 在远端节点） |
| Federation | `/federation` | **联邦聚合视图**：所有 Company、远端 agents/tasks/metrics |

### Federation 页面功能

1. **Company 卡片** — 显示每个节点的 endpoint、状态（Online/Offline）、agents
2. **展开详情** — 点击卡片展开，查看该节点的：
   - Remote Agents（远端 agent 列表及状态）
   - Task Summary（运行中/完成/失败计数）
   - Remote Metrics（token 消耗、cost）
   - Open Company Dashboard（跳转到节点 UI，需配 dashboardUrl）
3. **注册公司** — 表单注册新的联邦节点
4. **下发联邦 Goal** — 选择目标公司，发送 Goal

---

## 涉及的功能模块

### 后端（Go）

| 模块 | 文件 | 功能 |
|------|------|------|
| **联邦控制器** | `pkg/federation/controller.go` | Company 注册/注销、状态管理、状态持久化 |
| **心跳监控** | `pkg/federation/heartbeat.go` | 30s 间隔 ping 各节点，自动更新 Online/Offline |
| **传输层** | `pkg/federation/transport.go` | HMAC 签名的 HTTP 通信、Ping 健康检查 |
| **Goal DAG** | `pkg/goal/dag.go` | Project 依赖验证（环检测）+ 拓扑排序分层 |
| **血缘模型** | `pkg/goal/lineage.go` | LineageRef 类型、不可变追加、JSON 序列化 |
| **联邦 Goal** | `pkg/server/server.go` | `createFederatedGoal` 创建+分层+dispatch |
| **回调推进** | `pkg/server/server.go` | `advanceFederatedGoal` 收回调、推进下一层、级联失败 |
| **远端执行** | `pkg/server/server.go` | `runTask` 接收联邦任务、自动创建 agent、执行、回调 |
| **OTel 追踪** | `pkg/trace/tracer.go` | OTLP/HTTP 导出到 Jaeger |
| **日志配置** | `internal/config/config.go` | `--log-level` 支持 debug/info/warn/error |
| **状态隔离** | `internal/config/config.go` | `--state-dir` 多实例隔离 |
| **审计追踪** | `pkg/audit/audit.go` | AuditEvent 含 GoalRef/ProjectRef/TaskRef/IssueRef |
| **数据库** | `pkg/storage/sqlite/sqlite.go` | tasks 含 goal_id/project_id/lineage_json；issues 含 trace_id/span_id/parent_spans |

### 前端（Next.js）

| 页面 | 文件 | 功能 |
|------|------|------|
| Federation | `app/federation/page.tsx` | 公司列表、注册、详情展开、聚合 agents/tasks/metrics |
| Goals | `app/goals/page.tsx` | Goal 列表、状态、AI 拆解 |
| Tasks | `app/tasks/page.tsx` | 三视图（列表/层级/看板）、搜索筛选 |

### 数据模型

```
Goal (DB: goals)
  │  id, name, description, status, phase
  │
  ├─► FederatedGoalRun (内存: federatedGoalRuns map)
  │     goalId, projects map, layers, results map
  │
  └─► Project (内存: FederatedGoalRun.Projects)
        id, companyId, name, dependencies[], status, result
        │
        └─► Task (DB: tasks, 在远端节点)
              id, agentName, message, status, result
              goalId, projectId, lineageJSON
```

### CLI Flags

```bash
opctl serve \
  --port 9527             # HTTP 端口
  --host 0.0.0.0          # 监听地址
  --state-dir /data/opc   # 状态目录（多实例隔离）
  --log-level debug       # 日志级别: debug|info|warn|error
  --otel                  # 启用 OpenTelemetry
  --otel-endpoint host:4318  # OTLP 端点
  --otel-service opc-master  # 服务名
```

---

## 手动逐步操作

如果不用一键脚本，可以手动逐步操作：

```bash
# 1. 编译
go build -o opctl ./cmd/opctl

# 2. 启动 Jaeger
docker run -d --name jaeger -p 16686:16686 -p 4318:4318 \
  -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.54

# 3. 启动 Master
export ANTHROPIC_API_KEY="sk-ant-..."
./opctl serve --port 9527 --state-dir /tmp/opc-master --otel

# 4. 启动 Design 节点（另一个终端）
export ANTHROPIC_API_KEY="sk-ant-..."
./opctl serve --port 9528 --state-dir /tmp/opc-design --otel --otel-service opc-design

# 5. 注册联邦
curl -X POST http://localhost:9527/api/federation/companies \
  -H "Content-Type: application/json" \
  -d '{"name":"design-team","endpoint":"http://localhost:9528","type":"software","agents":["designer"]}'
# 返回 {"id":"xxx", "status":"Online", ...}

# 6. 下发 Goal
curl -X POST http://localhost:9527/api/goals/federated \
  -H "Content-Type: application/json" \
  -d '{
    "name": "设计登录页",
    "description": "设计高保真登录页面",
    "projects": [
      {"name": "ui-design", "companyId": "<id>", "agent": "designer", "description": "设计登录页"}
    ]
  }'

# 7. 查看执行结果
curl http://localhost:9528/api/tasks | python3 -m json.tool   # Design 节点 task
curl http://localhost:9527/api/goals | python3 -m json.tool   # Master goal 状态

# 8. 查看追踪
open http://localhost:16686   # Jaeger UI
```

---

## 已知限制

| 限制 | 说明 | 计划 |
|------|------|------|
| Agent 默认 Claude Code | `ensureAgent` 创建的 agent 固定 `claude-code` 类型 | 支持按 company 配置 agent 类型 |
| 联邦 Goal 状态部分在内存 | `FederatedGoalRun` 不持久化，重启丢失 | 持久化到数据库 |
| 单向编排 | 只有 Master→Node，Node 之间不直接通信 | P2P 联邦模式 |
| Provenance DAG（方案 C） | 血缘图谱未实现 | 独立图存储 |
| Postgres 未同步 | 新字段只加了 SQLite | 同步 Postgres schema |
| NEXT_PUBLIC 构建时注入 | 各节点不能运行时切换 API 地址 | 运行时 API URL 配置 |
