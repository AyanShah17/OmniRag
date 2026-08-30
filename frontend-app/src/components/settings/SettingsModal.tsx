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
import { Sliders, Key, Cpu, Database, CheckCircle2, AlertCircle, ShieldCheck } from "lucide-react"
import { apiHeaders } from "@/lib/api"

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
  const [licenseKey, setLicenseKey] = useState(() => localStorage.getItem("omnirag_license_key") || "")
  const [licenseActive, setLicenseActive] = useState(false)
  const [licenseRequired, setLicenseRequired] = useState(false)
  const [isActivating, setIsActivating] = useState(false)

  useEffect(() => {
    if (open) {
      loadLicense()
      loadSettings()
    }
  }, [open])

  const pythonPath = window.location.port === "3000" ? "/api/py" : "/api/v1"

  const loadLicense = async () => {
    try {
      const res = await fetch(`${pythonPath}/auth/license`, { headers: apiHeaders(workspaceId) })
      if (res.ok) {
        const data = await res.json()
        setLicenseRequired(Boolean(data.license_required))
        setLicenseActive(Boolean(data.active))
      }
    } catch {
      setLicenseActive(false)
    }
  }

  const activateLicense = async () => {
    if (!licenseKey.trim()) return
    setIsActivating(true)
    try {
      const res = await fetch(`${pythonPath}/auth/license`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ license_key: licenseKey.trim() }),
      })
      const result = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(result.detail || "License activation failed")
      localStorage.setItem("omnirag_license_key", licenseKey.trim())
      setLicenseActive(true)
      setStatusMsg({ text: "License activated on this device.", type: "success" })
    } catch (err: any) {
      setLicenseActive(false)
      setStatusMsg({ text: err.message || "License activation failed", type: "error" })
    } finally {
      setIsActivating(false)
    }
  }

  const loadSettings = async () => {
    try {
      const res = await fetch(`${pythonPath}/settings`, {
        headers: apiHeaders(workspaceId),
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
      const res = await fetch(`${pythonPath}/settings`, {
        method: "POST",
        headers: apiHeaders(workspaceId, true),
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

      const result = await res.json()
      setStatusMsg({ text: result.message || "Settings saved. Restart the service to apply them.", type: "success" })
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
            Configure models, embedding engines, and API keys for the next service start.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSave} className="space-y-4 my-1">
          <section className="rounded-md border border-border bg-background p-3 space-y-2">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-foreground" />
                <div>
                  <p className="text-xs font-semibold text-foreground">Product license</p>
                  <p className="text-[11px] text-muted-foreground">Required for licensed deployments.</p>
                </div>
              </div>
              <Badge variant={licenseActive ? "success" : licenseRequired ? "warning" : "outline"} className="text-[9px]">
                {licenseActive ? "ACTIVE" : licenseRequired ? "REQUIRED" : "OPTIONAL"}
              </Badge>
            </div>
            <div className="flex gap-2">
              <Input type="password" value={licenseKey} onChange={(e) => setLicenseKey(e.target.value)} placeholder="Enter license key" className="h-8 text-xs font-mono" aria-label="OmniRAG license key" />
              <Button type="button" variant="outline" size="sm" onClick={activateLicense} disabled={isActivating || !licenseKey.trim()} className="h-8 shrink-0 text-xs">
                {isActivating ? "Checking" : "Activate"}
              </Button>
            </div>
          </section>
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
