// Unified Dual-Backend API Client for Go Engine and Python RAG Core

const isViteDev = typeof window !== "undefined" && window.location.port === "3000"
const PY_BASE = isViteDev ? "/api/py" : "/api/v1"
const GO_BASE = isViteDev ? "/api/go" : "http://localhost:8080/api/v1"

export interface Citation {
  index: number
  document_id: string
  version_id?: string
  file_name: string
  source_uri: string
  page_number?: number
  heading?: string
  snippet: string
  score: number
}

export interface DiffResult {
  status: string
  document_id: string
  version_id: string
  version_number: number
  file_name: string
  total_chunks: number
  reused_chunks_count: number
  new_chunks_embedded: number
  cost_savings_percent: string
}

export interface DocumentItem {
  id: string
  workspace_id: string
  connector_id?: string
  external_id: string
  file_name: string
  file_type: string
  file_size: number
  current_version_id?: string
  status: string
  created_at?: string
}

export interface DocumentVersionItem {
  id: string
  document_id: string
  version_number: number
  file_hash: string
  total_chunks: number
  created_at?: string
}

export interface ConnectorItem {
  id?: string
  workspace_id: string
  type: string
  name: string
  config: Record<string, any>
  is_active?: boolean
  sync_frequency?: string
  last_synced_at?: string
}

export interface ConversationItem {
  id: string
  workspace_id: string
  user_id?: string
  title: string
  created_at?: string
}

// --------------------------------------------------------------------------
// Python RAG Service API
// --------------------------------------------------------------------------
export const pyApi = {
  async getHealth() {
    try {
      const res = await fetch(`${PY_BASE}/healthz`)
      return await res.json()
    } catch {
      return { status: "offline", service: "python-rag" }
    }
  },

  async uploadDocument(file: File, workspaceId = "ws_demo_enterprise"): Promise<DiffResult> {
    const formData = new FormData()
    formData.append("file", file)

    const res = await fetch(`${PY_BASE}/documents/upload`, {
      method: "POST",
      headers: {
        "X-Workspace-ID": workspaceId,
      },
      body: formData,
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.detail || `Upload failed with status ${res.status}`)
    }
    return await res.json()
  },

  async listDocuments(workspaceId = "ws_demo_enterprise"): Promise<DocumentItem[]> {
    const res = await fetch(`${PY_BASE}/documents`, {
      headers: { "X-Workspace-ID": workspaceId },
    })
    if (!res.ok) return []
    return await res.json()
  },

  async getDocumentVersions(docId: string): Promise<DocumentVersionItem[]> {
    const res = await fetch(`${PY_BASE}/documents/${docId}/versions`)
    if (!res.ok) return []
    return await res.json()
  },

  async createConversation(title = "New Chat", workspaceId = "ws_demo_enterprise"): Promise<ConversationItem> {
    const res = await fetch(`${PY_BASE}/chat/conversations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Workspace-ID": workspaceId,
      },
      body: JSON.stringify({ title }),
    })
    return await res.json()
  },

  async listConversations(workspaceId = "ws_demo_enterprise"): Promise<ConversationItem[]> {
    const res = await fetch(`${PY_BASE}/chat/conversations`, {
      headers: { "X-Workspace-ID": workspaceId },
    })
    if (!res.ok) return []
    return await res.json()
  },

  streamChat(
    messages: { role: string; content: string }[],
    workspaceId = "ws_demo_enterprise",
    onToken: (token: string) => void,
    onCitations: (citations: Citation[]) => void,
    onDone: () => void,
    onError: (err: any) => void
  ) {
    const controller = new AbortController()

    fetch(`${PY_BASE}/chat/completions/stream`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Workspace-ID": workspaceId,
      },
      body: JSON.stringify({
        messages,
        top_k: 8,
        rerank_top_n: 4,
      }),
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`HTTP Error: ${response.status}`)
        }
        const reader = response.body?.getReader()
        if (!reader) throw new Error("Response body is not readable")

        const decoder = new TextDecoder("utf-8")

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          const chunk = decoder.decode(value, { stream: true })
          const lines = chunk.split("\n\n")

          for (const line of lines) {
            if (!line.startsWith("data: ")) continue
            const dataStr = line.replace("data: ", "").trim()
            if (!dataStr) continue

            try {
              const eventObj = JSON.parse(dataStr)
              if (eventObj.event === "citations") {
                onCitations(eventObj.data || [])
              } else if (eventObj.event === "token") {
                onToken(eventObj.data || "")
              } else if (eventObj.event === "done") {
                onDone()
                return
              }
            } catch (e) {
              console.error("Stream parse error:", e)
            }
          }
        }
        onDone()
      })
      .catch((err) => {
        if (err.name !== "AbortError") {
          onError(err)
        }
      })

    return () => controller.abort()
  },
}

// --------------------------------------------------------------------------
// Go Connector Engine API
// --------------------------------------------------------------------------
export const goApi = {
  async getHealth() {
    try {
      const res = await fetch(`${GO_BASE}/healthz`)
      return await res.json()
    } catch {
      return { status: "offline", service: "go-engine" }
    }
  },

  async listConnectors(workspaceId = "ws_demo_enterprise"): Promise<ConnectorItem[]> {
    try {
      const res = await fetch(`${GO_BASE}/connectors?workspace_id=${workspaceId}`)
      if (!res.ok) return []
      return await res.json()
    } catch {
      return []
    }
  },

  async createConnector(conn: ConnectorItem): Promise<ConnectorItem> {
    const res = await fetch(`${GO_BASE}/connectors`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(conn),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || `Failed to create connector: ${res.status}`)
    }
    return await res.json()
  },

  async testConnector(type: string, name: string, config: Record<string, any>) {
    const res = await fetch(`${GO_BASE}/connectors/test`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type, name, config }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || "Connection test failed")
    }
    return await res.json()
  },

  async triggerSync(connectorId: string) {
    const res = await fetch(`${GO_BASE}/connectors/${connectorId}/sync`, {
      method: "POST",
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || "Sync trigger failed")
    }
    return await res.json()
  },
}
