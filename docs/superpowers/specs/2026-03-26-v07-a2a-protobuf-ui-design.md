# OPC Platform v0.7 — A2A Protobuf 通信 + UI 全面打磨 设计文档

**日期**: 2026-03-26
**分支**: feature/prd-entropy-alignment
**基线 commit**: 82e7ad5
**状态**: 已审批，待实施

---

## 1. 目标

在 PRD v0.7 基础上实现三大方向：

1. **A2A Protobuf 通信**：将所有 OPC 内部通信（Master ↔ Agent、Master ↔ Master、Agent 管控）替换为 Google A2A 标准协议 + protobuf 序列化
2. **Dashboard 前端全面打磨**：补全树形可视化、RFC 审批、系统指标等页面，增加无代码 Agent 创建向导，整体 UI 打磨
3. **PRD v0.7 代码补全**：测试覆盖率提升、Self-Evolving 预研、Workflow 执行详情等

## 2. 关键决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| A2A 协议标准 | Google A2A（HTTP + JSON-RPC 语义 + protobuf 序列化） | 行业标准，未来可对接第三方 A2A Agent |
| A2A 定位 | 通信协议层，OPC 保留自有概念模型（Goal/Project/Task/Issue），内部做映射转换 | 最小侵入性，现有逻辑不动 |
| protobuf 工具链 | 标准 google.golang.org/protobuf + google.golang.org/grpc | 最主流，社区支持最好 |
| REST API 处理 | 双协议并存 — gRPC 管 Agent/Federation 通信，REST 保留给 Dashboard/opctl CLI | 改动最小风险最低 |
| AgentCard 发现机制 | 注册时静态声明 — AgentCard 信息在 AgentSpec YAML 中声明，OPC Server 存储后对外提供查询 API | OPC 管理的外部 Agent 不会自行暴露 A2A 端点 |
| 实施策略 | 分层渐进 + 交叉推进 | 每步可验证，不丢代码 |
| 并行策略 | 严格串行执行，不使用并行 agent 开发代码 | 避免互相覆盖文件 |

## 3. Proto 定义与 A2A 模型映射

### 3.1 Proto 文件结构

```
proto/
├── a2a/
│   └── a2a.proto              # Google A2A 核心类型（AgentCard, Task, Message, Part, Artifact）
├── opc/
│   ├── agent_service.proto    # Master ↔ Agent 管控（Start/Stop/Execute/Stream/Health）
│   ├── federation_service.proto # Master ↔ Master 联邦协同（DispatchGoal/Callback/Heartbeat）
│   └── types.proto            # OPC 扩展类型（Goal, Project, Issue, CostEvent 等）
└── buf.yaml                   # 仅用于 proto lint，不用 buf 生成
```

### 3.2 Google A2A 核心类型映射

| A2A 标准概念 | Proto Message | OPC 现有概念 | 映射方式 |
|-------------|---------------|-------------|---------|
| AgentCard | `a2a.AgentCard` | `v1.AgentSpec` | AgentSpec YAML → AgentCard（注册时转换，存储） |
| Task | `a2a.Task` | `v1.TaskRecord` | 双向转换：TaskRecord ↔ A2A Task |
| Message | `a2a.Message` | 任务输入/输出 | Execute 请求 = Message(role=user)，结果 = Message(role=agent) |
| Part | `a2a.TextPart / DataPart / FilePart` | string Output | Output 包装为 TextPart；未来支持 FilePart |
| Artifact | `a2a.Artifact` | `ExecuteResult` | ExecuteResult → Artifact（含 cost metadata） |
| TaskStatus | `a2a.TaskState` enum | `v1.AgentPhase` / Task status | 状态机映射 |

### 3.3 A2A Task 状态机 → OPC 映射

```
A2A:  submitted → working → completed/failed/canceled/input-required
OPC:  created   → running → completed/failed/terminated

映射:
  submitted      ↔ created
  working        ↔ running
  completed      ↔ completed
  failed         ↔ failed
  canceled       ↔ terminated
  input-required → OPC 暂不支持（预留字段）
```

### 3.4 映射层代码位置

```
pkg/a2a/
├── convert.go          # OPC Model ↔ A2A Proto 双向转换函数
├── convert_test.go
├── agentcard.go        # AgentSpec → AgentCard 转换
└── agentcard_test.go
```

## 4. gRPC 服务定义

### 4.1 AgentService — Master ↔ Agent 管控

```protobuf
service AgentService {
  // 生命周期管控
  rpc Start(StartRequest) returns (StartResponse);
  rpc Stop(StopRequest) returns (StopResponse);
  rpc Health(HealthRequest) returns (HealthResponse);

  // 任务执行（A2A 语义：发送 Message，返回 Task）
  rpc SendTask(SendTaskRequest) returns (SendTaskResponse);
  rpc SendTaskStreaming(SendTaskStreamingRequest) returns (stream TaskStatusUpdate);

  // A2A 标准：任务状态查询
  rpc GetTask(GetTaskRequest) returns (a2a.Task);
  rpc CancelTask(CancelTaskRequest) returns (a2a.Task);

  // Agent 能力发现
  rpc GetAgentCard(GetAgentCardRequest) returns (a2a.AgentCard);
}
```

### 4.2 FederationService — Master ↔ Master 联邦

```protobuf
service FederationService {
  // 节点注册与心跳
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc HeartbeatStream(stream HeartbeatPing) returns (stream HeartbeatPong);

  // 联邦 Goal 调度
  rpc DispatchProject(DispatchProjectRequest) returns (DispatchProjectResponse);

  // Worker 回调结果
  rpc ReportTaskResult(ReportTaskResultRequest) returns (ReportTaskResultResponse);

  // 跨节点 A2A 评审
  rpc AssessResult(AssessRequest) returns (AssessResponse);

  // 联邦状态查询
  rpc GetFederationStatus(GetFederationStatusRequest) returns (FederationStatusResponse);
}
```

### 4.3 Federation Proto 消息

```protobuf
message DispatchProjectRequest {
  string goal_id = 1;
  string project_name = 2;
  string agent_name = 3;
  string company = 4;
  a2a.Message task_message = 5;
  repeated string dependencies = 6;
  string trace_context = 7;
  CostConstraints cost_constraints = 8;
}

message ReportTaskResultRequest {
  string goal_id = 1;
  string project_name = 2;
  a2a.Task completed_task = 3;
  CostReport cost = 4;
  AssessmentResult assessment = 5;
}

message CostReport {
  int64 tokens_in = 1;
  int64 tokens_out = 2;
  double cost_usd = 3;
  int64 duration_ms = 4;
  bool estimated = 5;
}

message HeartbeatPing {
  string node_id = 1;
  string company = 2;
  repeated string available_agents = 3;
  ResourceUsage resources = 4;
  int64 timestamp = 5;
}

message HeartbeatPong {
  bool accepted = 1;
  repeated PendingDispatch pending = 2;
}
```

### 4.4 认证机制

- **AgentService**：本地通信，默认无认证（可选 mTLS）
- **FederationService**：复用现有 HMAC-SHA256，放入 gRPC metadata（key: `x-opc-api-key`），gRPC interceptor 统一校验

### 4.5 端口规划

```
:9527  — REST API（Gin，Dashboard/opctl 用，保持不变）
:9528  — gRPC（AgentService + FederationService）
```

## 5. Adapter 层改造 — Bridge 模式

### 5.1 架构

```
Controller → gRPC AgentService.SendTask()
                ↓
         A2A Bridge（pkg/a2a/bridge.go）
                ↓
         原生 Adapter 执行（openclaw/claudecode/custom — 零改动）
                ↓
         ExecuteResult → A2A Artifact
                ↓
         gRPC Response 返回
```

### 5.2 现有 Adapter 全部保留，零改动

```
pkg/adapter/
├── adapter.go          # 接口不变
├── openclaw/           # WebSocket RPC — 不动
├── claudecode/         # CLI JSON — 不动
├── codex/              # CLI — 不动
├── custom/             # stdin/HTTP — 不动
└── openai/             # 不动
```

### 5.3 新增 A2A 层

```go
// pkg/a2a/bridge.go
type Bridge struct {
    adapters map[string]adapter.Adapter
}

func (b *Bridge) SendTask(ctx context.Context, req *pb.SendTaskRequest) (*pb.SendTaskResponse, error) {
    task := convert.SendTaskRequestToTask(req)
    result, err := b.adapters[agentName].Execute(ctx, task)
    return convert.ResultToSendTaskResponse(result), err
}
```

```go
// pkg/a2a/client.go — A2AClient 实现 adapter.Adapter 接口，底层走 gRPC
type A2AClient struct {
    conn   *grpc.ClientConn
    client pb.AgentServiceClient
}
```

```go
// pkg/a2a/server.go — gRPC 服务端，委托给 Bridge
type AgentServiceServer struct {
    bridge *Bridge
}
```

### 5.4 调用链路

| 场景 | 链路 |
|------|------|
| 本地 OpenClaw | Controller → gRPC → Bridge → OpenClaw Adapter → WebSocket → OpenClaw 进程 |
| 本地 Claude Code | Controller → gRPC → Bridge → ClaudeCode Adapter → CLI exec → claude 进程 |
| 本地 Custom | Controller → gRPC → Bridge → Custom Adapter → stdin/stdout |
| 远程 Federation | Master Controller → gRPC FederationService → Worker gRPC → Bridge → 原生 Adapter |
| 未来原生 A2A Agent | Controller → gRPC → 直接对接（不经 Bridge） |

### 5.5 AgentSpec 扩展

```yaml
spec:
  protocol:
    transport: bridge    # bridge（经 Bridge 翻译）| native-a2a（直连 gRPC）
```

## 6. Federation 层改造

### 6.1 改造前后对比

```
改造前：Master ──HTTP POST──► Worker（JSON + HMAC header）
改造后：Master ──gRPC──► Worker（protobuf + HMAC metadata）
```

### 6.2 双协议过渡期

```
Phase A：gRPC 服务启动，但 Federation 仍走 HTTP（现状）
Phase B：新建连接优先 gRPC，HTTP 作为 fallback
Phase C：全部切 gRPC，移除 HTTP Federation 代码

if worker.SupportsGRPC() {
    federationClient.DispatchProject(grpcReq)
} else {
    http.Post(worker.URL + "/api/agents/...")  // 旧 fallback
}
```

### 6.3 Heartbeat 改造

```
现在：HTTP POST /api/companies（注册 + 心跳混合）
改后：gRPC FederationService.HeartbeatStream() — 双向流
```

### 6.4 gRPC Interceptor — HMAC 认证

```go
// pkg/a2a/interceptor.go
func HMACUnaryInterceptor(apiKeyStore federation.APIKeyStore) grpc.UnaryServerInterceptor
func HMACStreamInterceptor(apiKeyStore federation.APIKeyStore) grpc.StreamServerInterceptor
```

### 6.5 代码位置

```
pkg/federation/
├── federation.go          # 现有逻辑保留
├── auth.go                # HMAC 逻辑保留，新增 gRPC interceptor 调用
├── retry.go               # RetryQueue 保留，改为重试 gRPC 调用
├── grpc_client.go         # 新增：gRPC 客户端（Master → Worker）
└── grpc_server.go         # 新增：gRPC 服务端（Worker 接收 Master 派发）
```

## 7. Dashboard 前端全面打磨

### 7.1 新增 & 增强页面

| 页面 | 状态 | 内容 |
|------|------|------|
| Goals 树形可视化 | 新增 | Goal → Project → Task → Issue 树形 + DAG 依赖连线 + 进度 + A2A 评审轮次 |
| RFC 审批 | 新增 | RFC 列表 + 详情 + 审批/拒绝操作 |
| 系统指标 | 新增 | SuccessRate / AvgLatency / CostPerGoal / RetryRate 时序图表 |
| Workflow 执行详情 | 增强 | Run 详情页：Step 状态/耗时/输出/DAG 图 |
| Federation 增强 | 增强 | 节点心跳实时状态 + gRPC 连接状态 |
| Agent 详情 | 增强 | AgentCard 展示 + A2A 协议状态 + 历史任务时间线 |
| 成本报表增强 | 增强 | 多维度切换 + 趋势图 + 预算进度条 |
| 整体 UI | 打磨 | 响应式 + 暗色模式 + 动效 + 骨架屏 + 空状态 |

### 7.2 新增组件

```
dashboard/src/components/
├── GoalTree.tsx
├── DAGVisualization.tsx     # @xyflow/react
├── RFCCard.tsx
├── MetricsChart.tsx         # recharts
├── WorkflowRunDetail.tsx
├── NodeStatusBadge.tsx
├── AgentCardView.tsx
├── BudgetProgress.tsx
├── Skeleton.tsx
├── EmptyState.tsx
└── ThemeToggle.tsx
```

### 7.3 新增页面路由

```
dashboard/src/app/
├── goals/[id]/page.tsx          # Goal 详情 + 树形
├── rfcs/page.tsx                # RFC 审批
├── metrics/page.tsx             # 系统指标
└── workflows/[name]/runs/[id]/page.tsx  # Run 详情
```

### 7.4 图表库

- **recharts** — 时序图表（指标、成本趋势）
- **@xyflow/react** — DAG 可视化（Goal 依赖、Workflow 步骤）

## 8. 无代码 Agent 创建向导

### 8.1 向导流程

```
Step 1: 选择 Agent 类型（卡片选择：代码助手 / 研究分析 / 内容创作 / 自定义 / OpenAI）
Step 2: 描述能力（自然语言）+ 选择模型（分组展示：Anthropic / OpenAI / 自定义）+ 备用模型
Step 3: 资源与预算（预设方案 + 滑块 + 超限策略）
Step 4: 确认 & 注册（AgentSpec YAML 预览 + A2A AgentCard 预览 + 创建）
```

### 8.2 模型选择

按 Provider 分组展示，支持自定义模型 ID 输入：

- **Anthropic**：Claude Sonnet 4.6（推荐）、Claude Opus 4.6、Claude Haiku 4.5
- **OpenAI**：GPT-4o、GPT-4o-mini、o3
- **自定义**：用户输入模型 ID（deepseek-v3, qwen-72b 等）
- **备用模型**：下拉选择，可选

### 8.3 后端 API

```
POST /api/agents/wizard    — 向导式创建（简化入参 → 完整 AgentSpec + AgentCard）
GET  /api/models           — 模型注册表（前端下拉数据源）
POST /api/models           — 添加自定义模型
```

### 8.4 ModelInfo 结构

```go
type ModelInfo struct {
    ID          string  `json:"id"`
    Provider    string  `json:"provider"`
    DisplayName string  `json:"displayName"`
    Tier        string  `json:"tier"`        // "economy" | "standard" | "premium"
    CostPer1K   float64 `json:"costPer1k"`
    Capability  string  `json:"capability"`  // "fast" | "balanced" | "reasoning"
    Default     bool    `json:"default"`
}
```

### 8.5 智能推断

- **v0.7**：规则匹配（关键词 → 技能标签映射表）
- **v0.8**：AI 推断（调 claude-haiku 从描述生成 skills + 推荐模型）

### 8.6 前端组件

```
dashboard/src/components/AgentWizard/
├── AgentWizard.tsx
├── StepTypeSelect.tsx
├── StepDescribe.tsx       # 含模型选择
├── StepResources.tsx
├── StepConfirm.tsx
└── YAMLPreview.tsx
```

## 9. PRD v0.7 代码补全

### 9.1 测试覆盖率提升（P0）

| 包 | 当前 | 目标 |
|----|------|------|
| pkg/controller | 55.1% | 80% |
| pkg/server | 55.6% | 80% |
| pkg/adapter/claudecode | 29.9% | 80% |
| pkg/storage/sqlite | 37.7% | 80% |

### 9.2 功能补全

- OpenClaw 密钥持久化 + 自动读 Token（5.5.1）
- Goal 分解持久化 + 成本追踪（5.5.2）
- Workflow 执行详情 API（5.5.4）
- pkg/evolve MetricsCollector 骨架（v0.8 预研）

### 9.3 Self-Evolving Loop 预研

```
pkg/evolve/
├── metrics.go         # MetricsCollector — 采集 SuccessRate/AvgLatency/RetryRate 等
├── analyzer.go        # 骨架 — AI 异常分析（v0.8 实现）
├── rfc.go             # RFC 数据结构 + 存储
└── metrics_test.go
```

## 10. 实施阶段划分

### Phase A：Proto 定义 + A2A 基础框架
- 定义 proto 文件（a2a.proto, agent_service.proto, federation_service.proto, types.proto）
- protoc 代码生成
- pkg/a2a/convert.go 映射层
- pkg/a2a/bridge.go 骨架
- 单元测试

### Phase B：AgentService gRPC 实现
- pkg/a2a/server.go — AgentServiceServer
- pkg/a2a/client.go — A2AClient
- Controller 集成 gRPC 调用
- gRPC Server 启动（:9528）
- 集成测试

### Phase C：FederationService gRPC 实现
- pkg/federation/grpc_server.go
- pkg/federation/grpc_client.go
- HMAC gRPC interceptor
- HeartbeatStream 双向流
- 双协议过渡期支持
- 集成测试

### Phase D：Dashboard 前端 — 新页面
- Goals 树形可视化
- RFC 审批页面
- 系统指标页面
- Workflow 执行详情页
- 对应后端 API 补全

### Phase E：无代码 Agent 向导 + 模型注册
- GET /api/models + POST /api/models
- POST /api/agents/wizard
- AgentWizard 前端组件（4 步向导）
- 模型选择 UI

### Phase F：Dashboard 整体打磨
- Federation 增强、Agent 详情、成本报表增强
- 响应式 + 暗色模式 + 动效 + 骨架屏 + 空状态
- framer-motion 集成

### Phase G：测试覆盖率 + 功能补全
- controller/server/claudecode/sqlite 覆盖率 → 80%
- OpenClaw 密钥持久化
- Goal 分解持久化
- Workflow 执行详情 API
- pkg/evolve 骨架

### Phase H：集成验证 + 清理
- 全链路集成测试（本地 A2A + Federation A2A）
- CI 更新
- HTTP Federation 代码清理（过渡期结束）
- 文档更新

---

*设计文档版本*: v1.0
*审批状态*: 用户已确认
*下一步*: 调用 writing-plans skill 生成详细实施计划
