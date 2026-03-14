# Hybrid Multi-Company Example (OpenClaw + Claude)

This example demonstrates the most powerful setup: combining **OPC Federation** for company-level organization with **Claude Code** for intelligent task execution.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    OPC Federation                            │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐      │
│  │ Software Co.  │ │ Operations Co.│ │   Sales Co.   │      │
│  │ (Claude Code) │ │  (OpenClaw)   │ │ (Claude Code) │      │
│  └───────────────┘ └───────────────┘ └───────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Key Concept

- **OPC Federation**: Manages company boundaries, goal distribution, and audit trails
- **Claude Code**: Executes complex coding tasks within companies
- **OpenClaw Agents**: Handles lightweight automation tasks

## Files

- `federation.yaml` - Federation controller and company definitions
- `claude-agents.yaml` - Claude Code agents for development tasks
- `openclaw-agents.yaml` - OpenClaw agents for automation
- `hybrid-goal.yaml` - Goal that uses both agent types

## Quick Start

```bash
# 1. Start OPC daemon
opctl serve &

# 2. Initialize federation
opctl federation init

# 3. Add companies
opctl apply -f federation.yaml

# 4. Deploy agents (both types)
opctl apply -f claude-agents.yaml
opctl apply -f openclaw-agents.yaml

# 5. Start all agents
opctl restart agent --all

# 6. Create and dispatch a goal
opctl goal create -f hybrid-goal.yaml

# 7. Monitor progress
opctl goal status messaging-system
```

## When to Use Hybrid

| Use Case | Agent Type |
|----------|------------|
| Complex coding | Claude Code |
| Code review | Claude Code |
| File operations | OpenClaw |
| API calls | OpenClaw |
| Documentation | Claude Code |
| Monitoring | OpenClaw |

## Best Practices

1. **Use Claude for creative work** - coding, writing, problem-solving
2. **Use OpenClaw for automation** - scripts, pipelines, integrations
3. **Federation for boundaries** - audit, cost tracking, isolation
4. **Context injection** - share relevant info between companies
