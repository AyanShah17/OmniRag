import React, { useRef, useEffect } from "react"
import { MessageItem, MessageBubble } from "@/components/chat/MessageBubble"
import { ChatInput } from "@/components/chat/ChatInput"
import { Layers, ShieldCheck, Database, Zap } from "lucide-react"

interface ChatInterfaceProps {
  messages: MessageItem[]
  onSendMessage: (text: string) => void
  isStreaming: boolean
  onAttachClick: () => void
}

export const ChatInterface: React.FC<ChatInterfaceProps> = ({
  messages,
  onSendMessage,
  isStreaming,
  onAttachClick,
}) => {
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden relative">
      {/* Scrollable Messages Area */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-6 sm:px-6 space-y-6">
        {messages.length === 0 ? (
          <div className="mx-auto flex max-w-3xl flex-col items-start space-y-7 py-8 text-left sm:py-14">
            <div className="flex h-11 w-11 items-center justify-center rounded-md border border-primary/30 bg-primary/10 text-primary">
              <Layers className="w-6 h-6" />
            </div>

            <div className="space-y-2">
              <h2 className="text-2xl font-semibold tracking-tight text-foreground">
                Your research desk
              </h2>
              <p className="text-sm text-muted-foreground max-w-md">
                Search across connected documents with answers that show their source and respect workspace access.
              </p>
            </div>

            {/* Quick Prompt Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 w-full text-left pt-2">
              <button type="button"
                onClick={() => onSendMessage("What is our cloud data retention and encryption policy?")}
                className="rounded-lg border border-border/70 bg-card/55 p-4 text-left transition-colors hover:border-primary/50 hover:bg-secondary/60 focus-visible:ring-2 focus-visible:ring-ring"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <ShieldCheck className="w-4 h-4 text-emerald-400" />
                  Find a policy
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "What is our cloud data retention and encryption policy?"
                </p>
              </button>

              <button type="button"
                onClick={() => onSendMessage("What is our incident response SLA for cloud breaches?")}
                className="rounded-lg border border-border/70 bg-card/55 p-4 text-left transition-colors hover:border-primary/50 hover:bg-secondary/60 focus-visible:ring-2 focus-visible:ring-ring"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <Zap className="w-4 h-4 text-amber-400" />
                  Trace an incident
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "What is our incident response SLA for cloud breaches?"
                </p>
              </button>

              <button type="button"
                onClick={() => onSendMessage("How does the SHA-256 chunk diffing engine work?")}
                className="rounded-lg border border-border/70 bg-card/55 p-4 text-left transition-colors hover:border-primary/50 hover:bg-secondary/60 focus-visible:ring-2 focus-visible:ring-ring"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <Database className="w-4 h-4 text-cyan-400" />
                  Compare revisions
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "How does the SHA-256 chunk diffing engine work?"
                </p>
              </button>

              <button type="button"
                onClick={() => onSendMessage("What model providers and embedding dimensions are configured?")}
                className="rounded-lg border border-border/70 bg-card/55 p-4 text-left transition-colors hover:border-primary/50 hover:bg-secondary/60 focus-visible:ring-2 focus-visible:ring-ring"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <Layers className="w-4 h-4 text-indigo-400" />
                  Inspect the stack
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "What model providers and embedding dimensions are configured?"
                </p>
              </button>
            </div>
          </div>
        ) : (
          messages.map((msg, i) => <MessageBubble key={i} message={msg} />)
        )}
      </div>

      {/* Input Area */}
      <div className="border-t border-border/40 bg-background/90 p-3 sm:p-4">
        <ChatInput
          onSend={onSendMessage}
          disabled={isStreaming}
          onAttachClick={onAttachClick}
        />
      </div>
    </div>
  )
}
