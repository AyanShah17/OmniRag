import React, { useRef } from "react"
import { UploadCloud, Activity, Zap } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"

interface HeaderProps {
  pyHealth: string
  goHealth: string
  onFileUpload: (file: File) => void
  isUploading: boolean
}

export const Header: React.FC<HeaderProps> = ({
  pyHealth,
  goHealth,
  onFileUpload,
  isUploading,
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
    <header className="h-14 border-b border-border/60 bg-card/30 backdrop-blur-xl flex items-center justify-between px-6 shrink-0 z-10">
      <div className="flex items-center gap-3">
        <div className="flex flex-col">
          <div className="flex items-center gap-2">
            <h1 className="text-sm font-semibold text-foreground">Dynamic RAG Assistant</h1>
            <span className="text-[11px] text-muted-foreground">•</span>
            <span className="text-xs text-muted-foreground font-mono">Pinecone + FastEmbed ONNX</span>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-3">
        {/* Backend Status Indicators */}
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 bg-secondary/50 border border-border/60 px-2.5 py-1 rounded-md text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full ${pyHealth === "healthy" ? "bg-emerald-400" : "bg-amber-400"}`} />
            <span className="text-muted-foreground">Python RAG:</span>
            <span className="font-mono text-foreground font-medium">{pyHealth === "healthy" ? "Online" : "Connecting"}</span>
          </div>

          <div className="flex items-center gap-1.5 bg-secondary/50 border border-border/60 px-2.5 py-1 rounded-md text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full ${goHealth === "healthy" ? "bg-cyan-400" : "bg-amber-400"}`} />
            <span className="text-muted-foreground">Go Engine:</span>
            <span className="font-mono text-foreground font-medium">{goHealth === "healthy" ? "Online" : "Connecting"}</span>
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
