import { Play } from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { formatDateTime } from "@/lib/format";
import {
  useCancelJobRun,
  useJobRunPolling,
  useJobRuns,
  useJobs,
  useRunJob,
} from "../hooks/use-ops";

export const JobsPage = () => {
  const { data: jobs } = useJobs();
  const { page, size, setPage } = usePagedQuery();
  const { data: runs } = useJobRuns(page, size);
  const runJob = useRunJob();
  const cancelRun = useCancelJobRun();
  const [pollingRunId, setPollingRunId] = useState<number | null>(null);
  const { data: pollingRun } = useJobRunPolling(pollingRunId);

  const handleRun = async (jobId: number) => {
    const result = await runJob.mutateAsync({
      id: jobId,
      type: "incremental",
      market: "CN",
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
                {job.job_type}
              </p>
              <p>
                <span className="text-muted-foreground">Cron </span>
                <code className="text-xs">{job.cron_expr}</code>
              </p>
              <p>
                <span className="text-muted-foreground">分批 / 并发 </span>
                {job.batch_size} / {job.concurrency}
              </p>
              <Button
                size="sm"
                className="mt-2"
                disabled={runJob.isPending}
                onClick={() => void handleRun(job.id)}
              >
                <Play className="mr-1 h-3 w-3" />
                手动触发
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

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
              <span>
                {pollingRun.processed} / {pollingRun.total}（成功 {pollingRun.succeeded}，失败{" "}
                {pollingRun.failed}）
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
                    <TableHead>状态</TableHead>
                    <TableHead>进度</TableHead>
                    <TableHead>开始时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {runs.list.map((run) => (
                    <TableRow key={run.id}>
                      <TableCell>#{run.id}</TableCell>
                      <TableCell>Job {run.job_id}</TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            run.status === "done"
                              ? "success"
                              : run.status === "running"
                                ? "default"
                                : run.status === "failed"
                                  ? "destructive"
                                  : "secondary"
                          }
                        >
                          {run.status}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {run.processed}/{run.total}
                      </TableCell>
                      <TableCell>{formatDateTime(run.started_at)}</TableCell>
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
