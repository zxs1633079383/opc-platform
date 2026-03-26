# OPC Platform 开发指南

## 项目目标
构建 OPC Platform — AI Agent 的 Kubernetes，让一个人能像管理容器集群一样管理 AI Agent 集群。

**终极目标**：GitHub 全球前 20，OPC Platform 品类第一

## 核心文档
- `OPC_Platform_PRD.md` — 完整产品需求文档
- `OPC_Platform_TaskManager.md` — 任务清单（跟踪进度）

## 技术栈
- **语言**：Go 1.22+
- **CLI 框架**：Cobra
- **存储**：SQLite (BoltDB 备选)
- **通信**：WebSocket / stdin-stdout
- **构建**：Make + GoReleaser

## 开发规范

### 项目结构
```
opc_platform/
├── cmd/
│   └── opctl/           # CLI 入口
├── pkg/
│   ├── a2a/             # A2A protobuf 通信层（Bridge/Server/Client/Convert）
│   ├── adapter/         # Agent 适配器
│   │   ├── openclaw/
│   │   ├── claudecode/
│   │   ├── codex/
│   │   ├── openai/
│   │   └── custom/
│   ├── auth/            # 认证授权
│   ├── controller/      # 生命周期控制器
│   ├── cost/            # 成本追踪 + 配额执行
│   ├── dispatcher/      # 智能调度
│   ├── evolve/          # 自演进循环：指标采集 + RFC（v0.7 预研）
│   ├── federation/      # 联邦协同 + gRPC 通信
│   ├── gateway/         # 外部接入（Telegram/Discord）
│   ├── goal/            # Goal 分解 + DAG 执行 + A2A 评审
│   ├── model/           # 模型注册表（v0.7）
│   ├── server/          # REST API + gRPC Server
│   ├── storage/         # 状态存储（SQLite/PostgreSQL）
│   ├── tenant/          # 多租户
│   ├── trace/           # 分布式追踪
│   ├── workflow/        # 工作流引擎
│   └── audit/           # 审计日志
├── proto/
│   ├── a2a/             # Google A2A 标准 protobuf 定义
│   └── opc/             # OPC gRPC 服务定义
├── gen/                 # protobuf 生成代码（make proto）
├── api/
│   └── v1/              # YAML 规范定义
├── dashboard/           # Next.js + React 前端（:4000）
├── test/
│   └── integration/     # gRPC 集成测试
├── internal/
│   └── config/          # 配置管理
├── docs/
├── examples/
└── Makefile             # proto 生成 + 构建
```

### 开发流程
1. 每完成一个 Task，更新 `OPC_Platform_TaskManager.md` 中对应的状态（⚪ → 🟢）
2. 每个 Issue 完成后 commit
3. 每个 Task 完成后 push
4. 保持代码质量：测试覆盖率 > 80%

### Git 规范
- 分支：`feature/<task-id>-<name>`
- Commit：`feat(module): description`
- 每个 Phase 完成后打 tag

## 当前阶段
Phase 7: Production Hardening + Self-Evolving Loop（v0.7 进行中）
- A2A protobuf 通信已实现（gRPC :9528）
- Dashboard 全面升级（Agent 向导 / Goal 树形 / RFC 审批 / 指标仪表盘）
- 测试覆盖率 P0 包全部 80%+

<!-- gitnexus:start -->
# GitNexus MCP

This project is indexed by GitNexus as **opc_platform** (1014 symbols, 2756 relationships, 111 execution flows).

GitNexus provides a knowledge graph over this codebase — call chains, blast radius, execution flows, and semantic search.

## Always Start Here

For any task involving code understanding, debugging, impact analysis, or refactoring, you must:

1. **Read `gitnexus://repo/{name}/context`** — codebase overview + check index freshness
2. **Match your task to a skill below** and **read that skill file**
3. **Follow the skill's workflow and checklist**

> If step 1 warns the index is stale, run `npx gitnexus analyze` in the terminal first.

## Skills

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/refactoring/SKILL.md` |

## Tools Reference

| Tool | What it gives you |
|------|-------------------|
| `query` | Process-grouped code intelligence — execution flows related to a concept |
| `context` | 360-degree symbol view — categorized refs, processes it participates in |
| `impact` | Symbol blast radius — what breaks at depth 1/2/3 with confidence |
| `detect_changes` | Git-diff impact — what do your current changes affect |
| `rename` | Multi-file coordinated rename with confidence-tagged edits |
| `cypher` | Raw graph queries (read `gitnexus://repo/{name}/schema` first) |
| `list_repos` | Discover indexed repos |

## Resources Reference

Lightweight reads (~100-500 tokens) for navigation:

| Resource | Content |
|----------|---------|
| `gitnexus://repo/{name}/context` | Stats, staleness check |
| `gitnexus://repo/{name}/clusters` | All functional areas with cohesion scores |
| `gitnexus://repo/{name}/cluster/{clusterName}` | Area members |
| `gitnexus://repo/{name}/processes` | All execution flows |
| `gitnexus://repo/{name}/process/{processName}` | Step-by-step trace |
| `gitnexus://repo/{name}/schema` | Graph schema for Cypher |

## Graph Schema

**Nodes:** File, Function, Class, Interface, Method, Community, Process
**Edges (via CodeRelation.type):** CALLS, IMPORTS, EXTENDS, IMPLEMENTS, DEFINES, MEMBER_OF, STEP_IN_PROCESS

```cypher
MATCH (caller)-[:CodeRelation {type: 'CALLS'}]->(f:Function {name: "myFunc"})
RETURN caller.name, caller.filePath
```

<!-- gitnexus:end -->
