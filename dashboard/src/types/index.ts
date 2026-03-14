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
