# OPC Platform 公司内部落地指南

## 架构总览

```
你（主控）                    设计组               前端组               后端组               测试组
┌──────────────┐         ┌──────────────┐   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│  OPC Master  │         │  OPC Node    │   │  OPC Node    │   │  OPC Node    │   │  OPC Node    │
│  :9527       │◄───────►│  :9528       │   │  :9529       │   │  :9530       │   │  :9531       │
│              │         │              │   │              │   │              │   │              │
│  Federation  │         │ Agent:       │   │ Agent:       │   │ Agent:       │   │ Agent:       │
│  Hub         │         │  claude-code │   │  claude-code │   │  claude-code │   │  claude-code │
│              │         │  (设计师账号) │   │  (前端账号)  │   │  (后端账号)  │   │  (测试账号)  │
│  Dashboard   │         │  openclaw    │   │  openclaw    │   │  openclaw    │   │  openclaw    │
│  :3000       │         │  (可选)      │   │  (可选)      │   │  (可选)      │   │  (可选)      │
└──────────────┘         └──────────────┘   └──────────────┘   └──────────────┘   └──────────────┘
     你的电脑                小王的电脑           小李的电脑           小张的电脑           小赵的电脑
```

**核心思想**：每个人的电脑是一个 OPC Node（类比 K8s Worker Node），你的电脑是 Master。不需要任何定制——纯用 OPC 标准功能。

---

## Step 1: 每个人安装 OPC（5 分钟）

每个团队成员在自己电脑上执行：

```bash
# 克隆 & 编译
git clone https://github.com/zxs1633079383/opc-platform.git
cd opc-platform
go build -o opctl ./cmd/opctl/
sudo cp opctl /usr/local/bin/

# 验证
opctl version
```

---

## Step 2: 每个人注册自己的 Agent

### 设计组（小王的电脑）

创建 `~/.opc/agents/designer.yaml`：

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ui-designer
  labels:
    role: design
    team: design
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: 1800s    # 设计任务可能需要较长时间
  resources:
    costLimit:
      perTask: "$5"
      perDay: "$50"
  context:
    workdir: /Users/xiaowang/projects/ui-design   # 设计项目目录
  recovery:
    enabled: true
    maxRestarts: 3
```

```bash
# 启动 OPC daemon（端口 9528，避免和 Master 冲突）
opctl serve --port 9528 &

# 注册 agent
opctl apply -f ~/.opc/agents/designer.yaml

# 验证
opctl get agents
# NAME          TYPE         STATUS   RESTARTS  AGE
# ui-designer   claude-code  Created  0         5s

# 启动 agent
opctl restart agent ui-designer

# 测试
opctl run --agent ui-designer "列出当前项目的页面结构"
```

### 前端组（小李的电脑）

```yaml
# ~/.opc/agents/frontend.yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: frontend-dev
  labels:
    role: frontend
    team: frontend
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: 1200s
  resources:
    costLimit:
      perTask: "$3"
      perDay: "$30"
  context:
    workdir: /Users/xiaoli/projects/web-app
  recovery:
    enabled: true
    maxRestarts: 3
```

```bash
opctl serve --port 9529 &
opctl apply -f ~/.opc/agents/frontend.yaml
opctl restart agent frontend-dev
```

### 后端组（小张的电脑）

```yaml
# ~/.opc/agents/backend.yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: backend-dev
  labels:
    role: backend
    team: backend
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: 1200s
  context:
    workdir: /Users/xiaozhang/projects/api-server
```

```bash
opctl serve --port 9530 &
opctl apply -f ~/.opc/agents/backend.yaml
opctl restart agent backend-dev
```

### 测试组（小赵的电脑）

```yaml
# ~/.opc/agents/tester.yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: qa-tester
  labels:
    role: testing
    team: qa
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: 900s
  context:
    workdir: /Users/xiaozhao/projects/test-suite
```

```bash
opctl serve --port 9531 &
opctl apply -f ~/.opc/agents/tester.yaml
opctl restart agent qa-tester
```

### OpenClaw 共存（可选）

如果某个成员同时使用 OpenClaw：

```yaml
# 在同一台机器上注册两个 agent
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: frontend-openclaw
spec:
  type: openclaw
  protocol:
    type: websocket
    format: "ws://localhost:18789"
```

```bash
# 注册两个 agent，按任务类型选择
opctl apply -f frontend-claude.yaml    # claude-code agent
opctl apply -f frontend-openclaw.yaml  # openclaw agent
```

---

## Step 3: 你（Master）注册联邦公司

在你的电脑上：

```bash
# 启动 Master OPC
opctl serve --port 9527 &
```

注册每个团队为一个"公司"：

```yaml
# federation-companies.yaml
---
apiVersion: opc/v1
kind: Company
metadata:
  name: design-team
spec:
  type: software          # software | operations | sales | custom
  endpoint: http://xiaowang.local:9528    # 小王电脑的 IP:端口
  agents:
    - ui-designer
---
apiVersion: opc/v1
kind: Company
metadata:
  name: frontend-team
spec:
  type: software
  endpoint: http://xiaoli.local:9529
  agents:
    - frontend-dev
---
apiVersion: opc/v1
kind: Company
metadata:
  name: backend-team
spec:
  type: software
  endpoint: http://xiaozhang.local:9530
  agents:
    - backend-dev
---
apiVersion: opc/v1
kind: Company
metadata:
  name: qa-team
spec:
  type: operations
  endpoint: http://xiaozhao.local:9531
  agents:
    - qa-tester
```

```bash
# 逐个注册（apply 不支持多文档，分开执行）
opctl federation add-company --name design-team \
  --type software --endpoint http://xiaowang.local:9528

opctl federation add-company --name frontend-team \
  --type software --endpoint http://xiaoli.local:9529

opctl federation add-company --name backend-team \
  --type software --endpoint http://xiaozhang.local:9530

opctl federation add-company --name qa-team \
  --type operations --endpoint http://xiaozhao.local:9531

# 验证
opctl federation status
```

---

## Step 4: 创建 Goal，自动分发

### 方式一：CLI（推荐起步）

```yaml
# goal-user-system.yaml
apiVersion: opc/v1
kind: Goal
metadata:
  name: build-user-system
spec:
  description: "构建用户管理系统：注册、登录、个人中心、权限管理"
  owner: CTO
  deadline: 2026-04-01
  targetCompanies:
    - design-team
    - frontend-team
    - backend-team
    - qa-team
  decomposition:
    projects:
      # ===== 设计组 =====
      - name: user-system-design
        company: design-team
        description: "用户系统 UI/UX 设计"
        tasks:
          - name: design-user-flows
            description: |
              设计用户系统的完整交互流程：
              1. 注册流程（邮箱/手机/第三方登录）
              2. 登录流程（密码/验证码/OAuth）
              3. 个人中心页面布局
              4. 权限管理后台界面
              输出 Figma 设计稿或 HTML mockup。
            assignAgent: ui-designer

      # ===== 后端组 =====
      - name: user-system-backend
        company: backend-team
        description: "用户系统后端 API"
        tasks:
          - name: design-user-api
            description: |
              设计并实现用户系统 REST API：
              - POST /api/auth/register
              - POST /api/auth/login
              - GET /api/users/me
              - PUT /api/users/me
              - GET /api/admin/users (管理员)
              使用 JWT 认证，写完整的 Go 代码和测试。
            assignAgent: backend-dev
          - name: implement-rbac
            description: |
              实现 RBAC 权限系统：
              - 角色定义（admin/editor/viewer）
              - 权限中间件
              - 数据库 schema
            assignAgent: backend-dev
            dependsOn: [design-user-api]

      # ===== 前端组 =====
      - name: user-system-frontend
        company: frontend-team
        description: "用户系统前端页面"
        tasks:
          - name: implement-auth-pages
            description: |
              实现认证相关页面：
              - 注册页（表单验证、密码强度）
              - 登录页（记住我、忘记密码）
              - OAuth 回调页
              使用 React + TypeScript，对接后端 API。
            assignAgent: frontend-dev
          - name: implement-user-center
            description: |
              实现个人中心：
              - 个人信息编辑
              - 头像上传
              - 密码修改
              - 登录设备管理
            assignAgent: frontend-dev
            dependsOn: [implement-auth-pages]

      # ===== 测试组 =====
      - name: user-system-testing
        company: qa-team
        description: "用户系统测试"
        tasks:
          - name: write-api-tests
            description: |
              编写用户系统 API 集成测试：
              - 注册/登录流程测试
              - 权限验证测试
              - 边界 case（重复注册、无效 token 等）
              使用 Go testing + httptest。
            assignAgent: qa-tester
          - name: write-e2e-tests
            description: |
              编写端到端测试：
              - 完整注册→登录→操作→登出流程
              - 权限隔离验证
              使用 Playwright。
            assignAgent: qa-tester
            dependsOn: [write-api-tests]
```

```bash
opctl apply -f goal-user-system.yaml
# goal/build-user-system created (decomposed: 4 projects, 6 tasks, 0 issues, 6 dispatched)
```

**发生了什么**：
1. OPC 解析 Goal YAML
2. 创建 4 个 Project（每个团队一个）
3. 创建 6 个 Task，分配给对应的 Agent
4. 根据 company 字段，把 task 分发到对应团队的 OPC Node
5. 每个 Node 上的 Agent 开始执行自己的 task
6. 执行结果通过 callback 回传给 Master

### 方式二：Dashboard（日常使用）

打开 http://localhost:3000

1. **Goals 页面** → Create Goal → 填写名称和描述 → 开启 Auto-Decompose → Create
2. **Federation 页面** → 查看各公司状态 → Create Federated Goal → 选择目标公司
3. **Tasks 页面** → Hierarchy 视图 → 查看 Goal → Project → Task 完整链路

---

## Step 5: 监控和管理

### 实时状态

```bash
# Master 上查看全局状态
opctl status

# 查看所有公司的 agent 状态
opctl federation status

# 查看 Goal 进度
opctl goal status build-user-system
# Goal: build-user-system (in_progress)
# ├── Project: user-system-design (design-team)
# │   └── Task: design-user-flows [Running] → ui-designer
# ├── Project: user-system-backend (backend-team)
# │   ├── Task: design-user-api [Completed] → backend-dev
# │   └── Task: implement-rbac [Running] → backend-dev
# ├── Project: user-system-frontend (frontend-team)
# │   ├── Task: implement-auth-pages [Pending] → frontend-dev
# │   └── Task: implement-user-center [Pending] → frontend-dev
# └── Project: user-system-testing (qa-team)
#     ├── Task: write-api-tests [Pending] → qa-tester
#     └── Task: write-e2e-tests [Pending] → qa-tester
```

### Dashboard 监控

- **首页**：Agent 总数、任务完成率、今日/本月成本
- **Tasks → Kanban**：Todo | Running | Complete 三列看板
- **Tasks → Hierarchy**：Goal → Project → Task → Issue 树形追溯
- **Costs**：每日成本趋势、按 Agent/Goal 分组
- **Logs**：实时系统日志
- **Federation**：各公司在线状态、远程 Agent 查看

### 成本控制

```bash
# 查看成本报告
opctl cost report --by agent
opctl cost report --by goal

# 设置预算
opctl budget set --daily $50 --monthly $1000
```

---

## Step 6: 工作流编排（持续集成）

日常开发中，定义可复用的工作流：

```yaml
# workflow-feature-dev.yaml
apiVersion: opc/v1
kind: Workflow
metadata:
  name: feature-development
spec:
  steps:
    - name: design
      agent: ui-designer
      input:
        message: "根据需求文档设计 UI 交互稿"
      outputs:
        - name: design-spec

    - name: backend
      agent: backend-dev
      dependsOn: [design]
      input:
        message: "根据设计稿实现后端 API"
        context:
          - "${{ steps.design.outputs.design-spec }}"
      outputs:
        - name: api-spec

    - name: frontend
      agent: frontend-dev
      dependsOn: [design, backend]
      input:
        message: "根据设计稿和 API 文档实现前端页面"
        context:
          - "${{ steps.design.outputs.design-spec }}"
          - "${{ steps.backend.outputs.api-spec }}"
      outputs:
        - name: frontend-build

    - name: test
      agent: qa-tester
      dependsOn: [frontend, backend]
      input:
        message: "对接口和页面进行完整测试"
        context:
          - "${{ steps.backend.outputs.api-spec }}"
          - "${{ steps.frontend.outputs.frontend-build }}"
```

```bash
opctl apply -f workflow-feature-dev.yaml
opctl run workflow feature-development
```

---

## 网络配置

### 局域网内（推荐）

各电脑在同一网络，用局域网 IP：

```bash
# 小王的电脑
opctl serve --host 0.0.0.0 --port 9528

# Master 注册时用局域网 IP
opctl federation add-company --name design-team \
  --endpoint http://192.168.1.101:9528
```

### 远程（Tailscale/ZeroTier）

如果不在同一网络，用 Tailscale：

```bash
# 每个人安装 tailscale
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up

# 用 Tailscale IP 注册
opctl federation add-company --name design-team \
  --endpoint http://100.x.x.x:9528
```

---

## 日常操作速查

```bash
# ===== 你（Master）=====

# 创建需求
opctl apply -f goal-xxx.yaml

# 查看全局进度
opctl goal list
opctl goal status <goal-name>

# 查看各组状态
opctl federation status

# 查看成本
opctl cost report

# 打开 Dashboard
open http://localhost:3000

# ===== 团队成员 =====

# 查看分配给我的任务
opctl get tasks

# 查看任务详情
opctl logs <task-id>

# 查看我的 agent 状态
opctl get agents
opctl health
```

---

## FAQ

**Q: Claude 账号怎么配置？**
A: 每个人用自己的 Claude 账号。OPC 通过 `claude --print` 调用本地 Claude CLI，使用的是本地已登录的账号。不需要在 OPC 中配置 API key。

**Q: OpenClaw 和 Claude Code 可以同时用吗？**
A: 可以。在同一台机器上注册两个 agent（一个 `type: claude-code`，一个 `type: openclaw`），按任务类型选择。

**Q: 如果有人的电脑关机了怎么办？**
A: Master 的 Federation 心跳会检测到 Offline。分配给该公司的 task 会等待，不会丢失。对方开机后 `opctl serve` 恢复，task 继续执行。

**Q: 成本怎么算？**
A: 每个人的 Claude 账号消耗各自的 token。OPC 记录每个 task 的 token 用量和成本，在 Master Dashboard 汇总展示。

**Q: 能不能限制某个人的用量？**
A: 在 AgentSpec 的 `resources.costLimit` 中设置 `perTask`/`perDay` 预算。超限后 task 暂停。

**Q: Goal 执行完成怎么通知？**
A: 配置 Telegram/Discord Bot，设置 `OPC_TELEGRAM_TOKEN` 环境变量，Goal 完成时自动通知。
