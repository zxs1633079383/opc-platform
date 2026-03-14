# OPC Platform

**Kubernetes for AI Agents** — Manage AI agent clusters like container orchestration.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Status](https://img.shields.io/badge/Status-v0.4_Federation-green)]()
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-15%2F15_Passing-brightgreen)]()

**[English](README.md) | [中文](README_CN.md)**

---

## What is OPC Platform?

OPC Platform is a **standalone** AI Agent orchestration platform. Just like Kubernetes manages containers, OPC Platform lets you manage multiple AI Agents with declarative YAML configs, enabling automated workflows, cost control, and intelligent scheduling.

**v0.4 Federation** introduces multi-company collaboration — organize AI agent teams into independent companies and coordinate them through a federated goal hierarchy.

**Important: OPC Platform does NOT depend on Kubernetes! It IS the orchestration system for AI Agents.**

| K8s Concept | OPC Equivalent |
|-------------|----------------|
| Docker Container | AI Agent (Claude Code / OpenAI / Codex / OpenClaw / Custom) |
| Kubernetes | **OPC Platform (Standalone)** |
| Pod | Agent Instance |
| Deployment | AgentSpec |
| kubectl | `opctl` |
| Node | OPC Node (Cluster Node) |
| Namespace | Company (Federation) |
| CRD | Goal / Project / Task / Issue |

---

## Quick Start

### 30-Second Start

```bash
# Install
git clone https://github.com/zxs1633079383/opc-platform.git
cd opc-platform
go build -o opctl ./cmd/opctl/
sudo mv opctl /usr/local/bin/

# Start daemon
opctl serve &

# Create and run an agent
opctl apply -f examples/agent-claude-code.yaml
opctl run --agent coder "Write hello world in Go"
opctl get tasks
```

### Multi-Company Quickstart (Federation)

Set up a 3-company federation in under 2 minutes:

```bash
# 1. Initialize federation
opctl federation init --name "tech-group"

# 2. Register companies
opctl federation add-company --name software \
  --type software --endpoint http://localhost:9527

opctl federation add-company --name operations \
  --type operations --endpoint http://localhost:9528

opctl federation add-company --name sales \
  --type sales --endpoint http://localhost:9529

# 3. Apply company configs with agents
opctl apply -f examples/quickstart/openclaw-multi-company/software-company.yaml
opctl apply -f examples/quickstart/openclaw-multi-company/operations-company.yaml
opctl apply -f examples/quickstart/openclaw-multi-company/sales-company.yaml

# 4. Create a strategic goal — auto-decomposes into Projects > Tasks > Issues
opctl apply -f examples/quickstart/openclaw-multi-company/goal-messaging-system.yaml

# 5. Monitor
opctl federation status
opctl goal status develop-messaging-system
opctl goal trace develop-messaging-system
```

See [docs/quickstart.md](docs/quickstart.md) for the full guide, or explore ready-to-run examples:
- [OpenClaw Multi-Company](examples/quickstart/openclaw-multi-company/) — Pure OpenClaw agents
- [Claude Multi-Company](examples/quickstart/claude-multi-company/) — Claude Code agents
- [Hybrid Multi-Company](examples/quickstart/hybrid-multi-company/) — OpenClaw + Claude mixed

---

## Real Demo (Verified)

```bash
# 1. Build
go build -o opctl ./cmd/opctl/

# 2. Start daemon
./opctl serve &

# 3. Create Agent
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

# 4. Start and run task
./opctl restart agent coder
curl -X POST http://127.0.0.1:9527/api/run \
  -H "Content-Type: application/json" \
  -d '{"agent":"coder","message":"Reply only: Hello from OPC"}'
```

**Actual Output:**
```json
{"output":"Hello from OPC","taskId":"task-xxx","tokensIn":3,"tokensOut":6}
```

---

## Features

### Agent Management
- **Multi-type Support** — Claude Code, OpenAI GPT-4o, Codex, OpenClaw, Custom
- **Declarative Config** — Define desired state in YAML, platform implements it
- **Lifecycle Management** — Start, stop, restart, scale
- **Health Checks** — Auto-detect failures, trigger self-healing

### Workflow Engine
- **DAG Workflows** — Multi-step directed acyclic graph orchestration
- **Parallel Execution** — Independent steps run concurrently
- **Context Passing** — Variable substitution between steps
- **Cron Scheduling** — Timed execution support

### Federation (v0.4)
- **Multi-Company Architecture** — Register independent companies into a federated network, each with its own agents and capabilities
- **Goal Hierarchy** — Strategic goals auto-decompose: Goal > Project > Task > Issue
- **Cross-Company Dispatch** — Goals are distributed to the right company based on type and capability
- **Context Injection** — Cross-company context flows automatically between dependent tasks
- **Heartbeat Monitoring** — Real-time health tracking of all federated companies
- **Human-in-the-Loop** — Intervention system with approval gates for critical decisions
- **[Federation Guide](docs/federation.md)** — Full documentation

### Gateway
- **Telegram Bot** — /run, /status, /agents commands
- **Discord Bot** — Slash Commands support
- **Web Dashboard** — Next.js management UI

### Cost Control
- **Token Metering** — Precise tracking per task
- **Budget Management** — Daily/monthly limits
- **Cost Reports** — Analysis by Agent/Project

### Security (v0.3)
- **JWT Authentication** — Token generation and validation
- **RBAC Authorization** — admin/operator/viewer roles
- **Multi-tenancy** — Tenant-level resource isolation

### OPC Cluster (Standalone, No K8s Required)
- **Cluster Management** — init/join/leave
- **Node Discovery** — Auto-discover cluster nodes
- **Cross-node Scheduling** — Smart agent distribution

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      User Layer                          │
│  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐      │
│  │ CLI  │  │ API  │  │Telegr│  │Discor│  │ Web  │      │
│  │opctl │  │      │  │  am  │  │  d   │  │  UI  │      │
│  └──────┘  └──────┘  └──────┘  └──────┘  └──────┘      │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                   OPC Control Plane                      │
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
│                   Goal Dispatcher                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │   Goal   │  │ Project  │  │   Task   │               │
│  │ Decompose│  │ Dispatch │  │  Assign  │               │
│  └──────────┘  └──────────┘  └──────────┘               │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│               Federated Companies                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │ Software │  │Operations│  │  Sales   │  │  Custom  │ │
│  │ Company  │  │ Company  │  │ Company  │  │ Company  │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
│       │              │             │              │       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │ Claude   │  │ OpenClaw │  │  Codex   │  │  Custom  │ │
│  │  Code    │  │  Agents  │  │  Agents  │  │  Agents  │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## CLI Commands

```bash
# Agent Management
opctl apply -f agent.yaml     # Create/Update
opctl get agents              # List
opctl describe agent <name>   # Details
opctl delete agent <name>     # Delete
opctl restart agent <name>    # Restart

# Task Execution
opctl run --agent <name> "message"
opctl get tasks
opctl logs <task-id>

# Cluster Management
opctl cluster init            # Initialize master
opctl cluster join <addr>     # Join cluster
opctl cluster nodes           # List nodes
opctl cluster status          # Cluster status
opctl cluster leave           # Leave cluster

# Workflow
opctl run workflow <name>
opctl get workflows

# Cost
opctl cost report
opctl cost watch

# Federation (v0.4)
opctl federation init              # Initialize federation
opctl federation add-company       # Register a company
opctl federation companies         # List companies
opctl federation status            # Federation health

# Goal (v0.4)
opctl goal create                  # Create strategic goal
opctl goal list                    # List all goals
opctl goal status <goal-id>        # Goal hierarchy tree
opctl goal trace <goal-id>         # Audit trail
opctl goal intervene               # Human intervention
```

See [docs/COMMANDS.md](docs/COMMANDS.md) for the full command reference.

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

### Cluster Deployment

```bash
# Node 1 (Master)
opctl cluster init --advertise-addr 192.168.1.1

# Node 2, 3... (Worker)
opctl cluster join --master 192.168.1.1:9527

# View cluster
opctl cluster nodes
opctl cluster status
```

---

## Testing

```bash
# Unit tests
go test ./...

# E2E tests (v0.1 + v0.2 + v0.3)
./tests/e2e_test.sh
```

**Test Results: 15/15 Passing**

---

## Roadmap

### v0.1 Alpha
- [x] CLI framework (`opctl`)
- [x] Agent adapters
- [x] Workflow engine
- [x] Daemon mode

### v0.2 Beta
- [x] Web Dashboard
- [x] Telegram/Discord Gateway
- [x] OpenAI GPT-4o Adapter
- [x] PostgreSQL support
- [x] Docker deployment

### v0.3 Production
- [x] JWT authentication
- [x] RBAC authorization
- [x] Multi-tenant support
- [x] **OPC native cluster management** (no K8s dependency)

### v0.4 Federation
- [x] Multi-company federation architecture
- [x] Goal hierarchy system (Goal > Project > Task > Issue)
- [x] Cross-company goal dispatch
- [x] Heartbeat monitoring
- [x] Human-in-the-loop intervention
- [x] Approval gates

### v0.5 (Planned)
- [ ] Web UI improvements
- [ ] More agent types (Gemini, Cursor)
- [ ] Agent marketplace
- [ ] Plugin system

---

## Contributing

Issues and PRs welcome!

## License

MIT License

---

**One person, infinite agents.**
