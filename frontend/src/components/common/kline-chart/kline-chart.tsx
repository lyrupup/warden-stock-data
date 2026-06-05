import {
  CandlestickSeries,
  ColorType,
  createChart,
  HistogramSeries,
  LineSeries,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type UTCTimestamp,
} from "lightweight-charts";
import { useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/cn";
import { changeColor, formatPrice, formatVolume, toNumber } from "@/lib/decimal";
import type { TKline } from "@/types/market";

/** 可叠加的 MA 周期，覆盖国内常用 5/10/20/30/60/120 日均线 */
export const MA_PERIODS = [5, 10, 20, 30, 60, 120] as const;
export type TMAPeriod = (typeof MA_PERIODS)[number];

/** MA 系列配色，对齐同花顺/通达信经典：橙→黄→紫→绿→蓝→玫红 */
export const MA_COLOR: Record<TMAPeriod, string> = {
  5: "#ea580c",
  10: "#eab308",
  20: "#a855f7",
  30: "#15803d",
  60: "#2563eb",
  120: "#be185d",
};

/** A 股配色：涨红跌绿 */
const UP_COLOR = "#dc2626";
const DOWN_COLOR = "#16a34a";

/** 成交量柱用半透明涨跌色，避免压过主图 K 线 */
const UP_VOLUME_COLOR = "rgba(220,38,38,0.7)";
const DOWN_VOLUME_COLOR = "rgba(22,163,74,0.7)";

const parseTime = (date: string): UTCTimestamp =>
  Math.floor(new Date(date).getTime() / 1000) as UTCTimestamp;

/** 收盘价简单移动平均；样本不足该期不输出，避免首端误导性均值 */
export const computeMA = (
  klines: TKline[],
  period: number,
): LineData<UTCTimestamp>[] => {
  if (period <= 0 || klines.length < period) return [];
  const out: LineData<UTCTimestamp>[] = [];
  let sum = 0;
  for (let i = 0; i < klines.length; i++) {
    sum += toNumber(klines[i].close);
    if (i >= period) sum -= toNumber(klines[i - period].close);
    if (i >= period - 1) {
      out.push({
        time: parseTime(klines[i].trade_date),
        value: +(sum / period).toFixed(4),
      });
    }
  }
  return out;
};

type TKlineChartProps = {
  klines: TKline[];
  /** 叠加的 MA 周期集合 */
  enabledMAs?: readonly TMAPeriod[];
  height?: number;
  /** 初始可视 K 线根数；超过总数则按总数 fit。默认 60（约 3 个月日线） */
  initialVisibleBars?: number;
};

/** K 线图（lightweight-charts v5，可叠加 MA + hover OHLC 浮层） */
export const KlineChart = ({
  klines,
  enabledMAs = [],
  height = 520,
  initialVisibleBars = 60,
}: TKlineChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const lineRefs = useRef<ISeriesApi<"Line">[]>([]);

  const [hoverTime, setHoverTime] = useState<number | null>(null);

  const maKey = useMemo(
    () => [...enabledMAs].sort((a, b) => a - b).join(","),
    [enabledMAs],
  );

  const byTime = useMemo(() => {
    const m = new Map<number, TKline>();
    klines.forEach((k) => m.set(parseTime(k.trade_date), k));
    return m;
  }, [klines]);

  const latest = klines.length > 0 ? klines[klines.length - 1] : null;
  const display =
    (hoverTime != null ? byTime.get(hoverTime) : undefined) ?? latest;

  const maMaps = useMemo(() => {
    const out: Partial<Record<TMAPeriod, Map<number, number>>> = {};
    for (const p of enabledMAs) {
      out[p] = new Map(
        computeMA(klines, p).map((it) => [it.time as number, it.value]),
      );
    }
    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [klines, maKey]);

  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      height,
      layout: {
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: "#9ca3af",
      },
      grid: {
        vertLines: { color: "rgba(128,128,128,0.2)" },
        horzLines: { color: "rgba(128,128,128,0.2)" },
      },
      rightPriceScale: { borderVisible: false },
      timeScale: { borderVisible: false },
    });

    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: UP_COLOR,
      downColor: DOWN_COLOR,
      borderUpColor: UP_COLOR,
      borderDownColor: DOWN_COLOR,
      wickUpColor: UP_COLOR,
      wickDownColor: DOWN_COLOR,
    });

    const volumeSeries = chart.addSeries(
      HistogramSeries,
      {
        priceFormat: { type: "custom", formatter: formatVolume, minMove: 1 },
        priceLineVisible: false,
        lastValueVisible: false,
      },
      1,
    );
    volumeSeries.priceScale().applyOptions({
      borderVisible: false,
      scaleMargins: { top: 0.1, bottom: 0 },
    });

    // 主图与成交量子图按 3:1 分配高度
    chart.panes()[0]?.setStretchFactor(3);
    chart.panes()[1]?.setStretchFactor(1);

    chartRef.current = chart;
    candleRef.current = candleSeries;
    volumeRef.current = volumeSeries;

    const handler = (param: { time?: unknown }) => {
      setHoverTime(typeof param.time === "number" ? param.time : null);
    };
    chart.subscribeCrosshairMove(handler);

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) chart.applyOptions({ width: entry.contentRect.width });
    });
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      chart.unsubscribeCrosshairMove(handler);
      lineRefs.current.forEach((s) => chart.removeSeries(s));
      lineRefs.current = [];
      chart.remove();
      chartRef.current = null;
      candleRef.current = null;
      volumeRef.current = null;
    };
  }, [height]);

  useEffect(() => {
    if (!candleRef.current || klines.length === 0) return;

    candleRef.current.setData(
      klines.map((k) => ({
        time: parseTime(k.trade_date),
        open: toNumber(k.open),
        high: toNumber(k.high),
        low: toNumber(k.low),
        close: toNumber(k.close),
      })),
    );

    volumeRef.current?.setData(
      klines.map((k) => ({
        time: parseTime(k.trade_date),
        value: toNumber(k.volume),
        color:
          toNumber(k.close) >= toNumber(k.open)
            ? UP_VOLUME_COLOR
            : DOWN_VOLUME_COLOR,
      })),
    );

    const chart = chartRef.current;
    if (!chart) return;

    lineRefs.current.forEach((s) => chart.removeSeries(s));
    lineRefs.current = [];

    [...enabledMAs]
      .sort((a, b) => a - b)
      .forEach((p) => {
        const data = computeMA(klines, p);
        if (data.length === 0) return;
        const line = chart.addSeries(LineSeries, {
          color: MA_COLOR[p],
          lineWidth: 1,
          priceLineVisible: false,
          lastValueVisible: false,
          crosshairMarkerVisible: false,
        });
        line.setData(data);
        lineRefs.current.push(line);
      });

    const bars = Math.min(initialVisibleBars, klines.length);
    if (bars > 0 && bars < klines.length) {
      chart.timeScale().setVisibleLogicalRange({
        from: klines.length - bars - 0.5,
        to: klines.length - 0.5,
      });
    } else {
      chart.timeScale().fitContent();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [klines, maKey, initialVisibleBars]);

  const maItems = useMemo(
    () =>
      [...enabledMAs]
        .sort((a, b) => a - b)
        .map((p) => ({
          period: p,
          value: display
            ? maMaps[p]?.get(parseTime(display.trade_date))
            : undefined,
        })),
    [enabledMAs, display, maMaps],
  );

  return (
    <div>
      {display ? (
        <div className="mb-3 space-y-1">
          <OhlcStrip k={display} isHover={hoverTime !== null} />
          {maItems.length > 0 ? <MaStrip items={maItems} /> : null}
        </div>
      ) : null}
      <div
        ref={containerRef}
        className="w-full"
        onMouseLeave={() => setHoverTime(null)}
      />
    </div>
  );
};

const OhlcStrip = ({ k, isHover }: { k: TKline; isHover: boolean }) => {
  const open = toNumber(k.open);
  const close = toNumber(k.close);
  const change = close - open;
  const color = changeColor(change);
  const pct = open === 0 ? 0 : (change / open) * 100;
  const sign = change >= 0 ? "+" : "";

  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs tabular-nums">
      <span
        className={cn(
          "text-muted-foreground",
          isHover && "font-medium text-foreground",
        )}
      >
        {k.trade_date.slice(0, 10)}
      </span>
      <Item label="开" value={k.open} color={color} />
      <Item label="高" value={k.high} color={color} />
      <Item label="低" value={k.low} color={color} />
      <Item label="收" value={k.close} color={color} />
      <span className={color}>
        {sign}
        {formatPrice(change)} ({sign}
        {pct.toFixed(2)}%)
      </span>
      <span>
        <span className="text-muted-foreground">量 </span>
        <span className="font-medium text-foreground">
          {formatVolume(k.volume)}
        </span>
      </span>
    </div>
  );
};

const Item = ({
  label,
  value,
  color,
}: {
  label: string;
  value: string | number;
  color: string;
}) => (
  <span>
    <span className="text-muted-foreground">{label} </span>
    <span className={cn("font-medium", color)}>{formatPrice(value)}</span>
  </span>
);

const MaStrip = ({
  items,
}: {
  items: { period: TMAPeriod; value: number | undefined }[];
}) => (
  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs tabular-nums">
    {items.map(({ period, value }) => (
      <span
        key={period}
        className="font-medium"
        style={{ color: MA_COLOR[period] }}
      >
        MA{period} {value !== undefined ? formatPrice(value) : "--"}
      </span>
    ))}
  </div>
);
