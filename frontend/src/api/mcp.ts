import { api } from './client'

// AI connections (spec 016). The URL of a connection comes back only when
// it is created (RF-11, RF-12).

export interface Via {
  connection_id: string
  name: string
  client?: string
}

export interface McpConnection {
  id: string
  name: string
  created_at: string
  expires_at: string
  last_used_at: string | null
  last_client: string | null
  user?: { id: string; name: string; email: string }
}

export interface McpCreated extends McpConnection {
  url: string
}

export interface MyMcpConnections {
  enabled: boolean
  max: number
  items: McpConnection[]
}

export const MCP_DAYS = [30, 90, 365] as const

export const getMyMcpConnections = () => api<MyMcpConnections>('/api/me/mcp-connections')

export const createMcpConnection = (body: { name: string; days: (typeof MCP_DAYS)[number] }) =>
  api<McpCreated>('/api/me/mcp-connections', { method: 'POST', body })

export const revokeMyMcpConnection = (id: string) => api<void>(`/api/me/mcp-connections/${id}`, { method: 'DELETE' })

export const getAllMcpConnections = () => api<{ items: McpConnection[] }>('/api/system/mcp-connections')

export const revokeMcpConnection = (id: string) => api<void>(`/api/system/mcp-connections/${id}`, { method: 'DELETE' })
