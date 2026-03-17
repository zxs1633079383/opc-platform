# OPC Platform PRD
## OpenClaw 的 K8s — 统一 AI Agent 编排平台

**版本**: v0.1 Draft
**日期**: 2026-03-14
**作者**: OPC Team

---

## 一、产品定位

### 1.1 一句话定位

> **OPC Platform 是 AI Agent 的 Kubernetes** —— 让一个人能像管理容器集群一样管理 AI Agent 集群。

### 1.2 类比

| K8s 世界 | OPC 世界 |
|----------|----------|
| Docker 容器 | AI Agent（OpenClaw / Claude Code / Codex / 自定义） |
| Kubernetes | OPC Platform |
| Pod | Agent Instance（运行中的 Agent 实例） |
| Deployment | Agent Spec（Agent 配置定义） |
| Service | Agent Endpoint（Agent 通信接口） |
| Ingress | Gateway（用户入口） |
| kubectl | opctl（命令行工具） |

### 1.3 核心价值

1. **统一编排**：一个平台管理所有类型的 Agent
2. **声明式配置**：定义期望状态，平台自动实现
3. **自动运维**：健康检查、故障恢复、弹性伸缩
4. **成本可控**：Token 预算、用量监控、超支告警
5. **开发者友好**：CLI 原生、可脚本化、支持 GitOps

---

## 二、目标用户

### 2.1 主要用户画像

| 画像 | 描述 | 核心需求 |
|------|------|----------|
| **Solo Founder** | 一人公司，用 AI 替代团队 | 同时管理 5-10 个 Agent，成本敏感 |
| **AI Engineer** | 构建 Agent 应用的开发者 | 编排复杂工作流，需要可编程接口 |
| **Tech Lead** | 小团队技术负责人 | 团队共享 Agent 资源，权限管理 |

### 2.2 使用场景

1. **日常自动化**：多个 Agent 并行处理邮件、日程、代码
2. **复杂工作流**：A Agent 完成后触发 B Agent，链式处理
3. **成本优化**：用便宜模型处理简单任务，复杂任务升级模型
4. **团队协作**：多人共享 Agent 池，避免重复配置

---

## 三、核心概念

### 3.1 Agent（类比 Container）

**定义**：可执行 AI 任务的最小单元

**支持类型**：
```yaml
agentTypes:
  - openclaw     # OpenClaw Agent
  - claude-code  # Claude Code CLI
  - codex        # OpenAI Codex CLI
  - cursor       # Cursor Agent
  - custom       # 自定义 Agent（任何支持 stdin/stdout 的程序）
```

**Agent 生命周期**：
```
Created → Starting → Running → Completing → Completed
                 ↓
              Failed → Retrying → Running
                 ↓
            Terminated
```

### 3.2 AgentSpec（类比 Deployment）

**定义**：声明式 Agent 配置，支持灵活的运行时参数

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: code-reviewer
  labels:
    role: reviewer
    project: opc-platform
spec:
  type: claude-code
  
  # ===== 副本与扩缩容 =====
  replicas: 2                    # 副本数量
  scaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetUtilization: 70%       # 负载超过 70% 自动扩容
    scaleDownDelay: 300s         # 缩容冷却时间
  
  # ===== 运行时配置 =====
  runtime:
    # 模型配置（可热更新）
    model:
      provider: anthropic        # anthropic | openai | custom
      name: claude-sonnet-4      # 模型名称
      fallback: claude-haiku-4   # 降级模型
      
    # 推理参数
    inference:
      thinking: low              # off | low | medium | high
      temperature: 0.7
      maxTokens: 8192
      
    # 超时配置
    timeout:
      task: 300s                 # 单任务超时
      idle: 600s                 # 空闲超时（自动休眠）
      startup: 30s               # 启动超时
      
  # ===== 资源配额 =====
  resources:
    tokenBudget:
      perTask: 100000            # 单任务 token 上限
      perHour: 500000            # 每小时 token 上限
      perDay: 2000000            # 每日 token 上限
    costLimit:
      perTask: $1
      perDay: $20
      perMonth: $500
    # 超限策略
    onExceed: pause              # pause | alert | downgrade
    
  # ===== 上下文与记忆 =====
  context:
    workdir: /workspace/opc-platform
    skills:
      - code-review
      - git-operations
      
    # 记忆系统
    memory:
      enabled: true
      path: ./memory/
      # 持久化策略
      persistence:
        type: file               # file | redis | postgres
        syncInterval: 30s        # 同步间隔
        maxSize: 10MB
      # 恢复策略
      recovery:
        enabled: true
        strategy: latest         # latest | checkpoint | manual
        
  # ===== 健康检查 =====
  healthCheck:
    type: heartbeat
    interval: 60s
    timeout: 10s
    retries: 3
    
  # ===== 崩溃恢复 =====
  recovery:
    enabled: true
    maxRestarts: 5               # 最大重启次数
    restartDelay: 10s            # 重启延迟
    backoff: exponential         # linear | exponential
    
    # 记忆恢复
    memoryRecovery:
      enabled: true
      strategy: auto             # auto | manual | skip
      sources:
        - type: checkpoint       # 从检查点恢复
          path: ./checkpoints/
        - type: memory           # 从 memory 文件恢复
          path: ./memory/
        - type: audit            # 从审计日志重建
          lookback: 1h
          
    # 任务恢复
    taskRecovery:
      enabled: true
      # 恢复未完成任务
      resumeIncomplete: true
      # 重放最近的上下文
      replayContext:
        enabled: true
        maxMessages: 50
        
  # ===== 配置热更新 =====
  configHotReload:
    enabled: true
    watchFields:
      - runtime.model
      - runtime.inference
      - resources.tokenBudget
    # 无需重启即可更新的配置
```

**Agent 配置管理命令**：
```bash
# 查看当前配置
opctl config get agent code-reviewer

# 热更新模型（无需重启）
opctl config set agent code-reviewer runtime.model.name=claude-opus-4

# 调整副本数
opctl scale agent code-reviewer --replicas=3

# 更新资源配额
opctl config set agent code-reviewer resources.costLimit.perDay='$50'

# 查看配置历史
opctl config history agent code-reviewer
```

### 3.3 任务层级结构（Goal → Project → Task → Issue）

**设计理念**：像企业管理一样管理 AI 工作，支持完整追溯和审计

```
┌─────────────────────────────────────────────────────────┐
│  Goal（战略目标）                                        │
│  例：构建 OPC Platform MVP                              │
│  ├── Project 1: 用户研究                                │
│  │   ├── Task 1.1: HN 用户调研                         │
│  │   │   ├── Issue 1.1.1: 搜索相关帖子                 │
│  │   │   ├── Issue 1.1.2: 分析用户痛点                 │
│  │   │   └── Issue 1.1.3: 生成报告                     │
│  │   └── Task 1.2: 用户访谈                            │
│  ├── Project 2: MVP 开发                                │
│  │   ├── Task 2.1: Agent 适配器                        │
│  │   └── Task 2.2: CLI 工具                            │
│  └── Project 3: 商业化                                  │
└─────────────────────────────────────────────────────────┘
```

#### 3.3.1 Goal（战略目标）

```yaml
apiVersion: opc/v1
kind: Goal
metadata:
  name: opc-platform-mvp
  labels:
    priority: P0
    quarter: Q1-2026
spec:
  description: "构建 OPC Platform MVP，验证 PMF"
  
  owner: founder
  deadline: 2026-04-30
  
  successCriteria:
    - metric: paying_users
      target: 5
    - metric: nps_score
      target: 50
      
  budget:
    total: $1000
    alert: 80%
    
  audit:
    enabled: true
    retention: 365d
```

#### 3.3.2 Project（项目）

```yaml
apiVersion: opc/v1
kind: Project
metadata:
  name: user-research
  labels:
    goal: opc-platform-mvp
spec:
  description: "用户研究与需求验证"
  
  goalRef: opc-platform-mvp  # 关联 Goal
  
  agents:
    - researcher
    - analyst
    
  budget:
    allocated: $200
    
  timeline:
    start: 2026-03-14
    end: 2026-03-28
```

#### 3.3.3 Task（任务）

```yaml
apiVersion: opc/v1
kind: Task
metadata:
  name: hn-user-research
  labels:
    project: user-research
    goal: opc-platform-mvp
spec:
  description: "HN 用户调研"
  
  projectRef: user-research  # 关联 Project
  agentRef: researcher
  
  input:
    message: "Search HN for AI Agent discussions"
    
  output:
    format: markdown
    destination: 
      - type: file
        path: ./research/hn-findings.md
        
  # 成本事件追踪
  costTracking:
    enabled: true
    labels:
      department: research
      costCenter: mvp-validation
```

#### 3.3.4 Issue（执行单元）

```yaml
apiVersion: opc/v1
kind: Issue
metadata:
  name: search-hn-posts
  labels:
    task: hn-user-research
    project: user-research
    goal: opc-platform-mvp
spec:
  description: "搜索 HN 相关帖子"
  
  taskRef: hn-user-research  # 关联 Task
  agentRef: researcher
  
  input:
    message: "Search HN for 'AI agent orchestration'"
    
  # 执行约束
  constraints:
    timeout: 60s
    maxTokens: 10000
    
  # 成本事件
  costEvent:
    type: execution
    trackTokens: true
    trackTime: true
```

#### 3.3.5 审计追溯

```yaml
apiVersion: opc/v1
kind: AuditLog
metadata:
  name: audit-log-config
spec:
  # 记录所有层级的事件
  capture:
    - goals
    - projects
    - tasks
    - issues
    
  # 事件类型
  events:
    - type: created
    - type: started
    - type: completed
    - type: failed
    - type: cost_incurred
    - type: agent_assigned
    - type: context_transferred
    
  # 存储
  storage:
    type: sqlite  # sqlite | postgres | s3
    path: ~/.opc/audit/
    retention: 365d
    
  # 查询接口
  query:
    enabled: true
    api: /api/v1/audit
```

**审计查询示例**：
```bash
# 查看 Goal 的所有活动
opctl audit goal opc-platform-mvp

# 追溯某个 Issue 的完整链路
opctl audit trace issue search-hn-posts
# 输出：
# Goal: opc-platform-mvp
#   └── Project: user-research
#       └── Task: hn-user-research
#           └── Issue: search-hn-posts
#               ├── Agent: researcher
#               ├── Started: 2026-03-14 10:00:00
#               ├── Completed: 2026-03-14 10:01:23
#               ├── Tokens: 2,340 (in: 1,200 / out: 1,140)
#               └── Cost: $0.023

# 查看成本报告（按层级）
opctl cost report --group-by goal,project
```

#### 3.3.6 Cost Event（成本事件）

```yaml
apiVersion: opc/v1
kind: CostEvent
metadata:
  name: cost-event-001
  timestamp: 2026-03-14T10:01:23Z
spec:
  # 关联层级
  refs:
    goal: opc-platform-mvp
    project: user-research
    task: hn-user-research
    issue: search-hn-posts
    agent: researcher
    
  # 成本详情
  cost:
    tokens:
      input: 1200
      output: 1140
      total: 2340
    price:
      inputPer1k: $0.003
      outputPer1k: $0.015
      total: $0.0207
    duration: 83s
    
  # 模型信息
  model:
    provider: anthropic
    name: claude-sonnet-4
    
  # 标签（用于成本分析）
  labels:
    department: research
    costCenter: mvp-validation
    environment: production
```

**成本追踪命令**：
```bash
# 实时成本
opctl cost watch

# 按 Goal 统计
opctl cost report --by goal

# 按 Agent 统计
opctl cost report --by agent

# 导出成本报告
opctl cost export --format csv --period 30d
```

### 3.4 Workflow（类比 CronJob + DAG）

**定义**：多 Agent 协作的有向无环图

```yaml
apiVersion: opc/v1
kind: Workflow
metadata:
  name: daily-research
spec:
  schedule: "0 7 * * *"  # 每天早上7点
  
  steps:
    - name: research
      agent: researcher
      input:
        message: "Search for AI Agent news"
      outputs:
        - name: findings
          
    - name: analyze
      agent: analyst
      dependsOn: [research]
      input:
        message: "Analyze research findings"
        context:
          - ${{ steps.research.outputs.findings }}
      outputs:
        - name: report
        
    - name: publish
      agent: writer
      dependsOn: [analyze]
      input:
        message: "Write daily report"
        context:
          - ${{ steps.analyze.outputs.report }}
      delivery:
        channel: telegram
```

### 3.5 Gateway（类比 Ingress）

**定义**：用户与 Agent 集群的统一入口

```yaml
apiVersion: opc/v1
kind: Gateway
metadata:
  name: main-gateway
spec:
  channels:
    - telegram
    - discord
    - slack
    - cli
    
  routing:
    default: dispatcher
    rules:
      - match:
          prefix: "/code"
        route: code-reviewer
      - match:
          prefix: "/research"
        route: researcher
          
  rateLimit:
    requests: 100/minute
    tokens: 50000/hour
```

### 3.6 Dispatcher（类比 Scheduler）

**定义**：智能任务分发器

```yaml
apiVersion: opc/v1
kind: Dispatcher
metadata:
  name: smart-dispatcher
spec:
  strategy: auto  # auto | round-robin | least-busy | cost-optimized
  
  routing:
    # 根据任务类型自动选择 Agent
    - match:
        taskType: coding
      agents: [claude-code, codex]
      preference: least-cost
      
    - match:
        taskType: research
      agents: [researcher]
      
    - match:
        taskType: writing
      agents: [writer]
      preference: quality
      
  fallback:
    agent: general-assistant
```

---

## 四、系统架构

### 4.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        用户层                                │
│  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐          │
│  │Telegram│ │Discord│ │ Slack│  │ CLI  │  │ API  │          │
│  └───┬───┘  └───┬───┘  └───┬───┘  └───┬───┘  └───┬───┘      │
│      └──────────┴──────────┴──────────┴──────────┘          │
│                            │                                 │
│  ┌─────────────────────────▼─────────────────────────────┐  │
│  │                     Gateway                            │  │
│  │  • 认证/授权  • 速率限制  • 路由分发                    │  │
│  └─────────────────────────┬─────────────────────────────┘  │
└────────────────────────────┼────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                       控制面板                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  Dispatcher │  │  Scheduler  │  │  Controller │         │
│  │  智能路由    │  │  任务调度    │  │  生命周期   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   Monitor   │  │ Cost Ctrl   │  │  Governance │         │
│  │  监控告警    │  │  成本控制    │  │  权限治理   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                        数据面板                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   State Store                        │   │
│  │  • Agent 状态  • 任务状态  • Context 缓存  • 日志    │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                       Agent 运行时                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ OpenClaw │  │  Claude  │  │  Codex   │  │  Custom  │    │
│  │  Agent   │  │   Code   │  │   CLI    │  │  Agent   │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 组件说明

| 组件 | 职责 | K8s 对应 |
|------|------|----------|
| **Gateway** | 统一入口，认证授权，路由分发 | Ingress Controller |
| **Dispatcher** | 智能任务分发，选择最优 Agent | kube-scheduler |
| **Scheduler** | Cron 任务，工作流调度 | CronJob Controller |
| **Controller** | Agent 生命周期管理，期望状态同步 | Deployment Controller |
| **Monitor** | 指标采集，健康检查，告警 | Prometheus + AlertManager |
| **Cost Ctrl** | Token 计量，预算控制，超支告警 | ResourceQuota |
| **Governance** | 权限管理，审计日志，合规 | RBAC |
| **State Store** | 状态持久化，Context 缓存 | etcd |

---

## 五、核心功能

### 5.1 P0 功能（MVP 必须）

#### 5.1.1 Agent 管理

```bash
# 列出所有 Agent
opctl get agents

# 查看 Agent 详情
opctl describe agent code-reviewer

# 创建 Agent
opctl apply -f agent-spec.yaml

# 删除 Agent
opctl delete agent code-reviewer
```

#### 5.1.2 任务执行

```bash
# 发送任务
opctl run --agent code-reviewer "Review this PR"

# 查看任务状态
opctl get tasks

# 查看任务日志
opctl logs task-123
```

#### 5.1.3 状态监控

```bash
# 集群状态总览
opctl status

# 实时监控
opctl top agents

# Agent 健康检查
opctl health
```

### 5.2 P1 功能（核心体验）

#### 5.2.1 工作流编排

```bash
# 执行工作流
opctl apply -f workflow.yaml
opctl run workflow daily-research

# 查看工作流状态
opctl get workflows
opctl describe workflow daily-research
```

#### 5.2.2 成本控制

```bash
# 查看成本报告
opctl cost report --period 7d

# 设置预算
opctl budget set --daily $10 --monthly $200

# 成本告警
opctl alert create --type cost --threshold $5/day
```

#### 5.2.3 Context 管理

```bash
# Relay/Baton 模式：Agent 间上下文传递
opctl context export task-123 --format baton
opctl context import --from task-123 --to task-456

# 清理过期 context
opctl context gc --older-than 7d
```

### 5.3 P2 功能（差异化）

#### 5.3.1 智能调度

- 根据任务类型自动选择最优 Agent
- 根据成本/质量偏好调整模型
- 负载均衡：多个同类 Agent 分担任务

#### 5.3.2 自愈能力与记忆恢复

**Agent 崩溃恢复流程**：

```
Agent 崩溃
    │
    ▼
┌─────────────────────────────────────┐
│  1. 检测崩溃（健康检查失败）          │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│  2. 保存崩溃现场                     │
│  • 最后执行的 Task/Issue            │
│  • 未完成的输出                      │
│  • 上下文快照                        │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│  3. 重启 Agent                      │
│  • 等待 restartDelay               │
│  • 指数退避（如配置）               │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│  4. 记忆恢复                        │
│  • 加载 memory/ 文件               │
│  • 恢复 checkpoint                 │
│  • 重建近期上下文（从审计日志）      │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│  5. 任务恢复                        │
│  • 检查未完成的 Task/Issue          │
│  • 重放上下文（最近 N 条消息）       │
│  • 继续执行或重新开始               │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│  6. 恢复完成，发送通知               │
└─────────────────────────────────────┘
```

**记忆恢复机制**：

```yaml
apiVersion: opc/v1
kind: MemoryConfig
metadata:
  name: agent-memory-config
spec:
  # 检查点机制
  checkpoint:
    enabled: true
    interval: 5m                 # 每 5 分钟创建检查点
    maxCheckpoints: 10           # 保留最近 10 个检查点
    storage:
      type: file
      path: ./checkpoints/
      
  # 记忆文件
  memoryFiles:
    - path: ./memory/YYYY-MM-DD.md   # 每日记忆
    - path: ./MEMORY.md              # 长期记忆
    
  # 恢复策略
  recovery:
    # 优先级：checkpoint > memory > audit
    priority:
      - checkpoint
      - memory  
      - audit
      
    # 上下文重建
    contextReconstruction:
      enabled: true
      # 从审计日志重建最近对话
      fromAudit:
        lookback: 1h
        maxMessages: 100
      # 重放给 Agent
      replay:
        enabled: true
        format: summary          # full | summary | key-points
        
  # 记忆同步
  sync:
    # Agent 间记忆共享
    crossAgent:
      enabled: false
      sharedMemory: ./shared-memory/
    # 云同步
    cloud:
      enabled: false
      provider: s3
      bucket: opc-memory-backup
```

**崩溃恢复命令**：
```bash
# 查看 Agent 崩溃历史
opctl crashes agent code-reviewer

# 手动触发记忆恢复
opctl recovery agent code-reviewer --from checkpoint

# 查看恢复状态
opctl recovery status agent code-reviewer

# 列出可用检查点
opctl checkpoints list agent code-reviewer

# 从特定检查点恢复
opctl recovery agent code-reviewer --checkpoint cp-20260314-100000
```

**自动恢复通知**：
```yaml
# 恢复完成后发送通知
notifications:
  onRecovery:
    channels:
      - telegram
      - slack
    message: |
      🔄 Agent {{ .AgentName }} 已自动恢复
      • 崩溃时间: {{ .CrashTime }}
      • 恢复时间: {{ .RecoveryTime }}
      • 恢复来源: {{ .RecoverySource }}
      • 未完成任务: {{ .PendingTasks }}
```

**核心能力**：
- Agent 无响应自动重启（健康检查 + 自动重启）
- 任务失败自动重试（指数退避）
- **记忆恢复**：从 checkpoint / memory 文件 / 审计日志恢复上下文
- **任务恢复**：未完成任务自动续接
- 上下文污染自动检测和清理

#### 5.3.3 Goal 智能自动分解

**核心理念**：用户只需声明一个战略目标（Goal），系统自动用 AI 拆解为 Project → Task → Issue，自动分配/创建 Agent，自动开始执行。这是 OPC 作为"AI Agent 的 Kubernetes"的核心价值——**声明期望状态，系统自动实现**。

**Goal 自动分解流程**：

```
用户: opctl goal create --name "构建IM系统" --auto-decompose
  │
  ▼
┌─────────────────────────────────────┐
│  1. 创建 Goal（状态: Decomposing）    │
└──────────────────┬──────────────────┘
                   ▼
┌─────────────────────────────────────┐
│  2. AI 分解引擎（调用 LLM）          │
│  • 构造 Decomposition Prompt        │
│  • 通过 claude --print 调用         │
│  • 解析 JSON → Projects/Tasks/Issues│
│  • 验证结构完整性                    │
└──────────────────┬──────────────────┘
                   ▼
┌─────────────────────────────────────┐
│  3. 分解结果持久化（状态: Planned）    │
│  • 创建 ProjectRecord[]             │
│  • 创建 IssueRecord[]               │
│  • 预估成本和 token 消耗             │
└──────────────────┬──────────────────┘
                   ▼
┌─────────────────────────────────────┐
│  4. 审查确认                         │
│  • opctl goal plan <id>   查看方案   │
│  • opctl goal approve <id> 确认执行  │
│  • opctl goal revise <id>  修改方案  │
│  （或 --auto-approve 跳过此步）      │
└──────────────────┬──────────────────┘
                   ▼
┌─────────────────────────────────────┐
│  5. 自动执行（状态: InProgress）      │
│  • 自动创建/启动 Agent              │
│  • 按依赖顺序异步执行 Task          │
│  • Federation 跨公司分发（如配置）   │
└─────────────────────────────────────┘
```

**Goal YAML 规范（扩展）**：

```yaml
apiVersion: opc/v1
kind: Goal
metadata:
  name: build-messaging-system
spec:
  description: "构建一个支持实时消息、群聊、文件传输的即时通讯系统"
  owner: founder
  deadline: 2026-04-30

  # ===== AI 自动分解配置 =====
  autoDecompose: true           # 启用 AI 自动分解
  approval: required            # required | auto（是否需要人工确认）

  # 分解约束（Guardrails）
  constraints:
    maxProjects: 5              # 最多 5 个 Project
    maxTasksPerProject: 10      # 每个 Project 最多 10 个 Task
    maxAgents: 8                # 最多创建 8 个 Agent
    maxBudget: "$50"            # 分解+执行总预算
    preferredAgentTypes:        # 偏好的 Agent 类型
      - claude-code

  # Federation 公司映射（可选）
  companyMapping:
    "backend-*": software       # 后端相关 Project 分配给 software 公司
    "frontend-*": frontend      # 前端相关 Project 分配给 frontend 公司

  budget:
    total: "$100"
    alert: "80%"
```

**审查命令**：
```bash
# 查看 AI 生成的分解方案
opctl goal plan build-messaging-system

# 确认执行
opctl goal approve build-messaging-system

# 修改后重新提交
opctl goal revise build-messaging-system --file revised-plan.yaml

# 一步到位（带 guardrail 的自动模式）
opctl goal create --name "构建IM系统" --auto-decompose --auto-approve \
  --max-cost $10 --max-agents 5 --max-tasks 20
```

**Guardrails（安全阀）**：
- 最大 project/task/issue 数量限制
- 最大预估成本限制
- 最大 Agent 创建数量限制
- 超过任何 guardrail 自动降级为 Plan 模式（需人工确认）
- 分解成本独立追踪

#### 5.3.4 团队协作

- 多用户共享 Agent 池
- RBAC 权限控制
- 审计日志

---

## 六、Agent 适配器

### 6.1 适配器接口

```go
// AgentAdapter 是所有 Agent 类型的统一接口
type AgentAdapter interface {
    // 生命周期
    Start(ctx context.Context, spec AgentSpec) error
    Stop(ctx context.Context) error
    Health() HealthStatus
    
    // 任务执行
    Execute(ctx context.Context, task Task) (Result, error)
    Stream(ctx context.Context, task Task) (<-chan Chunk, error)
    
    // 状态
    Status() AgentStatus
    Metrics() AgentMetrics
}
```

### 6.2 内置适配器

| Agent 类型 | 启动方式 | 通信协议 |
|-----------|----------|----------|
| **openclaw** | `openclaw agent start` | WebSocket |
| **claude-code** | `claude --print` | stdin/stdout |
| **codex** | `codex --quiet` | stdin/stdout |
| **cursor** | Cursor API | HTTP |
| **custom** | 用户定义 | stdin/stdout / HTTP |

### 6.3 自定义 Agent

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: my-custom-agent
spec:
  type: custom
  
  runtime:
    command: ["python", "my_agent.py"]
    args: ["--mode", "daemon"]
    env:
      OPENAI_API_KEY: ${{ secrets.OPENAI_KEY }}
      
  protocol:
    type: stdio  # stdio | http | websocket
    format: jsonl
    
  healthCheck:
    command: ["python", "my_agent.py", "--health"]
    interval: 30s
```

---

## 七、配置管理

### 7.1 配置文件结构

```
~/.opc/
├── config.yaml           # 全局配置
├── credentials/          # 凭证（加密存储）
│   ├── openai.enc
│   ├── anthropic.enc
│   └── github.enc
├── agents/               # Agent 定义
│   ├── code-reviewer.yaml
│   ├── researcher.yaml
│   └── writer.yaml
├── workflows/            # 工作流定义
│   ├── daily-research.yaml
│   └── pr-review.yaml
└── state/                # 运行时状态
    ├── agents.db
    └── tasks.db
```

### 7.2 全局配置

```yaml
# ~/.opc/config.yaml
apiVersion: opc/v1
kind: Config

cluster:
  name: my-opc-cluster
  
gateway:
  port: 8080
  channels:
    telegram:
      enabled: true
      token: ${{ secrets.TELEGRAM_TOKEN }}
    discord:
      enabled: false
      
defaults:
  agent:
    timeout: 300s
    retries: 3
  cost:
    dailyBudget: $20
    alertThreshold: 80%
    
logging:
  level: info
  output: ~/.opc/logs/
  
telemetry:
  enabled: true
  endpoint: https://telemetry.opc.dev
```

---

## 八、MVP 范围

### 8.1 Phase 1: Foundation（2 周）

**目标**：能跑起来，能管理 Agent

- [ ] opctl CLI 基础框架
- [ ] Agent 适配器：OpenClaw
- [ ] 基础 CRUD：create/get/delete agent
- [ ] 简单任务执行：opctl run
- [ ] 状态存储：SQLite

**交付物**：
```bash
opctl apply -f agent.yaml
opctl get agents
opctl run --agent my-agent "Hello"
opctl logs <task-id>
```

### 8.2 Phase 2: Multi-Agent（2 周）

**目标**：支持多种 Agent 类型

- [ ] Agent 适配器：Claude Code
- [ ] Agent 适配器：Codex
- [ ] 任务状态追踪
- [ ] 基础健康检查

**交付物**：
```bash
opctl get agents
# NAME           TYPE          STATUS    AGE
# coder          claude-code   Running   2d
# reviewer       openclaw      Running   1d
# automation     codex         Running   3h
```

### 8.3 Phase 3: Orchestration（2 周）

**目标**：工作流和调度

- [ ] Workflow 定义和执行
- [ ] Cron 调度
- [ ] Dispatcher 智能路由
- [ ] 依赖管理（DAG）

**交付物**：
```bash
opctl apply -f workflow.yaml
opctl run workflow daily-research
opctl get workflows
```

### 8.4 Phase 4: Production Ready（2 周）

**目标**：可用于生产

- [ ] 成本监控和预算
- [ ] Gateway 集成（Telegram/Discord）
- [ ] 持久化和恢复
- [ ] 基础 Dashboard（Web UI）

---

## 九、成功指标

### 9.1 北极星指标

**月活 Agent 实例数**：用户在 OPC 上运行的 Agent 总数

### 9.2 关键指标

| 指标 | 目标（6个月） |
|------|--------------|
| 注册用户 | 1,000 |
| 月活用户 | 300 |
| 日均任务数 | 10,000 |
| 付费用户 | 50 |
| MRR | $5,000 |

### 9.3 质量指标

| 指标 | 目标 |
|------|------|
| Agent 启动成功率 | > 99% |
| 任务完成率 | > 95% |
| P99 延迟 | < 5s |
| 系统可用性 | > 99.5% |

---

## 十、风险与对策

| 风险 | 可能性 | 影响 | 对策 |
|------|--------|------|------|
| Claude/Codex API 变更 | 中 | 高 | 适配器抽象，快速适配 |
| 成本超支 | 高 | 中 | 硬性预算限制，超支熔断 |
| Agent 协作冲突 | 中 | 中 | Git Worktree 隔离 |
| Context 污染 | 高 | 中 | Relay/Baton 模式 |

---

## 十一、开放问题

1. **定价模式**：按 Agent 数？按任务数？按 Token 用量？
2. **云 vs 本地**：优先 SaaS 还是自托管？
3. **开源策略**：完全开源？核心开源+商业插件？
4. **社区建设**：如何吸引第三方贡献 Agent 适配器？

---

## 附录

### A. 竞品分析

| 产品 | 定位 | 优势 | 劣势 |
|------|------|------|------|
| **Spine Swarm** | 可视化 Agent 协作 | YC 背书，UI 好 | 必须用 GUI，不够灵活 |
| **Stint** | Claude Code 编排 | 简单好用 | 只支持 Claude |
| **LangGraph** | Agent 工作流框架 | 成熟稳定 | 太重，学习成本高 |
| **CrewAI** | 多 Agent 协作 | 社区活跃 | 侧重 Python 生态 |

**OPC 差异化**：
- CLI-first，支持 headless
- 多 Agent 类型（不只是 Claude）
- 声明式配置（YAML，GitOps 友好）
- 成本控制内置

### B. 参考资料

- [Kubernetes 架构](https://kubernetes.io/docs/concepts/architecture/)
- [Borg 论文](https://research.google/pubs/pub43438/)
- [Tarvos Relay Architecture](https://github.com/Photon48/tarvos)
- [Spine Swarm](https://www.getspine.ai)

---

*PRD 版本: v0.1 | 最后更新: 2026-03-14*
