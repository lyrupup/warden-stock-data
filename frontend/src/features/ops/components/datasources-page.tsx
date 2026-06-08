import { Activity } from "lucide-react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatDateTime } from "@/lib/format";
import type { TFreshness } from "@/types/admin";
import { useDatasources, useFreshness, useHealthcheck } from "../hooks/use-ops";

const healthVariant = (
  health: string,
): "success" | "warning" | "destructive" | "secondary" => {
  if (health === "ok") return "success";
  if (health === "degraded") return "warning";
  if (health === "down") return "destructive";
  return "secondary";
};

/** 各类数据的存储与计算策略（静态说明，便于运维区分「落库快照 / 实时回源 / 实时计算」） */
const STORAGE_STRATEGY: {
  data: string;
  storage: string;
  timing: string;
  source: string;
  api: string;
}[] = [
  {
    data: "日 K 线",
    storage: "本地落库（前复权）",
    timing: "盘后增量（默认 17:00）",
    source: "stock_daily_klines",
    api: "/kline?period=day",
  },
  {
    data: "周 K / 月 K",
    storage: "不落库 · 实时回源",
    timing: "每次请求",
    source: "gotdx 实时",
    api: "/kline?period=week|month",
  },
  {
    data: "分时走势",
    storage: "不落库 · 实时透传",
    timing: "每次请求（前端 60s 轮询）",
    source: "gotdx 实时",
    api: "/intraday",
  },
  {
    data: "日线技术指标",
    storage: "本地快照",
    timing: "盘后逐日扫描",
    source: "stock_indicator_snapshots",
    api: "/indicators · /kline?indicators=",
  },
  {
    data: "周 / 月技术指标",
    storage: "不落库 · 实时计算",
    timing: "每次请求",
    source: "返回 bars 上逐 bar 计算",
    api: "/kline?period=week&indicators=",
  },
  {
    data: "非默认指标（如 MA120）",
    storage: "不落库 · 实时计算",
    timing: "每次请求",
    source: "返回 bars 上逐 bar 计算",
    api: "/kline?indicators=ma120",
  },
];

export const DatasourcesPage = () => {
  const { data, isLoading } = useDatasources();
  const { data: freshness } = useFreshness();
  const healthcheck = useHealthcheck();

  return (
    <>
      <PageHeader
        title="数据源管理"
        description="数据源健康、存储/计算策略与快照完整性，便于数据运维"
      />

      <DataCompletenessCard f={freshness} />
      <StorageStrategyCard />

      <Card className="mt-6">
        <CardHeader className="pb-3">
          <CardTitle className="text-lg">数据源</CardTitle>
        </CardHeader>
        <CardContent>
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
                      <Badge variant={healthVariant(ds.health)}>
                        {ds.health}
                      </Badge>
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
        </CardContent>
      </Card>
    </>
  );
};

const pct = (num: number, denom: number): number =>
  denom > 0 ? (num / denom) * 100 : 0;

const CoverageBar = ({
  label,
  num,
  denom,
  suffix,
}: {
  label: string;
  num: number;
  denom: number;
  suffix?: string;
}) => {
  const p = pct(num, denom);
  return (
    <div>
      <div className="flex items-baseline justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className="tabular-nums font-medium">
          {num.toLocaleString()} / {denom.toLocaleString()}
          {denom > 0 ? `（${p.toFixed(1)}%）` : ""}
          {suffix ? ` · ${suffix}` : ""}
        </span>
      </div>
      <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-all"
          style={{ width: `${Math.min(100, p)}%` }}
        />
      </div>
    </div>
  );
};

const Stat = ({ label, value }: { label: string; value: string }) => (
  <div className="rounded-md border p-3">
    <div className="text-xs text-muted-foreground">{label}</div>
    <div className="mt-1 font-medium tabular-nums">{value}</div>
  </div>
);

const DataCompletenessCard = ({ f }: { f: TFreshness | undefined }) => {
  const securities = f?.securities_count ?? 0;
  return (
    <Card className="mb-6">
      <CardHeader className="pb-3">
        <CardTitle className="text-lg">数据完整性与新鲜度</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="最新交易日" value={f?.latest_trade_date || "—"} />
          <Stat label="K 线更新至" value={f?.kline_updated_to || "—"} />
          <Stat
            label="指标快照最新日"
            value={f?.indicator_snapshot_latest_date || "—"}
          />
          <Stat
            label="指标快照起始日"
            value={f?.indicator_snapshot_earliest_date || "—"}
          />
          <Stat label="证券总数" value={securities.toLocaleString()} />
          <Stat
            label="最近扫描"
            value={formatDateTime(f?.last_scan_at) || "—"}
          />
          <Stat label="行情源" value="gotdx（通达信）" />
          <Stat label="市场" value={f?.market || "CN"} />
        </div>

        <div className="space-y-3">
          <CoverageBar
            label="日 K 线覆盖（已落库股票数 / 证券总数）"
            num={f?.kline_stock_count ?? 0}
            denom={securities}
          />
          <CoverageBar
            label={`指标快照覆盖（${f?.indicator_snapshot_latest_date || "最新日"} 有快照股票数 / 证券总数）`}
            num={f?.indicator_snapshot_stock_count ?? 0}
            denom={securities}
          />
        </div>

        <div>
          <div className="mb-1.5 text-sm text-muted-foreground">
            默认逐日快照指标（可批量按交易日 point-in-time 读取；其余指标走实时计算）
          </div>
          <div className="flex flex-wrap gap-1.5">
            {(f?.default_snapshot_types ?? []).length > 0 ? (
              f!.default_snapshot_types.map((t) => (
                <span
                  key={t}
                  className="rounded-full border bg-muted px-2 py-0.5 font-mono text-xs"
                >
                  {t}
                </span>
              ))
            ) : (
              <span className="text-sm text-muted-foreground">—</span>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
};

const StorageStrategyCard = () => (
  <Card className="mb-6">
    <CardHeader className="pb-3">
      <CardTitle className="text-lg">数据存储与计算策略</CardTitle>
    </CardHeader>
    <CardContent>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>数据类型</TableHead>
            <TableHead>存储方式</TableHead>
            <TableHead>计算 / 更新时机</TableHead>
            <TableHead>来源 / 落库</TableHead>
            <TableHead>接口</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {STORAGE_STRATEGY.map((row) => (
            <TableRow key={row.data}>
              <TableCell className="font-medium">{row.data}</TableCell>
              <TableCell>
                <Badge
                  variant={
                    row.storage.includes("落库") &&
                    !row.storage.includes("不落库")
                      ? "success"
                      : "secondary"
                  }
                >
                  {row.storage}
                </Badge>
              </TableCell>
              <TableCell className="text-muted-foreground">
                {row.timing}
              </TableCell>
              <TableCell className="font-mono text-xs">{row.source}</TableCell>
              <TableCell className="font-mono text-xs">{row.api}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </CardContent>
  </Card>
);
