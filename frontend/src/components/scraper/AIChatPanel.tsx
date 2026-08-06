"use client";

import { useEffect, useRef } from "react";
import { Bot, X, Eraser, Square, Send, Zap, Brain, Database, HelpCircle, CheckSquare, Filter, Command } from "lucide-react";
import { sendAIChatMessage, setAIChatInput, clearAIChatMessages, abortAIChat } from "@/lib/scraper-store";
import type { AIChatMessage } from "@/lib/scraper-store";

export function AIChatPanel({
  messages,
  loading,
  input,
  scraperT,
  onClose,
}: {
  messages: AIChatMessage[];
  loading: boolean;
  input: string;
  scraperT: Record<string, string>;
  onClose: () => void;
}) {
  const chatEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // 自动滚动到底部
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // 自动聚焦输入框
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleSend = () => {
    if (!input.trim() || loading) return;
    sendAIChatMessage();
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // 快捷指令
  const quickCommands = [
    { label: scraperT.aiChatQuickScrapeAll || "刮削缺失项", prompt: "请帮我刮削所有缺失元数据的书籍", icon: "zap" },
    { label: scraperT.aiChatQuickSetAI || "切换AI模式", prompt: "切换到AI智能刮削模式", icon: "brain" },
    { label: scraperT.aiChatQuickStats || "查看统计", prompt: "告诉我当前书库的元数据统计情况", icon: "chart" },
    { label: scraperT.aiChatQuickHelp || "使用帮助", prompt: "请告诉我如何使用元数据刮削功能", icon: "help" },
    { label: scraperT.aiChatQuickSelectAll || "全选当页", prompt: "全选当前页面的所有书籍", icon: "check" },
    { label: scraperT.aiChatQuickFilter || "筛选缺失", prompt: "筛选出缺失元数据的书籍", icon: "filter" },
  ];

  const visibleMessages = messages.filter((m) => m.role !== "system" || m.commandResult);

  return (
    <div className="flex flex-col h-full">
      {/* 头部 */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border/30 flex-shrink-0 bg-gradient-to-r from-violet-500/5 to-purple-500/5">
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 shadow-sm">
            <Bot className="h-3.5 w-3.5 text-white" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-foreground">
              {scraperT.aiChatTitle || "AI 刮削助手"}
            </h3>
            <p className="text-[10px] text-muted -mt-0.5">
              {scraperT.aiChatSubtitle || "智能对话 · 指令控制"}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          {messages.length > 0 && (
            <button
              onClick={clearAIChatMessages}
              disabled={loading}
              className="flex h-7 w-7 items-center justify-center rounded-lg text-muted hover:text-foreground hover:bg-card-hover transition-colors disabled:opacity-50"
              title={scraperT.aiChatClear || "清空对话"}
            >
              <Eraser className="h-3.5 w-3.5" />
            </button>
          )}
          <button
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center rounded-lg text-muted hover:text-foreground hover:bg-card-hover transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* 消息区域 */}
      <div className="flex-1 overflow-y-auto p-3 space-y-3 min-h-0">
        {visibleMessages.length === 0 ? (
          /* 空状态 — 欢迎词 + 快捷指令 */
          <div className="flex flex-col items-center justify-center h-full space-y-4 py-6">
            <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-violet-500/10 to-purple-500/10">
              <Bot className="h-8 w-8 text-purple-400" />
            </div>
            <div className="text-center space-y-1">
              <h4 className="text-sm font-semibold text-foreground">
                {scraperT.aiChatEmpty || "你好！我是你的刮削助手 🤖"}
              </h4>
              <p className="text-xs text-muted leading-relaxed max-w-[280px]">
                {scraperT.aiChatEmptyDesc || "你可以问我关于元数据刮削的问题，或者直接用自然语言下指令。试试看吧！"}
              </p>
            </div>

            {/* 快捷指令网格 */}
            <div className="grid grid-cols-2 gap-1.5 w-full max-w-[340px]">
              {quickCommands.map((cmd) => (
                <button
                  key={cmd.prompt}
                  onClick={() => sendAIChatMessage(cmd.prompt)}
                  disabled={loading}
                  className="flex items-center gap-1.5 rounded-lg border border-border/40 bg-card-hover/30 px-2.5 py-2 text-[11px] font-medium text-muted hover:text-foreground hover:border-purple-500/30 hover:bg-purple-500/5 transition-all disabled:opacity-50 text-left"
                >
                  {cmd.icon === "zap" && <Zap className="h-3 w-3 text-amber-500 flex-shrink-0" />}
                  {cmd.icon === "brain" && <Brain className="h-3 w-3 text-purple-500 flex-shrink-0" />}
                  {cmd.icon === "chart" && <Database className="h-3 w-3 text-blue-500 flex-shrink-0" />}
                  {cmd.icon === "help" && <HelpCircle className="h-3 w-3 text-emerald-500 flex-shrink-0" />}
                  {cmd.icon === "check" && <CheckSquare className="h-3 w-3 text-accent flex-shrink-0" />}
                  {cmd.icon === "filter" && <Filter className="h-3 w-3 text-orange-500 flex-shrink-0" />}
                  <span className="truncate">{cmd.label}</span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          /* 消息列表 */
          visibleMessages.map((msg) => (
            <div key={msg.id}>
              {msg.role === "user" ? (
                /* 用户消息 */
                <div className="flex justify-end">
                  <div className="max-w-[85%] rounded-2xl rounded-br-md bg-accent px-3.5 py-2 shadow-sm">
                    <p className="text-xs text-white leading-relaxed whitespace-pre-wrap">{msg.content}</p>
                  </div>
                </div>
              ) : msg.role === "system" && msg.commandResult ? (
                /* 指令执行结果 */
                <div className="flex justify-center">
                  <div className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[10px] font-medium ${
                    msg.commandResult.success
                      ? "bg-emerald-500/10 text-emerald-500"
                      : "bg-red-500/10 text-red-500"
                  }`}>
                    <Command className="h-3 w-3" />
                    {msg.commandResult.message}
                  </div>
                </div>
              ) : (
                /* 助手消息 */
                <div className="flex gap-2">
                  <div className="flex h-6 w-6 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 flex-shrink-0 mt-0.5">
                    <Bot className="h-3 w-3 text-white" />
                  </div>
                  <div className="max-w-[85%] rounded-2xl rounded-bl-md bg-card-hover/60 border border-border/20 px-3.5 py-2 shadow-sm">
                    <div className="text-xs text-foreground leading-relaxed whitespace-pre-wrap">
                      {msg.content}
                      {loading && msg === visibleMessages[visibleMessages.length - 1] && !msg.content && (
                        <span className="inline-flex gap-1 ml-1">
                          <span className="h-1.5 w-1.5 rounded-full bg-purple-400 animate-bounce" style={{ animationDelay: "0ms" }} />
                          <span className="h-1.5 w-1.5 rounded-full bg-purple-400 animate-bounce" style={{ animationDelay: "150ms" }} />
                          <span className="h-1.5 w-1.5 rounded-full bg-purple-400 animate-bounce" style={{ animationDelay: "300ms" }} />
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
          ))
        )}
        <div ref={chatEndRef} />
      </div>

      {/* 快捷指令条（有消息时显示在输入框上方） */}
      {visibleMessages.length > 0 && (
        <div className="flex items-center gap-1 px-3 py-1.5 border-t border-border/10 overflow-x-auto scrollbar-hide">
          {quickCommands.slice(0, 4).map((cmd) => (
            <button
              key={cmd.prompt}
              onClick={() => sendAIChatMessage(cmd.prompt)}
              disabled={loading}
              className="flex-shrink-0 rounded-full border border-border/30 bg-card-hover/30 px-2.5 py-1 text-[10px] text-muted hover:text-foreground hover:border-purple-500/30 transition-all disabled:opacity-50"
            >
              {cmd.label}
            </button>
          ))}
        </div>
      )}

      {/* 输入区域 */}
      <div className="flex items-end gap-2 px-3 py-3 border-t border-border/30 flex-shrink-0 bg-card/30">
        <textarea
          ref={inputRef}
          value={input}
          onChange={(e) => setAIChatInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={scraperT.aiChatPlaceholder || "输入问题或指令..."}
          disabled={loading}
          rows={1}
          className="flex-1 rounded-xl bg-card-hover/50 px-3.5 py-2 text-xs text-foreground placeholder-muted/50 outline-none border border-border/40 focus:border-purple-500/50 focus:ring-1 focus:ring-purple-500/20 transition-all resize-none max-h-24 disabled:opacity-50"
          style={{ minHeight: "36px" }}
          onInput={(e) => {
            const el = e.currentTarget;
            el.style.height = "auto";
            el.style.height = Math.min(el.scrollHeight, 96) + "px";
          }}
        />
        {loading ? (
          <button
            onClick={abortAIChat}
            className="flex h-9 w-9 items-center justify-center rounded-xl bg-red-500 text-white shadow-sm transition-colors hover:bg-red-600 flex-shrink-0"
          >
            <Square className="h-3.5 w-3.5" />
          </button>
        ) : (
          <button
            onClick={handleSend}
            disabled={!input.trim()}
            className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 text-white shadow-sm transition-all hover:shadow-md disabled:opacity-40 flex-shrink-0"
          >
            <Send className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
    </div>
  );
}

