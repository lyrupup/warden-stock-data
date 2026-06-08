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
import type { TIndicatorResult, TKline } from "@/types/market";

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

/** BOLL 主图叠加三轨（上轨红 / 中轨紫 / 下轨蓝） */
export const BOLL_SERIES = [
  { type: "boll_upper", label: "UP", color: "#ea580c" },
  { type: "boll_mid", label: "MID", color: "#a855f7" },
  { type: "boll_lower", label: "LOW", color: "#2563eb" },
] as const;

/** 副图指标组：每组独立一个子窗格，line 画线、hist 画柱 */
export const SUB_INDICATORS = [
  {
    key: "macd",
    label: "MACD",
    series: [
      { type: "macd_bar", label: "BAR", kind: "hist", color: "#9ca3af" },
      { type: "macd_dif", label: "DIF", kind: "line", color: "#ea580c" },
      { type: "macd_dea", label: "DEA", kind: "line", color: "#2563eb" },
    ],
  },
  {
    key: "kdj",
    label: "KDJ",
    series: [
      { type: "kdj_k", label: "K", kind: "line", color: "#ea580c" },
      { type: "kdj_d", label: "D", kind: "line", color: "#2563eb" },
      { type: "kdj_j", label: "J", kind: "line", color: "#be185d" },
    ],
  },
  {
    key: "rsi",
    label: "RSI",
    series: [
      { type: "rsi6", label: "RSI6", kind: "line", color: "#ea580c" },
      { type: "rsi12", label: "RSI12", kind: "line", color: "#2563eb" },
      { type: "rsi24", label: "RSI24", kind: "line", color: "#be185d" },
    ],
  },
  {
    key: "atr",
    label: "ATR",
    series: [
      { type: "atr14", label: "ATR14", kind: "line", color: "#ea580c" },
      { type: "atr20", label: "ATR20", kind: "line", color: "#2563eb" },
    ],
  },
  {
    key: "mom",
    label: "动量%",
    series: [
      { type: "pct_change20", label: "20日", kind: "line", color: "#ea580c" },
      { type: "pct_change60", label: "60日", kind: "line", color: "#2563eb" },
    ],
  },
] as const;

export type TSubIndicatorKey = (typeof SUB_INDICATORS)[number]["key"];

/** 由启用的 MA / BOLL / 副图集合推导出需要向后端请求的指标类型集合 */
export const indicatorTypesFor = (
  mas: readonly TMAPeriod[],
  opts: { boll?: boolean; panes?: readonly TSubIndicatorKey[] },
): string[] => {
  const set = new Set<string>();
  mas.forEach((p) => set.add(`ma${p}`));
  if (opts.boll) BOLL_SERIES.forEach((s) => set.add(s.type));
  (opts.panes ?? []).forEach((key) => {
    SUB_INDICATORS.find((g) => g.key === key)?.series.forEach((s) =>
      set.add(s.type),
    );
  });
  return [...set];
};

/** A 股配色：涨红跌绿 */
const UP_COLOR = "#dc2626";
const DOWN_COLOR = "#16a34a";

/** 成交量柱用半透明涨跌色，避免压过主图 K 线 */
const UP_VOLUME_COLOR = "rgba(220,38,38,0.7)";
const DOWN_VOLUME_COLOR = "rgba(22,163,74,0.7)";

const parseTime = (date: string): UTCTimestamp =>
  Math.floor(new Date(date).getTime() / 1000) as UTCTimestamp;

/** 收盘价简单移动平均；保留导出供其它模块复用（图表绘制已改为读后端指标） */
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
  /** 后端逐 bar 指标（与 klines 按 trade_date 对齐） */
  indicators?: TIndicatorResult[];
  /** 叠加的 MA 周期集合 */
  enabledMAs?: readonly TMAPeriod[];
  /** 主图叠加 BOLL 通道 */
  showBoll?: boolean;
  /** 启用的副图指标组 */
  enabledPanes?: readonly TSubIndicatorKey[];
  height?: number;
  /** 初始可视 K 线根数；超过总数则按总数 fit。默认 60（约 3 个月日线） */
  initialVisibleBars?: number;
};

/** K 线图（lightweight-charts v5）：MA/BOLL 主图叠加 + MACD/KDJ/RSI/ATR/动量 副图，指标值全部来自后端接口 */
export const KlineChart = ({
  klines,
  indicators = [],
  enabledMAs = [],
  showBoll = false,
  enabledPanes = [],
  height = 520,
  initialVisibleBars = 60,
}: TKlineChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const extraRefs = useRef<ISeriesApi<"Line" | "Histogram">[]>([]);

  const [hoverTime, setHoverTime] = useState<number | null>(null);

  const maKey = useMemo(
    () => [...enabledMAs].sort((a, b) => a - b).join(","),
    [enabledMAs],
  );
  const paneKey = useMemo(() => [...enabledPanes].join(","), [enabledPanes]);

  const byTime = useMemo(() => {
    const m = new Map<number, TKline>();
    klines.forEach((k) => m.set(parseTime(k.trade_date), k));
    return m;
  }, [klines]);

  // 后端逐 bar 指标：time -> { type -> number }
  const indByTime = useMemo(() => {
    const m = new Map<number, Record<string, number>>();
    indicators.forEach((r) => {
      const rec: Record<string, number> = {};
      Object.entries(r.values).forEach(([k, v]) => {
        rec[k] = toNumber(v);
      });
      m.set(parseTime(r.trade_date), rec);
    });
    return m;
  }, [indicators]);

  const latest = klines.length > 0 ? klines[klines.length - 1] : null;
  const display =
    (hoverTime != null ? byTime.get(hoverTime) : undefined) ?? latest;
  const displayInd = display ? indByTime.get(parseTime(display.trade_date)) : undefined;

  // 主图/副图布局变化（高度或副图集合）时重建图表，避免遗留空窗格
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
      extraRefs.current = [];
      chart.remove();
      chartRef.current = null;
      candleRef.current = null;
      volumeRef.current = null;
    };
  }, [height, paneKey]);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || !candleRef.current || klines.length === 0) return;

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

    extraRefs.current.forEach((s) => chart.removeSeries(s));
    extraRefs.current = [];

    // 取某指标类型在各 bar 上的折线数据（缺失点跳过）
    const lineOf = (type: string): LineData<UTCTimestamp>[] => {
      const out: LineData<UTCTimestamp>[] = [];
      for (const k of klines) {
        const t = parseTime(k.trade_date);
        const v = indByTime.get(t)?.[type];
        if (v !== undefined) out.push({ time: t, value: v });
      }
      return out;
    };

    const addLine = (type: string, color: string, pane: number) => {
      const data = lineOf(type);
      if (data.length === 0) return;
      const line = chart.addSeries(
        LineSeries,
        {
          color,
          lineWidth: 1,
          priceLineVisible: false,
          lastValueVisible: false,
          crosshairMarkerVisible: false,
        },
        pane,
      );
      line.setData(data);
      extraRefs.current.push(line);
    };

    // 主图：MA 叠加
    [...enabledMAs]
      .sort((a, b) => a - b)
      .forEach((p) => addLine(`ma${p}`, MA_COLOR[p], 0));

    // 主图：BOLL 通道
    if (showBoll) BOLL_SERIES.forEach((s) => addLine(s.type, s.color, 0));

    // 副图：每组一个窗格（从 pane 2 起，pane 1 为成交量）
    let paneIdx = 2;
    for (const key of enabledPanes) {
      const group = SUB_INDICATORS.find((g) => g.key === key);
      if (!group) continue;
      const idx = paneIdx;
      for (const s of group.series) {
        if (s.kind === "hist") {
          const data = klines
            .map((k) => {
              const t = parseTime(k.trade_date);
              const v = indByTime.get(t)?.[s.type];
              if (v === undefined) return null;
              return { time: t, value: v, color: v >= 0 ? UP_COLOR : DOWN_COLOR };
            })
            .filter((x): x is NonNullable<typeof x> => x !== null);
          if (data.length > 0) {
            const hist = chart.addSeries(
              HistogramSeries,
              { priceLineVisible: false, lastValueVisible: false },
              idx,
            );
            hist.setData(data);
            extraRefs.current.push(hist);
          }
        } else {
          addLine(s.type, s.color, idx);
        }
      }
      paneIdx += 1;
    }

    // 窗格高度比例：主图 3，成交量 1，各副图 1.5
    const panes = chart.panes();
    panes[0]?.setStretchFactor(3);
    panes[1]?.setStretchFactor(1);
    for (let i = 2; i < panes.length; i++) panes[i]?.setStretchFactor(1.5);

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
  }, [klines, indByTime, maKey, showBoll, paneKey, initialVisibleBars]);

  return (
    <div>
      {display ? (
        <div className="mb-3 space-y-1">
          <OhlcStrip k={display} isHover={hoverTime !== null} />
          {enabledMAs.length > 0 ? (
            <MaStrip
              items={[...enabledMAs]
                .sort((a, b) => a - b)
                .map((p) => ({ period: p, value: displayInd?.[`ma${p}`] }))}
            />
          ) : null}
          {showBoll ? (
            <IndStrip
              label="BOLL"
              items={BOLL_SERIES.map((s) => ({
                label: s.label,
                color: s.color,
                value: displayInd?.[s.type],
              }))}
              digits={2}
            />
          ) : null}
          {enabledPanes.map((key) => {
            const g = SUB_INDICATORS.find((x) => x.key === key);
            if (!g) return null;
            return (
              <IndStrip
                key={key}
                label={g.label}
                items={g.series.map((s) => ({
                  label: s.label,
                  color: s.color,
                  value: displayInd?.[s.type],
                }))}
                digits={2}
              />
            );
          })}
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

const IndStrip = ({
  label,
  items,
  digits = 2,
}: {
  label: string;
  items: { label: string; color: string; value: number | undefined }[];
  digits?: number;
}) => (
  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs tabular-nums">
    <span className="text-muted-foreground">{label}</span>
    {items.map((it) => (
      <span key={it.label} className="font-medium" style={{ color: it.color }}>
        {it.label} {it.value !== undefined ? it.value.toFixed(digits) : "--"}
      </span>
    ))}
  </div>
);
