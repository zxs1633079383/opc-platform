export interface Agent {
  name: string
  type: string
  phase: string
  replicas?: number
  restarts?: number
  message?: string
  createdAt: string
  updatedAt: string
  metrics?: AgentMetrics
}

export interface AgentMetrics {
  tasksCompleted: number
  tasksFailed: number
  tasksRunning: number
  totalTokensIn: number
  totalTokensOut: number
  totalCost: number
  uptimeSeconds: number
}

export interface Task {
  id: string
  agentName: string
  message: string
  status: 'Pending' | 'Running' | 'Completed' | 'Failed' | 'Cancelled'
  result?: string
  error?: string
  tokensIn?: number
  tokensOut?: number
  cost?: number
  issueId?: string
  projectId?: string
  goalId?: string
  createdAt: string
  updatedAt: string
  startedAt?: string
  endedAt?: string
}

export interface Workflow {
  name: string
  schedule?: string
  enabled: boolean
  steps: WorkflowStep[]
  createdAt: string
  updatedAt: string
}

export interface WorkflowStep {
  name: string
  agent: string
  dependsOn?: string[]
  status?: string
}

export interface WorkflowRun {
  id: string
  workflowName: string
  status: 'Pending' | 'Running' | 'Completed' | 'Failed'
  steps: WorkflowStepResult[]
  stepsJson?: string
  startedAt: string
  endedAt?: string
}

export interface WorkflowStepResult {
  name: string
  status: string
  result?: string
  error?: string
}

export interface CostDataPoint {
  date: string
  cost: number
}

export interface CostEvent {
  id: string
  agentName: string
  taskId: string
  goalRef?: string
  projectRef?: string
  tokensIn: number
  tokensOut: number
  cost: number
  model: string
  createdAt: string
}

export interface Metrics {
  totalAgents: number
  runningAgents: number
  totalTasks: number
  runningTasks: number
  completedTasks: number
  failedTasks: number
  todayCost: number
  monthCost: number
  dailyBudget: number
  monthlyBudget: number
}

export interface LogEntry {
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  agent?: string
  taskId?: string
  fields?: Record<string, unknown>
}

export interface Company {
  id: string
  name: string
  endpoint: string
  dashboardUrl?: string
  type: 'software' | 'operations' | 'sales' | 'custom'
  status: 'Online' | 'Offline' | 'Busy'
  agents?: string[]
  joinedAt: string
}

export interface Goal {
  id: string
  name: string
  description: string
  owner?: string
  deadline?: string
  status: string
  phase?: string
  decompositionPlan?: string
  decomposeCost?: number
  tokensIn: number
  tokensOut: number
  cost: number
  createdAt: string
  updatedAt: string
}

export interface Project {
  id: string
  name: string
  goalId: string
  description: string
  status: string
  tokensIn: number
  tokensOut: number
  cost: number
  createdAt: string
  updatedAt: string
}

export interface HierarchyStats {
  totalTokensIn: number
  totalTokensOut: number
  totalCost: number
  taskCount: number
  completedTasks: number
  failedTasks: number
}

export interface Issue {
  id: string
  name: string
  projectId: string
  description: string
  agentRef?: string
  status: string
  tokensIn: number
  tokensOut: number
  cost: number
  createdAt: string
  updatedAt: string
}

export interface GoalTask {
  name: string
  status: string
  agent?: string
  children?: GoalTask[]
}

export interface Settings {
  anthropicKey?: string
  openaiKey?: string
  telegramToken?: string
  discordToken?: string
  dailyBudget?: number
  monthlyBudget?: number
  notificationsEnabled?: boolean
  [key: string]: unknown
}

export interface ModelInfo {
  id: string
  provider: string
  displayName: string
  tier: 'economy' | 'standard' | 'premium'
  costPer1k: number
  capability: 'fast' | 'balanced' | 'reasoning'
  default?: boolean
}

export interface RFC {
  id: string
  title: string
  problem: string
  solution: string
  expectedBenefit: string
  risk: string
  status: 'pending' | 'approved' | 'rejected'
  createdAt: string
}

export interface SystemMetrics {
  successRate: number
  avgLatency: number
  costPerGoal: number
  retryRate: number
  coverageGap: number
  errorPatterns: string[]
  timestamp: string
}

export interface WizardRequest {
  type: string
  description: string
  model: string
  fallbackModel?: string
  preset: 'light' | 'standard' | 'power' | 'custom'
  replicas: number
  onExceed: string
}
