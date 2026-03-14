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
│   ├── adapter/         # Agent 适配器
│   │   ├── openclaw/
│   │   ├── claudecode/
│   │   └── codex/
│   ├── controller/      # 生命周期控制器
│   ├── dispatcher/      # 智能调度
│   ├── workflow/        # 工作流引擎
│   ├── storage/         # 状态存储
│   ├── cost/            # 成本追踪
│   └── audit/           # 审计日志
├── api/
│   └── v1/              # YAML 规范定义
├── internal/
│   └── config/          # 配置管理
├── docs/
├── examples/
└── test/
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
Phase 1: Foundation（Week 1-2）

## 立即开始
从 Task 1.1.1 开始：CLI 骨架搭建
