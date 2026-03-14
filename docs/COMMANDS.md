# OPC Platform CLI Command Reference

`opctl` 是 OPC Platform 的命令行工具，用于管理 AI Agent 集群。

---

## Global Flags

所有命令均支持以下全局参数：

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--config` | | string | `~/.opc/config.yaml` | 配置文件路径 |
| `--verbose` | `-v` | bool | `false` | 开启调试日志 |
| `--output` | `-o` | string | `table` | 输出格式：`json`、`yaml`、`table` |

---

## Agent Management

### `opctl apply`

从 YAML 文件创建或更新资源。

```
opctl apply -f <file>
```

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--file` | `-f` | string | Yes | YAML 文件路径 |

**支持的资源类型：** `AgentSpec`、`Workflow`

```bash
# 创建/更新 Agent
opctl apply -f agent.yaml

# 创建/更新 Workflow
opctl apply -f workflow.yaml
```

---

### `opctl get agents`

列出所有 Agent。

```
opctl get agents
```

**别名：** `opctl get agent`

**输出列：**

| Column | Description |
|--------|-------------|
| NAME | Agent 名称 |
| TYPE | Agent 类型（openclaw, claude-code, codex, custom） |
| STATUS | 当前状态（Created, Running, Stopped, Failed） |
| RESTARTS | 重启次数 |
| AGE | 创建时间距今 |

```bash
opctl get agents
opctl get agents -o json
```

---

### `opctl describe agent`

显示 Agent 详细信息。

```
opctl describe agent <name>
```

**输出内容：** 名称、类型、状态、重启次数、消息、创建/更新时间、最近 10 条任务。

```bash
opctl describe agent coder
```

---

### `opctl delete agent`

删除 Agent。如果 Agent 正在运行，会先停止。

```
opctl delete agent <name>
```

```bash
opctl delete agent coder
```

---

### `opctl restart agent`

重启 Agent。如果 Agent 未运行，等同于启动。手动重启会重置重启计数器。

```
opctl restart agent <name>
```

```bash
opctl restart agent coder
```

---

### `opctl scale agent`

调整 Agent 副本数量。

```
opctl scale agent <name> --replicas <n>
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--replicas` | int | `1` | 副本数量 |

```bash
opctl scale agent coder --replicas 3
```

---

## Task Execution

### `opctl run --agent`

向指定 Agent 发送并执行任务。

```
opctl run --agent <name> [--stream] <message>
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--agent` | string | (required) | 目标 Agent 名称 |
| `--stream` | bool | `false` | 开启流式输出 |

```bash
# 同步执行
opctl run --agent coder "Write unit tests for auth module"

# 流式输出
opctl run --agent coder --stream "Refactor the database layer"
```

**别名：** `opctl exec --agent <name> <message>`（隐藏命令）

---

### `opctl get tasks`

列出所有任务。

```
opctl get tasks
```

**别名：** `opctl get task`

**输出列：**

| Column | Description |
|--------|-------------|
| ID | 任务 ID |
| AGENT | 执行 Agent |
| STATUS | 状态（Pending, Running, Completed, Failed, Cancelled） |
| MESSAGE | 任务消息（截断至 40 字符） |
| AGE | 创建时间距今 |

```bash
opctl get tasks
opctl get tasks -o json
```

---

### `opctl logs`

查看任务的详细日志和输出。

```
opctl logs <task-id>
```

**输出内容：** 任务 ID、Agent、状态、时间戳、时长、Token 用量、成本、消息、输出结果、错误信息。

```bash
opctl logs task-1710349200000
```

---

## Workflow

### `opctl run workflow`

执行一个已注册的工作流。

```
opctl run workflow <name>
```

**输出内容：** 工作流运行 ID、整体状态、各步骤执行状态。

```bash
opctl run workflow daily-research
```

---

### `opctl get workflows`

列出所有工作流。

```
opctl get workflows
```

**别名：** `opctl get workflow`、`opctl get wf`

**输出列：**

| Column | Description |
|--------|-------------|
| NAME | 工作流名称 |
| SCHEDULE | Cron 表达式（无则显示 `-`） |
| ENABLED | 是否启用 |
| AGE | 创建时间距今 |

```bash
opctl get workflows
opctl get wf -o json
```

---

## Cron Scheduling

### `opctl cron list`

列出所有定时工作流。

```
opctl cron list
```

**输出列：** `NAME`、`SCHEDULE`、`ENABLED`

```bash
opctl cron list
```

---

### `opctl cron enable`

启用定时工作流。

```
opctl cron enable <workflow-name>
```

```bash
opctl cron enable daily-research
```

---

### `opctl cron disable`

禁用定时工作流。

```
opctl cron disable <workflow-name>
```

```bash
opctl cron disable daily-research
```

---

## Monitoring

### `opctl status`

显示集群状态总览。

```
opctl status
```

**输出内容：** Agent 总数及各状态计数、任务总数及各状态计数、Agent 列表表格。

```bash
opctl status
```

---

### `opctl top agents`

显示 Agent 实时资源使用情况。

```
opctl top agents
```

**输出列：**

| Column | Description |
|--------|-------------|
| NAME | Agent 名称 |
| STATUS | 当前状态 |
| TASKS(C/F/R) | 任务计数（完成/失败/运行中） |
| TOKENS(IN/OUT) | Token 用量（输入/输出） |
| COST | 累计成本 |
| UPTIME | 运行时长（秒） |

```bash
opctl top agents
```

---

### `opctl health`

检查所有 Agent 的健康状态。

```
opctl health
```

**输出列：** `NAME`、`TYPE`、`HEALTHY`、`MESSAGE`

```bash
opctl health
```

---

## Cost Control

### `opctl cost report`

生成成本报告。

```
opctl cost report [--by <group>] [--period <duration>]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--by` | string | (none) | 分组维度：`agent`、`goal`、`project` |
| `--period` | string | `30d` | 时间范围：`1d`、`7d`、`30d` |

```bash
opctl cost report
opctl cost report --by agent --period 7d
opctl cost report --by goal
opctl cost report -o json
```

---

### `opctl cost watch`

显示当前预算状态。

```
opctl cost watch
```

**输出内容：** 每日/每月支出、限额、百分比、是否超支。

```bash
opctl cost watch
```

---

### `opctl cost export`

导出成本数据。

```
opctl cost export [--format <fmt>]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `csv` | 导出格式 |

```bash
opctl cost export --format csv > report.csv
```

---

### `opctl budget set`

设置预算限额。

```
opctl budget set [--daily <amount>] [--monthly <amount>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--daily` | string | 每日预算（如 `$10`） |
| `--monthly` | string | 每月预算（如 `$200`） |

```bash
opctl budget set --daily $10 --monthly $200
opctl budget set --monthly $500
```

---

## Configuration

### `opctl config get`

查看 Agent 配置。

```
opctl config get agent <name>
```

**输出内容：** 名称、类型、状态、重启次数、时间戳、完整 Spec YAML。

```bash
opctl config get agent coder
opctl config get agent coder -o json
```

---

### `opctl config set`

热更新 Agent 配置（无需重启）。

```
opctl config set agent <name> <key=value> [<key=value>...]
```

```bash
opctl config set agent coder runtime.model.name=claude-opus-4
opctl config set agent coder runtime.inference.temperature=0.5 resources.costLimit.perDay='$100'
```

---

### `opctl config history`

查看 Agent 配置变更历史。

```
opctl config history agent <name>
```

```bash
opctl config history agent coder
```

---

## Recovery

### `opctl recovery agent`

从 Checkpoint 或内存恢复 Agent。

```
opctl recovery agent <name> [--from <source>] [--checkpoint <id>]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from` | string | `checkpoint` | 恢复来源：`checkpoint`、`memory`、`manual` |
| `--checkpoint` | string | (none) | 指定 Checkpoint ID |

```bash
# 从最近 checkpoint 恢复
opctl recovery agent coder

# 从存储的 spec 恢复
opctl recovery agent coder --from memory

# 从指定 checkpoint 恢复
opctl recovery agent coder --checkpoint cp-coder-1710349200000
```

---

### `opctl checkpoints list`

列出 Agent 的所有 Checkpoint。

```
opctl checkpoints list agent <name>
```

**输出列：** `ID`、`AGENT`、`PHASE`、`PENDING TASKS`、`TIMESTAMP`

```bash
opctl checkpoints list agent coder
opctl checkpoints list agent coder -o json
```

---

### `opctl crashes agent`

查看 Agent 崩溃历史。

```
opctl crashes agent <name>
```

**输出列：** `AGENT`、`ERROR`、`TIMESTAMP`

```bash
opctl crashes agent coder
opctl crashes agent coder -o json
```

---

## Audit

### `opctl audit goal`

查看 Goal 的审计日志。

```
opctl audit goal <name>
```

```bash
opctl audit goal opc-platform-mvp
```

---

### `opctl audit trace`

追溯资源的完整审计链路。

```
opctl audit trace <resourceType> <name>
```

**支持的 resourceType：** `agent`、`task`、`goal`、`project`、`issue`、`workflow`

```bash
opctl audit trace agent coder
opctl audit trace issue search-hn-posts
```

**输出列：** `TIMESTAMP`、`EVENT`、`RESOURCE`、`NAME`、`DETAILS`

---

### `opctl audit export`

导出审计日志。

```
opctl audit export [--format <fmt>]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `json` | 导出格式 |

```bash
opctl audit export --format json > audit.json
```

---

## Utility

### `opctl version`

显示版本信息。

```
opctl version
```

**输出内容：** 版本号、Git Commit、构建日期、Go 版本、OS/Arch。

```bash
opctl version
```

---

## Resource Types

### AgentSpec

声明式 Agent 配置。完整字段参考：

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: string                # Agent 名称（必填）
  labels:                     # 标签
    key: value
spec:
  type: string                # openclaw | claude-code | codex | cursor | custom（必填）
  replicas: int               # 副本数（默认 1）

  runtime:
    model:
      provider: string        # 提供商（anthropic, openai）
      name: string            # 模型名称
      fallback: string        # 降级模型
    inference:
      thinking: string        # off | low | medium | high
      temperature: float      # 0.0 - 2.0
      maxTokens: int          # 最大 Token 数
    timeout:
      task: duration          # 任务超时（如 300s）
      idle: duration          # 空闲超时
      startup: duration       # 启动超时

  resources:
    tokenBudget:
      perTask: int            # 单任务 Token 上限
      perHour: int            # 每小时上限
      perDay: int             # 每日上限
    costLimit:
      perTask: string         # 单任务成本上限（如 "$1"）
      perDay: string          # 每日上限
      perMonth: string        # 每月上限
    onExceed: string          # pause | alert | downgrade

  context:
    workdir: string           # 工作目录
    skills: [string]          # 可用技能

  healthCheck:
    type: string              # heartbeat
    interval: duration        # 检查间隔
    timeout: duration         # 超时时间
    retries: int              # 重试次数

  recovery:
    enabled: bool             # 启用自动恢复
    maxRestarts: int          # 最大重启次数
    restartDelay: duration    # 重启延迟
    backoff: string           # exponential | linear | fixed

  # 仅 custom 类型
  command: [string]           # 启动命令
  args: [string]              # 命令参数
  env:                        # 环境变量
    KEY: value
  protocol:
    type: string              # stdio | http
    format: string            # jsonl | text
```

### Workflow

多步骤工作流定义：

```yaml
apiVersion: opc/v1
kind: Workflow
metadata:
  name: string                # 工作流名称
spec:
  schedule: string            # Cron 表达式（可选）

  steps:
    - name: string            # 步骤名称
      agent: string           # 执行 Agent
      dependsOn: [string]     # 依赖步骤
      input:
        message: string       # 输入消息
        context: [string]     # 上下文引用（${{ steps.x.outputs.y }}）
      outputs:
        - name: string        # 输出名称
```

---

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | 成功 |
| 1 | 一般错误（参数错误、资源不存在等） |

---

## Environment

OPC Platform 使用以下目录：

| Path | Description |
|------|-------------|
| `~/.opc/config.yaml` | 全局配置 |
| `~/.opc/state/opc.db` | SQLite 数据库 |
| `~/.opc/checkpoints/` | Agent Checkpoint |
| `~/.opc/crashes/` | 崩溃报告 |
| `~/.opc/audit/` | 审计日志（JSONL） |
| `~/.opc/cost/` | 成本数据（JSONL） |
