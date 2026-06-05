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

export type EKlinePeriod = "day" | "week" | "month";
/** none=不复权，对应后端 adjust 空字符串 */
export type EKlineAdjust = "none" | "qfq" | "hfq";
