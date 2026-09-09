import { useEffect, useState } from "react";
import {
  Loader2,
  X,
  Bot,
  KeyRound,
  Globe,
  Cpu,
  CheckCircle2,
  AlertCircle,
  Link as LinkIcon,
} from "lucide-react";
import { settingsApi } from "@/api";
import type { AISettings } from "@/api";

const PRESETS: Record<string, { base_url: string; model: string; hint: string }> = {
  openai: { base_url: "https://api.openai.com/v1", model: "gpt-4o-mini", hint: "OpenAI 官方" },
  deepseek: { base_url: "https://api.deepseek.com", model: "deepseek-chat", hint: "DeepSeek（推荐国内）" },
  moonshot: { base_url: "https://api.moonshot.cn/v1", model: "moonshot-v1-8k", hint: "Kimi Moonshot" },
  dashscope: { base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1", model: "qwen-plus", hint: "通义千问" },
};

export default function AISettingsDialog({ onClose }: { onClose: () => void }) {
  const [cur, setCur] = useState<AISettings | null>(null);
  const [form, setForm] = useState({
    provider: "",
    base_url: "",
    model: "",
    api_key: "",
  });
  const [preset, setPreset] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; msg: string } | null>(null);

  const load = async () => {
    try {
      const r = await settingsApi.getAI();
      const s = r.data;
      setCur(s);
      setForm({
        provider: s.provider,
        base_url: s.base_url,
        model: s.model,
        api_key: "",
      });
      if (s.provider) setPreset(s.provider);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setLoading(true);
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const applyPreset = (key: string) => {
    if (!key) return;
    const p = PRESETS[key];
    setForm((f) => ({ ...f, provider: key, base_url: p.base_url, model: p.model }));
  };

  const handleSave = async () => {
    setSaving(true);
    setResult(null);
    try {
      const r = await settingsApi.updateAI(form);
      if (r.data.test_error) {
        setResult({ ok: false, msg: "配置已保存，但连通性测试失败：" + r.data.test_error });
      } else {
        setResult({ ok: true, msg: "已保存 ✓（配置的模型需支持函数调用以拉取 OKX 数据）" });
      }
      load();
    } catch (e) {
      setResult({ ok: false, msg: (e as Error).message });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4">
      <div className="card w-full max-w-lg p-0 overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between bg-slate-50">
          <h3 className="font-semibold text-slate-800 flex items-center gap-2">
            <Bot size={18} className="text-brand-600" />
            AI 大模型配置
          </h3>
          <button className="text-slate-400 hover:text-slate-600 text-xl leading-none" onClick={onClose}>
            ×
          </button>
        </div>

        <div className="p-6 space-y-4 max-h-[70vh] overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-10 text-slate-400">
              <Loader2 className="animate-spin mr-2" size={18} /> 加载中…
            </div>
          ) : (
            <>
              {/* 快捷预设 */}
              <div>
                <label className="label">快捷选择（可选）</label>
                <div className="flex flex-wrap gap-2">
                  {Object.entries(PRESETS).map(([k, v]) => (
                    <button
                      key={k}
                      onClick={() => applyPreset(k)}
                      className={`px-3 py-1.5 rounded-full text-xs border transition ${
                        preset === k
                          ? "bg-brand-600 text-white border-brand-600"
                          : "bg-white text-slate-600 border-slate-200 hover:border-brand-300"
                      }`}
                    >
                      {v.hint}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <label className="label flex items-center gap-1">
                  <LinkIcon size={13} /> API Base URL
                </label>
                <input
                  className="input font-mono"
                  value={form.base_url}
                  onChange={(e) => setForm((f) => ({ ...f, base_url: e.target.value }))}
                  placeholder="https://api.deepseek.com 或 https://api.openai.com/v1"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="label flex items-center gap-1">
                    <Cpu size={13} /> 模型名
                  </label>
                  <input
                    className="input font-mono"
                    value={form.model}
                    onChange={(e) => setForm((f) => ({ ...f, model: e.target.value }))}
                    placeholder="deepseek-chat"
                  />
                </div>
                <div>
                  <label className="label flex items-center gap-1">
                    <Bot size={13} /> Provider（备注）
                  </label>
                  <input
                    className="input"
                    value={form.provider}
                    onChange={(e) => setForm((f) => ({ ...f, provider: e.target.value }))}
                    placeholder="openai / deepseek"
                  />
                </div>
              </div>

              <div>
                <label className="label flex items-center gap-1">
                  <KeyRound size={13} /> API Key
                </label>
                <input
                  className="input font-mono"
                  type="password"
                  value={form.api_key}
                  onChange={(e) => setForm((f) => ({ ...f, api_key: e.target.value }))}
                  placeholder={cur?.has_key ? `已配置（${cur.api_key}），留空保持不变` : "粘贴 API Key"}
                />
                <p className="text-[11px] text-slate-400 mt-1">
                  保存后 AES 加密入库，界面只显示掩码。支持 OpenAI / DeepSeek / Moonshot / 通义等兼容接口。
                </p>
              </div>

              {result && (
                <div
                  className={`flex items-start gap-2 text-sm px-3 py-2.5 rounded-lg ${
                    result.ok ? "bg-green-50 text-green-700" : "bg-red-50 text-red-600"
                  }`}
                >
                  {result.ok ? <CheckCircle2 size={15} className="mt-0.5" /> : <AlertCircle size={15} className="mt-0.5" />}
                  <span className="flex-1">{result.msg}</span>
                </div>
              )}
            </>
          )}

          <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
            <button className="btn-outline" onClick={onClose}>
              关闭
            </button>
            <button className="btn-primary" onClick={handleSave} disabled={loading || saving}>
              {saving && <Loader2 size={15} className="animate-spin" />}
              保存并测试
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
