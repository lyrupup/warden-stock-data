import { getData, httpClient } from "@/core/http-client";
import type { TPagedList } from "@/types/api";
import type {
  TDataSource,
  TFreshness,
  TJobRun,
  TUpdateJob,
} from "@/types/admin";

export const opsApi = {
  datasources: () =>
    getData<TDataSource[]>(httpClient.get("datasources")),

  updateDatasource: (
    id: number,
    body: { enabled?: boolean; priority?: number; config?: Record<string, unknown> },
  ) => getData<unknown>(httpClient.put(`datasources/${id}`, { json: body })),

  healthcheck: (id: number) =>
    getData<unknown>(httpClient.post(`datasources/${id}/healthcheck`)),

  jobs: () => getData<TUpdateJob[]>(httpClient.get("jobs")),

  updateJob: (
    id: number,
    body: {
      cron_expr?: string;
      batch_size?: number;
      concurrency?: number;
      enabled?: boolean;
    },
  ) => getData<unknown>(httpClient.put(`jobs/${id}`, { json: body })),

  runJob: (
    id: number,
    body: {
      type?: string;
      market?: string;
      codes?: string[];
    },
  ) =>
    getData<{ runId: number }>(
      httpClient.post(`jobs/${id}/run`, { json: body }),
    ),

  jobRuns: (page: number, size: number) =>
    getData<TPagedList<TJobRun>>(
      httpClient.get("jobs/runs", { searchParams: { page, size } }),
    ),

  jobRun: (runId: number) =>
    getData<TJobRun>(httpClient.get(`jobs/runs/${runId}`)),

  cancelJobRun: (runId: number) =>
    getData<unknown>(httpClient.post(`jobs/runs/${runId}/cancel`)),

  freshness: (market = "CN") =>
    getData<TFreshness>(
      httpClient.get("freshness", { searchParams: { market } }),
    ),
};
