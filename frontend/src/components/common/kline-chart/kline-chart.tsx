import {
  CandlestickSeries,
  ColorType,
  createChart,
  HistogramSeries,
  LineSeries,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type LogicalRange,
  type UTCTimestamp,
} from "lightweight-charts";
import { useEffect, useMemo, useRef, useState } from "react";
import { formatPrice, formatVolume, toNumber } from "@/lib/decimal";
import { toast } from "@/stores/toast-store";
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

/** BOLL 副图说明（浮动图例用） */
export const BOLL_DESC = "布林通道";

/** 副图指标组：每组独立一个子窗格，line 画线、hist 画柱；desc 为浮动图例的简短说明 */
export const SUB_INDICATORS = [
  {
    key: "macd",
    label: "MACD",
    desc: "指数平滑异同",
    series: [
      { type: "macd_bar", label: "BAR", kind: "hist", color: "#9ca3af" },
      { type: "macd_dif", label: "DIF", kind: "line", color: "#ea580c" },
      { type: "macd_dea", label: "DEA", kind: "line", color: "#2563eb" },
    ],
  },
  {
    key: "kdj",
    label: "KDJ",
    desc: "随机指标",
    series: [
      { type: "kdj_k", label: "K", kind: "line", color: "#ea580c" },
      { type: "kdj_d", label: "D", kind: "line", color: "#2563eb" },
      { type: "kdj_j", label: "J", kind: "line", color: "#be185d" },
    ],
  },
  {
    key: "rsi",
    label: "RSI",
    desc: "相对强弱",
    series: [
      { type: "rsi6", label: "RSI6", kind: "line", color: "#ea580c" },
      { type: "rsi12", label: "RSI12", kind: "line", color: "#2563eb" },
      { type: "rsi24", label: "RSI24", kind: "line", color: "#be185d" },
    ],
  },
  {
    key: "atr",
    label: "ATR",
    desc: "真实波幅",
    series: [
      { type: "atr14", label: "ATR14", kind: "line", color: "#ea580c" },
      { type: "atr20", label: "ATR20", kind: "line", color: "#2563eb" },
    ],
  },
  {
    key: "mom",
    label: "动量%",
    desc: "价格动量",
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

/**
 * 全量指标类型集合：详情页固定一次性请求全部，使切换 MA/BOLL/副图开关时
 * 请求 queryKey 不变（命中缓存、不重拉、不闪烁），开关仅控制前端绘制。
 */
export const ALL_INDICATOR_TYPES: string[] = (() => {
  const set = new Set<string>();
  MA_PERIODS.forEach((p) => set.add(`ma${p}`));
  BOLL_SERIES.forEach((s) => set.add(s.type));
  SUB_INDICATORS.forEach((g) => g.series.forEach((s) => set.add(s.type)));
  return [...set];
})();

/** A 股配色：涨红跌绿 */
const UP_COLOR = "#dc2626";
const DOWN_COLOR = "#16a34a";

/** 成交量柱用半透明涨跌色，避免压过主图 K 线 */
const UP_VOLUME_COLOR = "rgba(220,38,38,0.7)";
const DOWN_VOLUME_COLOR = "rgba(22,163,74,0.7)";

const GRAY = "#9ca3af";

/** 距左边界小于该逻辑根数时触发加载更多历史 */
const LOAD_MORE_THRESHOLD = 8;

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
  /** 叠加的 MA 周期集合（主图，与 BOLL 互斥由上层保证） */
  enabledMAs?: readonly TMAPeriod[];
  /** 主图叠加 BOLL 通道（与 MA 互斥由上层保证） */
  showBoll?: boolean;
  /** 启用的副图指标组 */
  enabledPanes?: readonly TSubIndicatorKey[];
  height?: number;
  /** 初始可视 K 线根数；超过总数则按总数 fit。默认 60（约 3 个月日线） */
  initialVisibleBars?: number;
  /** 是否还有更早的历史可加载（左滑触发 onLoadMore） */
  hasMore?: boolean;
  /** 正在加载更多（避免重复触发） */
  isLoadingMore?: boolean;
  /** 左滑接近左边界时触发：上层增大 limit 拉取更早 K 线 */
  onLoadMore?: () => void;
};

/**
 * K 线图（lightweight-charts v5）：MA/BOLL 主图叠加 + MACD/KDJ/RSI/ATR/动量 副图。
 * 关键设计：图表仅创建一次；K 线/成交量与指标/副图分离管理，切换指标只增删对应
 * 系列而不重绘 K 线（无闪烁）；左滑到边界回调加载更多、右侧锁定最新一根。
 */
export const KlineChart = ({
  klines,
  indicators = [],
  enabledMAs = [],
  showBoll = false,
  enabledPanes = [],
  height = 520,
  initialVisibleBars = 60,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
}: TKlineChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  // 主图叠加（MA/BOLL）系列，独立于 K 线，切换时只增删这些
  const overlayRefs = useRef<ISeriesApi<"Line">[]>([]);
  // 副图（pane>=2）系列
  const subRefs = useRef<ISeriesApi<"Line" | "Histogram">[]>([]);
  // 各窗格浮动图例 DOM（key=paneIndex）
  const legendsRef = useRef<Map<number, HTMLDivElement>>(new Map());

  // 加载更多所需的最新状态（用 ref 传给只订阅一次的 range 监听器）
  const loadMoreRef = useRef({ hasMore, isLoadingMore, onLoadMore, len: klines.length });
  loadMoreRef.current = { hasMore, isLoadingMore, onLoadMore, len: klines.length };
  // 已对某长度触发过加载，避免滚动连续触发
  const requestedLenRef = useRef(-1);
  // 上一次数据长度 / 最新一根时间，用于区分「加载更多(前插)」与「切换周期(重置)」
  const prevLenRef = useRef(0);
  const prevLastTimeRef = useRef<number | null>(null);
  // 左滑到最左端且无更多数据时，每次「刚到达」左边界 toast 一次（离开后再滑回来会再提示）
  const atOldestToastRef = useRef(false);

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
  const displayInd = display
    ? indByTime.get(parseTime(display.trade_date))
    : undefined;

  // 取某指标类型在各 bar 上的折线数据（缺失点跳过）
  const lineOf = useMemo(() => {
    return (type: string): LineData<UTCTimestamp>[] => {
      const out: LineData<UTCTimestamp>[] = [];
      for (const k of klines) {
        const t = parseTime(k.trade_date);
        const v = indByTime.get(t)?.[type];
        if (v !== undefined) out.push({ time: t, value: v });
      }
      return out;
    };
  }, [klines, indByTime]);

  // ── 图表只创建一次（含 K 线、成交量、交互、加载更多监听）──
  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      height,
      layout: {
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: GRAY,
      },
      grid: {
        vertLines: { color: "rgba(128,128,128,0.2)" },
        horzLines: { color: "rgba(128,128,128,0.2)" },
      },
      rightPriceScale: { borderVisible: false },
      // fixRightEdge：右侧锁定最新一根，禁止右滑越过最新日期进入空白
      timeScale: { borderVisible: false, fixRightEdge: true, rightOffset: 0 },
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

    chart.panes()[0]?.setStretchFactor(3);
    chart.panes()[1]?.setStretchFactor(1);

    chartRef.current = chart;
    candleRef.current = candleSeries;
    volumeRef.current = volumeSeries;

    const handler = (param: { time?: unknown }) => {
      setHoverTime(typeof param.time === "number" ? param.time : null);
    };
    chart.subscribeCrosshairMove(handler);

    // 左滑接近左边界 → 有更多则分页加载；无更多则 toast 提示（每次滑到左端触发一次）
    const onRange = (range: LogicalRange | null) => {
      if (!range) return;
      const st = loadMoreRef.current;
      const nearLeft = range.from < LOAD_MORE_THRESHOLD;
      if (nearLeft && st.hasMore) {
        if (!st.isLoadingMore && requestedLenRef.current !== st.len) {
          requestedLenRef.current = st.len;
          st.onLoadMore?.();
        }
        atOldestToastRef.current = false;
      } else if (nearLeft && !st.hasMore) {
        if (!atOldestToastRef.current) {
          atOldestToastRef.current = true;
          toast({ title: "最老K线了哦！" });
        }
      } else {
        atOldestToastRef.current = false;
      }
    };
    chart.timeScale().subscribeVisibleLogicalRangeChange(onRange);

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) chart.applyOptions({ width: entry.contentRect.width });
    });
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      chart.unsubscribeCrosshairMove(handler);
      chart.timeScale().unsubscribeVisibleLogicalRangeChange(onRange);
      // chart.remove() 会销毁全部系列/窗格/图例 DOM，置空引用避免其它 effect 误用
      chart.remove();
      chartRef.current = null;
      candleRef.current = null;
      volumeRef.current = null;
      overlayRefs.current = [];
      subRefs.current = [];
    };
    // 仅创建一次：height 变化由独立 effect applyOptions，不重建图表
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ── 高度变化只 applyOptions，不重建（切换副图数量平滑改变高度，K 线不重绘）──
  useEffect(() => {
    chartRef.current?.applyOptions({ height });
  }, [height]);

  // ── 无更多历史时锁定左边界（fixLeftEdge），禁止越过最旧 K 线继续左滑 ──
  useEffect(() => {
    chartRef.current?.applyOptions({ timeScale: { fixLeftEdge: !hasMore } });
    if (hasMore) atOldestToastRef.current = false;
  }, [hasMore]);

  // ── K 线 + 成交量数据：仅在 klines 变化时 setData，保持可视区间 ──
  useEffect(() => {
    const chart = chartRef.current;
    const candle = candleRef.current;
    const volume = volumeRef.current;
    if (!chart || !candle || !volume || klines.length === 0) return;

    const timeScale = chart.timeScale();
    const before = timeScale.getVisibleLogicalRange();
    const newLen = klines.length;
    const newLastTime = parseTime(klines[newLen - 1].trade_date);
    const sameTail = prevLastTimeRef.current === newLastTime;

    candle.setData(
      klines.map((k) => ({
        time: parseTime(k.trade_date),
        open: toNumber(k.open),
        high: toNumber(k.high),
        low: toNumber(k.low),
        close: toNumber(k.close),
      })),
    );
    volume.setData(
      klines.map((k) => ({
        time: parseTime(k.trade_date),
        value: toNumber(k.volume),
        color:
          toNumber(k.close) >= toNumber(k.open)
            ? UP_VOLUME_COLOR
            : DOWN_VOLUME_COLOR,
      })),
    );

    if (prevLenRef.current > 0 && sameTail && newLen > prevLenRef.current) {
      // 加载更多：前插了 delta 根更早 K 线，整体右移以保持当前可视窗口
      const delta = newLen - prevLenRef.current;
      if (before) {
        timeScale.setVisibleLogicalRange({
          from: before.from + delta,
          to: before.to + delta,
        });
      }
    } else if (prevLenRef.current > 0 && sameTail && newLen === prevLenRef.current) {
      // 数据无实质变化（如已无更多可加载的回包）：维持当前视图
      if (before) timeScale.setVisibleLogicalRange(before);
    } else {
      // 首次或切换周期/复权：按初始可视根数定位最新一段
      const bars = Math.min(initialVisibleBars, newLen);
      if (bars > 0 && bars < newLen) {
        timeScale.setVisibleLogicalRange({
          from: newLen - bars - 0.5,
          to: newLen - 0.5,
        });
      } else {
        timeScale.fitContent();
      }
      requestedLenRef.current = -1;
    }

    prevLenRef.current = newLen;
    prevLastTimeRef.current = newLastTime;
  }, [klines, initialVisibleBars]);

  // ── 主图叠加（MA / BOLL）：只增删 pane 0 的叠加线，不触碰 K 线（无闪烁）──
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || klines.length === 0) return;

    overlayRefs.current.forEach((s) => chart.removeSeries(s));
    overlayRefs.current = [];

    const addLine = (type: string, color: string) => {
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
        0,
      );
      line.setData(data);
      overlayRefs.current.push(line);
    };

    if (showBoll) {
      BOLL_SERIES.forEach((s) => addLine(s.type, s.color));
    } else {
      [...enabledMAs]
        .sort((a, b) => a - b)
        .forEach((p) => addLine(`ma${p}`, MA_COLOR[p]));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [klines, indByTime, maKey, showBoll, lineOf]);

  // ── 副图（pane>=2）：只增删副图窗格与系列，不触碰 K 线（无闪烁）──
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || klines.length === 0) return;

    // 先移除旧副图系列，再回收多余窗格（图例 DOM 随窗格销毁，由 ensureLegend 重建）
    subRefs.current.forEach((s) => chart.removeSeries(s));
    subRefs.current = [];
    while (chart.panes().length > 2) {
      chart.removePane(chart.panes().length - 1);
    }

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
            subRefs.current.push(hist);
          }
        } else {
          const data = lineOf(s.type);
          if (data.length === 0) continue;
          const line = chart.addSeries(
            LineSeries,
            {
              color: s.color,
              lineWidth: 1,
              priceLineVisible: false,
              lastValueVisible: false,
              crosshairMarkerVisible: false,
            },
            idx,
          );
          line.setData(data);
          subRefs.current.push(line);
        }
      }
      paneIdx += 1;
    }

    // 窗格高度比例：主图 3，成交量 1，各副图 1.5
    const panes = chart.panes();
    panes[0]?.setStretchFactor(3);
    panes[1]?.setStretchFactor(1);
    for (let i = 2; i < panes.length; i++) panes[i]?.setStretchFactor(1.5);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [klines, indByTime, paneKey, lineOf]);

  // ── 各窗格浮动图例（仿分时图）：主图 OHLC/叠加值、量、各副图说明+值 ──
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;

    const span = (text: string, color: string) =>
      `<span style="color:${color}">${text}</span>`;

    const ensureLegend = (paneIndex: number): HTMLDivElement | null => {
      const cached = legendsRef.current.get(paneIndex);
      if (cached && cached.isConnected) return cached;
      const row = chart.panes()[paneIndex]?.getHTMLElement();
      const cell = row
        ? Array.from(row.querySelectorAll("td")).find((td) =>
            td.querySelector("canvas"),
          )
        : null;
      if (!cell) return null;
      const el = document.createElement("div");
      el.style.cssText =
        "position:absolute;left:8px;top:4px;z-index:3;pointer-events:none;" +
        "font-size:11px;line-height:1.5;white-space:nowrap;" +
        "font-variant-numeric:tabular-nums;";
      cell.appendChild(el);
      legendsRef.current.set(paneIndex, el);
      return el;
    };

    let raf = 0;
    let missing = false;
    const sync = () => {
      missing = false;
      const mainEl = ensureLegend(0);
      const volEl = ensureLegend(1);
      if (!mainEl || !volEl) missing = true;

      // 主图：日期 + OHLC + 涨跌幅 + 叠加(MA 或 BOLL)值
      if (mainEl) {
        if (display) {
          const open = toNumber(display.open);
          const close = toNumber(display.close);
          const change = close - open;
          const pct = open === 0 ? 0 : (change / open) * 100;
          const c = change > 0 ? UP_COLOR : change < 0 ? DOWN_COLOR : GRAY;
          const sign = change >= 0 ? "+" : "";
          const parts = [
            span(display.trade_date.slice(0, 10), GRAY),
            span(`开 ${formatPrice(display.open)}`, c),
            span(`高 ${formatPrice(display.high)}`, c),
            span(`低 ${formatPrice(display.low)}`, c),
            span(`收 ${formatPrice(display.close)}`, c),
            span(`${sign}${pct.toFixed(2)}%`, c),
          ];
          if (showBoll) {
            parts.push(span(BOLL_DESC, GRAY));
            BOLL_SERIES.forEach((s) => {
              const v = displayInd?.[s.type];
              parts.push(
                span(`${s.label} ${v !== undefined ? v.toFixed(2) : "--"}`, s.color),
              );
            });
          } else {
            [...enabledMAs]
              .sort((a, b) => a - b)
              .forEach((p) => {
                const v = displayInd?.[`ma${p}`];
                parts.push(
                  span(
                    `MA${p} ${v !== undefined ? formatPrice(v) : "--"}`,
                    MA_COLOR[p],
                  ),
                );
              });
          }
          mainEl.innerHTML = parts.join("&nbsp;&nbsp;");
        } else {
          mainEl.innerHTML = "";
        }
      }

      // 成交量
      if (volEl) {
        if (display) {
          const volColor =
            toNumber(display.close) >= toNumber(display.open)
              ? UP_COLOR
              : DOWN_COLOR;
          volEl.innerHTML = [
            span("成交量", GRAY),
            span(formatVolume(display.volume), volColor),
          ].join("&nbsp;&nbsp;");
        } else {
          volEl.innerHTML = "";
        }
      }

      // 各副图：说明 + 各序列值
      enabledPanes.forEach((key, i) => {
        const group = SUB_INDICATORS.find((g) => g.key === key);
        const el = ensureLegend(2 + i);
        if (!el) missing = true;
        if (!group || !el) return;
        const parts = [span(group.label, GRAY), span(group.desc, GRAY)];
        group.series.forEach((s) => {
          const v = displayInd?.[s.type];
          const color =
            s.kind === "hist" && v !== undefined
              ? v >= 0
                ? UP_COLOR
                : DOWN_COLOR
              : s.color;
          parts.push(
            span(`${s.label} ${v !== undefined ? v.toFixed(2) : "--"}`, color),
          );
        });
        el.innerHTML = parts.join("&nbsp;&nbsp;");
      });

      // 窗格刚重建时 DOM 可能尚未就绪，下一帧再补（含新建副图窗格）
      if (missing) raf = requestAnimationFrame(sync);
    };

    sync();
    return () => {
      if (raf) cancelAnimationFrame(raf);
    };
    // enabledMAs/enabledPanes 的内容变化已由 maKey/paneKey 覆盖
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [display, displayInd, showBoll, paneKey, maKey]);

  return (
    <div className="relative">
      {isLoadingMore ? (
        <div className="pointer-events-none absolute left-2 top-2 z-10 rounded bg-muted/80 px-2 py-0.5 text-[11px] text-muted-foreground">
          加载更早数据…
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
