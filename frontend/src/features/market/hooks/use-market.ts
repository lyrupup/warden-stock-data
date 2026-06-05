import { useQuery } from "@tanstack/react-query";
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
) =>
  useQuery({
    queryKey: marketKeys.kline(code ?? "", period, adjust),
    queryFn: () => marketApi.kline(code!, { period, adjust }),
    enabled: !!code,
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
