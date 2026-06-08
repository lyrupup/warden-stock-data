import { getData, httpClient } from "@/core/http-client";
import type {
  TIndexQuote,
  TIndicatorResult,
  TKline,
  TKlineIndicators,
  TStockBrief,
  TStockIntraday,
  TStockQuote,
  EKlineAdjust,
  EKlinePeriod,
} from "@/types/market";

/** 管理后台行情只读代理（/admin/market/*，复用开放 API service 层） */
export const marketApi = {
  indices: (market = "CN") =>
    getData<TIndexQuote[]>(
      httpClient.get("market/indices", { searchParams: { market } }),
    ),

  search: (kw: string, market = "CN") =>
    getData<TStockBrief[]>(
      httpClient.get("market/search", { searchParams: { kw, market } }),
    ),

  quote: (code: string, market = "CN") =>
    getData<TStockQuote>(
      httpClient.get(`market/stocks/${code}`, { searchParams: { market } }),
    ),

  kline: (
    code: string,
    opts: {
      period?: EKlinePeriod;
      adjust?: EKlineAdjust;
      limit?: number;
      market?: string;
    } = {},
  ) =>
    getData<TKline[]>(
      httpClient.get(`market/stocks/${code}/kline`, {
        searchParams: {
          period: opts.period ?? "day",
          adjust: opts.adjust === "none" ? "" : (opts.adjust ?? "qfq"),
          limit: opts.limit ?? 120,
          market: opts.market ?? "CN",
        },
      }),
    ),

  /** K 线 + 逐 bar 指标（后端快照优先 / 实时补齐，前端不再手算指标）。
   * offset 配合 limit 做分页：跳过最近 offset 根、取 limit 根历史，响应含 has_more。 */
  klineIndicators: (
    code: string,
    opts: {
      period?: EKlinePeriod;
      adjust?: EKlineAdjust;
      limit?: number;
      offset?: number;
      market?: string;
      types: string[];
    },
  ) =>
    getData<TKlineIndicators>(
      httpClient.get(`market/stocks/${code}/kline`, {
        searchParams: {
          period: opts.period ?? "day",
          adjust: opts.adjust === "none" ? "" : (opts.adjust ?? "qfq"),
          limit: opts.limit ?? 120,
          offset: opts.offset ?? 0,
          market: opts.market ?? "CN",
          indicators: opts.types.join(","),
        },
      }),
    ),

  intraday: (code: string, market = "CN") =>
    getData<TStockIntraday>(
      httpClient.get(`market/stocks/${code}/intraday`, {
        searchParams: { market },
      }),
    ),

  indicators: (code: string, market = "CN") =>
    getData<TIndicatorResult>(
      httpClient.get(`market/stocks/${code}/indicators`, {
        searchParams: {
          types: "ma5,ma10,ma20,ma30,ma60",
          market,
        },
      }),
    ),
};
