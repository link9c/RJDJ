import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Link2, Wallet, Layers, TrendingUp, RefreshCw, AlertTriangle } from "lucide-react";
import * as echarts from "echarts";
import { dataApi } from "@/api";
import type { Overview } from "@/api/types";
import { useConfigStore } from "@/store/config";
import { fmtUsd, fmtNum, fmtPct, upDownClass, instName, positionTotal } from "@/utils/format";

export default function DashboardPage() {
  const navigate = useNavigate();
  const { activeId, list } = useConfigStore();
  const [data, setData] = useState<Overview | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const chartRef = useRef<HTMLDivElement>(null);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const resp = await dataApi.overview(activeId ?? undefined);
      setData(resp.data);
      setLastUpdate(new Date());
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
  }, [activeId, list.length]);

  // 渲染资产占比饼图
  useEffect(() => {
    if (!data || !chartRef.current) return;
    const chart = echarts.init(chartRef.current);
    const details = (data.balance?.details || [])
      .filter((d) => Number(d.eqUsd) > 0)
      .slice(0, 8);
    chart.setOption({
      tooltip: { trigger: "item", formatter: "{b}: ${c} ({d}%)" },
      legend: {
        orient: "vertical",
        right: 8,
        top: "middle",
        textStyle: { fontSize: 12, color: "#475569" },
        formatter: (name: string) => {
          const d = details.find((x) => x.ccy === name);
          return name + (d ? `  ${fmtUsd(d.eqUsd)}` : "");
        },
      },
      series: [
        {
          type: "pie",
          radius: ["45%", "72%"],
          center: ["38%", "50%"],
          avoidLabelOverlap: false,
          itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 2 },
          label: { show: false },
          emphasis: { label: { show: false } },
          data: details.map((d, i) => ({
            name: d.ccy,
            value: Number(d.eqUsd),
            itemStyle: { color: palette[i % palette.length] },
          })),
        },
      ],
    });
    const onResize = () => chart.resize();
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      chart.dispose();
    };
  }, [data]);

  const coinRows = useMemo(() => {
    if (!data) return [];
    return (data.balance?.details || [])
      .slice()
      .sort((a, b) => Number(b.eqUsd) - Number(a.eqUsd));
  }, [data]);

  const pos = useMemo(() => {
    if (!data) return { list: [], summary: positionTotal([]) };
    const list = data.positions || [];
    return { list, summary: positionTotal(list) };
  }, [data]);

  const noConfig = list.length === 0;

  return (
    <div className="space-y-5">
      {/* 顶部提示/刷新条 */}
      <div className="flex items-center justify-between">
        <div className="text-sm text-slate-500">
          {noConfig ? (
            <span className="flex items-center gap-2 text-amber-600">
              <AlertTriangle size={16} />
              尚未配置 OKX 密钥，无法获取资产数据
            </span>
          ) : lastUpdate ? (
            <span>
              最近更新：<span className="text-slate-700 font-medium">{lastUpdate.toLocaleTimeString()}</span>
            </span>
          ) : (
            <span>正在加载…</span>
          )}
        </div>
        <div className="flex gap-2">
          {noConfig && (
            <button className="btn-primary" onClick={() => navigate("/config")}>
              <Link2 size={15} />
              去配置 OKX 密钥
            </button>
          )}
          <button className="btn-outline" onClick={load} disabled={loading || noConfig}>
            <RefreshCw size={15} className={loading ? "animate-spin" : ""} />
            刷新数据
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-600 text-sm rounded-lg px-4 py-3">
          {error}
        </div>
      )}

      {noConfig ? (
        <EmptyState />
      ) : !data ? (
        <SkeletonCards />
      ) : (
        <>
          {/* 统计卡片 */}
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
            <StatCard
              title="账户总权益 (USD)"
              value={fmtUsd(data.balance?.totalEq)}
              icon={<Wallet size={20} className="text-brand-600" />}
              iconBg="bg-brand-50"
            />
            <StatCard
              title="持仓数量"
              value={`${pos.summary.count} 个`}
              sub={`合约账户保证金 ≈ ${fmtUsd(pos.summary.totalMargin)}`}
              icon={<Layers size={20} className="text-indigo-600" />}
              iconBg="bg-indigo-50"
            />
            <StatCard
              title="未实现盈亏"
              value={fmtUsd(pos.summary.totalUpl)}
              valueClass={upDownClass(pos.summary.totalUpl)}
              sub="当前全部持仓浮动盈亏"
              icon={<TrendingUp size={20} className={upDownClass(pos.summary.totalUpl)} />}
              iconBg="bg-slate-100"
            />
            <StatCard
              title="非零资产币种"
              value={`${coinRows.filter((c) => Number(c.eqUsd)).length} 种`}
              sub="按等值 USD 排序"
              icon={<Wallet size={20} className="text-emerald-600" />}
              iconBg="bg-emerald-50"
            />
          </div>

          {/* 图表 + 持仓 */}
          <div className="grid grid-cols-1 xl:grid-cols-5 gap-4">
            <div className="card p-5 xl:col-span-2">
              <h3 className="text-sm font-semibold text-slate-700 mb-1">资产配置</h3>
              <p className="text-xs text-slate-400 mb-2">按币种折合 USD 占比</p>
              <div ref={chartRef} className="h-72" />
            </div>

            <div className="card overflow-hidden xl:col-span-3">
              <div className="flex items-center justify-between px-5 py-4 border-b border-slate-100">
                <h3 className="text-sm font-semibold text-slate-700">当前持仓</h3>
                <span className="text-xs text-slate-400">
                  未实现盈亏总计{" "}
                  <span className={`font-semibold ${upDownClass(pos.summary.totalUpl)}`}>
                    {fmtUsd(pos.summary.totalUpl)}
                  </span>
                </span>
              </div>
              {pos.list.length === 0 ? (
                <div className="text-center py-12 text-slate-400 text-sm">
                  暂无持仓
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr>
                        <th className="th">产品</th>
                        <th className="th">方向</th>
                        <th className="th">持仓量</th>
                        <th className="th">开仓均价</th>
                        <th className="th">标记价</th>
                        <th className="th">未实现盈亏</th>
                        <th className="th">盈亏率</th>
                        <th className="th">杠杆</th>
                      </tr>
                    </thead>
                    <tbody>
                      {pos.list.map((p, i) => (
                        <tr key={i} className="border-t border-slate-100 hover:bg-slate-50">
                          <td className="td font-medium">{p.instId}</td>
                          <td className="td">
                            <span
                              className={`px-2 py-0.5 rounded text-xs font-medium ${
                                p.posSide === "long" || p.pos.startsWith("-")
                                  ? "bg-red-50 text-red-600"
                                  : "bg-green-50 text-green-600"
                              }`}
                            >
                              {p.posSide === "long" || p.pos.startsWith("-") ? "多" : "空"}
                            </span>
                          </td>
                          <td className="td">{fmtNum(p.pos)}</td>
                          <td className="td">{fmtNum(p.avgPx)}</td>
                          <td className="td">{fmtNum(p.markPx)}</td>
                          <td className={`td font-medium ${upDownClass(p.upl)}`}>
                            {fmtUsd(p.upl)}
                          </td>
                          <td className={`td ${upDownClass(p.uplRatio)}`}>
                            {fmtPct(p.uplRatio)}
                          </td>
                          <td className="td">{p.lever}×</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>

          {/* 币种余额明细 */}
          <div className="card overflow-hidden">
            <div className="px-5 py-4 border-b border-slate-100 flex items-center justify-between">
              <h3 className="text-sm font-semibold text-slate-700">各币种余额</h3>
              <span className="text-xs text-slate-400">
                币种权益 / 可用 / 冻结 / 折合美元
              </span>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr>
                    <th className="th">币种</th>
                    <th className="th">权益 (eq)</th>
                    <th className="th">可用 (availBal)</th>
                    <th className="th">冻结</th>
                    <th className="th">折合 USD</th>
                    <th className="th">占比</th>
                  </tr>
                </thead>
                <tbody>
                  {coinRows.map((c, i) => {
                    const totalEqUsd = Number(data.balance?.totalEq || 0);
                    const usd = Number(c.eqUsd || 0);
                    const pct = totalEqUsd > 0 ? (usd / totalEqUsd) * 100 : 0;
                    return (
                      <tr key={c.ccy} className="border-t border-slate-100 hover:bg-slate-50">
                        <td className="td font-medium">
                          <div className="flex items-center gap-2">
                            <span
                              className="h-2 w-2 rounded-full"
                              style={{ background: palette[i % palette.length] }}
                            />
                            {c.ccy}
                          </div>
                        </td>
                        <td className="td">{fmtNum(c.eq)}</td>
                        <td className="td">{fmtNum(c.availBal)}</td>
                        <td className="td">{fmtNum(c.frozenBal)}</td>
                        <td className="td font-medium">{fmtUsd(c.eqUsd)}</td>
                        <td className="td">
                          <div className="flex items-center gap-2">
                            <div className="w-20 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                              <div
                                className="h-full bg-brand-500 rounded-full"
                                style={{ width: `${Math.min(pct, 100)}%` }}
                              />
                            </div>
                            <span className="text-xs text-slate-500">{pct.toFixed(1)}%</span>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

const palette = ["#1b6ff5", "#10b981", "#f59e0b", "#8b5cf6", "#ec4899", "#14b8a6", "#f43f5e", "#6366f1"];

function StatCard({
  title,
  value,
  sub,
  icon,
  iconBg,
  valueClass,
}: {
  title: string;
  value: string;
  sub?: string;
  icon: React.ReactNode;
  iconBg?: string;
  valueClass?: string;
}) {
  return (
    <div className="card p-5 flex items-start justify-between">
      <div>
        <p className="text-xs text-slate-500">{title}</p>
        <p className={`mt-2 text-2xl font-bold text-slate-800 ${valueClass || ""}`}>
          {value}
        </p>
        {sub && <p className="mt-1 text-xs text-slate-400">{sub}</p>}
      </div>
      <div className={`${iconBg || "bg-slate-100"} rounded-xl p-2.5`}>{icon}</div>
    </div>
  );
}

function EmptyState() {
  const navigate = useNavigate();
  return (
    <div className="card flex flex-col items-center justify-center py-20 text-center">
      <div className="h-14 w-14 rounded-2xl bg-slate-100 flex items-center justify-center mb-4">
        <Link2 size={26} className="text-slate-400" />
      </div>
      <h3 className="text-lg font-semibold text-slate-700">还没有配置 OKX API</h3>
      <p className="text-sm text-slate-400 mt-1 max-w-sm">
        前往「OKX 配置」页面填入你的 api_key / secret / passphrase
        （AES 加密存储，不会明文展示），即可在此查看账户资产与持仓。
      </p>
      <button className="btn-primary mt-5" onClick={() => navigate("/config")}>
        立即配置
      </button>
    </div>
  );
}

function SkeletonCards() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 animate-pulse">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="card p-5 h-28">
          <div className="h-3 w-20 bg-slate-200 rounded" />
          <div className="h-7 w-32 bg-slate-200 rounded mt-4" />
        </div>
      ))}
    </div>
  );
}
