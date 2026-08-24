import React, { useState, useRef, useEffect } from "react"
import { Send, Sparkles, Paperclip } from "lucide-react"
import { Button } from "@/components/ui/button"

interface ChatInputProps {
  onSend: (text: string) => void
  disabled: boolean
  onAttachClick: () => void
}

export const ChatInput: React.FC<ChatInputProps> = ({
  onSend,
  disabled,
  onAttachClick,
}) => {
  const [input, setInput] = useState("")
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto"
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 120)}px`
    }
  }, [input])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || disabled) return
    onSend(input.trim())
    setInput("")
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSubmit(e)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="relative w-full max-w-3xl mx-auto">
      <div className="flex items-center gap-2 p-2 bg-card/80 border border-border/80 rounded-xl shadow-lg shadow-black/20 focus-within:border-primary/70 backdrop-blur-xl transition-all">
        <button
          type="button"
          onClick={onAttachClick}
          className="p-2 text-muted-foreground hover:text-foreground transition-colors rounded-lg hover:bg-secondary"
          title="Upload Document"
        >
          <Paperclip className="w-4 h-4" />
        </button>

        <textarea
          ref={textareaRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Ask anything about your connected knowledge base..."
          rows={1}
          disabled={disabled}
          className="flex-1 bg-transparent border-none outline-none text-sm text-foreground placeholder:text-muted-foreground resize-none py-1.5 px-1 max-h-32"
        />

        <Button
          type="submit"
          disabled={!input.trim() || disabled}
          size="icon"
          className="h-8 w-8 rounded-lg bg-primary hover:bg-primary/90 text-primary-foreground shrink-0 shadow-sm"
        >
          <Send className="w-3.5 h-3.5" />
        </Button>
      </div>

      <div className="flex items-center justify-between text-[11px] text-muted-foreground px-2 pt-2">
        <span className="flex items-center gap-1">
          <Sparkles className="w-3 h-3 text-cyan-400" />
          RAG queries automatically apply multi-tenant ACL & dynamic re-ranking
        </span>
        <span className="font-mono text-[10px]">Enter to send</span>
      </div>
    </form>
  )
}
