import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  ArrowLeft,
  RefreshCw,
  AlertTriangle,
  TrendingUp,
  TrendingDown,
  Calculator,
  Wallet,
  Info,
} from "lucide-react";
import { dataApi } from "@/api";
import type { StrategyDetail } from "@/api";
import { useConfigStore } from "@/store/config";
import { fmtNum, fmtUsd, upDownClass, fmtPct } from "@/utils/format";

const INST_LABEL: Record<string, string> = {
  SPOT: "现货",
  SWAP: "永续合约",
  FUTURES: "交割合约",
  OPTION: "期权",
};

const ALGO_LABEL: Record<string, string> = {
  grid: "现货网格",
  contract_grid: "合约网格",
  spot_dca: "现货马丁格尔",
  contract_dca: "合约马丁格尔",
  recurring: "定投",
  signal: "信号",
  contract: "信号(合约)",
};

// 计算器输入（全部字符串，受控输入）
interface CalcInput {
  direction: "long" | "short";
  isContract: boolean;
  costPx: string;
  tpPx: string;
  currentPx: string;
  contracts: string; // 合约张数
  coin: string; // 现货持币量
  ctVal: string; // 合约面值
}

interface CalcResult {
  posCoin: number;
  costValue: number;
  currentValue: number;
  currentFloat: number;
  tpReturnValue: number;
  tpProfit: number;
  tpRatio: number;
}

function num(s: string): number {
  const n = Number(s);
  return Number.isFinite(n) ? n : 0;
}

// 与后端 BuildDetailCalc 同口径
function compute(inp: CalcInput): CalcResult {
  const posCoin = inp.isContract
    ? num(inp.contracts) * num(inp.ctVal)
    : num(inp.coin);
  const sign = inp.direction === "short" ? -1 : 1;
  const costPx = num(inp.costPx);
  const tpPx = num(inp.tpPx);
  const curPx = num(inp.currentPx);
  const costValue = costPx * posCoin;
  return {
    posCoin,
    costValue,
    currentValue: curPx * posCoin,
    currentFloat: (curPx - costPx) * posCoin * sign,
    tpReturnValue: tpPx * posCoin,
    tpProfit: (tpPx - costPx) * posCoin * sign,
    tpRatio: costValue > 0 ? ((tpPx - costPx) * sign) / costPx : 0,
  };
}

export default function StrategyDetailPage() {
  const navigate = useNavigate();
  const [sp] = useSearchParams();
  const algoId = sp.get("algoId") || "";
  const kind = sp.get("kind") || "dca";
  const history = sp.get("history") === "1";
  const instType = sp.get("instType") || "";

  const { list, activeId } = useConfigStore();
  const [detail, setDetail] = useState<StrategyDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const noConfig = list.length === 0;

  const load = async () => {
    if (!algoId) return;
    setLoading(true);
    setError("");
    try {
      const r = await dataApi.strategyDetail({
        algoId,
        kind,
        history,
        instType,
        configId: activeId ?? undefined,
      });
      setDetail(r.data.detail);
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

  const isContract =
    detail?.instType === "SWAP" || detail?.instType === "FUTURES";
  const ccy = useMemo(() => {
    const parts = (detail?.instId || "").split("-");
    return parts[1] || "USDT";
  }, [detail?.instId]);

  // 计算器输入：接口数据回填
  const [inp, setInp] = useState<CalcInput | null>(null);
  const [touched, setTouched] = useState(false);
  useEffect(() => {
    if (!detail) return;
    setTouched(false);
    const c = detail.calc;
    setInp({
      direction:
        detail.direction === "short" || detail.side === "short"
          ? "short"
          : "long",
      isContract: isContract,
      costPx: detail.avgPx || (c ? String(c.costPx || "") : "") || "",
      tpPx: detail.tpTriggerPx || (c ? String(c.tpPx || "") : "") || "",
      currentPx: detail.currentPx || (c ? String(c.currentPx || "") : "") || "",
      contracts:
        detail.posContracts ||
        (c && c.posContracts ? String(c.posContracts) : ""),
      coin: detail.baseSz || (c && !c.isContract ? String(c.posCoin) : "") || "",
      ctVal: detail.ctVal || (c && c.ctVal ? String(c.ctVal) : "0.01"),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detail]);

  const result = useMemo(() => (inp ? compute(inp) : null), [inp]);
  const hasLivePosition = detail?.posSource === "position";
  const reconstructed = detail?.posSource === "cycle";
  const baseCoin = (detail?.instId || "").split("-")[0];

  return (
    <div className="space-y-4">
      {/* 返回 + 操作 */}
      <div className="flex items-center justify-between gap-2">
        <button
          className="btn-outline min-h-[40px]"
          onClick={() => navigate("/strategies")}
        >
          <ArrowLeft size={15} />
          返回策略列表
        </button>
        <button
          className="btn-outline min-h-[40px]"
          onClick={load}
          disabled={loading || noConfig}
        >
          <RefreshCw size={14} className={loading ? "animate-spin" : ""} />
          刷新
        </button>
      </div>

      {noConfig ? (
        <div className="card flex flex-col items-center py-16 text-center">
          <AlertTriangle size={26} className="text-slate-300 mb-3" />
          <h3 className="text-slate-700 font-medium">请先配置 OKX API Key</h3>
          <button className="btn-primary mt-4" onClick={() => navigate("/config")}>
            去配置
          </button>
        </div>
      ) : error ? (
        <div className="flex items-start gap-2 bg-red-50 border border-red-200 text-red-600 text-sm rounded-lg px-4 py-3">
          <AlertTriangle size={16} className="mt-0.5 shrink-0" />
          <div>
            {error}
            <p className="text-xs text-red-400 mt-1">
              可能是策略已结束、被删除，或该策略类型暂不支持详情查询。
            </p>
          </div>
        </div>
      ) : !detail || !inp || !result ? (
        <div className="text-center py-16 text-slate-400">
          {loading ? "加载中…" : ""}
        </div>
      ) : (
        <>
          {/* 标题卡 */}
          <div className="card">
            <div className="flex flex-wrap items-center gap-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <h2 className="text-xl font-bold text-slate-800 break-all">
                    {detail.instId || "-"}
                  </h2>
                  {detail.instType && INST_LABEL[detail.instType] && (
                    <span className="px-2 py-0.5 rounded text-xs bg-slate-100 text-slate-500">
                      {INST_LABEL[detail.instType]}
                    </span>
                  )}
                  <StateBadge state={detail.state} />
                </div>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mt-1.5 text-xs text-slate-400">
                  <span>
                    {ALGO_LABEL[detail.algoOrdType || ""] ||
                      detail.algoOrdType ||
                      kind}
                  </span>
                  <span>
                    {inp.direction === "long" ? "🟢 做多" : "🔴 做空"}
                    {detail.lever ? ` · ${detail.lever}×` : ""}
                  </span>
                  <span className="break-all">ID: {detail.algoId}</span>
                </div>
              </div>
            </div>
          </div>

          {reconstructed ? (
            <div className="flex items-start gap-2 bg-amber-50 border border-amber-200 text-amber-700 text-sm rounded-lg px-4 py-3">
              <Info size={16} className="mt-0.5 shrink-0" />
              <div>
                策略当前无持仓（已停止或等待下一轮开仓）。下方计算器已按
                <span className="font-medium">最近第 {detail.cycleId} 轮</span>
                的成交记录自动回填开仓均价、止盈价与持仓量，可直接查看该轮止盈收入，也可手动修改测算。
              </div>
            </div>
          ) : !hasLivePosition ? (
            <div className="flex items-start gap-2 bg-amber-50 border border-amber-200 text-amber-700 text-sm rounded-lg px-4 py-3">
              <Info size={16} className="mt-0.5 shrink-0" />
              <div>
                该策略当前账户无持仓且无历史周期数据。可在下方计算器手动输入持仓量与成本价进行测算。
              </div>
            </div>
          ) : null}

          <div className="grid lg:grid-cols-5 gap-4">
            {/* 止盈计算器 */}
            <div className="card lg:col-span-3 space-y-4">
              <div className="flex items-center gap-2">
                <Calculator size={17} className="text-brand-600" />
                <h3 className="font-semibold text-slate-700">止盈收入计算器</h3>
                {touched && (
                  <button
                    className="ml-auto text-xs text-brand-600 min-h-[32px]"
                    onClick={() => setDetail((d) => (d ? { ...d } : d))}
                  >
                    恢复接口数据
                  </button>
                )}
              </div>

              {/* 方向切换 */}
              <div className="flex gap-2">
                <button
                  className={`flex-1 min-h-[40px] rounded-lg text-sm font-medium border flex items-center justify-center gap-1.5 transition ${
                    inp.direction === "long"
                      ? "bg-red-50 border-red-300 text-red-600"
                      : "bg-white border-slate-200 text-slate-500"
                  }`}
                  onClick={() => {
                    setInp({ ...inp, direction: "long" });
                    setTouched(true);
                  }}
                >
                  <TrendingUp size={15} /> 做多
                </button>
                <button
                  className={`flex-1 min-h-[40px] rounded-lg text-sm font-medium border flex items-center justify-center gap-1.5 transition ${
                    inp.direction === "short"
                      ? "bg-green-50 border-green-300 text-green-600"
                      : "bg-white border-slate-200 text-slate-500"
                  }`}
                  onClick={() => {
                    setInp({ ...inp, direction: "short" });
                    setTouched(true);
                  }}
                >
                  <TrendingDown size={15} /> 做空
                </button>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field
                  label="成本价（均价）"
                  value={inp.costPx}
                  onChange={(v) => {
                    setInp({ ...inp, costPx: v });
                    setTouched(true);
                  }}
                  placeholder="0.00"
                />
                <Field
                  label="止盈价"
                  value={inp.tpPx}
                  onChange={(v) => {
                    setInp({ ...inp, tpPx: v });
                    setTouched(true);
                  }}
                  placeholder="0.00"
                  highlight
                />
                <Field
                  label="最新价"
                  value={inp.currentPx}
                  onChange={(v) => {
                    setInp({ ...inp, currentPx: v });
                    setTouched(true);
                  }}
                  placeholder="0.00"
                />
                {inp.isContract ? (
                  <>
                    <Field
                      label="持仓量（张）"
                      value={inp.contracts}
                      onChange={(v) => {
                        setInp({ ...inp, contracts: v });
                        setTouched(true);
                      }}
                      placeholder="0"
                    />
                    <Field
                      label="合约面值（币/张）"
                      value={inp.ctVal}
                      onChange={(v) => {
                        setInp({ ...inp, ctVal: v });
                        setTouched(true);
                      }}
                      placeholder="0.01"
                    />
                  </>
                ) : (
                  <Field
                    label="持仓量（币）"
                    value={inp.coin}
                    onChange={(v) => {
                      setInp({ ...inp, coin: v });
                      setTouched(true);
                    }}
                    placeholder="0"
                  />
                )}
              </div>
              {inp.isContract && (
                <p className="text-[11px] text-slate-400">
                  持仓量（币）= 张数 × 面值 ={" "}
                  <span className="text-slate-500 font-medium">
                    {fmtNum(result.posCoin, 6)}
                  </span>{" "}
                  {detail.instId?.split("-")[0]}
                </p>
              )}

              {/* 计算结果 */}
              <div className="rounded-xl border border-slate-200 divide-y divide-slate-100 overflow-hidden">
                <div className="grid grid-cols-2 divide-x divide-slate-100">
                  <ResultCell
                    label="止盈卖出收入"
                    value={fmtUsd(result.tpReturnValue)}
                    ccy={ccy}
                  />
                  <ResultCell
                    label="持仓成本"
                    value={fmtUsd(result.costValue)}
                    ccy={ccy}
                    muted
                  />
                </div>
                <div className="grid grid-cols-2 divide-x divide-slate-100">
                  <ResultCell
                    label="止盈盈利"
                    value={
                      <span className={upDownClass(result.tpProfit)}>
                        {fmtUsd(result.tpProfit)}
                      </span>
                    }
                    ccy={ccy}
                  />
                  <ResultCell
                    label="止盈收益率"
                    value={
                      <span className={upDownClass(result.tpRatio)}>
                        {fmtPct(result.tpRatio)}
                      </span>
                    }
                  />
                </div>
                <div className="grid grid-cols-2 divide-x divide-slate-100">
                  <ResultCell
                    label="当前浮动盈亏"
                    value={
                      <span className={upDownClass(result.currentFloat)}>
                        {fmtUsd(result.currentFloat)}
                      </span>
                    }
                    ccy={ccy}
                  />
                  <ResultCell
                    label="当前市值"
                    value={fmtUsd(result.currentValue)}
                    ccy={ccy}
                    muted
                  />
                </div>
              </div>
              <p className="text-[11px] text-slate-400 leading-relaxed">
                口径：止盈卖出收入 = 止盈价 × 持仓量（合约=张数×面值）；
                止盈盈利 =（止盈价−成本价）× 持仓量（做空反向）；
                收益率 = 止盈盈利 ÷ 持仓成本。未计手续费与资金费。
              </p>
            </div>

            {/* 右侧：持仓、收益与参数 */}
            <div className="lg:col-span-2 space-y-4">
              {detail.position && (
                <div className="card space-y-3">
                  <div className="flex items-center gap-2">
                    <TrendingUp size={16} className="text-brand-600" />
                    <h3 className="font-semibold text-slate-700">
                      当前周期持仓
                    </h3>
                    <span className="ml-auto text-[11px] px-2 py-0.5 rounded bg-brand-50 text-brand-600">
                      第 {detail.position.curCycleId || detail.cycleId} 轮
                    </span>
                  </div>
                  <InfoRow label="开仓均价">
                    {fmtNum(detail.position.avgPx)}
                  </InfoRow>
                  <InfoRow label="止盈价">
                    <span className="text-red-600 font-medium">
                      {fmtNum(detail.position.tpPx)}
                    </span>
                  </InfoRow>
                  {detail.position.slPx && (
                    <InfoRow label="止损价">
                      <span className="text-green-600">
                        {fmtNum(detail.position.slPx)}
                      </span>
                    </InfoRow>
                  )}
                  <InfoRow label="持仓量">
                    {isContract
                      ? `${fmtNum(detail.position.sz)} 张（${fmtNum(
                          result?.posCoin,
                          4
                        )} ${baseCoin}）`
                      : `${fmtNum(detail.position.baseSz)} ${baseCoin}`}
                  </InfoRow>
                  <InfoRow label="仓位价值">
                    {fmtUsd(detail.position.notionalUsd)}
                  </InfoRow>
                  <InfoRow label="当前浮盈">
                    <span className={upDownClass(detail.position.upl)}>
                      {fmtUsd(detail.position.upl)}
                    </span>
                  </InfoRow>
                  {detail.position.initPx && (
                    <InfoRow label="首单成交价">
                      {fmtNum(detail.position.initPx)}
                    </InfoRow>
                  )}
                  {detail.position.liqPx && (
                    <InfoRow label="预估强平价">
                      {fmtNum(detail.position.liqPx)}
                    </InfoRow>
                  )}
                  <InfoRow label="已加仓次数">
                    {detail.position.fillSafetyOrds || "0"}
                    {detail.lever ? ` / 杠杆 ${detail.lever}×` : ""}
                  </InfoRow>
                  <InfoRow label="当轮资金费">
                    <span
                      className={upDownClass(detail.position.fundingFee)}
                    >
                      {fmtUsd(detail.position.fundingFee)}
                    </span>
                  </InfoRow>
                  <InfoRow label="当轮手续费">
                    {fmtUsd(detail.position.fee)}
                  </InfoRow>
                  <InfoRow label="周期开始">
                    {fmtTime(detail.position.startTime)}
                  </InfoRow>
                </div>
              )}

              <div className="card space-y-3">
                <div className="flex items-center gap-2">
                  <Wallet size={16} className="text-brand-600" />
                  <h3 className="font-semibold text-slate-700">收益概览</h3>
                </div>
                <InfoRow label="累计收益">
                  <span
                    className={`font-semibold ${upDownClass(
                      detail.totalPnl || detail.upl || detail.closePnl
                    )}`}
                  >
                    {fmtUsd(detail.totalPnl || detail.upl || detail.closePnl)}
                  </span>
                </InfoRow>
                {hasLivePosition && detail.upl && (
                  <InfoRow label="当前周期浮盈">
                    <span className={upDownClass(detail.upl)}>
                      {fmtUsd(detail.upl)}
                    </span>
                  </InfoRow>
                )}
                {detail.pnlRatio && (
                  <InfoRow label="累计收益率">
                    <span className={upDownClass(detail.pnlRatio)}>
                      {fmtPct(detail.pnlRatio)}
                    </span>
                  </InfoRow>
                )}
                {detail.realizedPnl && (
                  <InfoRow label="已实现盈亏">
                    <span className={upDownClass(detail.realizedPnl)}>
                      {fmtUsd(detail.realizedPnl)}
                    </span>
                  </InfoRow>
                )}
                {detail.floatProfit && (
                  <InfoRow label="浮动盈亏">
                    <span className={upDownClass(detail.floatProfit)}>
                      {fmtUsd(detail.floatProfit)}
                    </span>
                  </InfoRow>
                )}
                {detail.totalFundingFee &&
                  Number(detail.totalFundingFee) !== 0 && (
                    <InfoRow label="累计资金费">
                      <span className={upDownClass(detail.totalFundingFee)}>
                        {fmtUsd(detail.totalFundingFee)}
                      </span>
                    </InfoRow>
                  )}
                {detail.arbitragePnl &&
                  Number(detail.arbitragePnl) !== 0 && (
                    <InfoRow label="套利收益">
                      <span className={upDownClass(detail.arbitragePnl)}>
                        {fmtUsd(detail.arbitragePnl)}
                      </span>
                    </InfoRow>
                  )}
                {detail.annualizedRate && (
                  <InfoRow label="年化收益率">
                    {fmtPct(detail.annualizedRate)}
                  </InfoRow>
                )}
                {detail.gridProfit && (
                  <InfoRow label="网格收益">{fmtUsd(detail.gridProfit)}</InfoRow>
                )}
              </div>

              <div className="card space-y-3">
                <h3 className="font-semibold text-slate-700">策略参数</h3>
                <InfoRow label="累计投入">
                  {fmtUsd(detail.investmentAmt || detail.investAmt || detail.quoteSz)}
                </InfoRow>
                {detail.initOrdAmt && (
                  <InfoRow label="首单金额">{fmtUsd(detail.initOrdAmt)}</InfoRow>
                )}
                {detail.safetyOrdAmt && (
                  <InfoRow label="补仓单金额">
                    {fmtUsd(detail.safetyOrdAmt)}
                  </InfoRow>
                )}
                {detail.pxSteps && (
                  <InfoRow label="补仓价格步长">
                    {fmtPct(Number(detail.pxSteps) <= 10 ? detail.pxSteps : Number(detail.pxSteps) / 100)}
                  </InfoRow>
                )}
                {detail.volMult && (
                  <InfoRow label="补仓量倍数">×{fmtNum(detail.volMult)}</InfoRow>
                )}
                {detail.maxSafetyOrders && (
                  <InfoRow label="最大补仓次数">{detail.maxSafetyOrders}</InfoRow>
                )}
                {detail.gridNum && (
                  <InfoRow label="网格数量">{detail.gridNum}</InfoRow>
                )}
                {detail.minPx && detail.maxPx && (
                  <InfoRow label="网格区间">
                    {fmtNum(detail.minPx)} – {fmtNum(detail.maxPx)}
                  </InfoRow>
                )}
                {detail.lever && <InfoRow label="杠杆">{detail.lever}×</InfoRow>}
                {detail.cTime && (
                  <InfoRow label="创建时间">{fmtTime(detail.cTime)}</InfoRow>
                )}
                {detail.uTime && (
                  <InfoRow label="更新时间">{fmtTime(detail.uTime)}</InfoRow>
                )}
              </div>

              {detail.raw && (
                <details className="card">
                  <summary className="cursor-pointer text-sm text-slate-500 select-none min-h-[40px] flex items-center">
                    查看 OKX 原始返回字段
                  </summary>
                  <pre className="mt-3 text-[11px] leading-relaxed bg-slate-50 rounded-lg p-3 overflow-x-auto text-slate-600 max-h-96">
                    {JSON.stringify(detail.raw, null, 2)}
                  </pre>
                </details>
              )}
            </div>
          </div>

          {/* 成交记录（当前/最近周期子订单） */}
          {detail.orders && detail.orders.length > 0 && (
            <div className="card overflow-hidden">
              <div className="px-5 py-3 border-b border-slate-100 bg-slate-50 flex items-center gap-2">
                <h3 className="text-sm font-semibold text-slate-700">
                  成交记录
                </h3>
                <span className="text-xs text-slate-400">
                  第 {detail.cycleId} 轮 · {detail.orders.length} 笔子订单
                </span>
              </div>
              <div className="lg:hidden px-5 py-1.5 text-[11px] text-slate-400 bg-slate-50 border-b border-slate-100">
                ← 左右滑动查看更多列 →
              </div>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr>
                      <th className="th">成交时间</th>
                      <th className="th">类型</th>
                      <th className="th">方向</th>
                      <th className="th">成交价</th>
                      <th className="th">成交量{isContract ? "（张）" : `（${baseCoin}）`}</th>
                      <th className="th">手续费</th>
                      <th className="th">状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    {detail.orders.map((o) => (
                      <tr key={o.ordId} className="border-t border-slate-100">
                        <td className="td text-xs text-slate-400 whitespace-nowrap">
                          {fmtTime(o.fillTime || o.cTime)}
                        </td>
                        <td className="td">
                          <OrderTypeBadge type={o.ordType} />
                        </td>
                        <td className="td">
                          <span
                            className={`px-2 py-0.5 rounded text-xs font-medium ${
                              o.side === "buy"
                                ? "bg-red-50 text-red-600"
                                : o.side === "sell"
                                ? "bg-green-50 text-green-600"
                                : "bg-slate-100 text-slate-500"
                            }`}
                          >
                            {o.side === "buy"
                              ? "买入"
                              : o.side === "sell"
                              ? "卖出"
                              : o.side || "-"}
                          </span>
                        </td>
                        <td className="td">
                          {o.avgFillPx
                            ? fmtNum(o.avgFillPx)
                            : o.px
                            ? `${fmtNum(o.px)}（挂单）`
                            : "-"}
                        </td>
                        <td className="td">
                          {o.filledSz || o.sz || "-"}
                        </td>
                        <td
                          className={`td text-xs ${upDownClass(
                            Number(o.fee) > 0 ? o.fee : `-${Math.abs(Number(o.fee || 0))}`
                          )}`}
                        >
                          {o.fee ? fmtUsd(o.fee) : "-"}
                        </td>
                        <td className="td text-xs">
                          <OrderStateBadge state={o.state} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 历史周期 */}
          {detail.cycles && detail.cycles.length > 0 && (
            <div className="card overflow-hidden">
              <div className="px-5 py-3 border-b border-slate-100 bg-slate-50">
                <h3 className="text-sm font-semibold text-slate-700">
                  策略周期（{detail.cycles.length}）
                </h3>
              </div>
              <div className="lg:hidden px-5 py-1.5 text-[11px] text-slate-400 bg-slate-50 border-b border-slate-100">
                ← 左右滑动查看更多列 →
              </div>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr>
                      <th className="th">周期</th>
                      <th className="th">状态</th>
                      <th className="th">开始</th>
                      <th className="th">结束</th>
                      <th className="th">开仓均价</th>
                      <th className="th">止盈价</th>
                      <th className="th">已实现盈亏</th>
                      <th className="th">手续费</th>
                    </tr>
                  </thead>
                  <tbody>
                    {detail.cycles.map((cy) => (
                      <tr
                        key={cy.cycleId}
                        className={`border-t border-slate-100 ${
                          cy.currentCycle ? "bg-brand-50/30" : ""
                        }`}
                      >
                        <td className="td text-xs whitespace-nowrap">
                          第 {cy.cycleId} 轮
                          {cy.currentCycle && (
                            <span className="ml-1.5 px-1.5 py-0.5 rounded text-[10px] bg-brand-50 text-brand-600">
                              当前
                            </span>
                          )}
                        </td>
                        <td className="td text-xs">
                          {cy.cycleStatus === "running" ? (
                            <span className="text-emerald-600">运行中</span>
                          ) : (
                            <span className="text-slate-400">已结束</span>
                          )}
                        </td>
                        <td className="td text-xs text-slate-400 whitespace-nowrap">
                          {fmtTime(cy.startTime)}
                        </td>
                        <td className="td text-xs text-slate-400 whitespace-nowrap">
                          {cy.endTime ? fmtTime(cy.endTime) : "-"}
                        </td>
                        <td className="td">{fmtNum(cy.avgPx)}</td>
                        <td className="td text-red-600">{fmtNum(cy.tpPx)}</td>
                        <td className={`td font-medium ${upDownClass(cy.realizedPnl)}`}>
                          {cy.realizedPnl ? fmtUsd(cy.realizedPnl) : "-"}
                        </td>
                        <td className="td text-xs text-slate-500">
                          {cy.fee ? fmtUsd(cy.fee) : "-"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  highlight,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  highlight?: boolean;
}) {
  return (
    <label className="block">
      <span
        className={`block text-xs mb-1 ${
          highlight ? "text-brand-600 font-medium" : "text-slate-500"
        }`}
      >
        {label}
      </span>
      <input
        inputMode="decimal"
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`w-full min-h-[40px] px-3 rounded-lg border text-base text-slate-700 outline-none focus:ring-2 focus:ring-brand-200 ${
          highlight
            ? "border-brand-300 bg-brand-50/40"
            : "border-slate-200 bg-white"
        }`}
      />
    </label>
  );
}

function ResultCell({
  label,
  value,
  ccy,
  muted,
}: {
  label: string;
  value: React.ReactNode;
  ccy?: string;
  muted?: boolean;
}) {
  return (
    <div className="px-4 py-3">
      <div className="text-[11px] text-slate-400">{label}</div>
      <div
        className={`text-lg font-bold mt-0.5 break-all ${
          muted ? "text-slate-700" : ""
        }`}
      >
        {value}
      </div>
      {ccy && <div className="text-[10px] text-slate-300">{ccy} 计价</div>}
    </div>
  );
}

function InfoRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-sm">
      <span className="text-slate-400 shrink-0">{label}</span>
      <span className="text-slate-700 text-right break-all">{children}</span>
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
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium ${m.c}`}>
      {m.t}
    </span>
  );
}

const ORDER_TYPE_LABEL: Record<string, { t: string; c: string }> = {
  init_order: { t: "初始单", c: "bg-slate-100 text-slate-600" },
  safety_order: { t: "加仓单", c: "bg-amber-50 text-amber-600" },
  manual_add_order: { t: "手动加仓", c: "bg-amber-50 text-amber-600" },
  tp_order: { t: "止盈单", c: "bg-red-50 text-red-600" },
  sl_order: { t: "止损单", c: "bg-green-50 text-green-600" },
  close_position: { t: "平仓单", c: "bg-slate-100 text-slate-600" },
  manual_close_position: { t: "手动平仓", c: "bg-slate-100 text-slate-600" },
};

function OrderTypeBadge({ type }: { type?: string }) {
  const m = ORDER_TYPE_LABEL[type || ""] || {
    t: type || "订单",
    c: "bg-slate-100 text-slate-500",
  };
  return (
    <span className={`px-2 py-0.5 rounded text-xs whitespace-nowrap ${m.c}`}>
      {m.t}
    </span>
  );
}

function OrderStateBadge({ state }: { state?: string }) {
  const map: Record<string, { t: string; c: string }> = {
    live: { t: "等待成交", c: "text-blue-600" },
    partially_filled: { t: "部分成交", c: "text-amber-600" },
    filled: { t: "完全成交", c: "text-slate-500" },
    canceled: { t: "已撤单", c: "text-slate-400" },
    cancelling: { t: "撤单中", c: "text-slate-400" },
  };
  const m = map[state || ""] || { t: state || "-", c: "text-slate-400" };
  return <span className={m.c}>{m.t}</span>;
}

function fmtTime(ts?: string): string {
  if (!ts) return "-";
  const n = Number(ts);
  if (!n) return ts;
  const d = new Date(n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate()
  ).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(
    d.getMinutes()
  ).padStart(2, "0")}`;
}
