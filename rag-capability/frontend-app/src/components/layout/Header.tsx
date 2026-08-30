import React, { useRef } from "react"
import { UploadCloud, Menu } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"

interface HeaderProps {
  pyHealth: string
  goHealth: string
  onFileUpload: (file: File) => void
  isUploading: boolean
  onToggleSidebar: () => void
}

export const Header: React.FC<HeaderProps> = ({
  pyHealth,
  goHealth,
  onFileUpload,
  isUploading,
  onToggleSidebar,
}) => {
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      onFileUpload(file)
      if (fileInputRef.current) {
        fileInputRef.current.value = ""
      }
    }
  }

  return (
    <header className="h-16 border-b border-border bg-card flex items-center justify-between px-4 sm:px-6 shrink-0 z-10">
      <div className="flex items-center gap-3">
        <button type="button" onClick={onToggleSidebar} className="md:hidden inline-flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground" aria-label="Open navigation" title="Open navigation">
          <Menu className="h-4 w-4" />
        </button>
        <div className="flex flex-col">
          <div className="flex items-center gap-2">
            <h1 className="text-sm font-semibold text-foreground">OmniRAG workspace</h1>
            <span className="text-[11px] text-muted-foreground">•</span>
            <span className="hidden sm:inline text-xs text-muted-foreground">Knowledge desk</span>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-3">
        {/* Backend Status Indicators */}
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 border border-border bg-background px-2.5 py-1.5 rounded-md text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full ${pyHealth === "healthy" ? "bg-emerald-600" : pyHealth === "offline" ? "bg-rose-500" : "bg-amber-500"}`} />
            <span className="text-muted-foreground">RAG API</span>
            <span className="text-foreground font-medium">{pyHealth === "healthy" ? "Ready" : pyHealth === "offline" ? "Offline" : "Checking"}</span>
          </div>

          <div className="flex items-center gap-1.5 border border-border bg-background px-2.5 py-1.5 rounded-md text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full ${goHealth === "healthy" ? "bg-emerald-600" : goHealth === "offline" ? "bg-rose-500" : "bg-amber-500"}`} />
            <span className="text-muted-foreground">Connectors</span>
            <span className="text-foreground font-medium">{goHealth === "healthy" ? "Ready" : goHealth === "offline" ? "Offline" : "Checking"}</span>
          </div>
        </div>

        {/* Upload & Diff Button */}
        <Button
          onClick={() => fileInputRef.current?.click()}
          disabled={isUploading}
          size="sm"
          className="gap-2 text-xs bg-primary hover:bg-primary/90 text-primary-foreground shadow-sm h-8"
        >
          <UploadCloud className="w-3.5 h-3.5" />
          {isUploading ? "Diffing Chunks..." : "Upload & Diff"}
        </Button>
        <input
          type="file"
          ref={fileInputRef}
          onChange={handleFileChange}
          className="hidden"
          accept=".pdf,.docx,.doc,.md,.txt,.html,.csv"
        />
      </div>
    </header>
  )
}
