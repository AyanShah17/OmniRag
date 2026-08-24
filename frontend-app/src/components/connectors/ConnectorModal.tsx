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
  const [accessKey, setAccessKey] = useState("AKIAIOSFODNN7EXAMPLE")
  const [secretKey, setSecretKey] = useState("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
  const [statusMessage, setStatusMessage] = useState<{ text: string; type: "success" | "error" } | null>(null)
  const [isTesting, setIsTesting] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  const handleTabChange = (tab: string) => {
    setActiveTab(tab)
    setStatusMessage(null)
    if (tab === "s3") {
      setName("Corporate S3 Bucket")
      setBucket("my-company-knowledge-base")
    } else if (tab === "azure") {
      setName("Azure Blob Storage")
      setBucket("rag-documents-container")
    } else if (tab === "supabase") {
      setName("Supabase Storage")
      setBucket("knowledge-vault")
    } else if (tab === "confluence") {
      setName("Engineering Confluence Wiki")
      setBucket("ENG-SPACE")
    }
  }

  const handleTest = async () => {
    setIsTesting(true)
    setStatusMessage(null)
    try {
      await goApi.testConnector(activeTab, name, {
        bucket,
        prefix,
        access_key_id: accessKey,
        secret_access_key: secretKey,
      })
      setStatusMessage({ text: "Connection test succeeded! Bucket is accessible and permissions are valid.", type: "success" })
    } catch (err: any) {
      setStatusMessage({ text: err.message || "Failed to connect to storage provider", type: "error" })
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
        config: {
          bucket,
          prefix,
          access_key_id: accessKey,
          secret_access_key: secretKey,
        },
        sync_frequency: "hourly",
      })
      onConnectorSaved()
      onClose()
    } catch (err: any) {
      setStatusMessage({ text: err.message || "Failed to save connector", type: "error" })
    } finally {
      setIsSaving(false)
    }
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
                {activeTab === "confluence" ? "Space Key / Site" : "Bucket / Container Name"}
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
              <label className="text-xs font-medium text-foreground">Prefix / Directory Filter</label>
              <Input
                value={prefix}
                onChange={(e) => setPrefix(e.target.value)}
                placeholder="documents/"
                className="h-8 text-xs font-mono"
              />
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground">Access Key / Account</label>
                <Input
                  value={accessKey}
                  onChange={(e) => setAccessKey(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground">Secret Access Key</label>
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
