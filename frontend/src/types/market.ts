import type { TDecimal } from "./api";

export type TIndexQuote = {
  market: string;
  index_code: string;
  index_name: string;
  price: TDecimal;
  change_amount: TDecimal;
  change_percent: TDecimal;
  volume: TDecimal;
  amount: TDecimal;
  trade_date: string;
};

export type TStockQuote = {
  market: string;
  stock_code: string;
  stock_name: string;
  price: TDecimal;
  open: TDecimal;
  high: TDecimal;
  low: TDecimal;
  prev_close: TDecimal;
  change_percent: TDecimal;
  volume: TDecimal;
  amount: TDecimal;
  turnover_rate: TDecimal;
  trade_date: string;
  stale?: boolean;
};

export type TKline = {
  market?: string;
  stock_code?: string;
  trade_date: string;
  open: TDecimal;
  high: TDecimal;
  low: TDecimal;
  close: TDecimal;
  volume: TDecimal;
  amount: TDecimal;
  adjust?: string;
};

export type TIntradayPoint = {
  time: string;
  price: TDecimal;
  avg_price: TDecimal;
  volume: TDecimal;
};

export type TStockIntraday = {
  market: string;
  stock_code: string;
  stock_name?: string;
  trade_date: string;
  pre_close: TDecimal;
  points: TIntradayPoint[];
};

export type TStockBrief = {
  stock_code: string;
  stock_name: string;
  market: string;
  board?: string;
};

export type TIndicatorResult = {
  stock_code: string;
  trade_date: string;
  values: Record<string, TDecimal>;
};

/** K 线接口带指标返回：bars + 与之按 trade_date 对齐的逐 bar 指标 */
export type TKlineIndicators = {
  bars: TKline[];
  indicators: TIndicatorResult[];
  /** 当前窗口左侧（更早方向）是否还有可分页加载的历史 K 线 */
  has_more: boolean;
};

export type EKlinePeriod = "day" | "week" | "month";
/** none=不复权，对应后端 adjust 空字符串 */
export type EKlineAdjust = "none" | "qfq" | "hfq";
