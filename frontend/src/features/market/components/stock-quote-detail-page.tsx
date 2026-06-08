import { ArrowLeft, Loader2 } from "lucide-react";
import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { IntradayChart } from "@/components/common/intraday-chart";
import {
  KlineChart,
  MA_PERIODS,
  MA_COLOR,
  SUB_INDICATORS,
  indicatorTypesFor,
  type TMAPeriod,
  type TSubIndicatorKey,
} from "@/components/common/kline-chart";
import { PageHeader } from "@/components/common/page-header";
import { QuoteCell } from "@/components/common/quote-cell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/cn";
import { formatPrice } from "@/lib/decimal";
import type { EKlineAdjust, EKlinePeriod } from "@/types/market";
import {
  useStockIndicators,
  useStockIntraday,
  useStockKline,
  useStockKlineIndicators,
  useStockQuote,
} from "../hooks/use-market";

const DEFAULT_MAS: TMAPeriod[] = [5, 10, 20, 60];

export const StockQuoteDetailPage = () => {
  const navigate = useNavigate();
  const { code = "" } = useParams<{ code: string }>();
  const [period, setPeriod] = useState<EKlinePeriod>("day");
  const [adjust, setAdjust] = useState<EKlineAdjust>("qfq");
  const [enabledMAs, setEnabledMAs] = useState<TMAPeriod[]>(DEFAULT_MAS);
  const [showBoll, setShowBoll] = useState(false);
  const [enabledPanes, setEnabledPanes] = useState<TSubIndicatorKey[]>(["macd"]);

  // 绘图所需指标类型 → 一次性向后端请求（前端不再手算）
  const indicatorTypes = indicatorTypesFor(enabledMAs, {
    boll: showBoll,
    panes: enabledPanes,
  });

  const {
    data: quote,
    isLoading,
    isError,
  } = useStockQuote(code || null);
  const { data: klineData } = useStockKlineIndicators(
    code || null,
    period,
    adjust,
    indicatorTypes,
  );
  const klines = klineData?.bars;
  const klineIndicators = klineData?.indicators;
  // 做 T 历史基准固定用日线前复权，独立于上方 K 线周期选择
  const { data: dayKlines } = useStockKline(code || null, "day", "qfq");
  const { data: intraday } = useStockIntraday(code || null);
  const { data: indicators } = useStockIndicators(code || null);

  const toggleMA = (p: TMAPeriod) =>
    setEnabledMAs((prev) =>
      prev.includes(p) ? prev.filter((x) => x !== p) : [...prev, p],
    );
  const togglePane = (key: TSubIndicatorKey) =>
    setEnabledPanes((prev) =>
      prev.includes(key) ? prev.filter((x) => x !== key) : [...prev, key],
    );

  const backButton = (
    <Button variant="ghost" size="sm" onClick={() => navigate("/market/quote")}>
      <ArrowLeft className="mr-1 h-4 w-4" />
      返回搜索
    </Button>
  );

  if (isLoading) {
    return (
      <>
        <div className="mb-2">{backButton}</div>
        <div className="flex flex-col items-center justify-center py-24 text-muted-foreground">
          <Loader2 className="mb-3 h-8 w-8 animate-spin" />
          正在加载 {code} 行情数据…
        </div>
      </>
    );
  }

  if (isError || !quote) {
    return (
      <>
        <div className="mb-2">{backButton}</div>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-24 text-center">
            <p className="text-lg font-semibold">未找到该股票行情</p>
            <p className="mt-2 text-sm text-muted-foreground">
              代码 <span className="font-mono">{code}</span> 不存在或暂无数据，请确认后重试。
            </p>
            <Button className="mt-6" onClick={() => navigate("/market/quote")}>
              返回搜索
            </Button>
          </CardContent>
        </Card>
      </>
    );
  }

  const displayName =
    quote.stock_name && quote.stock_name !== quote.stock_code
      ? quote.stock_name
      : quote.stock_code;

  return (
    <>
      <div className="mb-2">{backButton}</div>
      <PageHeader title="个股详情" description="标的快照、K 线与均线指标" />

      <Card className="mb-6">
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle>
            {displayName}
            <span className="ml-2 font-mono text-sm text-muted-foreground">
              {quote.stock_code}
            </span>
          </CardTitle>
          {quote.stale ? <Badge variant="warning">数据延迟</Badge> : null}
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-6">
            <div className="text-4xl font-bold">{formatPrice(quote.price)}</div>
            <QuoteCell
              value={quote.change_percent}
              type="percent"
              className="text-lg"
            />
          </div>
          <div className="mt-4 grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
            <div>
              <span className="text-muted-foreground">开 </span>
              {formatPrice(quote.open)}
            </div>
            <div>
              <span className="text-muted-foreground">高 </span>
              {formatPrice(quote.high)}
            </div>
            <div>
              <span className="text-muted-foreground">低 </span>
              {formatPrice(quote.low)}
            </div>
            <div>
              <span className="text-muted-foreground">换手 </span>
              {formatPrice(quote.turnover_rate)}%
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="mb-6">
        <CardHeader className="pb-2">
          <CardTitle className="text-lg">
            分时走势
            {intraday?.trade_date ? (
              <span className="ml-2 text-sm font-normal text-muted-foreground">
                {intraday.trade_date}
              </span>
            ) : null}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {intraday?.points?.length ? (
            <IntradayChart intraday={intraday} klines={dayKlines} />
          ) : (
            <p className="py-8 text-center text-muted-foreground">
              暂无分时数据
            </p>
          )}
        </CardContent>
      </Card>

      <div className="mb-4 flex flex-wrap items-center gap-4">
        <Tabs value={period} onValueChange={(v) => setPeriod(v as EKlinePeriod)}>
          <TabsList>
            <TabsTrigger value="day">日K</TabsTrigger>
            <TabsTrigger value="week">周K</TabsTrigger>
            <TabsTrigger value="month">月K</TabsTrigger>
          </TabsList>
        </Tabs>
        <Select value={adjust} onValueChange={(v) => setAdjust(v as EKlineAdjust)}>
          <SelectTrigger className="w-[120px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="qfq">前复权</SelectItem>
            <SelectItem value="hfq">后复权</SelectItem>
            <SelectItem value="none">不复权</SelectItem>
          </SelectContent>
        </Select>
        <div className="flex flex-wrap items-center gap-1.5">
          {MA_PERIODS.map((p) => {
            const active = enabledMAs.includes(p);
            return (
              <button
                key={p}
                type="button"
                onClick={() => toggleMA(p)}
                className={cn(
                  "flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs transition-colors",
                  active
                    ? "border-transparent bg-muted font-medium"
                    : "text-muted-foreground hover:bg-muted/60",
                )}
              >
                <span
                  className="h-2 w-2 rounded-full"
                  style={{
                    backgroundColor: active ? MA_COLOR[p] : "transparent",
                    border: active ? undefined : `1px solid ${MA_COLOR[p]}`,
                  }}
                />
                MA{p}
              </button>
            );
          })}
        </div>
      </div>

      {/* 指标开关：BOLL 主图叠加 + MACD/KDJ/RSI/ATR/动量 副图，数据全部来自后端接口 */}
      <div className="mb-4 flex flex-wrap items-center gap-1.5">
        <span className="mr-1 text-xs text-muted-foreground">指标</span>
        <IndicatorToggle
          label="BOLL"
          active={showBoll}
          onClick={() => setShowBoll((v) => !v)}
        />
        {SUB_INDICATORS.map((g) => (
          <IndicatorToggle
            key={g.key}
            label={g.label}
            active={enabledPanes.includes(g.key)}
            onClick={() => togglePane(g.key)}
          />
        ))}
      </div>

      <Card className="mb-6">
        <CardContent className="pt-6">
          {klines?.length ? (
            <KlineChart
              klines={klines}
              indicators={klineIndicators}
              enabledMAs={enabledMAs}
              showBoll={showBoll}
              enabledPanes={enabledPanes}
              height={520 + enabledPanes.length * 120}
            />
          ) : (
            <p className="py-8 text-center text-muted-foreground">
              暂无 K 线数据
            </p>
          )}
        </CardContent>
      </Card>

      {indicators?.values ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">
              技术指标快照（{indicators.trade_date}）
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
              {Object.entries(indicators.values).map(([key, val]) => (
                <div key={key} className="rounded-md border p-3 text-center">
                  <div className="text-xs uppercase text-muted-foreground">
                    {key}
                  </div>
                  <div className="mt-1 text-lg font-semibold">
                    {formatPrice(val)}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      ) : null}
    </>
  );
};

const IndicatorToggle = ({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) => (
  <button
    type="button"
    onClick={onClick}
    className={cn(
      "rounded-full border px-2.5 py-1 text-xs transition-colors",
      active
        ? "border-transparent bg-muted font-medium"
        : "text-muted-foreground hover:bg-muted/60",
    )}
  >
    {label}
  </button>
);
