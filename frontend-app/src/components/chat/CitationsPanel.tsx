import React from "react"
import { Bookmark, FileText, ExternalLink } from "lucide-react"
import { Citation } from "@/lib/api"

interface CitationsPanelProps {
  citations: Citation[]
}

export const CitationsPanel: React.FC<CitationsPanelProps> = ({ citations }) => {
  if (!citations || citations.length === 0) return null

  return (
    <div className="mt-3 pt-3 border-t border-border/40">
      <div className="flex items-center gap-1.5 text-[11px] font-semibold text-cyan-400 uppercase tracking-wider mb-2">
        <Bookmark className="w-3.5 h-3.5" />
        Grounded Citations ({citations.length})
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
        {citations.map((c) => (
          <div
            key={c.index}
            className="p-2.5 rounded-md bg-secondary/40 border border-border/60 hover:border-cyan-500/50 transition-all text-xs flex flex-col gap-1 group"
          >
            <div className="flex items-center justify-between gap-1">
              <div className="flex items-center gap-1.5 font-medium text-foreground truncate">
                <FileText className="w-3 h-3 text-cyan-400 shrink-0" />
                <span className="truncate">[{c.index}] {c.file_name}</span>
              </div>
              {c.source_uri && (
                <span className="text-[10px] text-muted-foreground font-mono truncate max-w-[80px]">
                  {c.source_uri.split("/").pop()}
                </span>
              )}
            </div>

            <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
              {c.page_number && <span>Page {c.page_number}</span>}
              {c.heading && <span className="truncate">• {c.heading}</span>}
              <span className="ml-auto text-emerald-400 font-mono">Score: {c.score}</span>
            </div>

            <p className="text-[11px] text-muted-foreground line-clamp-2 italic mt-0.5">
              "{c.snippet}"
            </p>
          </div>
        ))}
      </div>
    </div>
  )
}
