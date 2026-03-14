import type { Agent, Task, Metrics, CostDataPoint, Workflow, CostEvent, LogEntry } from '@/types'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:9527/api'

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`)
  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`)
  }
  return response.json()
}

export async function fetchAgents(): Promise<Agent[]> {
  return fetchJson<Agent[]>('/agents')
}

export async function fetchTasks(): Promise<Task[]> {
  return fetchJson<Task[]>('/tasks')
}

export async function fetchMetrics(): Promise<Metrics> {
  return fetchJson<Metrics>('/metrics')
}

export async function fetchCostData(): Promise<CostDataPoint[]> {
  return fetchJson<CostDataPoint[]>('/costs/daily')
}

export async function fetchWorkflows(): Promise<Workflow[]> {
  return fetchJson<Workflow[]>('/workflows')
}

export async function fetchCostEvents(): Promise<CostEvent[]> {
  return fetchJson<CostEvent[]>('/costs/events')
}

export async function fetchLogs(limit = 100): Promise<LogEntry[]> {
  return fetchJson<LogEntry[]>(`/logs?limit=${limit}`)
}

export async function startAgent(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/agents/${name}/start`, { method: 'POST' })
  if (!response.ok) {
    throw new Error(`Failed to start agent: ${response.statusText}`)
  }
}

export async function stopAgent(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/agents/${name}/stop`, { method: 'POST' })
  if (!response.ok) {
    throw new Error(`Failed to stop agent: ${response.statusText}`)
  }
}

export async function restartAgent(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/agents/${name}/restart`, { method: 'POST' })
  if (!response.ok) {
    throw new Error(`Failed to restart agent: ${response.statusText}`)
  }
}

export async function deleteAgent(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/agents/${name}`, { method: 'DELETE' })
  if (!response.ok) {
    throw new Error(`Failed to delete agent: ${response.statusText}`)
  }
}

export async function runTask(agent: string, message: string): Promise<{ taskId: string }> {
  const response = await fetch(`${API_BASE}/run`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ agent, message }),
  })
  if (!response.ok) {
    throw new Error(`Failed to run task: ${response.statusText}`)
  }
  return response.json()
}

export async function runWorkflow(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/workflows/${name}/run`, { method: 'POST' })
  if (!response.ok) {
    throw new Error(`Failed to run workflow: ${response.statusText}`)
  }
}

export async function deleteWorkflow(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/workflows/${name}`, { method: 'DELETE' })
  if (!response.ok) {
    throw new Error(`Failed to delete workflow: ${response.statusText}`)
  }
}
