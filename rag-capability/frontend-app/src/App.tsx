import React, { useState, useEffect } from "react"
import { Sidebar } from "@/components/layout/Sidebar"
import { Header } from "@/components/layout/Header"
import { ChatInterface } from "@/components/chat/ChatInterface"
import { DiffModal } from "@/components/documents/DiffModal"
import { ConnectorModal } from "@/components/connectors/ConnectorModal"
import { DocumentExplorerModal } from "@/components/documents/DocumentExplorerModal"
import { SettingsModal } from "@/components/settings/SettingsModal"
import { MessageItem } from "@/components/chat/MessageBubble"
import {
  pyApi,
  goApi,
  DiffResult,
  ConnectorItem,
  ConversationItem,
  Citation,
} from "@/lib/api"

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

function updateLastAssistant(
  messages: MessageItem[],
  update: Partial<MessageItem>
): MessageItem[] {
  const next = [...messages]
  const last = next[next.length - 1]
  if (last?.role === "assistant") {
    next[next.length - 1] = { ...last, ...update }
  }
  return next
}

export function App() {
  const [workspaceId] = useState("ws_default")
  const [conversations, setConversations] = useState<ConversationItem[]>([])
  const [currentConvId, setCurrentConvId] = useState<string | null>(null)
  const [messages, setMessages] = useState<MessageItem[]>([])
  const [connectors, setConnectors] = useState<ConnectorItem[]>([])
  const [pyHealth, setPyHealth] = useState("checking")
  const [goHealth, setGoHealth] = useState("checking")
  const [isStreaming, setIsStreaming] = useState(false)
  const [isUploading, setIsUploading] = useState(false)
  const [isSidebarOpen, setIsSidebarOpen] = useState(false)

  // Modals
  const [diffResult, setDiffResult] = useState<DiffResult | null>(null)
  const [isDiffModalOpen, setIsDiffModalOpen] = useState(false)
  const [isConnectorModalOpen, setIsConnectorModalOpen] = useState(false)
  const [isDocExplorerModalOpen, setIsDocExplorerModalOpen] = useState(false)
  const [isSettingsModalOpen, setIsSettingsModalOpen] = useState(false)

  // Initial Load & Health Polling
  useEffect(() => {
    checkHealth()
    loadConnectors()
    loadConversations()

    const interval = setInterval(checkHealth, 15000)
    return () => clearInterval(interval)
  }, [workspaceId])

  const checkHealth = async () => {
    const py = await pyApi.getHealth()
    const go = await goApi.getHealth()
    setPyHealth(py.status || "offline")
    setGoHealth(go.status || "offline")
  }

  const loadConnectors = async () => {
    const list = await goApi.listConnectors(workspaceId)
    // Fallback default connectors for demo presentation if none registered yet
    if (list.length === 0) {
      setConnectors([
        { workspace_id: workspaceId, type: "s3", name: "AWS S3 Knowledge Base", config: {} },
        { workspace_id: workspaceId, type: "azure", name: "Azure Blob Container", config: {} },
        { workspace_id: workspaceId, type: "confluence", name: "Confluence Engineering Wiki", config: {} },
      ])
    } else {
      setConnectors(list)
    }
  }

  const loadConversations = async () => {
    const list = await pyApi.listConversations(workspaceId)
    setConversations(list)
  }

  const handleNewChat = () => {
    setCurrentConvId(null)
    setMessages([])
    setIsSidebarOpen(false)
  }

  const handleSelectConversation = async (id: string) => {
    setCurrentConvId(id)
    setIsSidebarOpen(false)
    try {
      const stored = await pyApi.getConversationMessages(id, workspaceId)
      setMessages(stored.map((message) => ({
        role: message.role,
        content: message.content,
        citations: message.citations,
      })))
    } catch {
      setMessages([])
    }
  }

  const handleSendMessage = async (text: string) => {
    if (isStreaming) return

    let conversationId = currentConvId
    if (conversationId === null) {
      const title = text.trim().slice(0, 60) || "New Chat"
      const conversation = await pyApi.createConversation(title, workspaceId)
      conversationId = conversation.id
      setCurrentConvId(conversationId)
      await loadConversations()
    }

    const userMsg: MessageItem = { role: "user", content: text }
    const updatedMessages = [...messages, userMsg]
    const assistantMsg: MessageItem = {
      role: "assistant",
      content: "",
      citations: [],
      isStreaming: true,
    }
    setMessages([...updatedMessages, assistantMsg])
    setIsStreaming(true)

    let accumulatedContent = ""
    let citationsCollected: Citation[] = []

    const historyForApi = updatedMessages.map((m) => ({
      role: m.role,
      content: m.content,
    }))

    pyApi.streamChat(
      historyForApi,
      workspaceId,
      conversationId,
      (token) => {
        accumulatedContent += token
        setMessages((prev) => updateLastAssistant(prev, {
          content: accumulatedContent,
          isStreaming: true,
        }))
      },
      (citations) => {
        citationsCollected = citations
        setMessages((prev) => updateLastAssistant(prev, { citations }))
      },
      () => {
        setIsStreaming(false)
        setMessages((prev) => updateLastAssistant(prev, {
          isStreaming: false,
          citations: citationsCollected,
        }))
      },
      (err) => {
        setIsStreaming(false)
        setMessages((prev) => updateLastAssistant(prev, {
          isStreaming: false,
          content: `Error: ${err.message || "Failed to generate response."}`,
        }))
      }
    )
  }

  const handleFileUpload = async (file: File) => {
    setIsUploading(true)
    try {
      const result = await pyApi.uploadDocument(file, workspaceId)
      setDiffResult(result)
      setIsDiffModalOpen(true)
    } catch (err: unknown) {
      alert(`Upload failed: ${errorMessage(err, "Unable to upload document.")}`)
    } finally {
      setIsUploading(false)
    }
  }

  return (
    <div className="flex h-screen w-screen bg-background text-foreground overflow-hidden font-sans">
      {/* Left Sidebar */}
      <Sidebar
        conversations={conversations}
        currentConvId={currentConvId}
        onSelectConversation={handleSelectConversation}
        onNewChat={handleNewChat}
        connectors={connectors}
        onOpenConnectors={() => setIsConnectorModalOpen(true)}
        onOpenDocuments={() => setIsDocExplorerModalOpen(true)}
        onOpenSettings={() => setIsSettingsModalOpen(true)}
        workspaceId={workspaceId}
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
      />

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col h-full overflow-hidden bg-background">
        <Header
          pyHealth={pyHealth}
          goHealth={goHealth}
          onFileUpload={handleFileUpload}
          isUploading={isUploading}
          onToggleSidebar={() => setIsSidebarOpen((open) => !open)}
        />

        <ChatInterface
          messages={messages}
          onSendMessage={handleSendMessage}
          isStreaming={isStreaming}
          onAttachClick={() => {
            const input = document.createElement("input")
            input.type = "file"
            input.accept = ".pdf,.docx,.doc,.md,.txt,.html,.csv"
            input.onchange = (event) => {
              const file = (event.target as HTMLInputElement).files?.[0]
              if (file) handleFileUpload(file)
            }
            input.click()
          }}
        />
      </div>

      {/* Modals */}
      <DiffModal
        open={isDiffModalOpen}
        onClose={() => setIsDiffModalOpen(false)}
        diffResult={diffResult}
      />

      <ConnectorModal
        open={isConnectorModalOpen}
        onClose={() => setIsConnectorModalOpen(false)}
        onConnectorSaved={loadConnectors}
        workspaceId={workspaceId}
      />

      <DocumentExplorerModal
        open={isDocExplorerModalOpen}
        onClose={() => setIsDocExplorerModalOpen(false)}
        workspaceId={workspaceId}
      />

      <SettingsModal
        open={isSettingsModalOpen}
        onClose={() => setIsSettingsModalOpen(false)}
        workspaceId={workspaceId}
        onSettingsSaved={checkHealth}
      />
    </div>
  )
}
