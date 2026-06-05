import { Pencil, Play } from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { usePagedQuery } from "@/hooks/use-paged-query";
import { AppError } from "@/core/http-client";
import { formatDateTime } from "@/lib/format";
import type { TJobRun, TUpdateJob } from "@/types/admin";
import {
  useCancelJobRun,
  useFreshness,
  useJobRunPolling,
  useJobRuns,
  useJobs,
  useRunJob,
  useUpdateJob,
} from "../hooks/use-ops";
import {
  formatJobRunProgress,
  jobRunProgressTitle,
} from "../lib/format-job-run-progress";

const STATUS_LABEL: Record<TJobRun["status"], string> = {
  waiting: "等待中",
  running: "运行中",
  done: "完成",
  failed: "失败",
  canceled: "已取消",
};

const JOB_TYPE_LABEL: Record<string, string> = {
  full: "全量回补",
  incremental: "增量更新",
  snapshot: "指标快照",
  indicator: "指标计算",
  securities: "证券列表同步",
};

const jobTypeLabel = (t: string) => JOB_TYPE_LABEL[t] ?? t;

const statusVariant = (status: TJobRun["status"]) => {
  switch (status) {
    case "done":
      return "success" as const;
    case "running":
      return "default" as const;
    case "failed":
      return "destructive" as const;
    default:
      return "secondary" as const;
  }
};

export const JobsPage = () => {
  const { data: jobs } = useJobs();
  const { page, size, setPage } = usePagedQuery();
  const { data: runs } = useJobRuns(page, size);
  const { data: freshness } = useFreshness();
  const securitiesCount = freshness?.securities_count;
  const runJob = useRunJob();
  const cancelRun = useCancelJobRun();
  const [pollingRunId, setPollingRunId] = useState<number | null>(null);
  const { data: pollingRun } = useJobRunPolling(pollingRunId);
  const [editJob, setEditJob] = useState<TUpdateJob | null>(null);

  const handleRun = async (job: TUpdateJob) => {
    const result = await runJob.mutateAsync({
      id: job.id,
      type: job.job_type,
      market: job.market,
    });
    setPollingRunId(result.runId);
  };

  const progress =
    pollingRun && pollingRun.total > 0
      ? Math.round((pollingRun.processed / pollingRun.total) * 100)
      : 0;

  const totalPages = runs ? Math.ceil(runs.total / size) : 1;

  return (
    <>
      <PageHeader
        title="更新作业"
        description="配置盘后调度、手动触发更新并查看执行进度"
      />

      <div className="mb-8 grid gap-4 md:grid-cols-2">
        {jobs?.map((job) => (
          <Card key={job.id}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-base">{job.name}</CardTitle>
              <Badge variant={job.enabled ? "success" : "secondary"}>
                {job.enabled ? "启用" : "停用"}
              </Badge>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <p>
                <span className="text-muted-foreground">类型 </span>
                {jobTypeLabel(job.job_type)}
              </p>
              <p>
                <span className="text-muted-foreground">Cron </span>
                <code className="text-xs">{job.cron_expr}</code>
              </p>
              <p>
                <span className="text-muted-foreground">分批 / 并发 </span>
                {job.batch_size} / {job.concurrency}
              </p>
              <div className="mt-2 flex gap-2">
                <Button
                  size="sm"
                  disabled={!job.enabled || runJob.isPending}
                  title={job.enabled ? undefined : "作业已停用，无法手动触发"}
                  onClick={() => void handleRun(job)}
                >
                  <Play className="mr-1 h-3 w-3" />
                  手动触发
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setEditJob(job)}
                >
                  <Pencil className="mr-1 h-3 w-3" />
                  编辑
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <JobEditDialog job={editJob} onClose={() => setEditJob(null)} />

      {pollingRun?.status === "running" ? (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="text-base">
              作业进度 Run #{pollingRun.id}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Progress value={progress} />
            <div className="flex justify-between text-sm text-muted-foreground">
              <span
                title={jobRunProgressTitle(pollingRun, securitiesCount)}
              >
                {formatJobRunProgress(pollingRun, securitiesCount)}（成功{" "}
                {pollingRun.succeeded.toLocaleString()}，失败{" "}
                {pollingRun.failed.toLocaleString()}
                {pollingRun.job_type === "incremental" && pollingRun.total > 0
                  ? `，待更新 ${pollingRun.total.toLocaleString()}`
                  : ""}
                ）
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void cancelRun.mutateAsync(pollingRun.id)}
              >
                取消
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : pollingRun?.status === "waiting" ? (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="text-base">
              作业排队中 Run #{pollingRun.id}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex items-center justify-between text-sm text-muted-foreground">
            <span>已有同类型作业运行中，本次将在其完成后自动执行。</span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void cancelRun.mutateAsync(pollingRun.id)}
            >
              取消
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">执行记录</CardTitle>
        </CardHeader>
        <CardContent>
          {!runs?.list.length ? (
            <EmptyState message="暂无执行记录" />
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>作业</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>进度</TableHead>
                    <TableHead>开始时间</TableHead>
                    <TableHead className="text-right">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {runs.list.map((run) => (
                    <TableRow key={run.id}>
                      <TableCell>#{run.id}</TableCell>
                      <TableCell>Job {run.job_id}</TableCell>
                      <TableCell>
                        {run.job_type ? jobTypeLabel(run.job_type) : "—"}
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(run.status)}>
                          {STATUS_LABEL[run.status] ?? run.status}
                        </Badge>
                      </TableCell>
                      <TableCell
                        className="tabular-nums"
                        title={jobRunProgressTitle(run, securitiesCount)}
                      >
                        {formatJobRunProgress(run, securitiesCount)}
                      </TableCell>
                      <TableCell>{formatDateTime(run.started_at)}</TableCell>
                      <TableCell className="text-right">
                        {run.status === "running" || run.status === "waiting" ? (
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={cancelRun.isPending}
                            onClick={() => void cancelRun.mutateAsync(run.id)}
                          >
                            取消
                          </Button>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <div className="mt-4 flex justify-end gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage(page - 1)}
                >
                  上一页
                </Button>
                <span className="flex items-center text-sm">
                  {page} / {totalPages}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= totalPages}
                  onClick={() => setPage(page + 1)}
                >
                  下一页
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </>
  );
};

const CRON_HINT = "6 段（含秒），例：每日 17:00 → 0 0 17 * * *";

/** 客户端粗校验：6 段或 @ 描述符；后端用调度器解析器做权威校验。 */
const looksLikeCron = (expr: string) => {
  const t = expr.trim();
  if (t.startsWith("@")) return true;
  return t.split(/\s+/).length === 6;
};

const JobEditDialog = ({
  job,
  onClose,
}: {
  job: TUpdateJob | null;
  onClose: () => void;
}) => (
  <Dialog
    open={!!job}
    onOpenChange={(open) => {
      if (!open) onClose();
    }}
  >
    <DialogContent>
      {job ? <JobEditForm key={job.id} job={job} onClose={onClose} /> : null}
    </DialogContent>
  </Dialog>
);

const JobEditForm = ({
  job,
  onClose,
}: {
  job: TUpdateJob;
  onClose: () => void;
}) => {
  const update = useUpdateJob();
  const [name, setName] = useState(job.name);
  const [market, setMarket] = useState(job.market);
  const [cronExpr, setCronExpr] = useState(job.cron_expr);
  const [batchSize, setBatchSize] = useState(String(job.batch_size));
  const [concurrency, setConcurrency] = useState(String(job.concurrency));
  const [enabled, setEnabled] = useState(job.enabled);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setError(null);
    if (!name.trim()) {
      setError("作业名称不能为空");
      return;
    }
    if (!looksLikeCron(cronExpr)) {
      setError(`cron 表达式格式疑似有误（${CRON_HINT}）`);
      return;
    }
    const bs = Number(batchSize);
    const cc = Number(concurrency);
    if (!Number.isInteger(bs) || bs <= 0) {
      setError("分批大小必须为正整数");
      return;
    }
    if (!Number.isInteger(cc) || cc <= 0) {
      setError("并发数必须为正整数");
      return;
    }
    try {
      await update.mutateAsync({
        id: job.id,
        name: name.trim(),
        market: market.trim(),
        cron_expr: cronExpr.trim(),
        batch_size: bs,
        concurrency: cc,
        enabled,
      });
      onClose();
    } catch (e) {
      setError(e instanceof AppError ? e.message : "保存失败，请重试");
    }
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>编辑作业</DialogTitle>
        <DialogDescription>类型与创建/更新时间不可修改。</DialogDescription>
      </DialogHeader>
      <div className="space-y-4">
        <div className="grid gap-2">
          <Label>类型（不可改）</Label>
          <Input value={jobTypeLabel(job.job_type)} disabled />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="job-name">名称</Label>
          <Input
            id="job-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="job-market">市场</Label>
          <Input
            id="job-market"
            value={market}
            onChange={(e) => setMarket(e.target.value)}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="job-cron">Cron 表达式</Label>
          <Input
            id="job-cron"
            value={cronExpr}
            onChange={(e) => setCronExpr(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">{CRON_HINT}</p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="grid gap-2">
            <Label htmlFor="job-batch">分批大小</Label>
            <Input
              id="job-batch"
              type="number"
              min={1}
              value={batchSize}
              onChange={(e) => setBatchSize(e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="job-conc">并发数</Label>
            <Input
              id="job-conc"
              type="number"
              min={1}
              value={concurrency}
              onChange={(e) => setConcurrency(e.target.value)}
            />
          </div>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="h-4 w-4"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          启用该作业
        </label>
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          取消
        </Button>
        <Button disabled={update.isPending} onClick={() => void submit()}>
          保存
        </Button>
      </DialogFooter>
    </>
  );
};
