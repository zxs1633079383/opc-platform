# OPC Platform

**AI Agent 的 Kubernetes** — 让一个人能像管理容器集群一样管理 AI Agent 集群。

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Status](https://img.shields.io/badge/Status-v0.4_Federation-green)]()
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-15%2F15_Passing-brightgreen)]()

**[English](README.md) | [中文](README_CN.md)**

---

## OPC Platform 是什么？

OPC Platform 是一个**独立运行**的 AI Agent 编排平台。就像 Kubernetes 管理容器一样，OPC Platform 让你用声明式 YAML 配置来管理多种 AI Agent，实现自动化工作流、成本控制和智能调度。

**v0.4 联邦管理**引入多公司协作 — 将 AI Agent 团队组织为独立公司，通过联邦目标层级进行协调。

**重要：OPC Platform 不依赖 Kubernetes！它本身就是 AI Agent 的编排系统。**

| K8s 概念 | OPC 对应 |
|----------|----------|
| Docker 容器 | AI Agent（Claude Code / OpenAI / Codex / OpenClaw / 自定义） |
| Kubernetes | **OPC Platform（独立运行）** |
| Pod | Agent Instance |
| Deployment | AgentSpec |
| kubectl | `opctl` |
| Node | OPC Node（集群节点） |
| Namespace | Company（联邦公司） |
| CRD | Goal / Project / Task / Issue |

---

## 快速开始

### 30 秒上手

```bash
# 安装
git clone https://github.com/zxs1633079383/opc-platform.git
cd opc-platform
go build -o opctl ./cmd/opctl/
sudo mv opctl /usr/local/bin/

# 启动 daemon
opctl serve &

# 创建并运行 Agent
opctl apply -f examples/agent-claude-code.yaml
opctl run --agent coder "Write hello world in Go"
opctl get tasks
```

### 多公司快速上手（联邦管理）

2 分钟内搭建 3 公司联邦：

```bash
# 1. 初始化联邦
opctl federation init --name "tech-group"

# 2. 注册公司
opctl federation add-company --name software \
  --type software --endpoint http://localhost:9527

opctl federation add-company --name operations \
  --type operations --endpoint http://localhost:9528

opctl federation add-company --name sales \
  --type sales --endpoint http://localhost:9529

# 3. 应用公司配置和 Agent
opctl apply -f examples/quickstart/openclaw-multi-company/software-company.yaml
opctl apply -f examples/quickstart/openclaw-multi-company/operations-company.yaml
opctl apply -f examples/quickstart/openclaw-multi-company/sales-company.yaml

# 4. 创建战略目标 — 自动分解为 Project > Task > Issue
opctl apply -f examples/quickstart/openclaw-multi-company/goal-messaging-system.yaml

# 5. 监控
opctl federation status
opctl goal status develop-messaging-system
opctl goal trace develop-messaging-system
```

查看 [docs/quickstart.md](docs/quickstart.md) 获取完整指南，或浏览可直接运行的示例：
- [OpenClaw 多公司](examples/quickstart/openclaw-multi-company/) — 纯 OpenClaw Agent
- [Claude 多公司](examples/quickstart/claude-multi-company/) — Claude Code Agent
- [混合多公司](examples/quickstart/hybrid-multi-company/) — OpenClaw + Claude 混合

---

## 真实演示（已验证）

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

## 功能特性

### Agent 管理
- **多类型支持** — Claude Code、OpenAI GPT-4o、Codex、OpenClaw、自定义
- **声明式配置** — YAML 定义期望状态，平台自动实现
- **生命周期管理** — 启动、停止、重启、扩缩容
- **健康检查** — 自动检测故障，触发自愈

### 工作流引擎
- **DAG 工作流** — 多步骤有向无环图编排
- **并行执行** — 无依赖的步骤自动并行
- **上下文传递** — 步骤间变量替换
- **Cron 调度** — 支持定时执行

### 联邦管理 (v0.4)
- **多公司联邦架构** — 将独立公司注册到联邦网络中，每个公司拥有独立的 Agent 和能力
- **目标层级系统** — 战略目标自动分解：Goal > Project > Task > Issue
- **跨公司调度** — 根据公司类型和能力自动分配目标
- **上下文注入** — 跨公司依赖任务间自动传递上下文
- **心跳监控** — 实时追踪所有联邦公司的健康状态
- **人工介入** — 审批门控机制，关键决策需人工确认
- **[联邦管理指南](docs/federation.md)** — 完整文档

### 网关
- **Telegram Bot** — /run, /status, /agents 命令
- **Discord Bot** — Slash Commands 支持
- **Web Dashboard** — Next.js 管理界面

### 成本控制
- **Token 计量** — 精确追踪每次任务用量
- **预算管理** — 设置每日/每月限额
- **成本报告** — 按 Agent/Project 维度分析

### 安全 (v0.3)
- **JWT 认证** — Token 生成和验证
- **RBAC 权限** — admin/operator/viewer 角色
- **多租户** — 租户级别资源隔离

### OPC 集群（独立运行，无需 K8s）
- **集群管理** — init/join/leave
- **节点发现** — 自动发现集群节点
- **跨节点调度** — Agent 智能分配

---

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                       用户层                              │
│  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐      │
│  │ CLI  │  │ API  │  │Telegr│  │Discor│  │ Web  │      │
│  │opctl │  │      │  │  am  │  │  d   │  │  UI  │      │
│  └──────┘  └──────┘  └──────┘  └──────┘  └──────┘      │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                   OPC 控制平面                            │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │  Cluster   │  │  Workflow  │  │ Controller │         │
│  │   Manager  │  │   Engine   │  │ Lifecycle  │         │
│  └────────────┘  └────────────┘  └────────────┘         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ Federation │  │   Cost     │  │   Auth     │         │
│  │ Controller │  │  Tracking  │  │  JWT/RBAC  │         │
│  └────────────┘  └────────────┘  └────────────┘         │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                    目标调度层                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │   Goal   │  │ Project  │  │   Task   │               │
│  │   分解    │  │   调度    │  │   分配    │               │
│  └──────────┘  └──────────┘  └──────────┘               │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                     联邦公司层                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │  软件公司  │  │  运维公司  │  │  销售公司  │  │  自定义   │ │
│  │ Software │  │Operations│  │  Sales   │  │ Company  │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
│       │              │             │              │       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │ Claude   │  │ OpenClaw │  │  Codex   │  │  Custom  │ │
│  │  Code    │  │  Agents  │  │  Agents  │  │  Agents  │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## CLI 命令

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

# 联邦管理 (v0.4)
opctl federation init              # 初始化联邦
opctl federation add-company       # 注册公司
opctl federation companies         # 公司列表
opctl federation status            # 联邦健康状态

# 目标管理 (v0.4)
opctl goal create                  # 创建战略目标
opctl goal list                    # 目标列表
opctl goal status <goal-id>        # 目标层级树
opctl goal trace <goal-id>         # 审计追踪
opctl goal intervene               # 人工介入
```

完整命令参考请见 [docs/COMMANDS.md](docs/COMMANDS.md)。

---

## 部署方式

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

## 测试

```bash
# 单元测试
go test ./...

# E2E 测试 (v0.1 + v0.2 + v0.3)
./tests/e2e_test.sh
```

**测试结果：15/15 通过**

---

## 路线图

### v0.1 Alpha
- [x] CLI 框架 (`opctl`)
- [x] Agent 适配器
- [x] 工作流引擎
- [x] Daemon 模式

### v0.2 Beta
- [x] Web Dashboard
- [x] Telegram/Discord Gateway
- [x] OpenAI GPT-4o Adapter
- [x] PostgreSQL 支持
- [x] Docker 部署

### v0.3 Production
- [x] JWT 认证
- [x] RBAC 权限
- [x] 多租户支持
- [x] **OPC 原生集群管理**（无 K8s 依赖）

### v0.4 联邦管理
- [x] 多公司联邦架构
- [x] 目标层级系统（Goal > Project > Task > Issue）
- [x] 跨公司目标调度
- [x] 心跳监控
- [x] 人工介入机制
- [x] 审批门控

### v0.5 (规划中)
- [ ] Web UI 完善
- [ ] 更多 Agent 类型 (Gemini, Cursor)
- [ ] Agent 市场
- [ ] 插件系统

---

## 贡献

欢迎提交 Issue 和 PR！

## 许可证

MIT License

---

**One person, infinite agents.**
