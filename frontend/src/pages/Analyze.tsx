import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Send,
  Bot,
  User as UserIcon,
  Sparkles,
  Loader2,
  AlertTriangle,
  Settings2,
} from "lucide-react";
import { aiApi } from "@/api";
import type { ChatMessage } from "@/api/types";
import { useConfigStore } from "@/store/config";
import AISettingsDialog from "./AISettingsDialog";

const SUGGESTIONS = [
  "我的账户总权益是多少？",
  "我现在有哪些持仓？帮我分析盈亏",
  "BTC 现在什么价位？",
  "帮我看看账户里各币种占比",
];

export default function AnalyzePage() {
  const navigate = useNavigate();
  const { list, activeId } = useConfigStore();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [thinking, setThinking] = useState(false);
  const [error, setError] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);
  const [showAISettings, setShowAISettings] = useState(false);
  const [historyLoaded, setHistoryLoaded] = useState(false);

  // 加载历史
  useEffect(() => {
    (async () => {
      try {
        const r = await aiApi.history(activeId ?? undefined);
        setMessages(r.data.list);
      } catch {
        setMessages([]);
      } finally {
        setHistoryLoaded(true);
      }
    })();
  }, [activeId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, thinking]);

  const send = async (text: string) => {
    const content = text.trim();
    if (!content || thinking) return;
    setError("");
    // 本地追加用户消息
    const tempUser: ChatMessage = {
      id: Date.now(),
      role: "user",
      content,
      config_id: activeId || 0,
      created_at: new Date().toISOString(),
    };
    setMessages((m) => [...m, tempUser]);
    setInput("");
    setThinking(true);
    try {
      const r = await aiApi.chat(content, activeId ?? undefined);
      const answer: ChatMessage = {
        id: Date.now() + 1,
        role: "assistant",
        content: r.data.answer,
        config_id: activeId || 0,
        created_at: new Date().toISOString(),
      };
      setMessages((m) => [...m, answer]);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setThinking(false);
    }
  };

  const noConfig = list.length === 0;

  return (
    <div className="flex flex-col h-full max-h-[calc(100vh-8rem)]">
      {/* 顶部操作条 */}
      <div className="flex items-center justify-between mb-3">
        <p className="text-xs text-slate-400">
          通过自然语言提问，AI 将实时调用 OKX 接口并分析
        </p>
        <button
          className="btn-outline !px-3 !py-1.5 text-xs"
          onClick={() => setShowAISettings(true)}
        >
          <Settings2 size={13} className="text-slate-500" />
          AI 模型配置
        </button>
      </div>
      {showAISettings && <AISettingsDialog onClose={() => setShowAISettings(false)} />}

      {/* 头部提示 */}
      {noConfig && (
        <div className="flex items-center justify-between bg-amber-50 border border-amber-200 text-amber-700 text-sm rounded-lg px-4 py-3 mb-4">
          <span className="flex items-center gap-2">
            <AlertTriangle size={16} />
            请先配置 OKX 密钥，AI 才能拉取你的账户数据
          </span>
          <button className="text-amber-800 font-medium underline" onClick={() => navigate("/config")}>
            去配置
          </button>
        </div>
      )}

      {/* 消息区 */}
      <div className="card flex-1 overflow-y-auto p-5 space-y-5 bg-white min-h-0">
        {!historyLoaded ? (
          <div className="text-center py-20 text-slate-400">加载中…</div>
        ) : messages.length === 0 && !thinking ? (
          <Welcome />
        ) : (
          <>
            {messages.map((m) => (
              <Bubble key={m.id} msg={m} />
            ))}
            {thinking && <Thinking />}
            <div ref={bottomRef} />
          </>
        )}
      </div>

      {/* 建议 chips */}
      {messages.length === 0 && (
        <div className="flex flex-wrap gap-2 mt-3">
          {SUGGESTIONS.map((s) => (
            <button
              key={s}
              disabled={thinking || noConfig}
              onClick={() => send(s)}
              className="px-3 py-1.5 text-xs rounded-full bg-white border border-slate-200 text-slate-600 hover:border-brand-300 hover:text-brand-600 disabled:opacity-50 transition"
            >
              <Sparkles size={12} className="inline mr-1 text-brand-500" />
              {s}
            </button>
          ))}
        </div>
      )}

      {/* 错误提示 */}
      {error && (
        <div className="mt-3 text-sm text-red-600 bg-red-50 rounded-lg px-4 py-2.5 flex items-start gap-2">
          <AlertTriangle size={15} className="mt-0.5 shrink-0" />
          <span className="flex-1">{error}</span>
          <button onClick={() => setError("")} className="text-slate-400 hover:text-slate-600">
            ×
          </button>
        </div>
      )}

      {/* 输入框 */}
      <div className="mt-3">
        <div className="card flex items-end gap-2 p-2 pl-4">
          <textarea
            className="flex-1 resize-none bg-transparent outline-none text-sm py-2 text-slate-800 placeholder:text-slate-400 max-h-32"
            rows={1}
            placeholder={noConfig ? "先配置 OKX 密钥后即可提问…" : "问问 AI：我的资产怎么样？持仓是否该止盈？"}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                send(input);
              }
            }}
          />
          <button
            className="btn-primary !rounded-lg !px-3.5"
            disabled={thinking || noConfig || !input.trim()}
            onClick={() => send(input)}
          >
            {thinking ? <Loader2 size={16} className="animate-spin" /> : <Send size={16} />}
            <span className="hidden sm:inline">{thinking ? "分析中" : "发送"}</span>
          </button>
        </div>
        <p className="text-[11px] text-slate-400 mt-1.5 px-1">
          Enter 发送，Shift+Enter 换行。AI 会自动调用 OKX 接口拉取真实数据后作答。
        </p>
      </div>
    </div>
  );
}

function Welcome() {
  return (
    <div className="text-center py-14">
      <div className="mx-auto w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-500 to-indigo-600 flex items-center justify-center mb-4 shadow-lg">
        <Bot size={30} className="text-white" />
      </div>
      <h2 className="text-xl font-bold text-slate-800">你好，我是你的 OKX 分析助手</h2>
      <p className="text-sm text-slate-400 mt-2 max-w-md mx-auto">
        你可以直接问我账户资产、持仓盈亏、币价走势等问题，
        我会实时调用 OKX 接口获取真实数据，再给你专业的分析和建议。
      </p>
    </div>
  );
}

function Bubble({ msg }: { msg: ChatMessage }) {
  const isUser = msg.role === "user";
  return (
    <div className={`flex gap-3 ${isUser ? "flex-row-reverse" : ""}`}>
      <div
        className={`shrink-0 h-8 w-8 rounded-lg flex items-center justify-center ${
          isUser ? "bg-slate-200" : "bg-gradient-to-br from-brand-500 to-indigo-600"
        }`}
      >
        {isUser ? (
          <UserIcon size={16} className="text-slate-600" />
        ) : (
          <Bot size={16} className="text-white" />
        )}
      </div>
      <div
        className={`max-w-[80%] px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap rounded-2xl ${
          isUser
            ? "bg-brand-600 text-white rounded-tr-sm"
            : "bg-slate-100 text-slate-700 rounded-tl-sm"
        }`}
      >
        {msg.content}
      </div>
    </div>
  );
}

function Thinking() {
  return (
    <div className="flex gap-3 items-center text-sm text-slate-400">
      <div className="h-8 w-8 rounded-lg bg-gradient-to-br from-brand-500 to-indigo-600 flex items-center justify-center">
        <Bot size={16} className="text-white" />
      </div>
      <div className="flex items-center gap-1 px-3 py-2.5 bg-slate-100 rounded-2xl rounded-tl-sm">
        <Loader2 size={14} className="animate-spin text-brand-500" />
        <span>正在查询 OKX 数据并分析…</span>
      </div>
    </div>
  );
}
