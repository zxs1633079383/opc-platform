# OPC Platform Quick Start Guide

This guide walks you through installing OPC Platform, running your first agent, and setting up a multi-company federation.

## Prerequisites

- **Go 1.22+** — [Install Go](https://go.dev/dl/)
- **Claude Code CLI** (optional) — Required only for `claude-code` type agents
- **Git** — For cloning the repository

## Installation

```bash
# Clone and build
git clone https://github.com/zxs1633079383/opc-platform.git
cd opc-platform
go build -o opctl ./cmd/opctl/

# Install globally (optional)
sudo mv opctl /usr/local/bin/
```

Verify the installation:

```bash
opctl version
opctl help
```

## Basic Usage

### 1. Start the Daemon

```bash
opctl serve &
```

The daemon listens on `http://127.0.0.1:9527` by default.

### 2. Create an Agent

Using a YAML file:

```bash
opctl apply -f examples/agent-claude-code.yaml
```

Or inline:

```bash
cat << 'EOF' | opctl apply -f -
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: my-agent
spec:
  type: claude-code
  context:
    workdir: /tmp/my-project
EOF
```

### 3. Run a Task

```bash
opctl run --agent my-agent "Write a hello world HTTP server in Go"
```

### 4. Check Results

```bash
# List all tasks
opctl get tasks

# View task logs
opctl logs <task-id>

# Check agent status
opctl get agents
opctl describe agent my-agent
```

### 5. Run a Workflow

```bash
opctl apply -f examples/workflow-daily-research.yaml
opctl run workflow daily-research
opctl get workflows
```

---

## Multi-Company Federation Setup

Federation lets you organize agents into independent companies that collaborate on strategic goals.

### Scenario

We'll create a tech group with 3 companies:
- **Software Company** — Backend/frontend development agents
- **Operations Company** — DevOps and monitoring agents
- **Sales Company** — Market research and content agents

### Step 1: Initialize Federation

```bash
opctl federation init --name "tech-group"
```

### Step 2: Register Companies

```bash
opctl federation add-company \
  --name software \
  --type software \
  --endpoint http://localhost:9527

opctl federation add-company \
  --name operations \
  --type operations \
  --endpoint http://localhost:9528

opctl federation add-company \
  --name sales \
  --type sales \
  --endpoint http://localhost:9529
```

### Step 3: Apply Company Configurations

Each company defines its agents and capabilities:

```bash
opctl apply -f examples/quickstart/openclaw-multi-company/software-company.yaml
opctl apply -f examples/quickstart/openclaw-multi-company/operations-company.yaml
opctl apply -f examples/quickstart/openclaw-multi-company/sales-company.yaml
```

### Step 4: Create a Strategic Goal

```bash
opctl apply -f examples/quickstart/openclaw-multi-company/goal-messaging-system.yaml
```

This creates a goal that automatically decomposes into:
- **Projects** — One per target company (backend, deployment, sales enablement)
- **Tasks** — Specific work items within each project
- **Issues** — Smallest executable units assigned to individual agents

### Step 5: Monitor Progress

```bash
# Federation health
opctl federation status
opctl federation companies

# Goal hierarchy tree
opctl goal status develop-messaging-system

# Audit trail — who did what, when
opctl goal trace develop-messaging-system

# Human intervention (when approval is needed)
opctl goal intervene --goal develop-messaging-system --action approve
```

### Goal Hierarchy Visualization

```
Goal: develop-messaging-system
├── Project: backend-api (→ software company)
│   ├── Task: design-api-schema
│   │   └── Issue: create-openapi-spec (→ backend-dev agent)
│   └── Task: implement-endpoints
│       └── Issue: build-rest-api (→ backend-dev agent)
├── Project: infrastructure (→ operations company)
│   ├── Task: setup-ci-cd
│   │   └── Issue: configure-pipeline (→ devops agent)
│   └── Task: monitoring-setup
│       └── Issue: add-dashboards (→ monitor agent)
└── Project: go-to-market (→ sales company)
    └── Task: sales-materials
        └── Issue: create-pitch-deck (→ researcher agent)
```

---

## Example Configurations

### OpenClaw Multi-Company (Recommended for starters)

Pure OpenClaw agents — lightweight, fast, cost-effective:

```bash
cd examples/quickstart/openclaw-multi-company/
opctl apply -f federation.yaml
opctl apply -f software-company.yaml
opctl apply -f operations-company.yaml
opctl apply -f sales-company.yaml
opctl apply -f goal-messaging-system.yaml
```

See [examples/quickstart/openclaw-multi-company/](../examples/quickstart/openclaw-multi-company/)

### Claude Multi-Company

Claude Code agents — maximum capability for complex tasks:

```bash
cd examples/quickstart/claude-multi-company/
opctl apply -f agents.yaml
opctl apply -f workflow.yaml
```

See [examples/quickstart/claude-multi-company/](../examples/quickstart/claude-multi-company/)

### Hybrid Multi-Company

Mix OpenClaw and Claude agents — balance cost and capability:

```bash
cd examples/quickstart/hybrid-multi-company/
opctl apply -f federation-with-claude.yaml
opctl apply -f openclaw-agents.yaml
opctl apply -f claude-agents.yaml
opctl apply -f hybrid-workflow.yaml
```

See [examples/quickstart/hybrid-multi-company/](../examples/quickstart/hybrid-multi-company/)

---

## Next Steps

- [Federation Guide](federation.md) — Deep dive into federation architecture
- [CLI Reference](COMMANDS.md) — All available commands
- [Agent Examples](../examples/) — More agent configurations
