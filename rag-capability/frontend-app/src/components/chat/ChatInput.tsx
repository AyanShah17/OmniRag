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
    <form onSubmit={handleSubmit} className="relative mx-auto w-full max-w-3xl">
      <div className="flex items-end gap-2 rounded-xl border border-border/80 bg-card/95 p-2 shadow-xl shadow-black/20 transition-colors focus-within:border-primary/70">
        <button
          type="button"
          onClick={onAttachClick}
          className="p-2 text-muted-foreground hover:text-foreground transition-colors rounded-lg hover:bg-secondary"
          title="Upload Document"
          aria-label="Upload document"
        >
          <Paperclip className="w-4 h-4" />
        </button>

        <textarea
          ref={textareaRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Ask about your connected knowledge base..."
          aria-label="Message"
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

      <div className="flex items-center justify-between gap-3 px-2 pt-2 text-[11px] text-muted-foreground">
        <span className="flex items-center gap-1">
          <Sparkles className="w-3 h-3 text-cyan-400" />
          <span className="hidden sm:inline">Queries apply tenant ACLs and dynamic re-ranking</span>
        </span>
        <span className="font-mono text-[10px]">Enter to send</span>
      </div>
    </form>
  )
}
