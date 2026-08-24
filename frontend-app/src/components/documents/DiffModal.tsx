import React from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DiffResult } from "@/lib/api"
import { CheckCircle2, Sparkles, Database, ArrowRight } from "lucide-react"

interface DiffModalProps {
  open: boolean
  onClose: () => void
  diffResult: DiffResult | null
}

export const DiffModal: React.FC<DiffModalProps> = ({
  open,
  onClose,
  diffResult,
}) => {
  if (!diffResult) return null

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-md bg-card/95 border-border/80 backdrop-blur-2xl">
        <DialogHeader className="space-y-1">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="w-5 h-5 text-emerald-400" />
            <DialogTitle className="text-base font-semibold">Document Synchronized</DialogTitle>
          </div>
          <DialogDescription className="text-xs text-muted-foreground">
            {diffResult.file_name} • Version {diffResult.version_number}
          </DialogDescription>
        </DialogHeader>

        {/* Diff Breakdown Metrics */}
        <div className="grid grid-cols-2 gap-2.5 my-2">
          <div className="p-3 rounded-lg bg-secondary/40 border border-border/60 text-center">
            <div className="text-lg font-bold text-foreground font-mono">{diffResult.total_chunks}</div>
            <div className="text-[11px] text-muted-foreground">Total Chunks</div>
          </div>

          <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-center">
            <div className="text-lg font-bold text-emerald-400 font-mono">{diffResult.reused_chunks_count}</div>
            <div className="text-[11px] text-emerald-400/80">Reused ($0 Cost)</div>
          </div>

          <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20 text-center">
            <div className="text-lg font-bold text-amber-400 font-mono">{diffResult.new_chunks_embedded}</div>
            <div className="text-[11px] text-amber-400/80">Newly Embedded</div>
          </div>

          <div className="p-3 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-center">
            <div className="text-lg font-bold text-cyan-400 font-mono">{diffResult.cost_savings_percent}</div>
            <div className="text-[11px] text-cyan-400/80">Cost Savings</div>
          </div>
        </div>

        <div className="p-3 rounded-md bg-secondary/30 border border-border/40 text-xs text-muted-foreground space-y-1.5">
          <div className="flex items-center gap-1.5 font-medium text-foreground text-[11px]">
            <Sparkles className="w-3.5 h-3.5 text-cyan-400" />
            Deterministic SHA-256 Chunk Hashing
          </div>
          <p className="text-[11px] leading-relaxed">
            Only the {diffResult.new_chunks_embedded} modified chunk(s) incurred embedding API fees. All other {diffResult.reused_chunks_count} chunk(s) were seamlessly re-linked to the new document version at zero cost.
          </p>
        </div>

        <div className="flex justify-end pt-2">
          <Button onClick={onClose} size="sm" className="text-xs h-8">
            Done
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
