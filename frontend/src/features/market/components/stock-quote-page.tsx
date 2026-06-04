import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import type { LineData, UTCTimestamp } from "lightweight-charts";
import { PageHeader } from "@/components/common/page-header";
import { KlineChart } from "@/components/common/kline-chart";
import { QuoteCell } from "@/components/common/quote-cell";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { formatPrice, toNumber } from "@/lib/decimal";
import type { EKlineAdjust, EKlinePeriod, TStockBrief } from "@/types/market";
import {
  useStockIndicators,
  useStockKline,
  useStockQuote,
  useStockSearch,
} from "../hooks/use-market";

export const StockQuotePage = () => {
  const [kw, setKw] = useState("");
  const [selected, setSelected] = useState<TStockBrief | null>(null);
  const [period, setPeriod] = useState<EKlinePeriod>("day");
  const [adjust, setAdjust] = useState<EKlineAdjust>("qfq");

  const { data: searchResults } = useStockSearch(kw);
  const code = selected?.stock_code ?? null;
  const { data: quote } = useStockQuote(code);
  const { data: klines } = useStockKline(code, period, adjust);
  const { data: indicators } = useStockIndicators(code);

  const maLines = useMemo(() => {
    if (!klines?.length || !indicators?.values) return {};
    const result: Record<string, LineData<UTCTimestamp>[]> = {};
    const maKeys = ["ma5", "ma10", "ma20", "ma30", "ma60"] as const;

    maKeys.forEach((key) => {
      const val = indicators.values[key];
      if (!val) return;
      const lastBar = klines[klines.length - 1];
      result[key] = [
        {
          time: (new Date(lastBar.date).getTime() / 1000) as UTCTimestamp,
          value: toNumber(val),
        },
      ];
    });
    return result;
  }, [klines, indicators]);

  return (
    <>
      <PageHeader title="个股行情" description="搜索标的查看快照、K 线与均线指标" />

      <div className="relative mb-6 max-w-md">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          className="pl-9"
          placeholder="输入代码或名称搜索…"
          value={kw}
          onChange={(e) => {
            setKw(e.target.value);
            setSelected(null);
          }}
        />
        {searchResults && searchResults.length > 0 && kw.length >= 2 && !selected ? (
          <div className="absolute z-10 mt-1 w-full rounded-md border bg-background shadow-lg">
            {searchResults.map((s) => (
              <button
                key={s.stock_code}
                type="button"
                className="flex w-full items-center justify-between px-4 py-2 text-left text-sm hover:bg-muted"
                onClick={() => {
                  setSelected(s);
                  setKw(`${s.stock_code} ${s.stock_name}`);
                }}
              >
                <span className="font-mono">{s.stock_code}</span>
                <span className="text-muted-foreground">{s.stock_name}</span>
              </button>
            ))}
          </div>
        ) : null}
      </div>

      {quote ? (
        <>
          <Card className="mb-6">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle>
                {quote.stock_name}
                <span className="ml-2 font-mono text-sm text-muted-foreground">
                  {quote.stock_code}
                </span>
              </CardTitle>
              {quote.stale ? <Badge variant="warning">数据延迟</Badge> : null}
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap items-end gap-6">
                <div className="text-4xl font-bold">{formatPrice(quote.price)}</div>
                <QuoteCell value={quote.change_percent} type="percent" className="text-lg" />
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
                <SelectItem value="">不复权</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Card className="mb-6">
            <CardContent className="pt-6">
              {klines?.length ? (
                <KlineChart klines={klines} maLines={maLines} />
              ) : (
                <p className="py-8 text-center text-muted-foreground">K 线加载中…</p>
              )}
            </CardContent>
          </Card>

          {indicators?.values ? (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">技术指标（{indicators.trade_date}）</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
                  {Object.entries(indicators.values).map(([key, val]) => (
                    <div key={key} className="rounded-md border p-3 text-center">
                      <div className="text-xs uppercase text-muted-foreground">{key}</div>
                      <div className="mt-1 text-lg font-semibold">{formatPrice(val)}</div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          ) : null}
        </>
      ) : (
        <p className="text-muted-foreground">请搜索并选择一只股票</p>
      )}
    </>
  );
};
