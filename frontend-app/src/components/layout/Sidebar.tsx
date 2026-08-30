import React from "react"
import { Plus, MessageSquare, Database, Settings, Layers, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ConnectorItem, ConversationItem } from "@/lib/api"

interface SidebarProps {
  conversations: ConversationItem[]
  currentConvId: string | null
  onSelectConversation: (id: string) => void
  onNewChat: () => void
  connectors: ConnectorItem[]
  onOpenConnectors: () => void
  onOpenDocuments: () => void
  onOpenSettings: () => void
  workspaceId: string
  isOpen: boolean
  onClose: () => void
}

export const Sidebar: React.FC<SidebarProps> = ({
  conversations,
  currentConvId,
  onSelectConversation,
  onNewChat,
  connectors,
  onOpenConnectors,
  onOpenDocuments,
  onOpenSettings,
  workspaceId,
  isOpen,
  onClose,
}) => {
  return (
    <>
    {isOpen && <button type="button" className="fixed inset-0 z-30 bg-black/60 md:hidden" onClick={onClose} aria-label="Close navigation" />}
    <aside className={`fixed inset-y-0 left-0 z-40 flex w-[286px] flex-col gap-4 border-r border-border/70 bg-card p-4 backdrop-blur-xl transition-transform duration-200 md:relative md:z-auto md:w-72 md:translate-x-0 ${isOpen ? "translate-x-0" : "-translate-x-full"}`}>
      <div className="flex items-center justify-between md:hidden">
        <span className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">Navigation</span>
        <button type="button" onClick={onClose} className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground" aria-label="Close navigation" title="Close navigation"><X className="h-4 w-4" /></button>
      </div>
      {/* Brand */}
      <div className="flex items-center gap-3 px-2 py-1">
        <div className="w-8 h-8 rounded-md border border-primary/30 bg-primary/15 flex items-center justify-center text-primary">
          <Layers className="w-4 h-4" />
        </div>
        <div className="flex flex-col">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm tracking-tight text-foreground">OmniRAG</span>
            <Badge variant="outline" className="text-[9px] px-1.5 py-0">WORKSPACE</Badge>
          </div>
          <span className="text-[11px] text-muted-foreground font-mono truncate max-w-[140px]">{workspaceId}</span>
        </div>
      </div>

      {/* New Conversation Button */}
      <Button
        onClick={onNewChat}
        variant="outline"
        className="w-full justify-start gap-2 bg-secondary/50 hover:bg-secondary border-border/80 text-foreground text-xs font-medium h-9"
      >
        <Plus className="w-4 h-4 text-primary" />
        New Chat
      </Button>

      {/* Conversations Section */}
      <div className="flex-1 min-h-0 overflow-y-auto flex flex-col gap-1 pr-1">
        <div className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-1">
          Recent Chats
        </div>
        {conversations.length === 0 ? (
          <div className="text-xs text-muted-foreground/60 px-2 py-3 italic">
            No active conversations
          </div>
        ) : (
          conversations.map((conv) => (
            <button
              key={conv.id}
              onClick={() => onSelectConversation(conv.id)}
              className={`flex items-center gap-2.5 px-2.5 py-2 rounded-md text-xs transition-all text-left truncate w-full ${
                currentConvId === conv.id
                  ? "bg-accent text-accent-foreground font-medium border-l-2 border-primary"
                  : "text-muted-foreground hover:bg-accent/40 hover:text-foreground"
              }`}
            >
              <MessageSquare className="w-3.5 h-3.5 shrink-0 opacity-70" />
              <span className="truncate">{conv.title}</span>
            </button>
          ))
        )}
      </div>

      {/* Cloud Connectors Section */}
      <div className="border-t border-border/40 pt-3 flex flex-col gap-2">
        <div className="flex items-center justify-between px-2">
          <span className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">
            Connectors
          </span>
          <button
            onClick={onOpenConnectors}
            className="text-muted-foreground hover:text-foreground transition-colors p-1 rounded hover:bg-accent/50"
            title="Manage Connectors"
          >
            <Settings className="w-3.5 h-3.5" />
          </button>
        </div>

        <div className="flex flex-col gap-1">
          {connectors.slice(0, 3).map((conn, i) => (
            <div
              key={conn.id || i}
              className="flex items-center justify-between px-2.5 py-1.5 rounded-md bg-secondary/30 border border-border/40 text-xs"
            >
              <div className="flex items-center gap-2 truncate">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-sm shadow-emerald-400/50" />
                <span className="text-foreground truncate max-w-[130px]">{conn.name}</span>
              </div>
              <Badge variant="outline" className="text-[9px] px-1 py-0 uppercase font-mono text-muted-foreground">
                {conn.type}
              </Badge>
            </div>
          ))}
        </div>

        {/* Knowledge Base & Settings Buttons */}
        <div className="flex flex-col gap-1 mt-1">
          <Button
            onClick={onOpenDocuments}
            variant="ghost"
            size="sm"
            className="w-full justify-start gap-2 text-xs text-muted-foreground hover:text-foreground h-8"
          >
            <Database className="w-3.5 h-3.5 text-cyan-400" />
            Browse Knowledge Base
          </Button>

          <Button
            onClick={onOpenSettings}
            variant="ghost"
            size="sm"
            className="w-full justify-start gap-2 text-xs text-muted-foreground hover:text-foreground h-8"
          >
            <Settings className="w-3.5 h-3.5 text-amber-400" />
            AI & Credentials Settings
          </Button>
        </div>
      </div>

      {/* Footer Profile */}
      <div className="border-t border-border/40 pt-3 flex items-center gap-2.5 px-2">
        <div className="w-7 h-7 rounded-md border border-primary/30 bg-primary/10 flex items-center justify-center text-[10px] font-bold text-primary">
          EA
        </div>
        <div className="flex flex-col truncate">
          <span className="text-xs font-medium text-foreground">Enterprise Admin</span>
          <span className="text-[10px] text-emerald-400 flex items-center gap-1">
            <span className="w-1 h-1 rounded-full bg-emerald-400" /> Connected
          </span>
        </div>
      </div>
    </aside>
    </>
  )
}
