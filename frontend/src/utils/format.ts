// 格式化工具
import type { Position } from "@/api/types";

export function fmtNum(v: string | number | undefined | null, digits = 2): string {
  if (v === undefined || v === null || v === "") return "-";
  const n = Number(v);
  if (Number.isNaN(n)) return "-";
  return n.toLocaleString("en-US", {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

export function fmtUsd(v: string | number | undefined | null): string {
  const n = Number(v ?? 0);
  if (Number.isNaN(n)) return "-";
  // 大额做缩写
  if (Math.abs(n) >= 1e9) return `$${(n / 1e9).toFixed(2)}B`;
  if (Math.abs(n) >= 1e6) return `$${(n / 1e6).toFixed(2)}M`;
  if (Math.abs(n) >= 1e4) return `$${(n / 1e3).toFixed(1)}K`;
  return `$${n.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

export function fmtPct(v: string | number | undefined | null): string {
  if (v === undefined || v === null || v === "") return "-";
  const n = Number(v);
  if (Number.isNaN(n)) return "-";
  // uplRatio 可能是 0.05 表示5% 也可能是百分比
  const val = Math.abs(n) <= 10 ? n * 100 : n;
  return `${val >= 0 ? "+" : ""}${val.toFixed(2)}%`;
}

export function upDownClass(v: string | number | undefined | null): string {
  const n = Number(v ?? 0);
  if (Number.isNaN(n) || n === 0) return "text-slate-600";
  return n > 0 ? "text-red-600" : "text-green-600"; // 中国习惯：涨红跌绿
}

export function instName(instId: string): string {
  // BTC-USDT-SWAP -> BTC-USDT
  return instId.split("-").slice(0, 2).join("-");
}

export function positionTotal(positions: Position[]): {
  count: number;
  totalUpl: number;
  totalMargin: number;
} {
  let totalUpl = 0;
  let totalMargin = 0;
  for (const p of positions) {
    totalUpl += Number(p.upl || 0);
    totalMargin += Math.abs(Number(p.margin || 0));
  }
  return { count: positions.length, totalUpl, totalMargin };
}
