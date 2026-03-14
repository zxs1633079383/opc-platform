# Claude Multi-Company Example

This example demonstrates managing multiple AI agent teams using **Claude Code** as the execution engine.

## Overview

Instead of using OPC's native federation, this example shows how to structure a multi-team workflow using Claude Code agents directly.

## Files

- `agents.yaml` - Define multiple Claude Code agents for different teams
- `workflow.yaml` - Coordinate tasks across teams

## Quick Start

```bash
# 1. Start OPC daemon
opctl serve &

# 2. Apply agent configurations
opctl apply -f agents.yaml

# 3. Start all agents
opctl restart agent software-team
opctl restart agent operations-team
opctl restart agent sales-team

# 4. Run a coordinated workflow
opctl run workflow multi-team-dev
```

## Agent Structure

```yaml
# Software Team - handles development
- name: software-team
  type: claude-code
  workdir: /workspace/software

# Operations Team - handles deployment
- name: operations-team
  type: claude-code
  workdir: /workspace/ops

# Sales Team - handles documentation
- name: sales-team
  type: claude-code
  workdir: /workspace/sales
```

## Use Cases

- **Single Organization**: When you don't need formal company separation
- **Quick Setup**: Faster to configure than full federation
- **CI/CD Integration**: Easy to integrate with existing pipelines
