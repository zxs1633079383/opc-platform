'use client'

import { useState } from 'react'
import { ChevronRight, ChevronDown } from 'lucide-react'

interface TreeNode {
  name: string
  status: string
  cost?: number
  agent?: string
  children?: TreeNode[]
}

function statusColor(status: string): string {
  const s = status.toLowerCase()
  if (s === 'completed' || s === 'done') return 'bg-green-500'
  if (s === 'running' || s === 'in_progress') return 'bg-yellow-500'
  if (s === 'failed') return 'bg-red-500'
  return 'bg-gray-400'
}

function TreeNodeItem({ node, depth = 0 }: { node: TreeNode; depth?: number }) {
  const [expanded, setExpanded] = useState(depth < 1)
  const hasChildren = node.children && node.children.length > 0

  return (
    <div>
      <button
        onClick={() => hasChildren && setExpanded(!expanded)}
        className="flex items-center gap-2 py-1.5 w-full text-left hover:bg-gray-50 dark:hover:bg-gray-800/50 rounded px-2 text-sm"
        style={{ marginLeft: `${depth * 24}px` }}
      >
        {hasChildren ? (
          expanded ? <ChevronDown className="w-4 h-4 text-gray-400 shrink-0" /> : <ChevronRight className="w-4 h-4 text-gray-400 shrink-0" />
        ) : (
          <span className="w-4" />
        )}
        <span className={`w-2 h-2 rounded-full shrink-0 ${statusColor(node.status)}`} />
        <span className="text-gray-900 dark:text-white truncate">{node.name}</span>
        {node.agent && (
          <span className="text-xs text-gray-400 ml-auto shrink-0">{node.agent}</span>
        )}
        {node.cost !== undefined && node.cost > 0 && (
          <span className="text-xs text-gray-400 shrink-0 ml-2">${node.cost.toFixed(4)}</span>
        )}
      </button>
      {expanded && hasChildren && (
        <div>
          {node.children!.map((child, i) => (
            <TreeNodeItem key={`${child.name}-${i}`} node={child} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  )
}

export function GoalTree({ nodes }: { nodes: TreeNode[] }) {
  if (nodes.length === 0) {
    return <p className="text-sm text-gray-400 italic py-4">No hierarchy data available</p>
  }
  return (
    <div className="space-y-0.5">
      {nodes.map((node, i) => (
        <TreeNodeItem key={`${node.name}-${i}`} node={node} />
      ))}
    </div>
  )
}
