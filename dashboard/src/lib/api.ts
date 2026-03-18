import type { Agent, Task, Metrics, CostDataPoint, Workflow, WorkflowRun, CostEvent, LogEntry, Company, Goal, Project, HierarchyStats, Issue } from '@/types'

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

// ---- federation ----

export async function fetchCompanies(): Promise<Company[]> {
  return fetchJson<Company[]>('/federation/companies')
}

export async function registerCompany(data: {
  name: string
  endpoint: string
  type: string
  agents?: string[]
}): Promise<Company> {
  const response = await fetch(`${API_BASE}/federation/companies`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || `Failed to register company: ${response.statusText}`)
  }
  return response.json()
}

export async function unregisterCompany(id: string): Promise<void> {
  const response = await fetch(`${API_BASE}/federation/companies/${id}`, { method: 'DELETE' })
  if (!response.ok) {
    throw new Error(`Failed to unregister company: ${response.statusText}`)
  }
}

export async function updateCompanyStatus(id: string, status: string): Promise<void> {
  const response = await fetch(`${API_BASE}/federation/companies/${id}/status`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  })
  if (!response.ok) {
    throw new Error(`Failed to update company status: ${response.statusText}`)
  }
}

export async function fetchCompanyAgents(companyId: string): Promise<Agent[]> {
  return fetchJson<Agent[]>(`/federation/companies/${companyId}/agents`)
}

export async function fetchCompanyTasks(companyId: string): Promise<Task[]> {
  return fetchJson<Task[]>(`/federation/companies/${companyId}/tasks`)
}

export async function fetchCompanyMetrics(companyId: string): Promise<Metrics> {
  return fetchJson<Metrics>(`/federation/companies/${companyId}/metrics`)
}

export async function pingCompany(companyId: string): Promise<{ healthy: boolean }> {
  return fetchJson<{ healthy: boolean }>(`/federation/companies/${companyId}/health`)
}

export interface FederatedAgent extends Agent {
  company: string
  companyId: string
}

export async function fetchFederatedAgents(): Promise<FederatedAgent[]> {
  return fetchJson<FederatedAgent[]>('/federation/aggregate/agents')
}

export interface AggregatedMetrics extends Metrics {
  companyCount: number
  onlineCount: number
}

export async function fetchFederatedMetrics(): Promise<AggregatedMetrics> {
  return fetchJson<AggregatedMetrics>('/federation/aggregate/metrics')
}

export interface FederatedGoalRequest {
  name: string
  description: string
  companies: string[]
}

export async function createFederatedGoal(data: FederatedGoalRequest): Promise<{
  goalId: string
  name: string
  description: string
  dispatched: Array<{
    companyId: string
    companyName: string
    status: string
    error?: string
  }>
}> {
  const response = await fetch(`${API_BASE}/goals/federated`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || `Failed to create federated goal: ${response.statusText}`)
  }
  return response.json()
}

export async function toggleWorkflow(name: string, enabled: boolean): Promise<void> {
  const response = await fetch(`${API_BASE}/workflows/${name}/toggle`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
  if (!response.ok) {
    throw new Error(`Failed to toggle workflow: ${response.statusText}`)
  }
}

// ---- Goals ----

export async function fetchGoals(): Promise<Goal[]> {
  return fetchJson<Goal[]>('/goals')
}

export async function fetchGoalProjects(goalId: string): Promise<Project[]> {
  return fetchJson<Project[]>(`/goals/${goalId}/projects`)
}

export async function fetchGoalStats(goalId: string): Promise<HierarchyStats> {
  return fetchJson<HierarchyStats>(`/goals/${goalId}/stats`)
}

export async function createGoal(data: {
  name: string; description?: string; owner?: string; deadline?: string;
  autoDecompose?: boolean; approval?: string;
}): Promise<Goal> {
  const response = await fetch(`${API_BASE}/goals`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || `Failed to create goal: ${response.statusText}`)
  }
  return response.json()
}

export async function deleteGoal(id: string): Promise<void> {
  const response = await fetch(`${API_BASE}/goals/${id}`, { method: 'DELETE' })
  if (!response.ok) throw new Error(`Failed to delete goal: ${response.statusText}`)
}

// ---- Projects ----

export async function createProject(data: Partial<Project> & { name?: string; goalId?: string }): Promise<Project> {
  const response = await fetch(`${API_BASE}/projects`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!response.ok) throw new Error(`Failed to create project: ${response.statusText}`)
  return response.json()
}

export async function deleteProject(id: string): Promise<void> {
  const response = await fetch(`${API_BASE}/projects/${id}`, { method: 'DELETE' })
  if (!response.ok) throw new Error(`Failed to delete project: ${response.statusText}`)
}

// ---- Agent apply ----

export async function applyAgent(yamlContent: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/apply`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-yaml' },
    body: yamlContent,
  })
  if (!response.ok) throw new Error(`Failed to apply: ${response.statusText}`)
  return response.json()
}

// ---- Task logs ----

export async function fetchTaskLogs(taskId: string): Promise<string> {
  const response = await fetch(`${API_BASE}/tasks/${taskId}/logs`)
  if (!response.ok) return ''
  const data = await response.json()
  return data.logs || data.result || ''
}

export async function fetchProjects(): Promise<Project[]> {
  return fetchJson<Project[]>('/projects')
}

export async function fetchIssues(): Promise<Issue[]> {
  return fetchJson<Issue[]>('/issues')
}

export async function fetchWorkflowRuns(name: string): Promise<WorkflowRun[]> {
  return fetchJson<WorkflowRun[]>(`/workflows/${name}/runs`)
}

// ---- Settings ----

export async function fetchSettings(): Promise<Record<string, unknown>> {
  return fetchJson<Record<string, unknown>>('/settings')
}

export async function updateSettings(settings: Record<string, unknown>): Promise<void> {
  const response = await fetch(`${API_BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  if (!response.ok) throw new Error('Failed to save settings')
}

export async function createAgent(agent: { name: string; type: string; model?: string }): Promise<Agent> {
  const yaml = `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ${agent.name}
spec:
  type: ${agent.type}
  runtime:
    model:
      name: ${agent.model || 'claude-sonnet-4'}
  context:
    workdir: /tmp/opc`
  const response = await fetch(`${API_BASE}/apply`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-yaml' },
    body: yaml,
  })
  if (!response.ok) throw new Error('Failed to create agent')
  return response.json()
}
