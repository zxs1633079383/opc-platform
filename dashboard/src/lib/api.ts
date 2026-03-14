import type { Agent, Task, Metrics, CostDataPoint, Workflow, CostEvent, LogEntry } from '@/types'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '/api'

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`)
  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`)
  }
  return response.json()
}

// Mock data for development
const mockAgents: Agent[] = [
  {
    name: 'coder-main',
    type: 'openclaw',
    phase: 'Running',
    replicas: 1,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    metrics: {
      tasksCompleted: 42,
      tasksFailed: 2,
      tasksRunning: 1,
      totalTokensIn: 150000,
      totalTokensOut: 75000,
      totalCost: 2.45,
      uptimeSeconds: 7200,
    },
  },
  {
    name: 'code-reviewer',
    type: 'claude-code',
    phase: 'Running',
    replicas: 1,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    metrics: {
      tasksCompleted: 28,
      tasksFailed: 0,
      tasksRunning: 0,
      totalTokensIn: 80000,
      totalTokensOut: 40000,
      totalCost: 1.23,
      uptimeSeconds: 3600,
    },
  },
  {
    name: 'task-runner',
    type: 'codex',
    phase: 'Stopped',
    replicas: 0,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    metrics: {
      tasksCompleted: 15,
      tasksFailed: 1,
      tasksRunning: 0,
      totalTokensIn: 30000,
      totalTokensOut: 15000,
      totalCost: 0.45,
      uptimeSeconds: 0,
    },
  },
]

const mockTasks: Task[] = [
  {
    id: 'task-001-abc123',
    agentName: 'coder-main',
    message: 'Implement user authentication with OAuth2',
    status: 'Running',
    cost: 0.12,
    createdAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'task-002-def456',
    agentName: 'code-reviewer',
    message: 'Review PR #42: Add payment integration',
    status: 'Completed',
    result: 'LGTM with minor suggestions',
    cost: 0.08,
    createdAt: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    updatedAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
  },
  {
    id: 'task-003-ghi789',
    agentName: 'coder-main',
    message: 'Fix bug in shopping cart calculation',
    status: 'Completed',
    cost: 0.05,
    createdAt: new Date(Date.now() - 1000 * 60 * 60).toISOString(),
    updatedAt: new Date(Date.now() - 1000 * 60 * 45).toISOString(),
  },
  {
    id: 'task-004-jkl012',
    agentName: 'task-runner',
    message: 'Generate API documentation',
    status: 'Failed',
    error: 'Timeout after 5 minutes',
    cost: 0.02,
    createdAt: new Date(Date.now() - 1000 * 60 * 120).toISOString(),
    updatedAt: new Date(Date.now() - 1000 * 60 * 115).toISOString(),
  },
]

const mockCostData: CostDataPoint[] = [
  { date: '2026-03-08', cost: 3.21 },
  { date: '2026-03-09', cost: 4.56 },
  { date: '2026-03-10', cost: 2.89 },
  { date: '2026-03-11', cost: 5.12 },
  { date: '2026-03-12', cost: 3.78 },
  { date: '2026-03-13', cost: 4.23 },
  { date: '2026-03-14', cost: 2.45 },
]

const mockMetrics: Metrics = {
  totalAgents: 3,
  runningAgents: 2,
  totalTasks: 85,
  runningTasks: 1,
  completedTasks: 80,
  failedTasks: 4,
  todayCost: 2.45,
  monthCost: 45.67,
  dailyBudget: 10,
  monthlyBudget: 200,
}

// Use mock data in development, real API in production
const isDev = process.env.NODE_ENV === 'development'

export async function fetchAgents(): Promise<Agent[]> {
  if (isDev) return mockAgents
  return fetchJson<Agent[]>('/agents')
}

export async function fetchTasks(): Promise<Task[]> {
  if (isDev) return mockTasks
  return fetchJson<Task[]>('/tasks')
}

export async function fetchMetrics(): Promise<Metrics> {
  if (isDev) return mockMetrics
  return fetchJson<Metrics>('/metrics')
}

export async function fetchCostData(): Promise<CostDataPoint[]> {
  if (isDev) return mockCostData
  return fetchJson<CostDataPoint[]>('/costs/daily')
}

export async function fetchWorkflows(): Promise<Workflow[]> {
  if (isDev) return []
  return fetchJson<Workflow[]>('/workflows')
}

export async function fetchCostEvents(): Promise<CostEvent[]> {
  if (isDev) return []
  return fetchJson<CostEvent[]>('/costs/events')
}

export async function fetchLogs(limit = 100): Promise<LogEntry[]> {
  if (isDev) return []
  return fetchJson<LogEntry[]>(`/logs?limit=${limit}`)
}
