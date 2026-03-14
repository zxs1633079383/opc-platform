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
| 3 | Orchestration | Week 5-6 | 🔵 进行中 | 0% |
| 4 | Production Ready | Week 7-8 | ⚪ 未开始 | 0% |

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
**优先级**: P0 | **预估**: 3d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.1.1.1 | 定义 Goal YAML 规范 | ⚪ | - |
| 3.1.1.2 | 定义 Project YAML 规范 | ⚪ | - |
| 3.1.1.3 | 定义 Task YAML 规范 | ⚪ | - |
| 3.1.1.4 | 定义 Issue YAML 规范 | ⚪ | - |
| 3.1.1.5 | 实现层级关系存储 | ⚪ | - |
| 3.1.1.6 | 实现 `opctl get goals/projects/tasks/issues` | ⚪ | - |

**验收标准**：
- [ ] 支持 4 层任务结构
- [ ] 层级关系可追溯

---

### Task 3.1.2: 审计追溯系统
**优先级**: P0 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.1.2.1 | 定义审计事件类型 | ⚪ | - |
| 3.1.2.2 | 实现审计日志记录 | ⚪ | - |
| 3.1.2.3 | 实现 `opctl audit <resource>` | ⚪ | - |
| 3.1.2.4 | 实现 `opctl audit trace <issue>` | ⚪ | - |
| 3.1.2.5 | 审计日志导出 | ⚪ | - |

**验收标准**：
- [ ] 所有操作有审计日志
- [ ] 可以追溯完整链路

---

## Project 3.2: Workflow 引擎

### Task 3.2.1: Workflow 定义与执行
**优先级**: P0 | **预估**: 4d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.2.1.1 | 定义 Workflow YAML 规范 | ⚪ | - |
| 3.2.1.2 | 实现 DAG 解析器 | ⚪ | - |
| 3.2.1.3 | 实现依赖管理 | ⚪ | - |
| 3.2.1.4 | 实现并行执行 | ⚪ | - |
| 3.2.1.5 | 实现上下文传递 (${{ steps.x.outputs }}) | ⚪ | - |
| 3.2.1.6 | 实现 `opctl run workflow <name>` | ⚪ | - |
| 3.2.1.7 | 实现 `opctl get workflows` | ⚪ | - |

**验收标准**：
- [ ] 支持多步骤工作流
- [ ] 支持步骤间依赖
- [ ] 支持并行执行

---

### Task 3.2.2: Cron 调度
**优先级**: P1 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.2.2.1 | 实现 Cron 表达式解析 | ⚪ | - |
| 3.2.2.2 | 实现调度器 | ⚪ | - |
| 3.2.2.3 | 实现 `opctl cron list` | ⚪ | - |
| 3.2.2.4 | 实现 `opctl cron enable/disable` | ⚪ | - |

**验收标准**：
- [ ] Workflow 可以定时执行
- [ ] 支持时区配置

---

## Project 3.3: Dispatcher 智能调度

### Task 3.3.1: 智能路由
**优先级**: P1 | **预估**: 3d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 3.3.1.1 | 定义 Dispatcher YAML 规范 | ⚪ | - |
| 3.3.1.2 | 实现任务类型识别 | ⚪ | - |
| 3.3.1.3 | 实现 Round-robin 策略 | ⚪ | - |
| 3.3.1.4 | 实现 Least-busy 策略 | ⚪ | - |
| 3.3.1.5 | 实现 Cost-optimized 策略 | ⚪ | - |
| 3.3.1.6 | 实现 Fallback 机制 | ⚪ | - |

**验收标准**：
- [ ] 自动选择最优 Agent
- [ ] 支持多种调度策略

---

## Phase 3 交付物清单

- [ ] Goal/Project/Task/Issue 层级结构
- [ ] 审计追溯系统
- [ ] Workflow 引擎
- [ ] Cron 调度器
- [ ] Dispatcher 智能路由

---

# Phase 4: Production Ready（Week 7-8）

## Project 4.1: 成本控制

### Task 4.1.1: Cost Event 系统
**优先级**: P0 | **预估**: 3d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.1.1.1 | 定义 CostEvent 数据模型 | ⚪ | - |
| 4.1.1.2 | 实现 Token 计量 | ⚪ | - |
| 4.1.1.3 | 实现成本计算 | ⚪ | - |
| 4.1.1.4 | 实现 `opctl cost report` | ⚪ | - |
| 4.1.1.5 | 实现按 Goal/Project/Agent 分组 | ⚪ | - |
| 4.1.1.6 | 实现 `opctl cost export --csv` | ⚪ | - |

**验收标准**：
- [ ] 每个任务都有成本记录
- [ ] 可以按层级查看成本

---

### Task 4.1.2: 预算控制
**优先级**: P0 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.1.2.1 | 实现预算配置 | ⚪ | - |
| 4.1.2.2 | 实现 `opctl budget set` | ⚪ | - |
| 4.1.2.3 | 实现超支告警 | ⚪ | - |
| 4.1.2.4 | 实现超支熔断 | ⚪ | - |

**验收标准**：
- [ ] 可以设置每日/每月预算
- [ ] 超支自动暂停

---

## Project 4.2: Gateway 集成

### Task 4.2.1: Telegram Channel
**优先级**: P1 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.2.1.1 | 实现 Telegram Bot 接入 | ⚪ | - |
| 4.2.1.2 | 实现消息路由到 Dispatcher | ⚪ | - |
| 4.2.1.3 | 实现结果回传 | ⚪ | - |
| 4.2.1.4 | 实现通知推送 | ⚪ | - |

**验收标准**：
- [ ] 通过 Telegram 可以下发任务
- [ ] 结果自动回传到 Telegram

---

### Task 4.2.2: Discord Channel
**优先级**: P2 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.2.2.1 | 实现 Discord Bot 接入 | ⚪ | - |
| 4.2.2.2 | 实现消息路由 | ⚪ | - |
| 4.2.2.3 | 实现结果回传 | ⚪ | - |

---

## Project 4.3: Agent 配置热更新

### Task 4.3.1: 配置热更新
**优先级**: P1 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.3.1.1 | 实现 `opctl config set` | ⚪ | - |
| 4.3.1.2 | 实现配置变更检测 | ⚪ | - |
| 4.3.1.3 | 实现热更新（无需重启） | ⚪ | - |
| 4.3.1.4 | 实现 `opctl config history` | ⚪ | - |

**验收标准**：
- [ ] 模型配置可以热更新
- [ ] 配置变更有历史记录

---

### Task 4.3.2: 扩缩容
**优先级**: P1 | **预估**: 2d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.3.2.1 | 实现 `opctl scale agent --replicas` | ⚪ | - |
| 4.3.2.2 | 实现自动扩缩容 | ⚪ | - |
| 4.3.2.3 | 实现负载均衡 | ⚪ | - |

**验收标准**：
- [ ] 可以手动扩缩容
- [ ] 负载高时自动扩容

---

## Project 4.4: Dashboard（Web UI）

### Task 4.4.1: 基础 Dashboard
**优先级**: P2 | **预估**: 3d | **状态**: ⚪

| Issue | 描述 | 状态 | 负责人 |
|-------|------|------|--------|
| 4.4.1.1 | 选型：Next.js / SvelteKit | ⚪ | - |
| 4.4.1.2 | Agent 状态总览页 | ⚪ | - |
| 4.4.1.3 | 任务列表页 | ⚪ | - |
| 4.4.1.4 | 成本报表页 | ⚪ | - |
| 4.4.1.5 | 实时日志页 | ⚪ | - |

**验收标准**：
- [ ] 基础 Dashboard 可用
- [ ] 实时数据更新

---

## Phase 4 交付物清单

- [ ] Cost Event 系统
- [ ] 预算控制和告警
- [ ] Telegram Gateway
- [ ] 配置热更新
- [ ] 扩缩容功能
- [ ] 基础 Dashboard

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

# 📅 里程碑

| 里程碑 | 日期 | 交付物 |
|--------|------|--------|
| M1: Foundation | Week 2 | opctl CLI + OpenClaw 适配器 |
| M2: Multi-Agent | Week 4 | Claude/Codex 支持 + 记忆恢复 |
| M3: Orchestration | Week 6 | Workflow + Dispatcher |
| M4: Production | Week 8 | 成本控制 + Gateway + Dashboard |
| **MVP Launch** | Week 8 | 公开发布 |

---

# 🚨 风险登记

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| Claude API 变更 | 中 | 高 | 适配器抽象，快速适配 |
| 开发资源不足 | 中 | 高 | P2 功能延后 |
| 用户需求偏差 | 中 | 中 | 每周用户访谈 |
| 技术债务累积 | 高 | 中 | 每个 Phase 预留重构时间 |

---

*最后更新: 2026-03-14*
*下次 Review: 每周一 10:00*
