import { Link } from "react-router-dom";
import { PageHeader } from "@/components/common/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatDateTime } from "@/lib/format";
import { useDatasources, useFreshness, useJobRuns } from "../hooks/use-ops";

export const DashboardPage = () => {
  const { data: freshness } = useFreshness();
  const { data: datasources } = useDatasources();
  const { data: runs } = useJobRuns(1, 5);

  const healthyCount =
    datasources?.filter((d) => d.health === "ok").length ?? 0;
  const totalSources = datasources?.length ?? 0;

  return (
    <>
      <PageHeader
        title="概览"
        description="数据新鲜度、数据源健康与近期作业"
      />

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              最新交易日
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {freshness?.latest_trade_date ?? "—"}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              K 线更新至 {freshness?.kline_updated_to ?? "—"}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              证券数量
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {freshness?.securities_count?.toLocaleString() ?? "—"}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              最近扫描 {formatDateTime(freshness?.last_scan_at)}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              数据源健康
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {healthyCount}/{totalSources}
            </div>
            <Button asChild variant="link" className="h-auto p-0 text-xs">
              <Link to="/ops/datasources">查看详情 →</Link>
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card className="mt-6">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-lg">近期作业执行</CardTitle>
          <Button asChild variant="outline" size="sm">
            <Link to="/ops/jobs">全部作业 →</Link>
          </Button>
        </CardHeader>
        <CardContent>
          {!runs?.list.length ? (
            <p className="text-sm text-muted-foreground">暂无执行记录</p>
          ) : (
            <ul className="space-y-3">
              {runs.list.map((run) => (
                <li
                  key={run.id}
                  className="flex items-center justify-between rounded-md border px-4 py-3 text-sm"
                >
                  <div>
                    <span className="font-medium">Run #{run.id}</span>
                    <span className="ml-2 text-muted-foreground">
                      {formatDateTime(run.started_at)}
                    </span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-muted-foreground">
                      {run.processed}/{run.total}
                    </span>
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
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </>
  );
};
