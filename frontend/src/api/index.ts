import { http } from "./http";
import type {
  AccountBalance,
  ChatMessage,
  OkxConfig,
  Overview,
  Position,
  User,
} from "./types";

// ===== Auth =====
export const authApi = {
  login: (username: string, password: string) =>
    http.post<{ token: string; user: User }>("/auth/login", { username, password }),
  register: (data: { username: string; password: string; nickname?: string }) =>
    http.post("/auth/register", data),
  me: () => http.get<{ data: User }>("/auth/me"),
};

// ===== OKX 配置 =====
export interface ConfigPayload {
  name?: string;
  api_key: string;
  api_secret: string;
  passphrase: string;
  base_url?: string;
  is_default?: boolean;
  description?: string;
}

export const configApi = {
  list: () => http.get<{ list: OkxConfig[] }>("/okx/configs"),
  create: (data: ConfigPayload) =>
    http.post<{ config: OkxConfig }>("/okx/configs", data),
  update: (id: number, data: Partial<ConfigPayload>) =>
    http.put<{ config: OkxConfig }>(`/okx/configs/${id}`, data),
  remove: (id: number) => http.delete(`/okx/configs/${id}`),
  test: (id: number) =>
    http.post<{ message: string; total_eq_usd?: string }>(
      `/okx/configs/${id}/test`
    ),
};

// ===== OKX 数据 =====
export interface BotStrategy {
  algoId: string;
  algoOrdType?: string; // grid / contract_grid / spot_dca / contract_dca / recurring / conditional ...
  algoClOrdType?: string; // DCA(马丁)返回的策略子类型
  instId?: string;
  instType?: string; // SPOT / SWAP / FUTURES
  ccy?: string;
  side?: string;
  state?: string;
  type?: string; // grid/dca/recurring/signal/algo
  direction?: string;
  lever?: string;
  maxPx?: string;
  minPx?: string;
  gridNum?: string;
  runType?: string;
  quoteSz?: string;
  investAmt?: string;
  baseSz?: string; // DCA 首单
  safetyOrderSz?: string; // DCA 补仓单
  maxSafetyOrders?: string; // DCA 最大补仓次数
  totalPnl?: string;
  pnlRatio?: string;
  closePnl?: string;
  upl?: string;
  triggerPx?: string;
  ordPx?: string;
  cTime?: string;
  uTime?: string;
}

// 止盈测算结果（后端 DetailCalc）
export interface DetailCalc {
  posCoin: number; // 持仓量（币）
  posContracts: number; // 持仓量（张，合约）
  costPx: number; // 成本价
  tpPx: number; // 止盈价
  currentPx: number; // 最新价
  costValue: number; // 持仓成本
  currentValue: number; // 当前市值
  currentFloat: number; // 当前浮动盈亏
  tpReturnValue: number; // 止盈卖出收入
  tpProfit: number; // 止盈盈利
  tpRatio: number; // 止盈收益率（小数）
  direction: string; // long / short
  isContract: boolean;
  ctVal: number; // 合约面值（币/张）
  ccy: string; // 计价币
}

// 马丁策略当前周期持仓（/dca/position-details）
export interface DcaPositionDetail {
  algoId: string;
  algoOrdType: string;
  instId: string;
  curCycleId: string;
  startTime: string;
  fillManualOrds: string;
  fillSafetyOrds: string;
  fundingFee: string;
  initPx: string;
  notionalUsd: string;
  avgPx: string;
  upl: string;
  liqPx: string;
  sz: string;
  baseSz: string;
  quoteSz: string;
  slPx: string;
  tpPx: string;
  fee: string;
}

// 马丁周期
export interface DcaCycle {
  cycleId: string;
  currentCycle: boolean;
  cycleStatus: string;
  realizedPnl: string;
  startTime: string;
  endTime: string;
  fee: string;
  avgPx: string;
  tpPx: string;
}

// 马丁子订单（成交记录）
export interface DcaSubOrder {
  cycleId: string;
  ordId: string;
  avgFillPx: string;
  direction: string;
  side: string; // buy / sell
  ordType: string;
  px: string;
  sz: string;
  filledSz: string;
  state: string;
  fee: string;
  rebate: string;
  rebateCcy: string;
  lever: string;
  instId: string;
  ctVal: string;
  fillTime: string;
  cTime: string;
  uTime: string;
}

// 策略详情
export interface StrategyDetail extends BotStrategy {
  avgPx?: string; // 平均持仓价
  posContracts?: string; // 合约持仓（张）
  ctVal?: string; // 合约面值
  tpTriggerPx?: string; // 止盈触发价（DCA 为 tpPriceRange）
  slTriggerPx?: string; // 止损触发价
  tpTriggerPxType?: string;
  slTriggerPxType?: string;
  // DCA 特有
  initOrdAmt?: string;
  safetyOrdAmt?: string;
  pxSteps?: string;
  volMult?: string;
  investmentAmt?: string;
  totalFundingFee?: string;
  arbitragePnl?: string;
  allowReinvest?: boolean;
  // DCA 当前周期持仓 / 周期 / 成交记录
  position?: DcaPositionDetail | null;
  cycles?: DcaCycle[];
  orders?: DcaSubOrder[];
  posSource?: "position" | "cycle" | "none" | string;
  cycleId?: string;
  notionalUsd?: string;
  // 网格特有
  arbitrageNum?: string;
  gridProfit?: string;
  floatProfit?: string;
  annualizedRate?: string;
  realizedPnl?: string;
  // 定投特有
  totalInvestedAmt?: string;
  // 实时
  currentPx?: string;
  calc?: DetailCalc | null;
  raw?: Record<string, unknown>;
}

export interface StrategyDetailResp {
  detail: StrategyDetail;
  fallback: boolean;
  current_px: string;
  ct_val: string;
}

export const dataApi = {
  overview: (configId?: number) =>
    http.get<Overview>("/okx/overview", { params: { config_id: configId } }),
  positions: (configId?: number, instType?: string) =>
    http.get<{ list: Position[] }>("/okx/positions", {
      params: { config_id: configId, inst_type: instType },
    }),
  ticker: (instId: string) => http.get("/okx/ticker", { params: { inst_id: instId } }),
  tickers: (instType?: string) =>
    http.get<{ list: unknown[] }>("/okx/tickers", { params: { inst_type: instType } }),
  candles: (instId: string, bar = "1H", limit = 100) =>
    http.get<{ list: string[][] }>("/okx/candles", {
      params: { inst_id: instId, bar, limit },
    }),
  strategies: (configId?: number, kind = "all", history = false) =>
    http.get<{ list: Record<string, BotStrategy[]>; total: number; history: boolean }>(
      "/okx/strategies",
      { params: { config_id: configId, kind, history: history ? 1 : 0 } }
    ),
  strategyDetail: (params: {
    algoId: string;
    kind: string;
    history?: boolean;
    instType?: string;
    configId?: number;
  }) =>
    http.get<StrategyDetailResp>("/okx/strategies/detail", {
      params: {
        config_id: params.configId,
        algo_id: params.algoId,
        kind: params.kind,
        history: params.history ? 1 : 0,
        inst_type: params.instType,
      },
    }),
  gridPositions: (algoId?: string, instId?: string) =>
    http.get("/okx/strategies/grid-positions", { params: { algo_id: algoId, inst_id: instId } }),
  pnlDaily: (configId?: number, days = 90, asset = "") =>
    http.get<{
      summary: PnlSummary;
      days: number;
      asset: string;
    }>("/okx/pnl/daily", { params: { config_id: configId, days, asset } }),
  // 异步拉取：先 start 拿 task_id，再轮询 status
  pnlDailyStart: (configId?: number, days = 90, asset = "") =>
    http.post<{
      task_id: string;
      days: number;
      asset: string;
      page_interval_ms: number;
      est_seconds: number;
    }>("/okx/pnl/daily/start", null, { params: { config_id: configId, days, asset } }),
  pnlDailyStatus: (taskId: string) =>
    http.get<{ task: PnlTaskStatus }>("/okx/pnl/daily/status", {
      params: { task_id: taskId },
    }),
};

export interface PnlPoint {
  date: string;
  pnl: number;
  fee: number;
  net: number;
}
export interface CcyPnl {
  ccy: string;
  pnl: number;
  fee: number;
  net: number;
  count: number;
}
export interface AssetPnl {
  asset: string;
  pnl: number;
  fee: number;
  net: number;
  count: number;
}
export interface PnlSummary {
  daily: PnlPoint[];
  byCcy: CcyPnl[];
  byAsset: AssetPnl[];
  totalPnl: number;
  totalFee: number;
  totalNet: number;
  days: number;
  records: number;
}
export interface PnlTaskStatus {
  id: string;
  done: boolean;
  error: string;
  pages: number;
  covered_days: number;
  new_saved: number;
  elapsed_ms: number;
  est_remain_sec: number;
  days: number;
  asset: string;
  summary?: PnlSummary | null;
}

// ===== AI 对话 =====
export const aiApi = {
  chat: (message: string, configId?: number) =>
    http.post<{ answer: string }>("/ai/chat", {
      message,
      config_id: configId,
    }),
  history: (configId?: number) =>
    http.get<{ list: ChatMessage[] }>("/ai/history", {
      params: { config_id: configId },
    }),
};

// ===== 系统设置（AI 大模型配置） =====
export interface AISettings {
  provider: string;
  base_url: string;
  model: string;
  api_key: string; // 掩码
  has_key: boolean;
}

export const settingsApi = {
  getAI: () => http.get<AISettings>("/settings/ai"),
  updateAI: (data: { provider?: string; base_url?: string; model?: string; api_key?: string }) =>
    http.put<{ message: string; test_error?: string; ai: AISettings }>("/settings/ai", data),
};

export type { AccountBalance, OkxConfig, Overview, Position, User };
