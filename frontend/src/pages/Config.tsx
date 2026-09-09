import { useState } from "react";
import {
  Plus,
  Trash2,
  Pencil,
  TestTube2,
  Loader2,
  KeyRound,
  Globe,
  Star,
} from "lucide-react";
import { configApi } from "@/api";
import type { OkxConfig } from "@/api/types";
import { useConfigStore } from "@/store/config";

export default function ConfigPage() {
  const { list, fetch, setActive, activeId } = useConfigStore();
  const [editing, setEditing] = useState<OkxConfig | null>(null);
  const [showForm, setShowForm] = useState(false);

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-slate-700 font-medium">已保存的 OKX 接口配置</h3>
          <p className="text-xs text-slate-400 mt-1">
            密钥经 AES 加密后存入本地 SQLite，任何接口都不返回明文密钥
          </p>
        </div>
        <button
          className="btn-primary"
          onClick={() => {
            setEditing(null);
            setShowForm(true);
          }}
        >
          <Plus size={16} />
          新增配置
        </button>
      </div>

      {list.length === 0 && !showForm ? (
        <div className="card flex flex-col items-center py-16 text-center">
          <div className="h-14 w-14 rounded-2xl bg-slate-100 flex items-center justify-center mb-4">
            <KeyRound size={26} className="text-slate-400" />
          </div>
          <h3 className="text-lg font-semibold text-slate-700">暂无配置</h3>
          <p className="text-sm text-slate-400 mt-1 max-w-sm">
            点击右上角「新增配置」，填入 OKX 的 api_key、secret 和 passphrase
            （在 OKX 账户 → API 中创建）。域名默认官方，也可填模拟盘地址。
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {list.map((c) => (
            <div key={c.id} className="card p-5 space-y-3">
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <h4 className="font-semibold text-slate-800">
                      {c.is_default && <Star size={14} className="inline text-amber-400 mb-0.5 mr-1" />}
                      {c.name || `配置 ${c.id}`}
                    </h4>
                    {c.is_default && (
                      <span className="px-2 py-0.5 rounded text-[11px] bg-amber-50 text-amber-600 font-medium">
                        默认
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-slate-400 mt-1 flex items-center gap-1">
                    <Globe size={12} /> {c.base_url}
                  </p>
                </div>
                <span className="px-2 py-1 rounded text-[11px] bg-emerald-50 text-emerald-600 font-medium">
                  api_key: {c.api_key}
                </span>
              </div>

              {c.description && (
                <p className="text-xs text-slate-500 bg-slate-50 rounded-lg px-3 py-2">
                  {c.description}
                </p>
              )}

              <div className="flex items-center gap-2 pt-1 border-t border-slate-100">
                <button
                  className="btn-ghost !px-2.5 !py-1.5 text-xs"
                  onClick={() => {
                    setEditing(c);
                    setShowForm(true);
                  }}
                >
                  <Pencil size={13} /> 编辑
                </button>
                <button
                  className="btn-ghost !px-2.5 !py-1.5 text-xs"
                  onClick={async () => {
                    try {
                      const r = await configApi.test(c.id);
                      alert(`✅ ${r.data.message}${r.data.total_eq_usd ? `，总权益 $${r.data.total_eq_usd}` : ""}`);
                    } catch (e) {
                      alert(`❌ 测试失败：${(e as Error).message}`);
                    }
                  }}
                >
                  <TestTube2 size={13} /> 测试连接
                </button>
                {activeId === c.id && (
                  <span className="text-[11px] text-brand-600">当前使用中</span>
                )}
                <div className="flex-1" />
                <button
                  className="btn-ghost !px-2.5 !py-1.5 text-xs text-red-500 hover:bg-red-50"
                  onClick={async () => {
                    if (!confirm(`确定删除配置「${c.name || c.id}」？`)) return;
                    await configApi.remove(c.id);
                    if (activeId === c.id) setActive(null);
                    fetch();
                  }}
                >
                  <Trash2 size={13} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {showForm && (
        <ConfigForm
          key={editing?.id || "new"}
          editing={editing}
          onClose={() => {
            setShowForm(false);
            setEditing(null);
          }}
          onSaved={() => {
            setShowForm(false);
            setEditing(null);
            fetch();
          }}
        />
      )}
    </div>
  );
}

function ConfigForm({
  editing,
  onClose,
  onSaved,
}: {
  editing: OkxConfig | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [form, setForm] = useState({
    name: editing?.name || "",
    api_key: "",
    api_secret: "",
    passphrase: "",
    base_url: editing?.base_url || "https://www.okx.com",
    is_default: editing?.is_default || false,
    description: editing?.description || "",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const set = (k: keyof typeof form, v: string | boolean) =>
    setForm((f) => ({ ...f, [k]: v }));

  const handleSave = async () => {
    setError("");
    // 编辑模式允许不填密钥（保持不变）
    if (!editing && (!form.api_key || !form.api_secret || !form.passphrase)) {
      setError("api_key / api_secret / passphrase 均为必填");
      return;
    }
    setLoading(true);
    try {
      if (editing) {
        await configApi.update(editing.id, form);
      } else {
        const r = await configApi.create(form);
        const uid = r.data.config?.id;
        if (uid) {
          // 若列表为空则自动设为当前
        }
      }
      onSaved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4">
      <div className="card w-full max-w-lg p-0 overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between bg-slate-50">
          <h3 className="font-semibold text-slate-800">
            {editing ? "编辑配置" : "新增 OKX 配置"}
          </h3>
          <button className="text-slate-400 hover:text-slate-600 text-xl leading-none" onClick={onClose}>
            ×
          </button>
        </div>

        <div className="p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="label">配置名称</label>
              <input
                className="input"
                value={form.name}
                onChange={(e) => set("name", e.target.value)}
                placeholder="如：我的主账户"
              />
            </div>
            <div>
              <label className="label">默认使用</label>
              <button
                type="button"
                onClick={() => set("is_default", !form.is_default)}
                className={`w-full flex items-center justify-between rounded-lg border px-3 py-2 text-sm ${
                  form.is_default
                    ? "border-amber-300 bg-amber-50 text-amber-700"
                    : "border-slate-300 bg-white text-slate-500"
                }`}
              >
                <span className="flex items-center gap-2">
                  <Star size={14} className={form.is_default ? "text-amber-400" : ""} />
                  设为默认账户
                </span>
                <span
                  className={`h-4 w-7 rounded-full transition relative ${
                    form.is_default ? "bg-amber-400" : "bg-slate-300"
                  }`}
                >
                  <span
                    className={`absolute top-0.5 h-3 w-3 rounded-full bg-white transition-all ${
                      form.is_default ? "left-3.5" : "left-0.5"
                    }`}
                  />
                </span>
              </button>
            </div>
          </div>

          <div>
            <label className="label">
              API 域名 <span className="text-slate-400 font-normal">（留空默认官方，可填模拟盘/代理）</span>
            </label>
            <input
              className="input"
              value={form.base_url}
              onChange={(e) => set("base_url", e.target.value)}
              placeholder="https://www.okx.com"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <label className="label">API Key</label>
              <input
                className="input font-mono"
                value={form.api_key}
                onChange={(e) => set("api_key", e.target.value)}
                placeholder={editing ? "留空表示不修改" : "粘贴你的 api_key"}
              />
            </div>
            <div>
              <label className="label">API Secret</label>
              <input
                className="input font-mono"
                type="password"
                value={form.api_secret}
                onChange={(e) => set("api_secret", e.target.value)}
                placeholder={editing ? "留空表示不修改" : "粘贴 secret"}
              />
            </div>
            <div>
              <label className="label">Passphrase</label>
              <input
                className="input font-mono"
                type="password"
                value={form.passphrase}
                onChange={(e) => set("passphrase", e.target.value)}
                placeholder={editing ? "留空表示不修改" : "API 创建时设置的 passphrase"}
              />
            </div>
          </div>

          <div>
            <label className="label">备注</label>
            <textarea
              className="input resize-none"
              rows={2}
              value={form.description}
              onChange={(e) => set("description", e.target.value)}
              placeholder="可选，说明用途"
            />
          </div>

          {error && (
            <div className="bg-red-50 text-red-600 text-sm rounded-lg px-3 py-2">{error}</div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <button className="btn-outline" onClick={onClose}>
              取消
            </button>
            <button className="btn-primary" onClick={handleSave} disabled={loading}>
              {loading && <Loader2 size={15} className="animate-spin" />}
              保存
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
