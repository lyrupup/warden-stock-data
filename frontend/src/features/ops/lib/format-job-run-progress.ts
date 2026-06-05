import type { TJobRun } from "@/types/admin";

/**
 * 格式化作业执行进度展示。
 * 增量作业 API 的 total 为「本次待更新标的数」（已过滤掉已是最新的股票），
 * 直接展示 processed/total 易被误解为全市场仅 3 只；此处对 incremental 使用
 * 全市场证券数作为分母，与概览页证券数量一致。
 */
export const formatJobRunProgress = (
  run: TJobRun,
  securitiesCount?: number,
): string => {
  const processed = run.processed.toLocaleString();
  if (run.job_type === "incremental" && securitiesCount && securitiesCount > 0) {
    return `${processed}/${securitiesCount.toLocaleString()}`;
  }
  const total = run.total.toLocaleString();
  return `${processed}/${total}`;
};

/** 增量作业进度条的说明（title / tooltip） */
export const jobRunProgressTitle = (
  run: TJobRun,
  securitiesCount?: number,
): string | undefined => {
  if (run.job_type !== "incremental") return undefined;
  const pending = run.total.toLocaleString();
  const market =
    securitiesCount && securitiesCount > 0
      ? securitiesCount.toLocaleString()
      : "—";
  return `全市场 ${market} 只，本次待更新 ${pending} 只，已处理 ${run.processed.toLocaleString()} 只`;
};
