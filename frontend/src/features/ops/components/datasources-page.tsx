import { Activity } from "lucide-react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useDatasources, useHealthcheck } from "../hooks/use-ops";

const healthVariant = (
  health: string,
): "success" | "warning" | "destructive" | "secondary" => {
  if (health === "ok") return "success";
  if (health === "degraded") return "warning";
  if (health === "down") return "destructive";
  return "secondary";
};

export const DatasourcesPage = () => {
  const { data, isLoading } = useDatasources();
  const healthcheck = useHealthcheck();

  return (
    <>
      <PageHeader
        title="数据源管理"
        description="查看数据源健康状态并触发连通性探测"
      />

      {isLoading ? (
        <p className="text-muted-foreground">加载中…</p>
      ) : !data?.length ? (
        <EmptyState message="暂无数据源配置" />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>来源</TableHead>
              <TableHead>市场</TableHead>
              <TableHead>优先级</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>健康</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((ds) => (
              <TableRow key={ds.id}>
                <TableCell className="font-medium">{ds.name}</TableCell>
                <TableCell>{ds.source}</TableCell>
                <TableCell>{ds.market}</TableCell>
                <TableCell>{ds.priority}</TableCell>
                <TableCell>
                  <Badge variant={ds.enabled ? "success" : "secondary"}>
                    {ds.enabled ? "启用" : "停用"}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={healthVariant(ds.health)}>{ds.health}</Badge>
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={healthcheck.isPending}
                    onClick={() => void healthcheck.mutateAsync(ds.id)}
                  >
                    <Activity className="mr-1 h-3 w-3" />
                    探测
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </>
  );
};
