import React, { useState, useEffect } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { pyApi, DocumentItem, DocumentVersionItem } from "@/lib/api"
import { Database, FileText, History, Layers } from "lucide-react"

interface DocumentExplorerModalProps {
  open: boolean
  onClose: () => void
  workspaceId: string
}

export const DocumentExplorerModal: React.FC<DocumentExplorerModalProps> = ({
  open,
  onClose,
  workspaceId,
}) => {
  const [documents, setDocuments] = useState<DocumentItem[]>([])
  const [selectedDocId, setSelectedDocId] = useState<string | null>(null)
  const [versions, setVersions] = useState<DocumentVersionItem[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) {
      loadDocs()
    }
  }, [open, workspaceId])

  const loadDocs = async () => {
    setLoading(true)
    try {
      const docs = await pyApi.listDocuments(workspaceId)
      setDocuments(docs)
      if (docs.length > 0) {
        selectDoc(docs[0].id)
      }
    } finally {
      setLoading(false)
    }
  }

  const selectDoc = async (id: string) => {
    setSelectedDocId(id)
    try {
      const v = await pyApi.getDocumentVersions(id)
      setVersions(v)
    } catch {
      setVersions([])
    }
  }

  const formatBytes = (bytes: number) => {
    if (!bytes) return "0 B"
    const k = 1024
    const sizes = ["B", "KB", "MB", "GB"]
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl bg-card/95 border-border/80 backdrop-blur-2xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <Database className="w-5 h-5 text-cyan-400" />
            <DialogTitle className="text-base font-semibold">Knowledge Base Explorer</DialogTitle>
          </div>
          <DialogDescription className="text-xs text-muted-foreground">
            View all documents, vector chunks, and version history indexed for workspace: <span className="font-mono text-foreground">{workspaceId}</span>
          </DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 my-2 max-h-96 min-h-[260px]">
          {/* Documents List */}
          <div className="border border-border/60 rounded-lg p-2 flex flex-col gap-1 overflow-y-auto bg-secondary/20">
            <div className="text-[11px] font-semibold text-muted-foreground uppercase px-2 py-1">
              Documents ({documents.length})
            </div>
            {documents.length === 0 ? (
              <div className="text-xs text-muted-foreground italic p-4 text-center">
                {loading ? "Loading documents..." : "No documents indexed yet"}
              </div>
            ) : (
              documents.map((doc) => (
                <button
                  key={doc.id}
                  onClick={() => selectDoc(doc.id)}
                  className={`p-2 rounded-md text-left text-xs transition-all flex items-start gap-2 ${
                    selectedDocId === doc.id
                      ? "bg-accent text-accent-foreground border border-primary/40 font-medium"
                      : "hover:bg-accent/40 text-muted-foreground"
                  }`}
                >
                  <FileText className="w-3.5 h-3.5 mt-0.5 shrink-0 text-cyan-400" />
                  <div className="flex flex-col truncate flex-1">
                    <span className="truncate text-foreground font-medium">{doc.file_name}</span>
                    <span className="text-[10px] text-muted-foreground font-mono">
                      {formatBytes(doc.file_size)} • {doc.status}
                    </span>
                  </div>
                </button>
              ))
            )}
          </div>

          {/* Versions History */}
          <div className="border border-border/60 rounded-lg p-3 flex flex-col gap-2 overflow-y-auto bg-secondary/20">
            <div className="flex items-center gap-1.5 text-[11px] font-semibold text-muted-foreground uppercase">
              <History className="w-3.5 h-3.5 text-amber-400" />
              Version Diff History
            </div>

            {versions.length === 0 ? (
              <div className="text-xs text-muted-foreground italic p-4 text-center">
                Select a document to view versions
              </div>
            ) : (
              versions.map((ver) => (
                <div
                  key={ver.id}
                  className="p-2.5 rounded-md bg-card/60 border border-border/60 text-xs flex flex-col gap-1"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-foreground">Version {ver.version_number}</span>
                    <Badge variant="outline" className="text-[9px] font-mono">
                      {ver.total_chunks} Chunks
                    </Badge>
                  </div>
                  <div className="text-[10px] text-muted-foreground font-mono truncate">
                    Hash: {ver.file_hash.substring(0, 16)}...
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        <div className="flex justify-end pt-2">
          <Button onClick={onClose} size="sm" className="text-xs h-8">
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
