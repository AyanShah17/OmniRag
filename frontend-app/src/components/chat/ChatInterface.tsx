import React, { useRef, useEffect } from "react"
import { MessageItem, MessageBubble } from "@/components/chat/MessageBubble"
import { ChatInput } from "@/components/chat/ChatInput"
import { Layers, ShieldCheck, Database, Zap } from "lucide-react"
import { Card } from "@/components/ui/card"

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
      <div ref={scrollRef} className="flex-1 overflow-y-auto p-6 space-y-6">
        {messages.length === 0 ? (
          <div className="max-w-2xl mx-auto my-12 flex flex-col items-center text-center space-y-6">
            <div className="w-12 h-12 rounded-2xl bg-gradient-to-tr from-blue-600 to-cyan-500 flex items-center justify-center text-white shadow-xl shadow-blue-500/20">
              <Layers className="w-6 h-6" />
            </div>

            <div className="space-y-2">
              <h2 className="text-xl font-bold tracking-tight text-foreground">
                Enterprise Dynamic RAG
              </h2>
              <p className="text-sm text-muted-foreground max-w-md">
                Connected to your multi-cloud buckets & workspaces. Only modified chunks are embedded, saving up to 90% in vectorization costs.
              </p>
            </div>

            {/* Quick Prompt Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 w-full text-left pt-2">
              <Card
                onClick={() => onSendMessage("What is our cloud data retention and encryption policy?")}
                className="p-3.5 bg-card/40 hover:bg-secondary/40 border-border/60 hover:border-primary/50 transition-all cursor-pointer group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <ShieldCheck className="w-4 h-4 text-emerald-400" />
                  Security & Policies
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "What is our cloud data retention and encryption policy?"
                </p>
              </Card>

              <Card
                onClick={() => onSendMessage("What is our incident response SLA for cloud breaches?")}
                className="p-3.5 bg-card/40 hover:bg-secondary/40 border-border/60 hover:border-primary/50 transition-all cursor-pointer group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <Zap className="w-4 h-4 text-amber-400" />
                  Incident Response
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "What is our incident response SLA for cloud breaches?"
                </p>
              </Card>

              <Card
                onClick={() => onSendMessage("How does the SHA-256 chunk diffing engine work?")}
                className="p-3.5 bg-card/40 hover:bg-secondary/40 border-border/60 hover:border-primary/50 transition-all cursor-pointer group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <Database className="w-4 h-4 text-cyan-400" />
                  Diff & Versioning
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "How does the SHA-256 chunk diffing engine work?"
                </p>
              </Card>

              <Card
                onClick={() => onSendMessage("What model providers and embedding dimensions are configured?")}
                className="p-3.5 bg-card/40 hover:bg-secondary/40 border-border/60 hover:border-primary/50 transition-all cursor-pointer group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground mb-1">
                  <Layers className="w-4 h-4 text-indigo-400" />
                  Architecture
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2">
                  "What model providers and embedding dimensions are configured?"
                </p>
              </Card>
            </div>
          </div>
        ) : (
          messages.map((msg, i) => <MessageBubble key={i} message={msg} />)
        )}
      </div>

      {/* Input Area */}
      <div className="p-4 bg-gradient-to-t from-background via-background/90 to-transparent">
        <ChatInput
          onSend={onSendMessage}
          disabled={isStreaming}
          onAttachClick={onAttachClick}
        />
      </div>
    </div>
  )
}
