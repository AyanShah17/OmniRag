import React from "react"
import { Bot, User } from "lucide-react"
import { Citation } from "@/lib/api"
import { CitationsPanel } from "@/components/chat/CitationsPanel"

export interface MessageItem {
  role: "user" | "assistant" | "system"
  content: string
  citations?: Citation[]
  isStreaming?: boolean
}

interface MessageBubbleProps {
  message: MessageItem
}

export const MessageBubble: React.FC<MessageBubbleProps> = ({ message }) => {
  const isUser = message.role === "user"

  // Simple clean markdown parser for bold, italic, code blocks, lists
  const renderFormattedContent = (text: string) => {
    const lines = text.split("\n")
    return lines.map((line, i) => {
      // Bold tags
      let parsed = line.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      // Italic tags
      parsed = parsed.replace(/\*(.*?)\*/g, '<em>$1</em>')
      // Inline code
      parsed = parsed.replace(/`(.*?)`/g, '<code class="bg-muted px-1 py-0.5 rounded text-xs font-mono text-cyan-300">$1</code>')
      // Citations [1], [2] styling
      parsed = parsed.replace(/\[(\d+)\]/g, '<span class="inline-flex items-center justify-center bg-cyan-500/20 text-cyan-300 font-mono text-[10px] px-1 rounded mx-0.5 font-bold cursor-pointer hover:bg-cyan-500/30">[$1]</span>')

      if (line.startsWith("• ") || line.startsWith("- ")) {
        return (
          <li key={i} className="ml-4 list-disc text-sm my-0.5" dangerouslySetInnerHTML={{ __html: parsed.substring(2) }} />
        )
      }

      if (line.trim() === "") {
        return <div key={i} className="h-2" />
      }

      return (
        <p key={i} className="text-sm leading-relaxed" dangerouslySetInnerHTML={{ __html: parsed }} />
      )
    })
  }

  return (
    <div className={`flex gap-3 max-w-3xl w-full mx-auto ${isUser ? "justify-end" : "justify-start"}`}>
      {!isUser && (
        <div className="w-7 h-7 rounded-md bg-gradient-to-br from-blue-600 to-cyan-600 flex items-center justify-center text-white shrink-0 mt-0.5 shadow-sm">
          <Bot className="w-3.5 h-3.5" />
        </div>
      )}

      <div
        className={`flex flex-col rounded-xl px-4 py-3 max-w-[85%] text-sm ${
          isUser
            ? "bg-primary text-primary-foreground shadow-sm shadow-blue-600/20 rounded-tr-sm"
            : "bg-card/70 border border-border/60 text-foreground backdrop-blur-md rounded-tl-sm shadow-sm"
        }`}
      >
        <div className="space-y-1">
          {renderFormattedContent(message.content)}
          {message.isStreaming && (
            <span className="inline-block w-1.5 h-3.5 bg-cyan-400 animate-pulse ml-0.5 align-middle" />
          )}
        </div>

        {!isUser && message.citations && message.citations.length > 0 && (
          <CitationsPanel citations={message.citations} />
        )}
      </div>

      {isUser && (
        <div className="w-7 h-7 rounded-md bg-secondary border border-border flex items-center justify-center text-muted-foreground shrink-0 mt-0.5">
          <User className="w-3.5 h-3.5" />
        </div>
      )}
    </div>
  )
}
