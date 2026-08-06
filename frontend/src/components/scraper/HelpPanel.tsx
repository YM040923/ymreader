"use client";

import { useState } from "react";
import { ChevronDown, ChevronUp, AlertCircle, Search, BookMarked, Lightbulb, Wrench, GraduationCap, CircleHelp, X } from "lucide-react";
import { setHelpSearchQuery, resetGuide, startGuide } from "@/lib/scraper-store";

export function HelpPanel({
  scraperT,
  searchQuery,
  onClose,
}: {
  scraperT: Record<string, string>;
  searchQuery: string;
  onClose: () => void;
}) {
  type HelpCategory = "faq" | "tips" | "troubleshoot";
  const [activeCategory, setActiveCategory] = useState<HelpCategory>("faq");

  // FAQ 数据
  const faqItems = [
    { q: scraperT.helpFaq1Q || "什么是元数据刮削？", a: scraperT.helpFaq1A || "" },
    { q: scraperT.helpFaq2Q || "标准模式和AI模式有什么区别？", a: scraperT.helpFaq2A || "" },
    { q: scraperT.helpFaq3Q || "刮削失败怎么办？", a: scraperT.helpFaq3A || "" },
    { q: scraperT.helpFaq4Q || "可以只刮削部分书籍吗？", a: scraperT.helpFaq4A || "" },
    { q: scraperT.helpFaq5Q || "如何编辑错误的元数据？", a: scraperT.helpFaq5A || "" },
  ];

  // Tips
  const tips = [
    scraperT.helpTip1 || "💡 使用AI模式刮削时，先确保在设置中配置了AI服务",
    scraperT.helpTip2 || "💡 文件名越接近正式书名，匹配率越高",
    scraperT.helpTip3 || "💡 通过AI助手可以用自然语言控制操作",
    scraperT.helpTip4 || "💡 点击书籍封面可查看详情并进行精准刮削",
    scraperT.helpTip5 || "💡 排序功能可以按刮削状态排序",
  ];

  // Troubleshoot
  const troubleshootItems = [
    { q: scraperT.helpTrouble1Q || "刮削一直显示失败", a: scraperT.helpTrouble1A || "" },
    { q: scraperT.helpTrouble2Q || "AI模式不可用", a: scraperT.helpTrouble2A || "" },
    { q: scraperT.helpTrouble3Q || "刮削结果不准确", a: scraperT.helpTrouble3A || "" },
  ];

  // 搜索过滤
  const lowerQ = searchQuery.toLowerCase();
  const filteredFaq = lowerQ ? faqItems.filter((f) => f.q.toLowerCase().includes(lowerQ) || f.a.toLowerCase().includes(lowerQ)) : faqItems;
  const filteredTips = lowerQ ? tips.filter((t) => t.toLowerCase().includes(lowerQ)) : tips;
  const filteredTroubleshoot = lowerQ ? troubleshootItems.filter((t) => t.q.toLowerCase().includes(lowerQ) || t.a.toLowerCase().includes(lowerQ)) : troubleshootItems;

  const hasResults = filteredFaq.length > 0 || filteredTips.length > 0 || filteredTroubleshoot.length > 0;

  // FAQ 展开/折叠
  const [expandedFaq, setExpandedFaq] = useState<number | null>(null);
  const [expandedTrouble, setExpandedTrouble] = useState<number | null>(null);

  return (
    <div className="flex flex-col h-full">
      {/* 头部 */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border/30 flex-shrink-0 bg-gradient-to-r from-emerald-500/5 to-teal-500/5">
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-emerald-500 to-teal-600 shadow-sm">
            <CircleHelp className="h-3.5 w-3.5 text-white" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-foreground">
              {scraperT.helpTitle || "帮助中心"}
            </h3>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => { resetGuide(); startGuide(); onClose(); }}
            className="flex items-center gap-1 rounded-lg text-[10px] font-medium text-muted hover:text-accent hover:bg-accent/5 px-2 py-1 transition-all"
            title={scraperT.guideRestartBtn || "重新引导"}
          >
            <GraduationCap className="h-3 w-3" />
            {scraperT.guideRestartBtn || "重新引导"}
          </button>
          <button
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center rounded-lg text-muted hover:text-foreground hover:bg-card-hover transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* 搜索框 */}
      <div className="px-4 py-2.5 border-b border-border/20">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setHelpSearchQuery(e.target.value)}
            placeholder={scraperT.helpSearchPlaceholder || "搜索帮助文档..."}
            className="w-full rounded-lg bg-card-hover/50 pl-8 pr-3 py-1.5 text-xs text-foreground placeholder-muted/50 outline-none border border-border/40 focus:border-emerald-500/50 focus:ring-1 focus:ring-emerald-500/20 transition-all"
          />
        </div>
      </div>

      {/* 分类标签 */}
      {!lowerQ && (
        <div className="flex items-center gap-1 px-4 py-2 border-b border-border/10">
          {(["faq", "tips", "troubleshoot"] as HelpCategory[]).map((cat) => (
            <button
              key={cat}
              onClick={() => setActiveCategory(cat)}
              className={`flex items-center gap-1 rounded-lg px-2.5 py-1 text-[11px] font-medium transition-all ${
                activeCategory === cat
                  ? "bg-emerald-500 text-white"
                  : "bg-card-hover text-muted hover:text-foreground"
              }`}
            >
              {cat === "faq" && <><BookMarked className="h-3 w-3" />{scraperT.helpFaqTitle || "常见问题"}</>}
              {cat === "tips" && <><Lightbulb className="h-3 w-3" />{scraperT.helpTipsTitle || "使用技巧"}</>}
              {cat === "troubleshoot" && <><Wrench className="h-3 w-3" />{scraperT.helpTroubleshootTitle || "故障排除"}</>}
            </button>
          ))}
        </div>
      )}

      {/* 内容区域 */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3 min-h-0">
        {!hasResults ? (
          <div className="text-center py-8 text-xs text-muted">
            {scraperT.helpNoResults || "没有找到匹配的帮助内容"}
          </div>
        ) : (
          <>
            {/* FAQ */}
            {(lowerQ || activeCategory === "faq") && filteredFaq.length > 0 && (
              <div className="space-y-1.5">
                {lowerQ && (
                  <h5 className="text-[10px] font-semibold text-muted uppercase tracking-wider mb-2">
                    {scraperT.helpFaqTitle || "常见问题"}
                  </h5>
                )}
                {filteredFaq.map((item, idx) => (
                  <div key={idx} className="rounded-xl border border-border/30 overflow-hidden">
                    <button
                      onClick={() => setExpandedFaq(expandedFaq === idx ? null : idx)}
                      className="flex w-full items-center justify-between px-3.5 py-2.5 text-left hover:bg-card-hover/30 transition-colors"
                    >
                      <span className="text-xs font-medium text-foreground pr-2">{item.q}</span>
                      {expandedFaq === idx
                        ? <ChevronUp className="h-3.5 w-3.5 text-muted flex-shrink-0" />
                        : <ChevronDown className="h-3.5 w-3.5 text-muted flex-shrink-0" />
                      }
                    </button>
                    {expandedFaq === idx && (
                      <div className="px-3.5 pb-3 border-t border-border/10">
                        <p className="text-xs text-muted leading-relaxed pt-2">{item.a}</p>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {/* Tips */}
            {(lowerQ || activeCategory === "tips") && filteredTips.length > 0 && (
              <div className="space-y-1.5">
                {lowerQ && (
                  <h5 className="text-[10px] font-semibold text-muted uppercase tracking-wider mb-2">
                    {scraperT.helpTipsTitle || "使用技巧"}
                  </h5>
                )}
                {filteredTips.map((tip, idx) => (
                  <div
                    key={idx}
                    className="rounded-xl border border-border/30 bg-amber-500/5 px-3.5 py-2.5"
                  >
                    <p className="text-xs text-foreground/80 leading-relaxed">{tip}</p>
                  </div>
                ))}
              </div>
            )}

            {/* Troubleshoot */}
            {(lowerQ || activeCategory === "troubleshoot") && filteredTroubleshoot.length > 0 && (
              <div className="space-y-1.5">
                {lowerQ && (
                  <h5 className="text-[10px] font-semibold text-muted uppercase tracking-wider mb-2">
                    {scraperT.helpTroubleshootTitle || "故障排除"}
                  </h5>
                )}
                {filteredTroubleshoot.map((item, idx) => (
                  <div key={idx} className="rounded-xl border border-border/30 overflow-hidden">
                    <button
                      onClick={() => setExpandedTrouble(expandedTrouble === idx ? null : idx)}
                      className="flex w-full items-center justify-between px-3.5 py-2.5 text-left hover:bg-card-hover/30 transition-colors"
                    >
                      <div className="flex items-center gap-2 pr-2">
                        <AlertCircle className="h-3.5 w-3.5 text-amber-500 flex-shrink-0" />
                        <span className="text-xs font-medium text-foreground">{item.q}</span>
                      </div>
                      {expandedTrouble === idx
                        ? <ChevronUp className="h-3.5 w-3.5 text-muted flex-shrink-0" />
                        : <ChevronDown className="h-3.5 w-3.5 text-muted flex-shrink-0" />
                      }
                    </button>
                    {expandedTrouble === idx && (
                      <div className="px-3.5 pb-3 border-t border-border/10">
                        <p className="text-xs text-muted leading-relaxed pt-2">{item.a}</p>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

