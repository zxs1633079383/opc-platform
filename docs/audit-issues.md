# OPC Platform 代码审计问题清单

**审计日期**: 2026-03-19
**版本**: v0.5-dev
**状态**: 存档待修复（本次仅记录，不修复）

---

## CRITICAL 级

### C-01: 零认证/零授权
- **文件**: `pkg/server/server.go`
- **问题**: 所有 API 端点无认证中间件，`Access-Control-Allow-Origin: *`
- **风险**: 任何人可启停 Agent、创建 Goal、注册假联邦公司
- **建议**: 添加 JWT/API Key 认证中间件

### C-02: 大面积错误吞没（15+ 处）
- **文件**: `pkg/server/server.go`
- **问题**: `agents, _ := s.controller.ListAgents(ctx)` 等 15+ 处丢弃 error
- **高危**: line 2026 `g2, _ := s.controller.Store().GetGoal(bgCtx, goalID)` 失败后操作空对象
- **建议**: 替换为正确错误处理 + 日志

### C-03: Federation RetryQueue 不持久化
- **文件**: `pkg/federation/retry.go`
- **问题**: RetryQueue 仅存内存，重启丢失所有失败回调
- **建议**: 持久化到 SQLite 表

### C-04: FederatedGoalRuns 竞态条件
- **文件**: `pkg/server/server.go`
- **问题**: `advanceFederatedGoal` 中锁过早释放，并发回调导致数据竞争
- **建议**: 扩大锁范围覆盖完整 project 状态修改

---

## HIGH 级

### H-01: server.go 2414 行超标
- **规范**: 800 行以内
- **建议**: 拆分为 handlers_goals.go、handlers_federation.go、handlers_workflow.go、handlers_metrics.go

### H-02: 输入验证缺失
| 端点 | 问题 |
|------|------|
| `/api/run` | message 无长度限制 |
| `/api/federation/companies` | endpoint 无 URL 校验（SSRF 风险）|
| `/api/goals/federated` | companyId 不检查是否存在，项目数无上限 |
| Goal 创建 | DAG 验证在存储之后 |

### H-03: Goroutine 泄漏风险
| 文件 | 问题 |
|------|------|
| `pkg/adapter/claudecode/claudecode.go` | `cmd.Wait()` goroutine 在 context 取消后孤立 |
| `pkg/adapter/custom/custom.go` | 流式 goroutine 无超时 |

### H-04: 资源泄漏
- **文件**: `pkg/adapter/custom/custom.go:176`
- **问题**: `waitForHTTPReady()` 成功路径不关闭 `resp.Body`

### H-05: 密钥明文存储
- OpenClaw token: `~/.openclaw/openclaw.json`（明文 JSON）
- OpenAI API key: struct 字段，可在内存 dump 暴露
- 无密钥轮换机制

---

## MEDIUM 级

### M-01: 硬编码值
| 位置 | 值 | 应该 |
|------|-----|------|
| server.go:70 | `Port = 9527` | 环境变量 |
| server.go:73 | `Host = "127.0.0.1"` | 配置文件 |
| openclaw.go | `ws://localhost:18789` | 配置 |
| server.go:1438 | `maxRounds = 3` | 配置 |

### M-02: Audit/Cost 持久化不可靠
- JSONL 文件追加写入，全量加载内存
- 无分页、无时间范围过滤、无保留策略（PRD 要求 365 天）
- 长期运行内存无限增长

### M-03: Federation 安全不足
- 无 TLS/mTLS
- APIKey 无轮换/撤销
- 时钟偏移处理不足
- 无按公司限速/配额

### M-04: PRD 功能缺失
| 功能 | 状态 |
|------|------|
| Agent 扩缩容（实际执行）| replicas 字段定义但未使用 |
| 配置热更新（持久化）| config set 只打印消息 |
| config history | 返回空 stub |
| CostEvent.IssueRef | 无法追踪到 Issue 级别 |
| Goal Plan/Approve/Revise UI | Dashboard 无对应按钮 |
| Workflow 编辑 | 只能创建不能编辑 |

### M-05: HTTP 状态码不一致
- `DeleteAgent` 失败返回 500（应为 404）
- `DeleteWorkflow` 失败返回 500（应为 404）
- 缺少 429 Too Many Requests

### M-06: 无速率限制
- 无请求限速中间件
- 无 DDoS 防护
- 无大 payload 限制

---

## 已知问题（v0.5 测试报告）

| 编号 | 描述 | 严重度 |
|------|------|--------|
| K-01 | health 检查期望 `ok` 但返回 `healthy` | Low |
| K-02 | `opctl goal list` CLI 返回空 | Medium |
| K-03 | OpenClaw Execute 结果不含在 logs 中 | 已修 |
| K-04 | Workflow toggle 前后端字段传递 | Low |

---

*最后更新: 2026-03-19*
