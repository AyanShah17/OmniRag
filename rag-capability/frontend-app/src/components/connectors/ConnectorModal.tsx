import React, { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { goApi, ConnectorItem } from "@/lib/api"
import { Cloud, CheckCircle, AlertCircle, RefreshCw } from "lucide-react"

const CONNECTOR_DEFAULTS: Record<string, { name: string; bucket: string }> = {
  s3: { name: "Corporate S3 Bucket", bucket: "my-company-knowledge-base" },
  azure: { name: "Azure Blob Storage", bucket: "rag-documents-container" },
  supabase: { name: "Supabase Storage", bucket: "knowledge-vault" },
  confluence: { name: "Engineering Confluence Wiki", bucket: "ENG-SPACE" },
}

interface ConnectorModalProps {
  open: boolean
  onClose: () => void
  onConnectorSaved: () => void
  workspaceId: string
}

export const ConnectorModal: React.FC<ConnectorModalProps> = ({
  open,
  onClose,
  onConnectorSaved,
  workspaceId,
}) => {
  const [activeTab, setActiveTab] = useState("s3")
  const [name, setName] = useState("Corporate S3 Bucket")
  const [bucket, setBucket] = useState("my-company-knowledge-base")
  const [prefix, setPrefix] = useState("documents/")
  const [accessKey, setAccessKey] = useState("")
  const [secretKey, setSecretKey] = useState("")
  const [statusMessage, setStatusMessage] = useState<{ text: string; type: "success" | "error" } | null>(null)
  const [isTesting, setIsTesting] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  const handleTabChange = (tab: string) => {
    setActiveTab(tab)
    setStatusMessage(null)
    const defaults = CONNECTOR_DEFAULTS[tab]
    if (defaults) {
      setName(defaults.name)
      setBucket(defaults.bucket)
    }
  }

  const handleTest = async () => {
    setIsTesting(true)
    setStatusMessage(null)
    try {
      await goApi.testConnector(workspaceId, activeTab, name, buildConfig())
      setStatusMessage({ text: "Connection test succeeded! Bucket is accessible and permissions are valid.", type: "success" })
    } catch (err: unknown) {
      setStatusMessage({ text: err instanceof Error ? err.message : "Failed to connect to storage provider", type: "error" })
    } finally {
      setIsTesting(false)
    }
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSaving(true)
    setStatusMessage(null)

    try {
      await goApi.createConnector({
        workspace_id: workspaceId,
        type: activeTab,
        name,
        config: buildConfig(),
        sync_frequency: "hourly",
      })
      onConnectorSaved()
      onClose()
    } catch (err: unknown) {
      setStatusMessage({ text: err instanceof Error ? err.message : "Failed to save connector", type: "error" })
    } finally {
      setIsSaving(false)
    }
  }

  const buildConfig = () => {
    if (activeTab === "azure") {
      return { container_name: bucket, prefix, account_name: accessKey, account_key: secretKey }
    }
    if (activeTab === "supabase") {
      return { bucket_name: bucket, prefix, supabase_url: accessKey, service_role_key: secretKey }
    }
    if (activeTab === "confluence") {
      return { space_key: bucket, email: prefix, domain: accessKey, api_token: secretKey }
    }
    return { bucket, prefix, access_key_id: accessKey, secret_access_key: secretKey }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-lg bg-card/95 border-border/80 backdrop-blur-2xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <Cloud className="w-5 h-5 text-primary" />
            <DialogTitle className="text-base font-semibold">Knowledge Connectors</DialogTitle>
          </div>
          <DialogDescription className="text-xs text-muted-foreground">
            Configure enterprise storage crawlers managed by the Go Ingestion Engine.
          </DialogDescription>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={handleTabChange} className="w-full">
          <TabsList className="grid grid-cols-4 w-full">
            <TabsTrigger value="s3">AWS S3</TabsTrigger>
            <TabsTrigger value="azure">Azure Blob</TabsTrigger>
            <TabsTrigger value="supabase">Supabase</TabsTrigger>
            <TabsTrigger value="confluence">Confluence</TabsTrigger>
          </TabsList>

          <form onSubmit={handleSave} className="space-y-3 mt-4">
            <div className="space-y-1">
              <label className="text-xs font-medium text-foreground">Connector Name</label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Production AWS S3 Knowledge Base"
                className="h-8 text-xs"
                required
              />
            </div>

            <div className="space-y-1">
              <label className="text-xs font-medium text-foreground">
                {activeTab === "confluence" ? "Space Key" : "Bucket / Container Name"}
              </label>
              <Input
                value={bucket}
                onChange={(e) => setBucket(e.target.value)}
                placeholder="my-company-bucket"
                className="h-8 text-xs font-mono"
                required
              />
            </div>

            <div className="space-y-1">
              <label className="text-xs font-medium text-foreground">
                {activeTab === "confluence" ? "Account Email" : "Prefix / Directory Filter"}
              </label>
              <Input
                value={prefix}
                onChange={(e) => setPrefix(e.target.value)}
                placeholder={activeTab === "confluence" ? "admin@company.com" : "documents/"}
                className="h-8 text-xs font-mono"
              />
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground">
                  {activeTab === "confluence" ? "Confluence URL" : activeTab === "supabase" ? "Project URL" : "Access Key / Account"}
                </label>
                <Input
                  value={accessKey}
                  onChange={(e) => setAccessKey(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground">
                  {activeTab === "confluence" ? "API Token" : activeTab === "supabase" ? "Service Role Key" : "Secret Access Key"}
                </label>
                <Input
                  type="password"
                  value={secretKey}
                  onChange={(e) => setSecretKey(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
            </div>

            {statusMessage && (
              <div
                className={`p-2.5 rounded-md text-xs flex items-center gap-2 ${
                  statusMessage.type === "success"
                    ? "bg-emerald-500/10 border border-emerald-500/20 text-emerald-400"
                    : "bg-destructive/10 border border-destructive/20 text-destructive"
                }`}
              >
                {statusMessage.type === "success" ? (
                  <CheckCircle className="w-4 h-4 shrink-0" />
                ) : (
                  <AlertCircle className="w-4 h-4 shrink-0" />
                )}
                <span>{statusMessage.text}</span>
              </div>
            )}

            <div className="flex items-center justify-between pt-3 border-t border-border/60">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleTest}
                disabled={isTesting}
                className="text-xs h-8"
              >
                {isTesting ? "Testing..." : "Test Connection"}
              </Button>

              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={onClose}
                  className="text-xs h-8"
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  size="sm"
                  disabled={isSaving}
                  className="text-xs h-8 bg-primary text-primary-foreground"
                >
                  {isSaving ? "Saving..." : "Save & Sync"}
                </Button>
              </div>
            </div>
          </form>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
