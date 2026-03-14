# Federation 多公司协同

## 概述

OPC Platform 支持一人管理多个 OPC 公司，通过 Federation Controller 实现跨公司协同。每个公司独立运行自己的 Agent 集群，Federation 层负责目标分解、任务下发和进度汇总。

## 架构

```
Federation Controller
├── Company: software    (软件开发公司)
│   ├── Agent: backend-dev
│   ├── Agent: frontend-dev
│   └── Agent: qa-engineer
├── Company: operations  (运营公司)
│   ├── Agent: content-writer
│   └── Agent: data-analyst
└── Company: sales       (销售公司)
    ├── Agent: sales-rep
    └── Agent: customer-support
```

## CLI 命令

### Federation 管理

```bash
# 初始化联邦
opctl federation init --name tech-group

# 添加公司
opctl federation add-company --name software --endpoint http://software.local:9527

# 查看所有公司
opctl federation companies

# 查看联邦状态
opctl federation status
```

### Goal 目标管理

```bash
# 创建目标
opctl goal create -f examples/federation/tech-group-goal.yaml

# 查看目标列表
opctl goal list

# 查看目标状态
opctl goal status develop-messaging-system

# 追踪目标执行
opctl goal trace develop-messaging-system

# 人工干预
opctl goal intervene develop-messaging-system --company software --action reassign
```

## 示例场景

### 科技集团开发消息系统

一个科技集团下辖三个公司，协同开发企业级消息系统：

- **软件公司** (software)：负责系统开发，包含后端、前端、测试 Agent
- **运营公司** (operations)：负责内容运营和数据分析
- **销售公司** (sales)：负责销售推广和客户支持

```bash
# 1. 初始化联邦
opctl federation init --name tech-group

# 2. 添加公司
opctl federation add-company --name software --endpoint http://software.local:9527
opctl federation add-company --name operations --endpoint http://operations.local:9527
opctl federation add-company --name sales --endpoint http://sales.local:9527

# 3. 创建目标
opctl goal create -f examples/federation/tech-group-goal.yaml

# 4. 追踪进度
opctl goal trace develop-messaging-system
```
