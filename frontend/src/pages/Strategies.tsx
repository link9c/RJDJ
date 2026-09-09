import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Bot, RefreshCw, AlertTriangle, Grid3x3, Repeat, Radio, Zap, Layers, TrendingUp } from "lucide-react";
import { dataApi } from "@/api";
import type { BotStrategy } from "@/api";
import { useConfigStore } from "@/store/config";
import { fmtNum, fmtUsd, upDownClass, fmtPct } from "@/utils/format";

const KIND_META: Record<string, { label: string; icon: any; color: string; bg: string }> = {
  all: { label: "全部策略", icon: Layers, color: "text-brand-600", bg: "bg-brand-50" },
  dca: { label: "马丁格尔 DCA", icon: TrendingUp, color: "text-rose-600", bg: "bg-rose-50" },
  grid: { label: "网格", icon: Grid3x3, color: "text-indigo-600", bg: "bg-indigo-50" },
  recurring: { label: "定投", icon: Repeat, color: "text-amber-600", bg: "bg-amber-50" },
  signal: { label: "信号", icon: Radio, color: "text-purple-600", bg: "bg-purple-50" },
  algo: { label: "条件单/止盈止损", icon: Zap, color: "text-emerald-600", bg: "bg-emerald-50" },
};

export default function StrategiesPage() {
  const navigate = useNavigate();
  const { list, activeId } = useConfigStore();
  const [kind, setKind] = useState("dca"); // 默认聚焦马丁格尔(DCA)，用户最常用
  const [history, setHistory] = useState(false);
  const [data, setData] = useState<Record<string, BotStrategy[]> | null>(null);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const noConfig = list.length === 0;

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const r = await dataApi.strategies(activeId ?? undefined, kind, history);
      setData(r.data.list);
      setTotal(r.data.total);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (list.length === 0) return;
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId, list.length, kind, history]);

  const kinds = useMemo(() => {
    if (!data) return [];
    return Object.entries(KIND_META).filter(
      ([k]) => k === "all" || (data[k] && data[k]!.length > 0)
    );
  }, [data, kind]);

  return (
    <div className="space-y-4">
      {/* 标题操作条 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="px-2.5 py-1 rounded-lg bg-slate-900 text-white text-sm flex items-center gap-1.5">
            <Bot size={16} />
            {KIND_META[kind]?.label}
            {!noConfig && <span className="text-xs text-slate-400">共 {total} 个</span>}
          </span>
        </div>
        <div className="flex gap-2">
          <div className="btn-outline !px-1 !py-0.5 flex overflow-hidden">
            <button
              className={`px-3 py-1.5 text-xs rounded transition ${!history ? "bg-slate-200 text-slate-800" : "text-slate-500"}`}
              onClick={() => setHistory(false)}
            >
              运行中
            </button>
            <button
              className={`px-3 py-1.5 text-xs rounded transition ${history ? "bg-slate-200 text-slate-800" : "text-slate-500"}`}
              onClick={() => setHistory(true)}
            >
              历史
            </button>
          </div>
          <button className="btn-outline" onClick={load} disabled={loading || noConfig}>
            <RefreshCw size={14} className={loading ? "animate-spin" : ""} />
            刷新
          </button>
        </div>
      </div>

      {/* 分类 tab */}
      <div className="flex flex-wrap gap-2">
        {Object.entries(KIND_META).map(([k, meta]) => {
          const Icon = meta.icon;
          return (
            <button
              key={k}
              onClick={() => setKind(k)}
              className={`flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-sm border transition ${
                kind === k
                  ? `${meta.bg} ${meta.color} border-transparent font-medium`
                  : "bg-white text-slate-500 border-slate-200 hover:border-slate-300"
              }`}
            >
              <Icon size={15} />
              {meta.label}
              {!noConfig && data && data[k] && k !== "all" && (
                <span className="text-xs opacity-60">{data[k]!.length}</span>
              )}
            </button>
          );
        })}
      </div>

      {noConfig ? (
        <div className="card flex flex-col items-center py-16 text-center">
          <div className="h-14 w-14 rounded-2xl bg-slate-100 flex items-center justify-center mb-4">
            <Bot size={26} className="text-slate-400" />
          </div>
          <h3 className="text-lg font-semibold text-slate-700">请先配置 OKX API Key</h3>
          <p className="text-sm text-slate-400 mt-1">配置后即可查看你的马丁格尔(DCA) / 网格 / 定投 / 信号 / 条件单等策略</p>
          <button className="btn-primary mt-5" onClick={() => navigate("/config")}>
            去配置
          </button>
        </div>
      ) : error ? (
        <div className="flex items-start gap-2 bg-red-50 border border-red-200 text-red-600 text-sm rounded-lg px-4 py-3">
          <AlertTriangle size={16} className="mt-0.5" />
          <div>
            {error}
            <p className="text-xs text-red-400 mt-1">
              提示：需在 OKX 为该 API Key 开启「交易」权限；若某类策略返回为空，通常是该类型未创建或账号所在区域不支持该策略的 API 查询。
            </p>
          </div>
        </div>
      ) : !data ? (
        <div className="text-center py-16 text-slate-400">加载中…</div>
      ) : Object.values(data).every((l) => l.length === 0) ? (
        <div className="card flex flex-col items-center py-16 text-center">
          <Bot size={28} className="text-slate-300 mb-3" />
          <h3 className="text-slate-700 font-medium">
            {history ? "暂无历史策略" : "暂无运行中的策略"}
          </h3>
          <p className="text-xs text-slate-400 mt-1">
            如果看不到你创建的网格 / 马丁策略，可能是该类型暂未开放 API 查询，或 API Key 无交易权限
          </p>
        </div>
      ) : (
        <StrategyTable groups={data} activeKind={kind} history={history} />
      )}
    </div>
  );
}

function StrategyTable({
  groups,
  activeKind,
  history,
}: {
  groups: Record<string, BotStrategy[]>;
  activeKind: string;
  history: boolean;
}) {
  const groupsToShow = Object.entries(groups).filter(([k, v]) => {
    if (activeKind === "all") return v.length > 0;
    return k === activeKind;
  });

  if (groupsToShow.length === 0) return null;

  return (
    <div className="space-y-4">
      {groupsToShow.map(([kind, items]) => (
        <div key={kind} className="card overflow-hidden">
          <div className="px-5 py-3 border-b border-slate-100 bg-slate-50 flex items-center gap-2">
            <span className="text-sm font-semibold text-slate-700">{KIND_META[kind]?.label}</span>
            <span className="text-xs text-slate-400">{history ? "历史" : "运行中"} {items.length} 个</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr>
                  <th className="th">产品</th>
                  <th className="th">策略类型</th>
                  <th className="th">状态</th>
                  <th className="th">方向/参数</th>
                  <th className="th">投入</th>
                  <th className="th">浮动/累计收益</th>
                  <th className="th">创建时间</th>
                </tr>
              </thead>
              <tbody>
                {items.map((s, i) => (
                  <tr key={i} className="border-t border-slate-100 hover:bg-slate-50">
                    <td className="td font-medium">
                      <span>{s.instId || s.instType || "-"}</span>
                      {s.instType && INST_LABEL[s.instType] && (
                        <span className="ml-1.5 px-1.5 py-0.5 rounded text-[10px] bg-slate-100 text-slate-500 align-middle">
                          {INST_LABEL[s.instType]}
                        </span>
                      )}
                    </td>
                    <td className="td">
                      <span className="px-2 py-0.5 rounded text-xs bg-slate-100 text-slate-600">
                        {algoTypeLabel(s)}
                      </span>
                    </td>
                    <td className="td">
                      <StateBadge state={s.state} />
                    </td>
                    <td className="td text-xs text-slate-500 max-w-[200px] truncate">
                      {directionLabel(s)} {paramSummary(s)}
                    </td>
                    <td className="td">{fmtUsd(s.investAmt || s.quoteSz)}</td>
                    <td className={`td font-medium ${upDownClass(s.totalPnl || s.upl || s.closePnl)}`}>
                      {s.totalPnl || s.upl || s.closePnl
                        ? fmtUsd(s.totalPnl || s.upl || s.closePnl)
                        : "-"}
                    </td>
                    <td className="td text-xs text-slate-400">{fmtTime(s.cTime)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ))}
    </div>
  );
}

function StateBadge({ state }: { state?: string }) {
  const map: Record<string, { t: string; c: string }> = {
    live: { t: "运行中", c: "bg-emerald-50 text-emerald-600" },
    running: { t: "运行中", c: "bg-emerald-50 text-emerald-600" },
    effective: { t: "已生效", c: "bg-blue-50 text-blue-600" },
    paused: { t: "已暂停", c: "bg-amber-50 text-amber-600" },
    canceled: { t: "已撤销", c: "bg-slate-100 text-slate-500" },
    stop: { t: "已停止", c: "bg-slate-100 text-slate-500" },
    filled: { t: "已成交", c: "bg-indigo-50 text-indigo-600" },
  };
  const m = map[state || ""] || { t: state || "未知", c: "bg-slate-100 text-slate-500" };
  return <span className={`px-2 py-0.5 rounded text-xs font-medium ${m.c}`}>{m.t}</span>;
}

function directionLabel(s: BotStrategy): string {
  if (!s.side && !s.direction) return "";
  const d = (s.direction || s.side || "").toLowerCase();
  if (d.includes("long") || d === "buy") return "🟢 做多 ";
  if (d.includes("short") || d === "sell") return "🔴 做空 ";
  if (d === "neutral") return "⚪ 中性 ";
  return "";
}

// algoOrdType / instType 友好映射（OKX「策略」里建的类型）
const ALGO_LABEL: Record<string, string> = {
  grid: "现货网格",
  contract_grid: "合约网格",
  spot_dca: "现货马丁格尔",
  contract_dca: "合约马丁格尔",
  recurring: "定投",
  conditional: "条件单",
  oco: "OCO",
  trailing: "移动止盈",
  move_order_stop: "移动止损",
  iceberg: "冰山委托",
  twap: "TWAP",
  signal: "信号",
  contract: "信号(合约)",
};
function algoTypeLabel(s: BotStrategy): string {
  const key = s.algoOrdType || s.algoClOrdType || s.type || "";
  return ALGO_LABEL[key] || key || "-";
}
const INST_LABEL: Record<string, string> = {
  SPOT: "现货",
  SWAP: "永续合约",
  FUTURES: "交割合约",
  OPTION: "期权",
};

function paramSummary(s: BotStrategy): string {
  const parts: string[] = [];
  if (s.gridNum) parts.push(`网格 ${s.gridNum}`);
  if (s.minPx && s.maxPx) parts.push(`${fmtNum(s.minPx)}–${fmtNum(s.maxPx)}`);
  if (s.baseSz && s.algoOrdType?.includes("dca")) parts.push(`首单 ${s.baseSz}`);
  if (s.safetyOrderSz && s.algoOrdType?.includes("dca"))
    parts.push(`补仓 ${s.safetyOrderSz}×${s.maxSafetyOrders || "-"}`);
  if (s.lever) parts.push(`${s.lever}×`);
  if (s.triggerPx) parts.push(`触发 ${fmtNum(s.triggerPx)}`);
  return parts.join(" · ");
}

function fmtTime(ts?: string): string {
  if (!ts) return "-";
  const n = Number(ts);
  if (!n) return ts;
  const d = new Date(n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}
