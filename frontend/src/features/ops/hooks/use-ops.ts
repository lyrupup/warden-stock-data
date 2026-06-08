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
    onSuccess: () => void qc.invalidateQueries({ queryKey: opsKeys.datasources() }),
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
    }: {
      id: number;
      type?: string;
      market?: string;
      codes?: string[];
    }) => opsApi.runJob(id, { type, market, codes }),
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
