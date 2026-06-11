import { Activity, RefreshCw } from "lucide-react";
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
import type { TFreshness, TSourceStat } from "@/types/admin";
import {
  useDatasources,
  useFreshness,
  useHealthcheck,
  useRefreshSourceStats,
  useSourceStats,
} from "../hooks/use-ops";

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
    storage: "本地落库（前复权 + 涨跌停/ST/停牌）",
    timing: "盘后 gotdx 增量（默认 17:00）+ 周级 baostock 对齐",
    source: "gotdx / baostock → stock_daily_klines",
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
    storage: "不落库 · Python 实时计算",
    timing: "每次请求",
    source: "quant 服务（pandas）",
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
      <SourceDistributionCard />
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
          <Stat label="证券总数" value={securities.toLocaleString()} />
          <Stat
            label="最近扫描"
            value={formatDateTime(f?.last_scan_at) || "—"}
          />
          <Stat label="实时行情源" value={f?.provider_source || "gotdx"} />
          <Stat label="日K采集源" value={f?.quant_source || "baostock"} />
          <Stat label="市场" value={f?.market || "CN"} />
        </div>

        <div className="space-y-3">
          <CoverageBar
            label="日 K 线覆盖（已落库股票数 / 证券总数）"
            num={f?.kline_stock_count ?? 0}
            denom={securities}
          />
        </div>

        <p className="text-sm text-muted-foreground">
          技术指标已改为 Python quant 服务实时计算，不再落库指标快照。
        </p>
      </CardContent>
    </Card>
  );
};

const SOURCE_META: Record<
  string,
  { label: string; variant: "success" | "secondary" | "warning" }
> = {
  baostock: { label: "baostock 日K采集", variant: "success" },
  gotdx: { label: "gotdx 实时/增量", variant: "warning" },
};

const SourceRow = ({ s }: { s: TSourceStat }) => {
  const meta = SOURCE_META[s.source] ?? {
    label: s.source,
    variant: "secondary" as const,
  };
  const range =
    s.min_date && s.max_date ? `${s.min_date} ~ ${s.max_date}` : "—";
  return (
    <TableRow>
      <TableCell>
        <Badge variant={meta.variant}>{meta.label}</Badge>
        <span className="ml-2 font-mono text-xs text-muted-foreground">
          {s.source}
        </span>
      </TableCell>
      <TableCell className="tabular-nums font-medium">
        {s.stocks.toLocaleString()} 只
      </TableCell>
      <TableCell className="tabular-nums text-muted-foreground">
        {s.rows.toLocaleString()} 行
      </TableCell>
      <TableCell className="font-mono text-xs">{range}</TableCell>
      <TableCell>
        {s.codes.length > 0 ? (
          <div className="flex max-w-md flex-wrap gap-1">
            {s.codes.map((code) => (
              <Badge key={code} variant="outline" className="font-mono text-xs">
                {code}
              </Badge>
            ))}
          </div>
        ) : s.stocks > 0 ? (
          <span className="text-xs text-muted-foreground">
            股票数较多，省略代码清单
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        )}
      </TableCell>
    </TableRow>
  );
};

const SourceDistributionCard = () => {
  const { data, isLoading, isError } = useSourceStats();
  const refresh = useRefreshSourceStats();
  // 初次加载或后端缓存命中均为「读缓存」；点刷新走 mutation 强制重算。
  const busy = isLoading || refresh.isPending;
  const stats = data?.stats ?? [];
  return (
    <Card className="mb-6">
      <CardHeader className="flex flex-row items-center justify-between gap-2 pb-3">
        <div>
          <CardTitle className="text-lg">数据来源分布（K 线落库 source 聚合）</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">
            {data
              ? `统计于 ${formatDateTime(data.generated_at)}${data.cached ? " · 来自缓存" : " · 实时重算"}`
              : "全库聚合，结果按 30 分钟缓存"}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={() => void refresh.mutateAsync()}
        >
          <RefreshCw
            className={`mr-1 h-3 w-3 ${refresh.isPending ? "animate-spin" : ""}`}
          />
          刷新
        </Button>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <p className="text-muted-foreground">统计中…（数千万行分组，约数秒）</p>
        ) : isError ? (
          <p className="text-destructive">统计失败，请稍后重试</p>
        ) : !stats.length ? (
          <EmptyState message="K 线库暂无数据" />
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>来源</TableHead>
                  <TableHead>覆盖股票数</TableHead>
                  <TableHead>数据行数</TableHead>
                  <TableHead>日期区间</TableHead>
                  <TableHead>股票代码（≤100 只时列出）</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {stats.map((s) => (
                  <SourceRow key={s.source} s={s} />
                ))}
              </TableBody>
            </Table>
            <p className="mt-3 text-xs text-muted-foreground">
              baostock 为日 K 全量与周级对齐采集源；gotdx 为盘后增量来源（对齐后会被翻为
              baostock）。覆盖股票数 ≤ 100 时列出具体代码，便于核对增量/对齐进度。
            </p>
          </>
        )}
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
