import type { TDecimal } from "./api";

export type TAdmin = {
  id: number;
  username: string;
  role: string;
};

export type TCredential = {
  id: number;
  secret_id: string;
  consumer_name: string;
  scope: string;
  rate_limit: number;
  daily_quota: number;
  status: number;
  expire_at: string | null;
  created_at: string;
};

export type TCredentialSecret = {
  secret_id: string;
  secret_key: string;
};

export type TCredentialAccessStat = {
  stat_date: string;
  call_count: number;
  error_count: number;
  last_access_at: string | null;
};

export type TCredentialDetail = TCredential & {
  access_stats: TCredentialAccessStat[];
};

export type TDataSource = {
  id: number;
  source: string;
  market: string;
  name: string;
  enabled: boolean;
  priority: number;
  health: "ok" | "degraded" | "down" | "unknown";
};

export type TUpdateJob = {
  id: number;
  name: string;
  job_type: "full" | "incremental" | "snapshot" | "indicator";
  market: string;
  cron_expr: string;
  batch_size: number;
  concurrency: number;
  enabled: boolean;
};

export type TJobRun = {
  id: number;
  job_id: number;
  status: "running" | "done" | "failed" | "canceled";
  total: number;
  processed: number;
  succeeded: number;
  failed: number;
  started_at: string;
  finished_at: string | null;
  error_msg?: string;
};

export type TFreshness = {
  market: string;
  latest_trade_date: string;
  kline_updated_to: string;
  last_scan_at: string | null;
  securities_count: number;
};

export type TCreateCredentialReq = {
  consumer_name: string;
  rate_limit?: number;
  daily_quota?: number;
  expire_at?: string | null;
};

export type TMeta = {
  markets: string[];
  indicator_catalog: Array<{
    type: string;
    name: string;
    value_type: string;
    unit?: string;
  }>;
  freshness: TFreshness;
};

export type { TDecimal };
