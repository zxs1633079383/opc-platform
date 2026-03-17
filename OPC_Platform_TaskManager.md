# OPC Platform 任务清单
## Task Manager — 完整开发任务追踪

**版本**: v0.1
**创建日期**: 2026-03-14
**目标交付**: 2026-05-14（8 周）
**负责人**: OPC Team

---

## 📊 进度总览

| Phase | 名称 | 周期 | 状态 | 进度 |
|-------|------|------|------|------|
| 1 | Foundation | Week 1-2 | 🟢 已完成 | 100% |
| 2 | Multi-Agent | Week 3-4 | 🟢 已完成 | 100% |
| 3 | Orchestration | Week 5-6 | 🟢 已完成 | 100% |
| 4 | Production Ready | Week 7-8 | 🟢 已完成 | 100% |
| 5 | AI Goal Decomposition | Week 9-10 | 🔵 进行中 | 0% |

**状态图例**：
- ⚪ 未开始
- 🔵 进行中
- 🟡 已阻塞
- 🟢 已完成
- 🔴 已取消

---

## 🎯 Goal: OPC Platform MVP

**成功标准**：
- [ ] 5 个付费用户
- [ ] NPS > 50
- [ ] 系统可用性 > 99.5%

**总预算**: $1,000
**Deadline**: 2026-05-14

---

# Phase 1: Foundation（Week 1-2）

## Project 1.1: opctl CLI 基础框架

### Task 1.1.1: CLI 骨架搭建
**优先级**: P0 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.1.1.1 | 初始化 Go 项目结构 | 🟢 | - |
| 1.1.1.2 | 集成 Cobra CLI 框架 | 🟢 | - |
| 1.1.1.3 | 实现 `opctl version` 命令 | 🟢 | - |
| 1.1.1.4 | 实现 `opctl help` 命令 | 🟢 | - |
| 1.1.1.5 | 配置文件加载 (`~/.opc/config.yaml`) | 🟢 | - |
| 1.1.1.6 | 日志系统集成 | 🟢 | - |

**验收标准**：
- [x] `opctl version` 输出版本信息
- [x] `opctl help` 显示所有可用命令
- [x] 配置文件自动创建和加载

---

### Task 1.1.2: Agent CRUD 命令
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.1.2.1 | 定义 AgentSpec YAML 结构 | 🟢 | - |
| 1.1.2.2 | 实现 `opctl apply -f agent.yaml` | 🟢 | - |
| 1.1.2.3 | 实现 `opctl get agents` | 🟢 | - |
| 1.1.2.4 | 实现 `opctl describe agent <name>` | 🟢 | - |
| 1.1.2.5 | 实现 `opctl delete agent <name>` | 🟢 | - |
| 1.1.2.6 | YAML 校验和错误提示 | 🟢 | - |

**验收标准**：
- [x] 能创建、查看、删除 Agent 配置
- [x] YAML 格式错误有清晰提示
- [x] Agent 状态持久化到本地存储

---

### Task 1.1.3: 状态存储层
**优先级**: P0 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.1.3.1 | 选型：SQLite vs BoltDB | 🟢 | - |
| 1.1.3.2 | 定义数据模型 (Agent, Task, Issue) | 🟢 | - |
| 1.1.3.3 | 实现 CRUD 接口 | 🟢 | - |
| 1.1.3.4 | 数据库迁移机制 | 🟢 | - |
| 1.1.3.5 | 单元测试 | 🟢 | - |

**验收标准**：
- [x] 数据正确持久化到 `~/.opc/state/`
- [x] 重启后数据不丢失
- [x] 测试覆盖率 > 80%

---

## Project 1.2: OpenClaw Agent 适配器

### Task 1.2.1: Agent 适配器接口定义
**优先级**: P0 | **预估**: 1d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.2.1.1 | 定义 AgentAdapter 接口 | 🟢 | - |
| 1.2.1.2 | 定义生命周期方法 (Start/Stop/Health) | 🟢 | - |
| 1.2.1.3 | 定义任务执行方法 (Execute/Stream) | 🟢 | - |
| 1.2.1.4 | 定义状态和指标方法 | 🟢 | - |

**验收标准**：
- [x] 接口定义清晰、可扩展
- [x] 支持同步和流式执行

---

### Task 1.2.2: OpenClaw 适配器实现
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.2.2.1 | 解析 OpenClaw 启动配置 | 🟢 | - |
| 1.2.2.2 | 实现 Start() - 启动 OpenClaw 进程 | 🟢 | - |
| 1.2.2.3 | 实现 Stop() - 优雅关闭 | 🟢 | - |
| 1.2.2.4 | 实现 Health() - 健康检查 | 🟢 | - |
| 1.2.2.5 | 实现 Execute() - 任务执行 | 🟢 | - |
| 1.2.2.6 | 实现 Stream() - 流式输出 | 🟢 | - |
| 1.2.2.7 | WebSocket 通信封装 | 🟢 | - |
| 1.2.2.8 | 错误处理和重试逻辑 | 🟢 | - |

**验收标准**：
- [x] 能启动和停止 OpenClaw Agent
- [x] 能发送任务并获取结果
- [x] 健康检查正常工作

---

## Project 1.3: 任务执行基础

### Task 1.3.1: 任务执行命令
**优先级**: P0 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.3.1.1 | 实现 `opctl run --agent <name> "message"` | 🟢 | - |
| 1.3.1.2 | 实现任务状态追踪 | 🟢 | - |
| 1.3.1.3 | 实现 `opctl get tasks` | 🟢 | - |
| 1.3.1.4 | 实现 `opctl logs <task-id>` | 🟢 | - |
| 1.3.1.5 | 实时输出流式显示 | 🟢 | - |

**验收标准**：
- [x] 能发送任务到指定 Agent
- [x] 能查看任务状态和日志
- [x] 流式输出实时显示

---

### Task 1.3.2: 基础监控
**优先级**: P1 | **预估**: 1d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 1.3.2.1 | 实现 `opctl status` 集群状态 | 🟢 | - |
| 1.3.2.2 | 实现 `opctl top agents` 实时监控 | 🟢 | - |
| 1.3.2.3 | 实现 `opctl health` 健康检查 | 🟢 | - |

**验收标准**：
- [x] 一眼看到所有 Agent 状态
- [x] 实时更新资源使用情况

---

## Phase 1 交付物清单

- [x] `opctl` CLI 可执行文件
- [x] 基础命令：`apply`, `get`, `describe`, `delete`, `run`, `logs`, `status`
- [x] OpenClaw Agent 适配器
- [x] SQLite 状态存储
- [x] 单元测试 (覆盖率 > 80%)
- [ ] README 文档

---

# Phase 2: Multi-Agent（Week 3-4）

## Project 2.1: Claude Code 适配器

### Task 2.1.1: Claude Code 集成
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 2.1.1.1 | 研究 Claude Code CLI 接口 | 🟢 | - |
| 2.1.1.2 | 实现 ClaudeCodeAdapter | 🟢 | - |
| 2.1.1.3 | 处理 `--print` 模式输出 | 🟢 | - |
| 2.1.1.4 | 处理权限绕过模式 | 🟢 | - |
| 2.1.1.5 | 实现健康检查 | 🟢 | - |
| 2.1.1.6 | 集成测试 | 🟢 | - |

**验收标准**：
- [x] 能通过 OPC 启动 Claude Code
- [x] 能发送任务并获取结果
- [x] 错误处理完善

---

## Project 2.2: Codex 适配器

### Task 2.2.1: Codex CLI 集成
**优先级**: P0 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 2.2.1.1 | 研究 Codex CLI 接口 | 🟢 | - |
| 2.2.1.2 | 实现 CodexAdapter | 🟢 | - |
| 2.2.1.3 | 处理安静模式输出 | 🟢 | - |
| 2.2.1.4 | 集成测试 | 🟢 | - |

**验收标准**：
- [x] 能通过 OPC 启动 Codex
- [x] 能发送任务并获取结果

---

## Project 2.3: 自定义 Agent 支持

### Task 2.3.1: Custom Agent 框架
**优先级**: P1 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 2.3.1.1 | 定义 custom agent YAML 规范 | 🟢 | - |
| 2.3.1.2 | 实现 stdin/stdout 通信 | 🟢 | - |
| 2.3.1.3 | 实现 HTTP 通信 | 🟢 | - |
| 2.3.1.4 | 实现 JSONL 协议解析 | 🟢 | - |
| 2.3.1.5 | 文档和示例 | 🟢 | - |

**验收标准**：
- [x] 用户可以接入任意自定义 Agent
- [x] 提供 Python/Node.js 示例

---

## Project 2.4: Agent 生命周期管理

### Task 2.4.1: 生命周期控制器
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 2.4.1.1 | 实现 Agent 状态机 | 🟢 | - |
| 2.4.1.2 | 实现健康检查循环 | 🟢 | - |
| 2.4.1.3 | 实现自动重启逻辑 | 🟢 | - |
| 2.4.1.4 | 实现指数退避重试 | 🟢 | - |
| 2.4.1.5 | 实现 `opctl restart agent <name>` | 🟢 | - |

**验收标准**：
- [x] Agent 崩溃后自动重启
- [x] 重启次数有上限
- [x] 健康检查超时触发重启

---

### Task 2.4.2: 记忆恢复系统
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 2.4.2.1 | 实现 Checkpoint 机制 | 🟢 | - |
| 2.4.2.2 | 实现 Memory 文件同步 | 🟢 | - |
| 2.4.2.3 | 实现崩溃现场保存 | 🟢 | - |
| 2.4.2.4 | 实现恢复时上下文重建 | 🟢 | - |
| 2.4.2.5 | 实现 `opctl recovery` 命令 | 🟢 | - |
| 2.4.2.6 | 实现 `opctl checkpoints list` | 🟢 | - |

**验收标准**：
- [x] Agent 崩溃后记忆不丢失
- [x] 可以从 checkpoint 恢复
- [x] 未完成任务自动续接

---

## Phase 2 交付物清单

- [x] Claude Code 适配器
- [x] Codex 适配器
- [x] Custom Agent 支持
- [x] 生命周期管理器
- [x] 记忆恢复系统
- [x] 集成测试套件

---

# Phase 3: Orchestration（Week 5-6）

## Project 3.1: 任务层级结构

### Task 3.1.1: Goal/Project/Task/Issue 模型
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.1.1.1 | 定义 Goal YAML 规范 | 🟢 | - |
| 3.1.1.2 | 定义 Project YAML 规范 | 🟢 | - |
| 3.1.1.3 | 定义 Task YAML 规范 | 🟢 | - |
| 3.1.1.4 | 定义 Issue YAML 规范 | 🟢 | - |
| 3.1.1.5 | 实现层级关系存储 | 🟢 | - |
| 3.1.1.6 | 实现 `opctl get goals/projects/tasks/issues` | 🟢 | - |

**验收标准**：
- [x] 支持 4 层任务结构
- [x] 层级关系可追溯

---

### Task 3.1.2: 审计追溯系统
**优先级**: P0 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.1.2.1 | 定义审计事件类型 | 🟢 | - |
| 3.1.2.2 | 实现审计日志记录 | 🟢 | - |
| 3.1.2.3 | 实现 `opctl audit <resource>` | 🟢 | - |
| 3.1.2.4 | 实现 `opctl audit trace <issue>` | 🟢 | - |
| 3.1.2.5 | 审计日志导出 | 🟢 | - |

**验收标准**：
- [x] 所有操作有审计日志
- [x] 可以追溯完整链路

---

## Project 3.2: Workflow 引擎

### Task 3.2.1: Workflow 定义与执行
**优先级**: P0 | **预估**: 4d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.2.1.1 | 定义 Workflow YAML 规范 | 🟢 | - |
| 3.2.1.2 | 实现 DAG 解析器 | 🟢 | - |
| 3.2.1.3 | 实现依赖管理 | 🟢 | - |
| 3.2.1.4 | 实现并行执行 | 🟢 | - |
| 3.2.1.5 | 实现上下文传递 (${{ steps.x.outputs }}) | 🟢 | - |
| 3.2.1.6 | 实现 `opctl run workflow <name>` | 🟢 | - |
| 3.2.1.7 | 实现 `opctl get workflows` | 🟢 | - |

**验收标准**：
- [x] 支持多步骤工作流
- [x] 支持步骤间依赖
- [x] 支持并行执行

---

### Task 3.2.2: Cron 调度
**优先级**: P1 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.2.2.1 | 实现 Cron 表达式解析 | 🟢 | - |
| 3.2.2.2 | 实现调度器 | 🟢 | - |
| 3.2.2.3 | 实现 `opctl cron list` | 🟢 | - |
| 3.2.2.4 | 实现 `opctl cron enable/disable` | 🟢 | - |

**验收标准**：
- [x] Workflow 可以定时执行
- [x] 支持时区配置

---

## Project 3.3: Dispatcher 智能调度

### Task 3.3.1: 智能路由
**优先级**: P1 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.3.1.1 | 定义 Dispatcher YAML 规范 | 🟢 | - |
| 3.3.1.2 | 实现任务类型识别 | 🟢 | - |
| 3.3.1.3 | 实现 Round-robin 策略 | 🟢 | - |
| 3.3.1.4 | 实现 Least-busy 策略 | 🟢 | - |
| 3.3.1.5 | 实现 Cost-optimized 策略 | 🟢 | - |
| 3.3.1.6 | 实现 Fallback 机制 | 🟢 | - |

**验收标准**：
- [x] 自动选择最优 Agent
- [x] 支持多种调度策略

---

## Phase 3 交付物清单

- [x] Goal/Project/Task/Issue 层级结构
- [x] 审计追溯系统
- [x] Workflow 引擎
- [x] Cron 调度器
- [x] Dispatcher 智能路由

---

# Phase 4: Production Ready（Week 7-8）

## Project 4.1: 成本控制

### Task 4.1.1: Cost Event 系统
**优先级**: P0 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.1.1.1 | 定义 CostEvent 数据模型 | 🟢 | - |
| 4.1.1.2 | 实现 Token 计量 | 🟢 | - |
| 4.1.1.3 | 实现成本计算 | 🟢 | - |
| 4.1.1.4 | 实现 `opctl cost report` | 🟢 | - |
| 4.1.1.5 | 实现按 Goal/Project/Agent 分组 | 🟢 | - |
| 4.1.1.6 | 实现 `opctl cost export --csv` | 🟢 | - |

**验收标准**：
- [x] 每个任务都有成本记录
- [x] 可以按层级查看成本

---

### Task 4.1.2: 预算控制
**优先级**: P0 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.1.2.1 | 实现预算配置 | 🟢 | - |
| 4.1.2.2 | 实现 `opctl budget set` | 🟢 | - |
| 4.1.2.3 | 实现超支告警 | 🟢 | - |
| 4.1.2.4 | 实现超支熔断 | 🟢 | - |

**验收标准**：
- [x] 可以设置每日/每月预算
- [x] 超支自动暂停

---

## Project 4.2: Gateway 集成

### Task 4.2.1: Telegram Channel
**优先级**: P1 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.2.1.1 | 实现 Telegram Bot 接入 | 🟢 | - |
| 4.2.1.2 | 实现消息路由到 Dispatcher | 🟢 | - |
| 4.2.1.3 | 实现结果回传 | 🟢 | - |
| 4.2.1.4 | 实现通知推送 | 🟢 | - |

**验收标准**：
- [x] 通过 Telegram 可以下发任务
- [x] 结果自动回传到 Telegram

---

### Task 4.2.2: Discord Channel
**优先级**: P2 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.2.2.1 | 实现 Discord Bot 接入 | 🟢 | - |
| 4.2.2.2 | 实现消息路由 | 🟢 | - |
| 4.2.2.3 | 实现结果回传 | 🟢 | - |

---

## Project 4.3: Agent 配置热更新

### Task 4.3.1: 配置热更新
**优先级**: P1 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.3.1.1 | 实现 `opctl config set` | 🟢 | - |
| 4.3.1.2 | 实现配置变更检测 | 🟢 | - |
| 4.3.1.3 | 实现热更新（无需重启） | 🟢 | - |
| 4.3.1.4 | 实现 `opctl config history` | 🟢 | - |

**验收标准**：
- [x] 模型配置可以热更新
- [x] 配置变更有历史记录

---

### Task 4.3.2: 扩缩容
**优先级**: P1 | **预估**: 2d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.3.2.1 | 实现 `opctl scale agent --replicas` | 🟢 | - |
| 4.3.2.2 | 实现自动扩缩容 | 🟢 | - |
| 4.3.2.3 | 实现负载均衡 | 🟢 | - |

**验收标准**：
- [x] 可以手动扩缩容
- [x] 负载高时自动扩容

---

## Project 4.4: Dashboard（Web UI）

### Task 4.4.1: 基础 Dashboard
**优先级**: P2 | **预估**: 3d | **状态**: 🟢

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.4.1.1 | 选型：Next.js / SvelteKit | 🟢 | - |
| 4.4.1.2 | Agent 状态总览页 | 🟢 | - |
| 4.4.1.3 | 任务列表页 | 🟢 | - |
| 4.4.1.4 | 成本报表页 | 🟢 | - |
| 4.4.1.5 | 实时日志页 | 🟢 | - |

**验收标准**：
- [x] 基础 Dashboard 可用
- [x] 实时数据更新

---

## Phase 4 交付物清单

- [x] Cost Event 系统
- [x] 预算控制和告警
- [x] Telegram Gateway
- [x] Discord Gateway
- [x] 配置热更新
- [x] 扩缩容功能
- [x] 基础 Dashboard (Next.js)

---

# 📋 完整命令清单

## Agent 管理
```bash
opctl apply -f agent.yaml          # 创建/更新 Agent
opctl get agents                    # 列出所有 Agent
opctl describe agent <name>         # Agent 详情
opctl delete agent <name>           # 删除 Agent
opctl restart agent <name>          # 重启 Agent
opctl scale agent <name> --replicas=3  # 扩缩容
```

## 任务执行
```bash
opctl run --agent <name> "message"  # 执行任务
opctl get tasks                     # 列出任务
opctl logs <task-id>                # 查看日志
opctl get goals/projects/tasks/issues  # 层级查询
```

## Workflow
```bash
opctl apply -f workflow.yaml        # 创建工作流
opctl run workflow <name>           # 执行工作流
opctl get workflows                 # 列出工作流
opctl cron list                     # 查看定时任务
```

## 监控
```bash
opctl status                        # 集群状态
opctl top agents                    # 实时监控
opctl health                        # 健康检查
```

## 成本
```bash
opctl cost report                   # 成本报告
opctl cost report --by goal         # 按 Goal 统计
opctl budget set --daily $10        # 设置预算
opctl cost export --csv             # 导出报告
```

## 配置
```bash
opctl config get agent <name>       # 查看配置
opctl config set agent <name> key=value  # 更新配置
opctl config history agent <name>   # 配置历史
```

## 恢复
```bash
opctl recovery agent <name>         # 触发恢复
opctl checkpoints list agent <name> # 列出检查点
opctl crashes agent <name>          # 崩溃历史
```

## 审计
```bash
opctl audit goal <name>             # 审计 Goal
opctl audit trace issue <name>      # 追溯链路
opctl audit export --format json    # 导出审计日志
```

---

# Phase 5: AI Goal Decomposition（Week 9-10）

## Project 5.1: Goal AI 分解引擎

### Task 5.1.1: 类型系统扩展
**优先级**: P0 | **预估**: 0.5d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.1.1.1 | GoalBody 新增 AutoDecompose/Constraints/Approval 字段 | ⚪ | - |
| 5.1.1.2 | GoalRecord 新增 Phase/DecompositionPlan 字段 | ⚪ | - |
| 5.1.1.3 | goal.go 新增 GoalPhase 状态枚举 | ⚪ | - |
| 5.1.1.4 | DecomposeResult 扩展（Complexity/DependsOn/AgentSuggestion） | ⚪ | - |

**验收标准**：
- [ ] 新字段向后兼容（AutoDecompose 默认 false）
- [ ] 编译通过

---

### Task 5.1.2: Decomposer 接口重构
**优先级**: P0 | **预估**: 0.5d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.1.2.1 | 将 Decomposer 改为 interface | ⚪ | - |
| 5.1.2.2 | 当前实现改名为 StaticDecomposer | ⚪ | - |

**验收标准**：
- [ ] 现有代码不受影响
- [ ] 编译通过

---

### Task 5.1.3: AI Decomposer 实现
**优先级**: P0 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.1.3.1 | 新建 pkg/goal/ai_decomposer.go | ⚪ | - |
| 5.1.3.2 | 新建 pkg/goal/prompt.go（Prompt 模板） | ⚪ | - |
| 5.1.3.3 | 通过 Controller 调用 claude agent 执行分解 | ⚪ | - |
| 5.1.3.4 | JSON 结构化输出解析和验证 | ⚪ | - |
| 5.1.3.5 | 分解成本独立追踪 | ⚪ | - |
| 5.1.3.6 | 单元测试（mock LLM 响应） | ⚪ | - |

**验收标准**：
- [ ] AI 能根据 Goal 描述生成合理的 Project/Task/Issue 层级
- [ ] JSON 解析失败有重试机制（最多 2 次）
- [ ] 测试覆盖率 > 80%

---

### Task 5.1.4: server.go Goal 流程改造
**优先级**: P0 | **预估**: 1d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.1.4.1 | KindGoal 分支支持 autoDecompose 检测 | ⚪ | - |
| 5.1.4.2 | 调用 AIDecomposer → 持久化分解结果 | ⚪ | - |
| 5.1.4.3 | 提取现有 decomposition 逻辑为 executeDecomposition() | ⚪ | - |
| 5.1.4.4 | 自动创建 Agent + 异步执行（复用现有逻辑） | ⚪ | - |

**验收标准**：
- [ ] autoDecompose=false 走原有流程（向后兼容）
- [ ] autoDecompose=true 走 AI 分解流程

---

## Project 5.2: Plan 审查流程

### Task 5.2.1: CLI 新增命令
**优先级**: P0 | **预估**: 1d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.2.1.1 | goal create 新增 --auto-decompose flag | ⚪ | - |
| 5.2.1.2 | 实现 opctl goal plan <id> 查看分解方案 | ⚪ | - |
| 5.2.1.3 | 实现 opctl goal approve <id> 确认执行 | ⚪ | - |
| 5.2.1.4 | 实现 opctl goal revise <id> --file 修改方案 | ⚪ | - |

**验收标准**：
- [ ] Plan 模式下创建 Goal 不会立即执行
- [ ] approve 后才触发 Agent 创建和任务执行

---

### Task 5.2.2: API 端点
**优先级**: P0 | **预估**: 1d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.2.2.1 | GET /api/goals/:id/plan 查看分解方案 | ⚪ | - |
| 5.2.2.2 | POST /api/goals/:id/approve 确认执行 | ⚪ | - |
| 5.2.2.3 | POST /api/goals/:id/revise 修改方案 | ⚪ | - |

**验收标准**：
- [ ] API 响应包含完整的分解层级树
- [ ] approve 触发执行流程

---

## Project 5.3: Guardrails 安全阀

### Task 5.3.1: 分解约束和自动模式
**优先级**: P1 | **预估**: 1d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 5.3.1.1 | 实现 DecomposeConstraints 校验 | ⚪ | - |
| 5.3.1.2 | 实现 --auto-approve 模式 | ⚪ | - |
| 5.3.1.3 | 超限自动降级为 Plan 模式 | ⚪ | - |
| 5.3.1.4 | 分解成本预估 | ⚪ | - |

**验收标准**：
- [ ] 超过 maxProjects/maxTasks/maxAgents 限制时自动降级
- [ ] --auto-approve 模式下带 guardrail 直接执行

---

## Phase 5 交付物清单

- [ ] AI Goal 分解引擎（AIDecomposer）
- [ ] Plan → Approve → Execute 三段式流程
- [ ] Guardrails 安全阀
- [ ] CLI 命令：goal plan/approve/revise
- [ ] API 端点：plan/approve/revise
- [ ] 单元测试

---

# Phase 5b: v0.5 TODO 落地（Week 10-11）

## Project 5.4: P0 紧急修复

### Task 5.4.1: OpenClaw Execute 结果存储
**优先级**: P0 | **预估**: 0.5h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.4.1.1 | openclaw Execute() 返回 result 写入 TaskRecord.Result | ⚪ |

### Task 5.4.2: Workflow Toggle 端点
**优先级**: P0 | **预估**: 0.5h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.4.2.1 | 后端 PUT /api/workflows/:name/toggle handler | ⚪ |

### Task 5.4.3: Federation 认证
**优先级**: P0 | **预估**: 3h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.4.3.1 | 新建 pkg/federation/auth.go (HMAC-SHA256) | ⚪ |
| 5.4.3.2 | Company 注册时生成 APIKey | ⚪ |
| 5.4.3.3 | server.go federation auth middleware | ⚪ |

## Project 5.5: P1 核心完善

### Task 5.5.1: OpenClaw 密钥持久化 + 自动读 Token
**优先级**: P1 | **预估**: 2h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.5.1.1 | loadOrCreateIdentity() 持久化到 ~/.opc/identity/ | ⚪ |
| 5.5.1.2 | 自动读取 ~/.openclaw/openclaw.json gateway token | ⚪ |

### Task 5.5.2: Goal 分解持久化 + 成本追踪
**优先级**: P1 | **预估**: 1h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.5.2.1 | DecompositionPlan JSON 写入 GoalRecord | ⚪ |
| 5.5.2.2 | DecomposeCost 字段写入 | ⚪ |

### Task 5.5.3: Federation 断线重试
**优先级**: P1 | **预估**: 3h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.5.3.1 | 新建 pkg/federation/retry.go (RetryQueue) | ⚪ |
| 5.5.3.2 | callback 失败入队 + 指数退避重试 | ⚪ |
| 5.5.3.3 | federation_retry_queue 表 | ⚪ |

### Task 5.5.4: Workflow 执行详情
**优先级**: P1 | **预估**: 3h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.5.4.1 | 后端 GET /api/workflows/:name/runs + runs/:id | ⚪ |
| 5.5.4.2 | 前端 WorkflowRunDetail 组件 | ⚪ |

### Task 5.5.5: 远程 Goal 自动分解
**优先级**: P1 | **预估**: 2h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.5.5.1 | 远程 OPC 收到 federated goal 后触发 AI 分解 | ⚪ |

### Task 5.5.6: Milestone 推送通知
**优先级**: P1 | **预估**: 1.5h | **状态**: ⚪
| Issue | 描述 | 状态 |
|-------|------|------|
| 5.5.6.1 | project 完成时 notifyMilestone() 回调主 OPC | ⚪ |

## Project 5.6: P2 体验增强

### Task 5.6.1: CLI goal plan 树形输出
**优先级**: P2 | **预估**: 1h | **状态**: ⚪

### Task 5.6.2: Dashboard Goals 分解可视化
**优先级**: P2 | **预估**: 2h | **状态**: ⚪

### Task 5.6.3: Workflow 编辑功能
**优先级**: P2 | **预估**: 4h | **状态**: ⚪

### Task 5.6.4: 迭代分解
**优先级**: P2 | **预估**: 6h | **状态**: ⚪

### Task 5.6.5: 智能路由（按公司类型）
**优先级**: P2 | **预估**: 2h | **状态**: ⚪

### Task 5.6.6: 跨公司依赖管理
**优先级**: P2 | **预估**: 4h | **状态**: ⚪

### Task 5.6.7: Federation 远程公司原生渲染
**优先级**: P2 | **预估**: 2h | **状态**: ⚪

---

# 📅 里程碑

| 里程碑 | 日期 | 交付物 |
|--------|------|--------|
| M1: Foundation | Week 2 | opctl CLI + OpenClaw 适配器 |
| M2: Multi-Agent | Week 4 | Claude/Codex 支持 + 记忆恢复 |
| M3: Orchestration | Week 6 | Workflow + Dispatcher |
| M4: Production | Week 8 | 成本控制 + Gateway + Dashboard |
| **MVP Launch** | Week 8 | 公开发布 |
| M5: AI Goal Decomposition | Week 10 | Goal 智能分解 + Plan 审查 + Guardrails |

---

# 🚨 风险登记

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| Claude API 变更 | 中 | 高 | 适配器抽象，快速适配 |
| 开发资源不足 | 中 | 高 | P2 功能延后 |
| 用户需求偏差 | 中 | 中 | 每周用户访谈 |
| 技术债务累积 | 高 | 中 | 每个 Phase 预留重构时间 |
| AI 分解质量不稳定 | 高 | 高 | Few-shot examples + JSON 校验 + Plan 模式 |
| 分解成本失控 | 中 | 中 | 独立预算 + Guardrails + 便宜模型优先 |
| 自动创建 Agent 安全风险 | 中 | 高 | 最大 Agent 数限制 + 白名单机制 |

---

*最后更新: 2026-03-17*
*下次 Review: 每周一 10:00*
