# OPC Platform Quick Start Guide

本指南将帮助你在 10 分钟内启动 OPC Platform，管理你的第一个 AI Agent 集群。

---

## 1. 安装

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/zlc-ai/opc-platform.git
cd opc-platform

# 编译
go build -o opctl ./cmd/opctl/

# 安装到 PATH（可选）
sudo mv opctl /usr/local/bin/

# 验证安装
opctl version
```

输出：
```
opctl version: 0.1.0-dev
  Git Commit:  (unknown)
  Build Date:  (unknown)
  Go Version:  go1.23.4
  OS/Arch:     darwin/arm64
```

首次运行时，OPC 会自动创建配置目录 `~/.opc/`。

---

## 2. 创建你的第一个 Agent

### 2.1 编写 Agent 配置

创建文件 `my-agent.yaml`：

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: coder
  labels:
    role: developer
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4-20250514
    inference:
      thinking: medium
      temperature: 0.7
      maxTokens: 16384
    timeout:
      task: 600s
  resources:
    tokenBudget:
      perTask: 200000
      perDay: 5000000
    costLimit:
      perTask: "$2"
      perDay: "$50"
    onExceed: pause
  context:
    workdir: /workspace/my-project
  healthCheck:
    type: heartbeat
    interval: 120s
    timeout: 10s
    retries: 3
  recovery:
    enabled: true
    maxRestarts: 3
    restartDelay: 15s
    backoff: exponential
```

### 2.2 应用配置

```bash
opctl apply -f my-agent.yaml
```

输出：
```
agent/coder configured
```

### 2.3 查看 Agent

```bash
opctl get agents
```

输出：
```
NAME    TYPE         STATUS   RESTARTS  AGE
coder   claude-code  Created  0         5s
```

### 2.4 查看 Agent 详情

```bash
opctl describe agent coder
```

---

## 3. 支持的 Agent 类型

OPC Platform 支持 5 种 Agent 类型：

| 类型 | 描述 | 启动方式 |
|------|------|----------|
| `openclaw` | OpenClaw Agent | `openclaw agent start` (stdin/stdout) |
| `claude-code` | Claude Code CLI | `claude --print` (per-task process) |
| `codex` | OpenAI Codex CLI | `codex -q --approval-mode full-auto` |
| `cursor` | Cursor Agent | Cursor API (HTTP) |
| `custom` | 自定义 Agent | 用户定义的命令 (stdio/http) |

### 使用示例 YAML

```bash
# OpenClaw Agent
opctl apply -f examples/agent-openclaw.yaml

# Claude Code Agent
opctl apply -f examples/agent-claude-code.yaml

# Codex Agent
opctl apply -f examples/agent-codex.yaml

# 自定义 Agent
opctl apply -f examples/agent-custom.yaml
```

---

## 4. 执行任务

### 4.1 同步执行

```bash
opctl run --agent coder "Write a function to check if a number is prime"
```

输出：
```
task/task-1710349200000 created (agent: coder)
<Agent 输出结果>

--- Tokens: in=1234 out=567 ---
```

### 4.2 流式输出

```bash
opctl run --agent coder --stream "Explain the OPC Platform architecture"
```

### 4.3 查看任务列表

```bash
opctl get tasks
```

输出：
```
ID                      AGENT   STATUS     MESSAGE                              AGE
task-1710349200000      coder   Completed  Write a function to check if a ...   2m
```

### 4.4 查看任务日志

```bash
opctl logs task-1710349200000
```

---

## 5. 工作流编排

### 5.1 定义工作流

创建文件 `my-workflow.yaml`：

```yaml
apiVersion: opc/v1
kind: Workflow
metadata:
  name: code-review-pipeline
spec:
  steps:
    - name: analyze
      agent: coder
      input:
        message: "Analyze the code in the current directory for potential issues"
      outputs:
        - name: analysis

    - name: review
      agent: coder
      dependsOn: [analyze]
      input:
        message: "Based on this analysis, provide a detailed code review"
        context:
          - "${{ steps.analyze.outputs.analysis }}"
      outputs:
        - name: review

    - name: summary
      agent: coder
      dependsOn: [review]
      input:
        message: "Summarize the code review into actionable items"
        context:
          - "${{ steps.review.outputs.review }}"
```

### 5.2 应用并执行

```bash
# 注册工作流
opctl apply -f my-workflow.yaml

# 查看工作流列表
opctl get workflows

# 执行工作流
opctl run workflow code-review-pipeline
```

### 5.3 定时执行

在 Workflow 中添加 `schedule` 字段即可实现 Cron 调度：

```yaml
spec:
  schedule: "0 9 * * 1-5"  # 工作日每天早上 9 点
```

管理定时任务：

```bash
# 查看所有定时工作流
opctl cron list

# 暂停定时工作流
opctl cron disable code-review-pipeline

# 恢复定时工作流
opctl cron enable code-review-pipeline
```

---

## 6. 监控与运维

### 6.1 集群状态总览

```bash
opctl status
```

输出：
```
OPC Cluster Status
==================

Agents: 3 total (2 running, 1 stopped, 0 failed)
Tasks:  15 total (0 pending, 1 running, 12 completed, 2 failed)

NAME       TYPE         STATUS   RESTARTS
coder      claude-code  Running  0
reviewer   openclaw     Running  0
automation codex        Stopped  1
```

### 6.2 实时资源监控

```bash
opctl top agents
```

输出：
```
NAME       STATUS   TASKS(C/F/R)  TOKENS(IN/OUT)     COST     UPTIME
coder      Running  10/1/1        125000/89000       $1.23    3600s
reviewer   Running  5/0/0         45000/32000        $0.45    7200s
```

### 6.3 健康检查

```bash
opctl health
```

### 6.4 重启 Agent

```bash
opctl restart agent coder
```

---

## 7. 成本控制

### 7.1 设置预算

```bash
# 设置每日预算 $10，每月 $200
opctl budget set --daily $10 --monthly $200
```

### 7.2 查看成本报告

```bash
# 默认 30 天报告
opctl cost report

# 按 Agent 分组
opctl cost report --by agent

# 按 Goal 分组
opctl cost report --by goal

# 最近 7 天
opctl cost report --period 7d
```

### 7.3 查看预算状态

```bash
opctl cost watch
```

输出：
```
Budget Status
Daily:   $3.45 / $10.00 (35%)
Monthly: $67.80 / $200.00 (34%)
```

### 7.4 导出成本数据

```bash
opctl cost export --format csv > cost-report.csv
```

---

## 8. 崩溃恢复

### 8.1 自动恢复

OPC Platform 会自动检测 Agent 崩溃并尝试恢复。配置在 AgentSpec 的 `recovery` 字段中：

```yaml
recovery:
  enabled: true
  maxRestarts: 5          # 最多重启 5 次
  restartDelay: 10s       # 初始重启延迟
  backoff: exponential    # 指数退避
```

### 8.2 手动恢复

```bash
# 从最近的 checkpoint 恢复
opctl recovery agent coder

# 从存储的 spec 恢复
opctl recovery agent coder --from memory

# 从指定 checkpoint 恢复
opctl recovery agent coder --checkpoint cp-coder-1710349200000
```

### 8.3 查看 Checkpoint

```bash
opctl checkpoints list agent coder
```

### 8.4 查看崩溃历史

```bash
opctl crashes agent coder
```

---

## 9. 审计追溯

```bash
# 查看某个 Goal 的所有活动
opctl audit goal my-goal

# 追溯完整链路
opctl audit trace agent coder

# 导出审计日志
opctl audit export --format json > audit.json
```

---

## 10. 配置管理

### 10.1 查看 Agent 配置

```bash
opctl config get agent coder
```

### 10.2 热更新配置（无需重启）

```bash
opctl config set agent coder runtime.model.name=claude-opus-4
```

### 10.3 查看配置变更历史

```bash
opctl config history agent coder
```

### 10.4 扩缩容

```bash
opctl scale agent coder --replicas 3
```

---

## 11. 自定义 Agent

任何支持 stdin/stdout 或 HTTP 协议的程序都可以接入 OPC Platform。

### stdin/stdout + JSONL 协议

Agent 需要支持以下 JSON 协议：

**请求（OPC → Agent）：**
```json
{"type": "execute", "message": "your task message", "id": "task-123"}
```

**响应（Agent → OPC）：**
```json
{"type": "response", "content": "partial output...", "done": false}
{"type": "response", "content": "final output", "done": true, "tokens_in": 100, "tokens_out": 200}
```

### Python 示例

```python
#!/usr/bin/env python3
import json
import sys

for line in sys.stdin:
    req = json.loads(line.strip())
    if req["type"] == "execute":
        result = f"Processed: {req['message']}"
        resp = {"type": "response", "content": result, "done": True}
        print(json.dumps(resp), flush=True)
```

对应的 YAML 配置：

```yaml
apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: my-python-agent
spec:
  type: custom
  command: ["python3", "agent.py"]
  protocol:
    type: stdio
    format: jsonl
```

---

## 配置文件位置

```
~/.opc/
├── config.yaml           # 全局配置
├── state/
│   └── opc.db            # SQLite 数据库
├── checkpoints/          # Agent 状态快照
├── crashes/              # 崩溃报告
├── audit/                # 审计日志
└── cost/                 # 成本数据
```

---

## 下一步

- 阅读 [docs/COMMANDS.md](docs/COMMANDS.md) 查看完整命令参考
- 查看 [examples/](examples/) 目录了解更多配置示例
- 阅读 [OPC_Platform_PRD.md](OPC_Platform_PRD.md) 了解产品设计理念
