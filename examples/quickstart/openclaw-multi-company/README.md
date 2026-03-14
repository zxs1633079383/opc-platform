# OpenClaw Multi-Company Federation Example

A complete multi-company federation setup using pure OpenClaw agents.

## Overview

This example sets up a tech group with 3 companies:

| Company | Type | Agents | Role |
|---------|------|--------|------|
| Software | software | backend-dev, frontend-dev | Build the product |
| Operations | operations | devops, monitor | Deploy and monitor |
| Sales | sales | researcher, content-writer | Market and sell |

## Files

- `federation.yaml` — Federation network configuration
- `software-company.yaml` — Software company with dev agents
- `operations-company.yaml` — Operations company with DevOps agents
- `sales-company.yaml` — Sales company with research agents
- `goal-messaging-system.yaml` — Strategic goal that spans all companies

## Usage

```bash
# 1. Start daemon
opctl serve &

# 2. Initialize federation
opctl apply -f federation.yaml

# 3. Register all companies
opctl apply -f software-company.yaml
opctl apply -f operations-company.yaml
opctl apply -f sales-company.yaml

# 4. Create strategic goal
opctl apply -f goal-messaging-system.yaml

# 5. Monitor
opctl federation status
opctl goal status develop-messaging-system
opctl goal trace develop-messaging-system
```

## What Happens

1. The federation controller registers all 3 companies
2. The goal `develop-messaging-system` is decomposed into projects per company
3. Each project is broken into tasks and issues
4. Issues are dispatched to the appropriate company's agents
5. Heartbeat monitoring tracks company health
6. Human intervention gates pause for approval on critical decisions
