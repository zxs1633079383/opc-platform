# OPC Platform

**AI Agent 的 Kubernetes** — 让一个人能像管理容器集群一样管理 AI Agent 集群。

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Status](https://img.shields.io/badge/Status-Alpha-orange)]
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## What is OPC Platform?

OPC Platform 是一个统一的 AI Agent 编排平台。就像 Kubernetes 管理容器一样，OPC Platform 让你用声明式 YAML 配置来管理多种 AI Agent（Claude Code、Codex、OpenClaw、自定义 Agent），实现自动化工作流、成本控制和智能调度。

| K8s 概念 | OPC 对应 |
|----------|----------|
| Docker 容器 | AI Agent（OpenClaw / Claude Code / Codex / 自定义） |
| Kubernetes | OPC Platform |
| Pod | Agent Instance（运行中的 Agent 实例） |
| Deployment | AgentSpec（Agent 配置定义） |
| kubectl | `opctl`（命令行工具） |
| CronJob | Workflow（定时工作流） |
| kube-scheduler | Dispatcher（智能调度） |

## Features

### Agent Management
- **多类型支持** — Claude Code、Codex、OpenClaw、Cursor、自定义 Agent
- **声明式配置** — YAML 定义期望状态，平台自动实现
- **生命周期管理** — 启动、停止、重启、扩缩容
- **健康检查** — 自动检测故障，触发自愈

### Workflow Engine
- **DAG 工作流** — 多步骤有向无环图编排
- **并行执行** — 无依赖的步骤自动并行
- **上下文传递** — `${{ steps.x.outputs.y }}` 变量替换
- **Cron 调度** — 支持 5 字段 Cron 表达式定时执行

### Intelligent Dispatch
- **Round-Robin** — 轮询分配
- **Least-Busy** — 最少任务优先
- **Cost-Optimized** — 成本最优选择
- **Fallback** — 自动降级机制

### Crash Recovery
- **Checkpoint** — 定时快照 Agent 状态
- **自动重启** — 指数退避重试策略
- **记忆恢复** — 从 Checkpoint/Memory 恢复上下文
- **崩溃报告** — 完整的崩溃历史记录

### Cost Control
- **Token 计量** — 精确追踪每次任务的 Token 用量
- **预算管理** — 设置每日/每月预算限额
- **成本报告** — 按 Agent/Goal/Project 维度分析
- **超支熔断** — 超过预算自动暂停

### Audit Trail
- **全链路审计** — 所有操作留痕
- **层级追溯** — Goal → Project → Task → Issue 完整链路
- **JSONL 持久化** — 审计日志可导出分析

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      用户层                              │
│  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐      │
│  │  CLI │  │ API  │  │Telegr│  │Discor│  │ Web  │      │
│  │opctl │  │      │  │  am  │  │  d   │  │  UI  │      │
│  └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘      │
│     └─────────┴─────────┴─────────┴─────────┘          │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                     控制面板                              │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ Dispatcher │  │  Workflow  │  │ Controller │         │
│  │  智能路由   │  │  工作流引擎 │  │  生命周期   │         │
│  └────────────┘  └────────────┘  └────────────┘         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │   Audit    │  │ Cost Ctrl  │  │   Cron     │         │
│  │  审计追溯   │  │  成本控制   │  │  定时调度   │         │
│  └────────────┘  └────────────┘  └────────────┘         │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                      数据层                              │
│  ┌──────────────────────────────────────────────────┐   │
│  │               SQLite State Store                  │   │
│  │  Agents · Tasks · Workflows · Checkpoints         │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                    Agent 运行时                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │ OpenClaw │  │  Claude  │  │  Codex   │  │  Custom  │ │
│  │  Agent   │  │   Code   │  │   CLI    │  │  Agent   │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+
- SQLite3

### Install

```bash
git clone https://github.com/zlc-ai/opc-platform.git
cd opc-platform
go build -o opctl ./cmd/opctl/
sudo mv opctl /usr/local/bin/
```

### 30 秒上手

```bash
# 1. 查看版本
opctl version

# 2. 创建一个 Agent
opctl apply -f examples/agent-claude-code.yaml

# 3. 查看 Agent 列表
opctl get agents

# 4. 执行一个任务
opctl run --agent coder "Write a hello world in Go"

# 5. 查看任务状态
opctl get tasks

# 6. 查看集群状态
opctl status
```

详细指南请参考 [QUICKSTART.md](QUICKSTART.md)。

## 🎬 Real Demo (验证通过)

以下是一个**真实可用**的端到端测试，已于 2024-03-14 验证通过：

```bash
# 1. 启动 daemon（持久化 Agent 状态）
opctl serve &

# 2. 创建 Claude Code Agent
cat <<EOF | opctl apply -f -
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: coder
spec:
  type: claude-code
  runtime:
    inference:
      thinking: low
      maxTokens: 8192
  context:
    workdir: /tmp
EOF

# 3. 启动 Agent
opctl restart agent coder

# 4. 执行任务
curl -X POST http://127.0.0.1:9527/api/run \
  -H "Content-Type: application/json" \
  -d '{"agent":"coder","message":"Reply only: Hello from OPC Platform"}'
```

**实际输出**：
```json
{
  "output": "Hello from OPC Platform",
  "taskId": "task-1773477106914",
  "tokensIn": 3,
  "tokensOut": 8
}
```

> 💡 **注意**：需要先安装并配置好 Claude Code CLI (`claude --version`)


## Documentation

| 文档 | 描述 |
|------|------|
| [QUICKSTART.md](QUICKSTART.md) | 详细的快速上手指南 |
| [docs/COMMANDS.md](docs/COMMANDS.md) | 完整的命令行指令大全 |
| [OPC_Platform_PRD.md](OPC_Platform_PRD.md) | 产品需求文档 |
| [examples/](examples/) | YAML 配置示例 |

## Project Structure

```
opc_platform/
├── cmd/opctl/              # CLI 入口和所有命令
├── api/v1/                 # API 类型定义
├── pkg/
│   ├── adapter/            # Agent 适配器
│   │   ├── openclaw/       #   OpenClaw 适配器
│   │   ├── claudecode/     #   Claude Code 适配器
│   │   ├── codex/          #   Codex 适配器
│   │   └── custom/         #   自定义 Agent 适配器
│   ├── controller/         # 生命周期控制器 + 恢复系统
│   ├── workflow/           # 工作流引擎 + Cron 调度
│   ├── dispatcher/         # 智能路由调度
│   ├── cost/               # 成本追踪
│   ├── audit/              # 审计日志
│   └── storage/            # 状态存储（SQLite）
├── internal/config/        # 配置管理
├── examples/               # YAML 配置示例
└── docs/                   # 文档
```

## Tech Stack

- **Language**: Go 1.22+
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
- **Configuration**: [Viper](https://github.com/spf13/viper)
- **Storage**: SQLite (WAL mode)
- **Logging**: [Zap](https://github.com/uber-go/zap)
- **Serialization**: YAML v3

## Development

```bash
# Build
go build ./...

# Run tests
go test ./... -v

# Run with verbose logging
opctl --verbose get agents
```

## Roadmap

- [x] Phase 1: Foundation — CLI + Storage + OpenClaw 适配器
- [x] Phase 2: Multi-Agent — Claude Code / Codex / Custom 适配器 + 生命周期管理
- [x] Phase 3: Orchestration — Workflow 引擎 + Dispatcher + 审计系统
- [x] Phase 4: Production Ready — 成本控制 + 预算管理 + Gateway + Dashboard

## Contributing

We welcome contributions! Please see our contributing guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**OPC Platform** — One person, infinite agents.

## 🗺️ Roadmap

OPC Platform 正在积极开发中，欢迎 Star ⭐ 关注！

### v0.1 (当前) - Alpha
- [x] CLI 框架 (`opctl`)
- [x] 多 Agent 适配器 (Claude Code, OpenClaw, Codex)
- [x] Daemon 模式
- [x] 工作流引擎
- [x] 成本追踪
- [x] 审计日志

### v0.2 - Beta
- [ ] Web Dashboard
- [ ] Telegram/Discord Gateway
- [ ] 分布式部署支持
- [ ] 更多 Agent 类型 (GPT-4o, Gemini)

### v0.3 - Production Ready
- [ ] 企业级安全
- [ ] 多租户支持
- [ ] Kubernetes Operator

---

## Contributing

欢迎提交 Issue 和 PR！这是一个早期项目，我们非常期待社区的反馈。

## License

MIT License - 详见 [LICENSE](LICENSE)

---

**One person, infinite agents.** 🚀
