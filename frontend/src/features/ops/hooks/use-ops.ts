import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { opsApi } from "../api";
import { usePollingQuery } from "@/hooks/use-polling-query";

export const opsKeys = {
  all: ["ops"] as const,
  datasources: () => [...opsKeys.all, "datasources"] as const,
  jobs: () => [...opsKeys.all, "jobs"] as const,
  jobRuns: (page: number, size: number) =>
    [...opsKeys.all, "jobRuns", page, size] as const,
  jobRun: (runId: number) => [...opsKeys.all, "jobRun", runId] as const,
  freshness: (market: string) => [...opsKeys.all, "freshness", market] as const,
  sourceStats: (market: string) =>
    [...opsKeys.all, "sourceStats", market] as const,
};

export const useDatasources = () =>
  useQuery({
    queryKey: opsKeys.datasources(),
    queryFn: opsApi.datasources,
  });

export const useHealthcheck = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => opsApi.healthcheck(id),
    // 探测无论成功/失败都会落库 health，统一在结束后刷新列表展示最新状态。
    onSettled: () => void qc.invalidateQueries({ queryKey: opsKeys.datasources() }),
  });
};

export const useJobs = () =>
  useQuery({
    queryKey: opsKeys.jobs(),
    queryFn: opsApi.jobs,
    refetchInterval: 30000,
  });

export const useUpdateJob = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...body
    }: {
      id: number;
      name?: string;
      market?: string;
      cron_expr?: string;
      batch_size?: number;
      concurrency?: number;
      enabled?: boolean;
    }) => opsApi.updateJob(id, body),
    onSuccess: () => void qc.invalidateQueries({ queryKey: opsKeys.jobs() }),
  });
};

export const useRunJob = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      type,
      market,
      codes,
      fromDate,
      toDate,
    }: {
      id: number;
      type?: string;
      market?: string;
      codes?: string[];
      fromDate?: string;
      toDate?: string;
    }) =>
      opsApi.runJob(id, {
        type,
        market,
        codes,
        from_date: fromDate,
        to_date: toDate,
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: opsKeys.all }),
  });
};

export const useJobRuns = (page: number, size: number) =>
  useQuery({
    queryKey: opsKeys.jobRuns(page, size),
    queryFn: () => opsApi.jobRuns(page, size),
    refetchInterval: 10000,
  });

export const useJobRunPolling = (runId: number | null) =>
  usePollingQuery({
    queryKey: opsKeys.jobRun(runId ?? 0),
    queryFn: () => opsApi.jobRun(runId!),
    enabled: runId !== null && runId > 0,
    shouldPoll: (data) =>
      data?.status === "running" || data?.status === "waiting",
    intervalMs: 2000,
  });

export const useCancelJobRun = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (runId: number) => opsApi.cancelJobRun(runId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: opsKeys.all }),
  });
};

export const useFreshness = (market = "CN") =>
  useQuery({
    queryKey: opsKeys.freshness(market),
    queryFn: () => opsApi.freshness(market),
  });

// source 聚合是数千万行 K 线上的分组统计（约数秒），后端按 Redis 缓存返回；
// 默认读缓存，前端较长 staleTime 避免重复请求，手动刷新走 useRefreshSourceStats 强制重算。
export const useSourceStats = (market = "CN") =>
  useQuery({
    queryKey: opsKeys.sourceStats(market),
    queryFn: () => opsApi.sourceStats(market, false),
    staleTime: 30 * 60 * 1000,
    gcTime: 60 * 60 * 1000,
  });

// 手动刷新：强制后端跳过缓存重算，并把结果写回查询缓存，刷新卡片展示。
export const useRefreshSourceStats = (market = "CN") => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => opsApi.sourceStats(market, true),
    onSuccess: (data) => qc.setQueryData(opsKeys.sourceStats(market), data),
  });
};
