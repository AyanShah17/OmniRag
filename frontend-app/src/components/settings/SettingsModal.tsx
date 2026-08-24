import React, { useState, useEffect } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Sliders, Key, Cpu, Database, CheckCircle2, AlertCircle } from "lucide-react"

interface SettingsModalProps {
  open: boolean
  onClose: () => void
  workspaceId: string
  onSettingsSaved?: () => void
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  open,
  onClose,
  workspaceId,
  onSettingsSaved,
}) => {
  const [embeddingProvider, setEmbeddingProvider] = useState("fastembed")
  const [llmProvider, setLlmProvider] = useState("groq")
  const [vectorStore, setVectorStore] = useState("mock")
  const [pineconeKey, setPineconeKey] = useState("")
  const [pineconeIndex, setPineconeIndex] = useState("omnirag-index")
  const [groqKey, setGroqKey] = useState("")
  const [openrouterKey, setOpenrouterKey] = useState("")
  const [openaiKey, setOpenaiKey] = useState("")
  const [configPath, setConfigPath] = useState("")
  const [isSaving, setIsSaving] = useState(false)
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: "success" | "error" } | null>(null)

  useEffect(() => {
    if (open) {
      loadSettings()
    }
  }, [open])

  const loadSettings = async () => {
    try {
      const res = await fetch("/api/v1/settings", {
        headers: { "X-Workspace-ID": workspaceId },
      })
      if (res.ok) {
        const data = await res.json()
        setEmbeddingProvider(data.embedding_provider || "fastembed")
        setLlmProvider(data.llm_provider || "groq")
        setVectorStore(data.vector_store_provider || "mock")
        setPineconeIndex(data.pinecone_index_name || "omnirag-index")
        setPineconeKey(data.pinecone_api_key_masked || "")
        setGroqKey(data.groq_api_key_masked || "")
        setOpenrouterKey(data.openrouter_api_key_masked || "")
        setOpenaiKey(data.openai_api_key_masked || "")
        setConfigPath(data.config_file_path || "")
      }
    } catch (e) {
      console.error("Failed to load settings:", e)
    }
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSaving(true)
    setStatusMsg(null)

    try {
      const res = await fetch("/api/v1/settings", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Workspace-ID": workspaceId,
        },
        body: JSON.stringify({
          embedding_provider: embeddingProvider,
          llm_provider: llmProvider,
          vector_store_provider: vectorStore,
          pinecone_index_name: pineconeIndex,
          pinecone_api_key: pineconeKey.startsWith("••") ? undefined : pineconeKey,
          groq_api_key: groqKey.startsWith("••") ? undefined : groqKey,
          openrouter_api_key: openrouterKey.startsWith("••") ? undefined : openrouterKey,
          openai_api_key: openaiKey.startsWith("••") ? undefined : openaiKey,
        }),
      })

      if (!res.ok) {
        throw new Error(`Failed to save: ${res.status}`)
      }

      setStatusMsg({ text: "Settings and credentials updated successfully!", type: "success" })
      if (onSettingsSaved) onSettingsSaved()
      setTimeout(() => {
        onClose()
      }, 1200)
    } catch (err: any) {
      setStatusMsg({ text: err.message || "Failed to update settings", type: "error" })
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-lg bg-card/95 border-border/80 backdrop-blur-2xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <Sliders className="w-5 h-5 text-primary" />
            <DialogTitle className="text-base font-semibold">Backend AI & Credentials Control</DialogTitle>
          </div>
          <DialogDescription className="text-xs text-muted-foreground">
            Configure dynamic models, embedding engines, and API keys. Changes take effect immediately.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSave} className="space-y-4 my-1">
          {/* Provider Selectors */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-foreground flex items-center gap-1.5">
                <Cpu className="w-3.5 h-3.5 text-cyan-400" />
                Embedding Provider
              </label>
              <select
                value={embeddingProvider}
                onChange={(e) => setEmbeddingProvider(e.target.value)}
                className="w-full h-8 px-2 rounded-md bg-secondary/70 border border-border text-xs text-foreground outline-none"
              >
                <option value="fastembed">FastEmbed ONNX ($0 Local)</option>
                <option value="openrouter">OpenRouter API</option>
                <option value="openai">OpenAI Embedding</option>
                <option value="mock">Mock Embeddings</option>
              </select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-foreground flex items-center gap-1.5">
                <Cpu className="w-3.5 h-3.5 text-primary" />
                LLM Inference
              </label>
              <select
                value={llmProvider}
                onChange={(e) => setLlmProvider(e.target.value)}
                className="w-full h-8 px-2 rounded-md bg-secondary/70 border border-border text-xs text-foreground outline-none"
              >
                <option value="groq">Groq (Ultra-fast)</option>
                <option value="openrouter">OpenRouter (Multi-Model)</option>
                <option value="openai">OpenAI (GPT-4o)</option>
                <option value="mock">Mock Offline Mode</option>
              </select>
            </div>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-foreground flex items-center gap-1.5">
              <Database className="w-3.5 h-3.5 text-emerald-400" />
              Vector Store Engine
            </label>
            <select
              value={vectorStore}
              onChange={(e) => setVectorStore(e.target.value)}
              className="w-full h-8 px-2 rounded-md bg-secondary/70 border border-border text-xs text-foreground outline-none"
            >
              <option value="pinecone">Pinecone Serverless</option>
              <option value="mock">In-Memory Local Vector Store</option>
            </select>
          </div>

          {/* API Keys */}
          <div className="space-y-2 pt-2 border-t border-border/40">
            <div className="flex items-center gap-1 text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">
              <Key className="w-3.5 h-3.5 text-amber-400" />
              API Keys & Credentials
            </div>

            {vectorStore === "pinecone" && (
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-1">
                  <label className="text-[11px] text-muted-foreground">Pinecone API Key</label>
                  <Input
                    type="password"
                    value={pineconeKey}
                    onChange={(e) => setPineconeKey(e.target.value)}
                    placeholder="pcsk_..."
                    className="h-8 text-xs font-mono"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-[11px] text-muted-foreground">Pinecone Index</label>
                  <Input
                    value={pineconeIndex}
                    onChange={(e) => setPineconeIndex(e.target.value)}
                    className="h-8 text-xs font-mono"
                  />
                </div>
              </div>
            )}

            {llmProvider === "groq" && (
              <div className="space-y-1">
                <label className="text-[11px] text-muted-foreground">Groq API Key</label>
                <Input
                  type="password"
                  value={groqKey}
                  onChange={(e) => setGroqKey(e.target.value)}
                  placeholder="gsk_..."
                  className="h-8 text-xs font-mono"
                />
              </div>
            )}

            {(llmProvider === "openrouter" || embeddingProvider === "openrouter") && (
              <div className="space-y-1">
                <label className="text-[11px] text-muted-foreground">OpenRouter API Key</label>
                <Input
                  type="password"
                  value={openrouterKey}
                  onChange={(e) => setOpenrouterKey(e.target.value)}
                  placeholder="sk-or-v1-..."
                  className="h-8 text-xs font-mono"
                />
              </div>
            )}

            {(llmProvider === "openai" || embeddingProvider === "openai") && (
              <div className="space-y-1">
                <label className="text-[11px] text-muted-foreground">OpenAI API Key</label>
                <Input
                  type="password"
                  value={openaiKey}
                  onChange={(e) => setOpenaiKey(e.target.value)}
                  placeholder="sk-..."
                  className="h-8 text-xs font-mono"
                />
              </div>
            )}
          </div>

          {configPath && (
            <div className="text-[10px] text-muted-foreground font-mono truncate">
              Config File: {configPath}
            </div>
          )}

          {statusMsg && (
            <div
              className={`p-2.5 rounded-md text-xs flex items-center gap-2 ${
                statusMsg.type === "success"
                  ? "bg-emerald-500/10 border border-emerald-500/20 text-emerald-400"
                  : "bg-destructive/10 border border-destructive/20 text-destructive"
              }`}
            >
              {statusMsg.type === "success" ? (
                <CheckCircle2 className="w-4 h-4 shrink-0" />
              ) : (
                <AlertCircle className="w-4 h-4 shrink-0" />
              )}
              <span>{statusMsg.text}</span>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2 border-t border-border/60">
            <Button type="button" variant="ghost" size="sm" onClick={onClose} className="text-xs h-8">
              Cancel
            </Button>
            <Button type="submit" size="sm" disabled={isSaving} className="text-xs h-8 bg-primary">
              {isSaving ? "Saving..." : "Save Changes"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
