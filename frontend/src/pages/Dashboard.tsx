import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Link2, Wallet, Layers, TrendingUp, RefreshCw, AlertTriangle, ChevronRight } from "lucide-react";
import * as echarts from "echarts";
import { dataApi, type PnlSummary, type PnlTaskStatus } from "@/api";
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

          {/* 合约历史盈亏 */}
          <PnlHistoryBlock configId={activeId ?? undefined} />
        </>
      )}
    </div>
  );
}

const palette = ["#1b6ff5", "#10b981", "#f59e0b", "#8b5cf6", "#ec4899", "#14b8a6", "#f43f5e", "#6366f1"];

// ===== 合约历史盈亏区块：每日已实现盈亏(涨红跌绿) + 各交易标的币盈亏 =====
function PnlHistoryBlock({ configId }: { configId?: number }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const [summary, setSummary] = useState<PnlSummary | null>(null);
  const [days, setDays] = useState(90);
  // 当前筛选标的币（BTC/ETH），空 = 全部；可用标的币列表来自全量任务的 byAsset
  const [asset, setAsset] = useState("");
  const [assetOptions, setAssetOptions] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  // 拉取阶段：仅在“完全没有旧图”时整块显示 loading，否则头部小进度
  const [progress, setProgress] = useState<{
    pages: number;
    covered: number;
    elapsedMs: number;
    estRemainSec: number;
  } | null>(null);
  const pollRef = useRef<number | null>(null);
  // 单调递增请求号：用于丢弃过期任务的异步回调（解决接口一直在调用/竞态闪烁）
  const runRef = useRef(0);

  const clearPoll = () => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  // 任务完成回调：写 summary / 错误 / 标的币选项
  const applyResult = (t: PnlTaskStatus, myRun: number) => {
    if (runRef.current !== myRun) return; // 已有更新请求，丢弃过期结果
    clearPoll();
    if (t.error) {
      setError(t.error);
    } else {
      setError("");
      setSummary(t.summary ?? null);
    }
    // 仅全量任务(asset 为空)刷新标的币下拉选项
    if (!t.asset && t.summary) {
      const opts = (t.summary.byAsset || [])
        .map((x) => x.asset)
        .filter((x) => x && x !== "?");
      setAssetOptions(opts.length ? opts : []);
    }
    setLoading(false);
    setProgress(null);
  };

  const startLoad = async (d: number, pick: string) => {
    const myRun = ++runRef.current; // 作废之前的任务与轮询
    clearPoll();
    setError("");
    // 仅当还没有任何图时才整块 loading；否则保留旧图、头部显示进度（避免闪烁）
    setLoading(true);
    setProgress({ pages: 0, covered: 0, elapsedMs: 0, estRemainSec: 1 });
    let taskId = "";
    try {
      const st = await dataApi.pnlDailyStart(configId, d, pick);
      if (runRef.current !== myRun) return; // 期间已切换，丢弃
      taskId = st.data.task_id;
      if (st.data.est_seconds > 0) {
        setProgress({ pages: 0, covered: 0, elapsedMs: 0, estRemainSec: st.data.est_seconds });
      }
    } catch (e) {
      if (runRef.current !== myRun) return;
      setError((e as Error).message);
      setLoading(false);
      setProgress(null);
      return;
    }
    const poll = async (): Promise<boolean> => {
      if (runRef.current !== myRun) return true; // 过期任务，停止
      try {
        const r = await dataApi.pnlDailyStatus(taskId);
        const t = r.data.task;
        if (t.done) {
          applyResult(t, myRun);
          return true;
        }
        if (runRef.current === myRun) {
          setProgress({
            pages: t.pages,
            covered: t.covered_days,
            elapsedMs: t.elapsed_ms,
            estRemainSec: t.est_remain_sec,
          });
        }
      } catch {
        // 单次轮询失败忽略，等待下次
      }
      return false;
    };
    if (await poll()) return; // 首轮即完成
    if (runRef.current !== myRun) return;
    pollRef.current = window.setInterval(() => {
      void poll();
    }, 1000);
  };

  // 天数 / 标的币 / 配置变化 → 重新开始任务
  useEffect(() => {
    void startLoad(days, asset);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [configId, days, asset]);

  // 卸载时清理轮询
  useEffect(() => () => clearPoll(), []);

  // 每日盈亏柱状图（涨=红，跌=绿，A股约定）
  useEffect(() => {
    if (!chartRef.current || !summary || summary.daily.length === 0) return;
    const chart = echarts.init(chartRef.current);
    const d = summary.daily;
    chart.setOption({
      grid: { left: 8, right: 8, top: 24, bottom: 4, containLabel: true },
      tooltip: {
        trigger: "axis",
        backgroundColor: "#fff",
        borderColor: "#e2e8f0",
        textStyle: { color: "#1e293b", fontSize: 12 },
        formatter: (ps: any[]) => {
          const it = ps[0];
          const row = d[it.dataIndex];
          if (!row) return "";
          const sign = (n: number) => (n > 0 ? "+" : "");
          const unit = assetLabel(asset);
          return `<b>${row.date}</b><br/>已实现盈亏: <span style="color:${row.net >= 0 ? "#e11d48" : "#10b981"}">${sign(row.net)}${fmtNum2(row.net)}</span> ${unit}<br/><span style="color:#94a3b8">手续费: ${sign(row.fee)}${fmtNum2(row.fee)}</span>`;
        },
      },
      xAxis: {
        type: "category",
        data: d.map((r) => r.date.slice(5)), // MM-DD
        axisLine: { lineStyle: { color: "#e2e8f0" } },
        axisLabel: { color: "#94a3b8", fontSize: 10 },
      },
      yAxis: {
        type: "value",
        axisLabel: { color: "#94a3b8", fontSize: 10 },
        splitLine: { lineStyle: { color: "#f1f5f9" } },
      },
      series: [
        {
          type: "bar",
          data: d.map((r) => ({
            value: Number(r.net.toFixed(2)),
            itemStyle: { color: r.net >= 0 ? "#ef4444" : "#10b981", borderRadius: [2, 2, 0, 0] },
          })),
          barMaxWidth: 22,
        },
      ],
    });
    const onResize = () => chart.resize();
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      chart.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [summary]);

  const hasData = summary && summary.daily.length > 0;

  return (
    <div className="card overflow-hidden">
      {/* 头部 */}
      <div className="px-5 py-4 border-b border-slate-100 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-slate-700">合约历史盈亏</h3>
          <span className="text-xs text-slate-400">按交易标的币(如 BTC/ETH)的已实现盈亏</span>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {hasData && !loading && (
            <div className="flex items-center gap-3 mr-1 text-xs">
              <span className="text-slate-500">
                {asset ? `${asset} 期间净盈亏` : "全部币 期间净盈亏"}{" "}
                <b className={upDownClass(String(summary!.totalNet))}>{fmtUsd(summary!.totalNet)}</b>
              </span>
              <span className="text-slate-400">手续费 {fmtUsd(summary!.totalFee)}</span>
            </div>
          )}
          {loading && progress && hasData && (
            <span className="inline-flex items-center gap-1.5 text-xs text-blue-500 mr-1">
              <RefreshCw size={11} className="animate-spin" />
              拉取中 · 已 {progress.pages} 页 / 约剩 {progress.estRemainSec}s
            </span>
          )}
          {/* 标的币筛选下拉 */}
          <select
            value={asset}
            onChange={(e) => setAsset(e.target.value)}
            className="text-xs rounded-md border border-slate-200 bg-white text-slate-600 px-1.5 py-1 outline-none focus:border-blue-400"
            title="按交易标的币(BTC/ETH)筛选每日柱状图"
          >
            <option value="">全部币</option>
            {assetOptions.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </select>
          <div className="btn-outline !px-1 !py-0.5 flex overflow-hidden">
            {[30, 90].map((d) => (
              <button
                key={d}
                className={`px-2.5 py-1 text-xs rounded transition ${
                  days === d ? "bg-slate-200 text-slate-800" : "text-slate-500"
                }`}
                onClick={() => setDays(d)}
              >
                近{d}天
              </button>
            ))}
          </div>
          <button
            className="btn-outline"
            onClick={() => void startLoad(days, asset)}
            disabled={loading}
            title="重新拉取账单"
          >
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} />
          </button>
        </div>
      </div>

      {!hasData && loading ? (
        <div className="py-10 text-center">
          <RefreshCw size={18} className="animate-spin inline-block text-blue-400" />
          <p className="text-sm text-slate-500 mt-2">
            {asset ? `正在拉取 ${asset} 合约账单流水…` : "正在拉取合约账单流水…"}
          </p>
          {progress && (
            <div className="text-xs text-slate-400 mt-2 space-y-1">
              <p>
                已拉取 <b className="text-slate-600">{progress.pages}</b> 页
                {progress.covered > 0 && (
                  <>
                    {" "}· 缓存已覆盖近 <b className="text-slate-600">{progress.covered}</b> 天
                  </>
                )}
              </p>
              <p>
                已用 <b className="text-slate-600">{(progress.elapsedMs / 1000).toFixed(1)}s</b>
                {" "}· 预计还需约 <b className="text-blue-500">{progress.estRemainSec}s</b>
              </p>
              <p className="text-slate-300">
                为规避 OKX 限流(50011)，逐页拉取并自动重试；账单较多时请耐心等待
              </p>
            </div>
          )}
        </div>
      ) : error ? (
        <div className="flex items-start gap-2 bg-red-50 border border-red-200 text-red-600 text-xs rounded-lg px-4 py-3 m-4">
          <AlertTriangle size={14} className="mt-0.5 shrink-0" />
          <div>
            {error}
            <p className="text-red-400 mt-1">需 API Key 开启「读取」权限；通常近几天无平仓交易时显示为空属正常。</p>
          </div>
        </div>
      ) : !hasData ? (
        <div className="py-10 text-center">
          <p className="text-sm text-slate-500">近 {days} 天暂无合约已实现盈亏记录</p>
          <p className="text-xs text-slate-400 mt-1">
            当产生平仓/强平/交割或资金费后，这里会展示每日盈亏与各交易标的币盈亏
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-0">
          {/* 左侧每日盈亏柱状图（占3列） */}
          <div className="lg:col-span-3 p-4 border-b lg:border-b-0 lg:border-r border-slate-100">
            <div ref={chartRef} style={{ height: 220, width: "100%" }} />
          </div>
          {/* 右侧各标的币盈亏（占2列） */}
          <div className="lg:col-span-2 p-4">
            <div className="text-xs text-slate-400 mb-2">
              {asset ? `已筛选：${asset}` : "按标的币(BTC/ETH)盈亏（点币看该币每日柱状图）"}
            </div>
            <table className="w-full text-sm">
              <tbody>
                {summary!.byAsset.slice(0, 10).map((c) => (
                  <tr
                    key={c.asset}
                    className={`border-b border-slate-50 last:border-0 cursor-pointer transition hover:bg-blue-50/60 ${
                      asset === c.asset ? "bg-blue-50" : ""
                    }`}
                    onClick={() => setAsset(asset === c.asset ? "" : c.asset)}
                    title="查看该币的每日盈亏"
                  >
                    <td className="py-1.5 font-medium text-slate-600 flex items-center gap-1.5">
                      {c.asset}
                      {asset !== c.asset && (
                        <span className="text-slate-300 hover:text-blue-500">
                          <ChevronRight size={12} />
                        </span>
                      )}
                    </td>
                    <td className={`py-1.5 text-right font-medium ${upDownClass(String(c.net))}`}>
                      {fmtUsd(c.net)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {summary!.byAsset.length === 0 && (
              <p className="text-xs text-slate-400">该筛选下暂无明细</p>
            )}
            {asset && (
              <button
                className="mt-2 text-[11px] text-blue-500 hover:text-blue-700"
                onClick={() => setAsset("")}
              >
                ← 返回全部币
              </button>
            )}
            <p className="text-[11px] text-slate-300 mt-2 leading-relaxed">
              同一币不同合约(永续/交割)的已实现盈亏按标的币汇总，数值以 USDT 计。
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

function fmtNum2(n: number): string {
  if (Math.abs(n) >= 1000) return n.toFixed(0);
  if (Math.abs(n) >= 1) return n.toFixed(2);
  return n.toFixed(4);
}

// 柱状图 tooltip 的单位：筛选到单币显示该币，否则显示 USDT 等值
function assetLabel(asset: string): string {
  return asset ? `${asset} 的 USDT 盈亏` : "USDT 等值";
}

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
