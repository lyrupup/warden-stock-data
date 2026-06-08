import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { marketApi } from "../api";
import type { EKlineAdjust, EKlinePeriod } from "@/types/market";

export const marketKeys = {
  all: ["market"] as const,
  indices: (market: string) => [...marketKeys.all, "indices", market] as const,
  search: (kw: string) => [...marketKeys.all, "search", kw] as const,
  quote: (code: string) => [...marketKeys.all, "quote", code] as const,
  kline: (code: string, period: EKlinePeriod, adjust: EKlineAdjust) =>
    [...marketKeys.all, "kline", code, period, adjust] as const,
  intraday: (code: string) => [...marketKeys.all, "intraday", code] as const,
  indicators: (code: string) =>
    [...marketKeys.all, "indicators", code] as const,
};

export const useIndices = (market = "CN") =>
  useQuery({
    queryKey: marketKeys.indices(market),
    queryFn: () => marketApi.indices(market),
    refetchInterval: 30_000,
  });

export const useStockSearch = (kw: string) =>
  useQuery({
    queryKey: marketKeys.search(kw),
    queryFn: () => marketApi.search(kw),
    enabled: kw.length >= 2,
  });

export const useStockQuote = (code: string | null) =>
  useQuery({
    queryKey: marketKeys.quote(code ?? ""),
    queryFn: () => marketApi.quote(code!),
    enabled: !!code,
    refetchInterval: 10_000,
  });

export const useStockKline = (
  code: string | null,
  period: EKlinePeriod,
  adjust: EKlineAdjust,
  enabled = true,
) =>
  useQuery({
    queryKey: marketKeys.kline(code ?? "", period, adjust),
    queryFn: () => marketApi.kline(code!, { period, adjust }),
    enabled: !!code && enabled,
  });

/**
 * K 线 + 逐 bar 指标分页：固定请求全量指标（types 不变），左滑时 fetchNextPage 按
 * pageSize 步进 offset 拉取更早历史（每页仅 pageSize 根，非全量），响应 has_more 决定
 * 是否还有更早数据。各页按「最近→更早」顺序累积，页面层再倒序拼接为升序整体序列。
 */
export const useStockKlineInfinite = (
  code: string | null,
  period: EKlinePeriod,
  adjust: EKlineAdjust,
  types: string[],
  pageSize = 120,
) =>
  useInfiniteQuery({
    queryKey: [
      ...marketKeys.kline(code ?? "", period, adjust),
      "ind-page",
      [...types].sort().join(","),
      pageSize,
    ],
    queryFn: ({ pageParam }) =>
      marketApi.klineIndicators(code!, {
        period,
        adjust,
        types,
        limit: pageSize,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.has_more ? allPages.length * pageSize : undefined,
    enabled: !!code && types.length > 0,
  });

export const useStockIntraday = (code: string | null) =>
  useQuery({
    queryKey: marketKeys.intraday(code ?? ""),
    queryFn: () => marketApi.intraday(code!),
    enabled: !!code,
    refetchInterval: 60_000,
  });

export const useStockIndicators = (code: string | null) =>
  useQuery({
    queryKey: marketKeys.indicators(code ?? ""),
    queryFn: () => marketApi.indicators(code!),
    enabled: !!code,
  });
