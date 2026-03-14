# OPC Platform

**AI Agent 的 Kubernetes** — 让一个人能像管理容器集群一样管理 AI Agent 集群。

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Status](https://img.shields.io/badge/Status-v0.3_Production-green)]()
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-15%2F15_Passing-brightgreen)]()

---

## What is OPC Platform?

OPC Platform 是一个**独立运行**的 AI Agent 编排平台。就像 Kubernetes 管理容器一样，OPC Platform 让你用声明式 YAML 配置来管理多种 AI Agent，实现自动化工作流、成本控制和智能调度。

**⚠️ 重要：OPC Platform 不依赖 Kubernetes！它本身就是 AI Agent 的编排系统。**

| K8s 概念 | OPC 对应 |
|----------|----------|
| Docker 容器 | AI Agent（Claude Code / OpenAI / Codex / 自定义） |
| Kubernetes | **OPC Platform（独立运行）** |
| Pod | Agent Instance |
| Deployment | AgentSpec |
| kubectl | `opctl` |
| Node | OPC Node（集群节点） |

---

## 🎬 Real Demo (验证通过)

```bash
# 1. 编译
go build -o opctl ./cmd/opctl/

# 2. 启动 daemon
./opctl serve &

# 3. 创建 Agent
cat << 'AGENT' | ./opctl apply -f -
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: coder
spec:
  type: claude-code
  context:
    workdir: /tmp
AGENT

# 4. 启动并执行任务
./opctl restart agent coder
curl -X POST http://127.0.0.1:9527/api/run \
  -H "Content-Type: application/json" \
  -d '{"agent":"coder","message":"Reply only: Hello from OPC"}'
```

**实际输出：**
```json
{"output":"Hello from OPC","taskId":"task-xxx","tokensIn":3,"tokensOut":6}
```

---

## Features

### 🤖 Agent Management
- **多类型支持** — Claude Code、OpenAI GPT-4o、Codex、OpenClaw、自定义
- **声明式配置** — YAML 定义期望状态，平台自动实现
- **生命周期管理** — 启动、停止、重启、扩缩容
- **健康检查** — 自动检测故障，触发自愈

### 🔄 Workflow Engine
- **DAG 工作流** — 多步骤有向无环图编排
- **并行执行** — 无依赖的步骤自动并行
- **上下文传递** — 变量替换
- **Cron 调度** — 支持定时执行

### 🌐 Gateway
- **Telegram Bot** — /run, /status, /agents 命令
- **Discord Bot** — Slash Commands 支持
- **Web Dashboard** — Next.js 管理界面

### 💰 Cost Control
- **Token 计量** — 精确追踪每次任务用量
- **预算管理** — 设置每日/每月限额
- **成本报告** — 按 Agent/Project 维度分析

### 🔐 Security (v0.3)
- **JWT 认证** — Token 生成和验证
- **RBAC 权限** — admin/operator/viewer 角色
- **多租户** — 租户级别资源隔离

### 🌍 OPC Cluster (独立运行，无需 K8s)
- **集群管理** — init/join/leave
- **节点发现** — 自动发现集群节点
- **跨节点调度** — Agent 智能分配

---

## Quick Start

### Prerequisites

- Go 1.22+
- Claude Code CLI (可选，用于 claude-code 类型 Agent)

### Install

```bash
git clone https://github.com/zxs1633079383/opc-platform.git
cd opc-platform
go build -o opctl ./cmd/opctl/
sudo mv opctl /usr/local/bin/
```

### 30 秒上手

```bash
# 1. 启动 daemon
opctl serve &

# 2. 查看状态
opctl status

# 3. 创建 Agent
opctl apply -f examples/agent-claude-code.yaml

# 4. 执行任务
opctl run --agent coder "Write hello world in Go"

# 5. 查看任务
opctl get tasks
```

### 集群部署

```bash
# 节点 1 (Master)
opctl cluster init --advertise-addr 192.168.1.1

# 节点 2, 3... (Worker)
opctl cluster join --master 192.168.1.1:9527

# 查看集群
opctl cluster nodes
opctl cluster status
```

---

## CLI Commands

```bash
# Agent 管理
opctl apply -f agent.yaml     # 创建/更新
opctl get agents              # 列表
opctl describe agent <name>   # 详情
opctl delete agent <name>     # 删除
opctl restart agent <name>    # 重启

# 任务执行
opctl run --agent <name> "message"
opctl get tasks
opctl logs <task-id>

# 集群管理
opctl cluster init            # 初始化 master
opctl cluster join <addr>     # 加入集群
opctl cluster nodes           # 节点列表
opctl cluster status          # 集群状态
opctl cluster leave           # 离开集群

# 工作流
opctl run workflow <name>
opctl get workflows

# 成本
opctl cost report
opctl cost watch
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      用户层                              │
│  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐      │
│  │ CLI  │  │ API  │  │Telegr│  │Discor│  │ Web  │      │
│  │opctl │  │      │  │  am  │  │  d   │  │  UI  │      │
│  └──────┘  └──────┘  └──────┘  └──────┘  └──────┘      │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                   OPC 控制平面                           │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │  Cluster   │  │  Workflow  │  │ Controller │         │
│  │   Manager  │  │   Engine   │  │ Lifecycle  │         │
│  └────────────┘  └────────────┘  └────────────┘         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │    Auth    │  │   Cost     │  │   Audit    │         │
│  │  JWT/RBAC  │  │  Tracking  │  │   Trail    │         │
│  └────────────┘  └────────────┘  └────────────┘         │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                    Agent 运行时                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │ Claude   │  │  OpenAI  │  │  Codex   │  │  Custom  │ │
│  │   Code   │  │  GPT-4o  │  │          │  │  Agent   │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## Deployment

### Docker

```bash
docker-compose up -d
```

### Systemd

```bash
sudo cp deploy/systemd/opc.service /etc/systemd/system/
sudo systemctl enable opc
sudo systemctl start opc
```

---

## Testing

```bash
# 单元测试
go test ./...

# E2E 测试 (v0.1 + v0.2 + v0.3)
./tests/e2e_test.sh
```

**测试结果：15/15 通过** ✅

---

## 🗺️ Roadmap

### v0.1 Alpha ✅
- [x] CLI 框架 (`opctl`)
- [x] Agent 适配器
- [x] 工作流引擎
- [x] Daemon 模式

### v0.2 Beta ✅
- [x] Web Dashboard
- [x] Telegram/Discord Gateway
- [x] OpenAI GPT-4o Adapter
- [x] PostgreSQL 支持
- [x] Docker 部署

### v0.3 Production ✅
- [x] JWT 认证
- [x] RBAC 权限
- [x] 多租户支持
- [x] **OPC 原生集群管理**（无 K8s 依赖）

### v0.4 (规划中)
- [ ] Web UI 完善
- [ ] 更多 Agent 类型 (Gemini, Cursor)
- [ ] Agent 市场
- [ ] 插件系统

---

## Contributing

欢迎提交 Issue 和 PR！

## License

MIT License

---

**One person, infinite agents.** 🚀
