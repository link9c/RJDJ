// API 数据类型
export interface User {
  id: number;
  username: string;
  nickname: string;
  role: string;
  email: string;
}

export interface OkxConfig {
  id: number;
  name: string;
  base_url: string;
  is_default: boolean;
  description: string;
  api_key: string;
  has_secret: boolean;
  created_at: string;
  updated_at: string;
}

export interface CoinBalance {
  ccy: string;
  eq: string; // 币种总权益
  eqUsd: string; // 折合美元
  cashBal: string;
  availBal: string;
  frozenBal: string;
}

export interface AccountBalance {
  totalEq: string; // 总权益 USD
  adjEq: string;
  ts: string;
  details: CoinBalance[];
}

export interface Position {
  instId: string;
  instType: string;
  posSide: string;
  pos: string; // 持仓数量
  avgPx: string;
  upl: string; // 未实现盈亏
  uplRatio: string;
  lever: string;
  markPx: string;
  liqPx: string;
  margin: string;
  mgnMode: string;
}

export interface Overview {
  balance: AccountBalance;
  positions: Position[];
  updated_at: number;
}

export interface ChatMessage {
  id: number;
  role: "user" | "assistant";
  content: string;
  config_id: number;
  created_at: string;
}

export interface Ticker {
  instId: string;
  last: string;
  askPx: string;
  bidPx: string;
  open24h: string;
  high24h: string;
  low24h: string;
  vol24h: string;
  ts: string;
}
