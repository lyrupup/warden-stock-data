import {
  BaselineSeries,
  ColorType,
  createChart,
  createSeriesMarkers,
  HistogramSeries,
  LineSeries,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type SeriesMarker,
  type Time,
  type UTCTimestamp,
} from "lightweight-charts";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/cn";
import { changeColor, formatPrice, formatVolume, toNumber } from "@/lib/decimal";
import {
  computeDayTradeBaseline,
  computeDayTradePlan,
  computeDayTradeSignals,
  computeDayTrend,
  computeSessionMaturity,
  DEFAULT_DAYTRADE_PARAMS,
  type EDaySessionLevel,
  type EDayTradeMode,
  type EDayTrendState,
  type TDaySessionMaturity,
  type TDayTradeParams,
  type TDayTradeSignal,
  type TDayTrend,
} from "@/lib/daytrade";
import {
  computeIntradayMetrics,
  computeIntradaySignals,
  DEFAULT_SIGNAL_PARAMS,
  type TIntradayMetric,
  type TIntradaySignal,
} from "@/lib/intraday-signals";
import type { TKline, TStockIntraday } from "@/types/market";

/** A 股配色：涨红跌绿 */
const PRICE_COLOR = "#2563eb";
const AVG_COLOR = "#ea580c";

/** 成交量柱用半透明涨跌色 */
const UP_VOLUME_COLOR = "rgba(220,38,38,0.5)";
const DOWN_VOLUME_COLOR = "rgba(22,163,74,0.5)";

/** 买卖点配色：买/低吸=红、卖/高抛=绿 */
const BUY_COLOR = "#dc2626";
const SELL_COLOR = "#16a34a";

/** 高抛低吸轨道：上轨偏红、下轨偏绿（半透明虚线） */
const UPPER_BAND_COLOR = "rgba(220,38,38,0.7)";
const LOWER_BAND_COLOR = "rgba(22,163,74,0.7)";

/** 乖离副图：上红下绿；阈值/门槛线用琥珀色 */
const BIAS_UP_COLOR = "#dc2626";
const BIAS_DOWN_COLOR = "#16a34a";
const THRESHOLD_COLOR = "#d97706";

/** 乖离阈值（百分比），与判定一致 */
const BIAS_THRESHOLD_PCT = DEFAULT_SIGNAL_PARAMS.biasThreshold * 100;

const formatPct = (v: number): string => `${v.toFixed(2)}%`;

/**
 * 分时点时间戳由 Date.UTC(墙钟时分) 构造，故按 UTC 取时分即为交易时刻。
 * 用作 crosshair 时间标签格式化，仅展示 HH:MM（去掉日期）。
 */
const formatHourMinute = (t: UTCTimestamp): string => {
  const d = new Date((t as number) * 1000);
  const hh = String(d.getUTCHours()).padStart(2, "0");
  const mm = String(d.getUTCMinutes()).padStart(2, "0");
  return `${hh}:${mm}`;
};

/**
 * 后端分时点时间为 Asia/Shanghai 的 RFC3339（含 +08:00）。lightweight-charts 按 UTC
 * 渲染坐标轴，这里取墙钟时分作为 UTC 时间戳，使横轴直接显示 09:30~15:00。
 */
const parseIntradayTime = (s: string): UTCTimestamp => {
  const m = s.match(/(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/);
  if (!m) return Math.floor(new Date(s).getTime() / 1000) as UTCTimestamp;
  const [, y, mo, d, hh, mm] = m;
  return Math.floor(Date.UTC(+y, +mo - 1, +d, +hh, +mm) / 1000) as UTCTimestamp;
};

type TIntradayChartProps = {
  intraday: TStockIntraday;
  /** 历史日 K（前复权），用于做 T 的历史振幅基准与支撑压力 */
  klines?: TKline[];
  height?: number;
};

/**
 * 分时图（lightweight-charts v5）+ 做 T 研判：
 * - 主图：价格线 + 均价线(VWAP) + 昨收基准 + 高抛低吸轨道带 + 做 T B/S 箭头
 * - 量副图：分时量柱 + 量能门槛线
 * - 乖离副图：(价−均价)/均价 曲线 + ±阈值参考线
 * 历史基准用昨日及以前日 K 计算，趋势态/轨道/信号基于今日分时实时计算。
 */
export const IntradayChart = ({
  intraday,
  klines,
  height = 480,
}: TIntradayChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const priceRef = useRef<ISeriesApi<"Line"> | null>(null);
  const avgRef = useRef<ISeriesApi<"Line"> | null>(null);
  const upperRef = useRef<ISeriesApi<"Line"> | null>(null);
  const lowerRef = useRef<ISeriesApi<"Line"> | null>(null);
  const volumeRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const volThreshRef = useRef<ISeriesApi<"Line"> | null>(null);
  const biasRef = useRef<ISeriesApi<"Baseline"> | null>(null);
  const baselineRef = useRef<IPriceLine | null>(null);
  const markersRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const legendMainRef = useRef<HTMLDivElement | null>(null);
  const legendVolRef = useRef<HTMLDivElement | null>(null);
  const legendBiasRef = useRef<HTMLDivElement | null>(null);

  const [hoverTime, setHoverTime] = useState<number | null>(null);
  // 图上买卖点标注模式：做 T 高抛低吸 / 趋势均价线交叉
  const [mode, setMode] = useState<"daytrade" | "trend">("daytrade");
  // 做 T 可调参数（默认值见 DEFAULT_DAYTRADE_PARAMS）
  const [params, setParams] =
    useState<Required<TDayTradeParams>>(DEFAULT_DAYTRADE_PARAMS);

  const preClose = toNumber(intraday.pre_close);
  const points = intraday.points;
  const tradeDate = intraday.trade_date;

  const metrics = useMemo(
    () =>
      computeIntradayMetrics(
        points.map((p) => ({
          time: p.time,
          price: p.price,
          avgPrice: p.avg_price,
          volume: p.volume,
        })),
      ),
    [points],
  );

  const baseline = useMemo(
    () =>
      computeDayTradeBaseline(
        (klines ?? []).map((k) => ({
          trade_date: k.trade_date,
          open: k.open,
          high: k.high,
          low: k.low,
          close: k.close,
          volume: k.volume,
        })),
        tradeDate,
        params,
      ),
    [klines, tradeDate, params],
  );

  const trend = useMemo(
    () => computeDayTrend(metrics, preClose, baseline, params),
    [metrics, preClose, baseline, params],
  );

  const plan = useMemo(
    () => computeDayTradePlan(trend, params),
    [trend, params],
  );

  const daytrade = useMemo(
    () => computeDayTradeSignals(metrics, baseline, trend, plan, params),
    [metrics, baseline, trend, plan, params],
  );

  const maturity = useMemo(() => computeSessionMaturity(points), [points]);

  const trendSignals = useMemo(
    () =>
      computeIntradaySignals(
        points.map((p) => ({
          time: p.time,
          price: p.price,
          avgPrice: p.avg_price,
          volume: p.volume,
        })),
      ),
    [points],
  );

  const byTime = useMemo(() => {
    const m = new Map<number, TStockIntraday["points"][number]>();
    points.forEach((p) => m.set(parseIntradayTime(p.time), p));
    return m;
  }, [points]);

  const metricByTime = useMemo(() => {
    const m = new Map<number, TIntradayMetric>();
    metrics.forEach((x) => m.set(parseIntradayTime(x.time), x));
    return m;
  }, [metrics]);

  const latest = points.length > 0 ? points[points.length - 1] : null;
  const display =
    (hoverTime != null ? byTime.get(hoverTime) : undefined) ?? latest;
  const displayMetric =
    (hoverTime != null ? metricByTime.get(hoverTime) : undefined) ??
    (metrics.length > 0 ? metrics[metrics.length - 1] : undefined);

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
      // 单日分时是完整一段：固定左右边界，缩放/平移都钳制在数据范围内，
      // 不会把数据范围之外的空白拉进绘图区（放大后仍可在范围内左右拖动查看）。
      timeScale: {
        borderVisible: false,
        timeVisible: true,
        secondsVisible: false,
        fixLeftEdge: true,
        fixRightEdge: true,
        lockVisibleTimeRangeOnResize: true,
      },
      localization: { timeFormatter: formatHourMinute },
    });

    const priceSeries = chart.addSeries(LineSeries, {
      color: PRICE_COLOR,
      lineWidth: 2,
      priceLineVisible: false,
      lastValueVisible: true,
    });
    const avgSeries = chart.addSeries(LineSeries, {
      color: AVG_COLOR,
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });

    // 高抛低吸轨道带（主图，与价格同刻度）：上轨高抛、下轨低吸
    const upperSeries = chart.addSeries(LineSeries, {
      color: UPPER_BAND_COLOR,
      lineWidth: 1,
      lineStyle: 2,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });
    const lowerSeries = chart.addSeries(LineSeries, {
      color: LOWER_BAND_COLOR,
      lineWidth: 1,
      lineStyle: 2,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
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

    // 量能门槛线（量比阈值 × 累计均量）：量柱高过此线即满足量比过滤
    const volThreshSeries = chart.addSeries(
      LineSeries,
      {
        color: THRESHOLD_COLOR,
        lineWidth: 1,
        lineStyle: 2,
        priceLineVisible: false,
        lastValueVisible: false,
        crosshairMarkerVisible: false,
      },
      1,
    );

    // 乖离 BIAS 副图：(价−均价)/均价，上红下绿，零轴即价/均价穿越点
    const biasSeries = chart.addSeries(
      BaselineSeries,
      {
        baseValue: { type: "price", price: 0 },
        topLineColor: BIAS_UP_COLOR,
        topFillColor1: "rgba(220,38,38,0.25)",
        topFillColor2: "rgba(220,38,38,0.02)",
        bottomLineColor: BIAS_DOWN_COLOR,
        bottomFillColor1: "rgba(22,163,74,0.02)",
        bottomFillColor2: "rgba(22,163,74,0.25)",
        lineWidth: 1,
        priceFormat: { type: "custom", formatter: formatPct, minMove: 0.01 },
        priceLineVisible: false,
        lastValueVisible: true,
      },
      2,
    );
    biasSeries.priceScale().applyOptions({
      borderVisible: false,
      scaleMargins: { top: 0.15, bottom: 0.15 },
    });
    biasSeries.createPriceLine({
      price: BIAS_THRESHOLD_PCT,
      color: THRESHOLD_COLOR,
      lineWidth: 1,
      lineStyle: 2,
      axisLabelVisible: false,
    });
    biasSeries.createPriceLine({
      price: -BIAS_THRESHOLD_PCT,
      color: THRESHOLD_COLOR,
      lineWidth: 1,
      lineStyle: 2,
      axisLabelVisible: false,
    });

    chart.panes()[0]?.setStretchFactor(2);
    chart.panes()[1]?.setStretchFactor(1);
    chart.panes()[2]?.setStretchFactor(1);

    chartRef.current = chart;
    priceRef.current = priceSeries;
    avgRef.current = avgSeries;
    upperRef.current = upperSeries;
    lowerRef.current = lowerSeries;
    volumeRef.current = volumeSeries;
    volThreshRef.current = volThreshSeries;
    biasRef.current = biasSeries;
    markersRef.current = createSeriesMarkers(priceSeries);

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
      chart.remove();
      chartRef.current = null;
      priceRef.current = null;
      avgRef.current = null;
      upperRef.current = null;
      lowerRef.current = null;
      volumeRef.current = null;
      volThreshRef.current = null;
      biasRef.current = null;
      baselineRef.current = null;
      markersRef.current = null;
      legendMainRef.current = null;
      legendVolRef.current = null;
      legendBiasRef.current = null;
    };
  }, [height]);

  useEffect(() => {
    const price = priceRef.current;
    const avg = avgRef.current;
    const upper = upperRef.current;
    const lower = lowerRef.current;
    const volume = volumeRef.current;
    const volThresh = volThreshRef.current;
    const bias = biasRef.current;
    const chart = chartRef.current;
    if (
      !price ||
      !avg ||
      !upper ||
      !lower ||
      !volume ||
      !volThresh ||
      !bias ||
      !chart ||
      points.length === 0
    )
      return;

    price.setData(
      points.map((p) => ({
        time: parseIntradayTime(p.time),
        value: toNumber(p.price),
      })),
    );
    avg.setData(
      points.map((p) => ({
        time: parseIntradayTime(p.time),
        value: toNumber(p.avg_price),
      })),
    );

    // 高抛低吸轨道带（仅做 T 模式展示；趋势模式清空）
    upper.setData(
      mode === "daytrade"
        ? daytrade.band.map((b) => ({
            time: parseIntradayTime(b.time),
            value: b.upper,
          }))
        : [],
    );
    lower.setData(
      mode === "daytrade"
        ? daytrade.band.map((b) => ({
            time: parseIntradayTime(b.time),
            value: b.lower,
          }))
        : [],
    );

    // 量柱着色对齐同花顺：本分钟价 >= 上一分钟价为红，否则绿；首根比昨收
    volume.setData(
      points.map((p, i) => {
        const prev = i > 0 ? toNumber(points[i - 1].price) : preClose;
        return {
          time: parseIntradayTime(p.time),
          value: toNumber(p.volume),
          color: toNumber(p.price) >= prev ? UP_VOLUME_COLOR : DOWN_VOLUME_COLOR,
        };
      }),
    );

    volThresh.setData(
      metrics.map((m) => ({
        time: parseIntradayTime(m.time),
        value: m.cumAvgVolume * DEFAULT_SIGNAL_PARAMS.volumeRatio,
      })),
    );
    bias.setData(
      metrics.map((m) => ({
        time: parseIntradayTime(m.time),
        value: m.bias,
      })),
    );

    if (baselineRef.current) {
      price.removePriceLine(baselineRef.current);
      baselineRef.current = null;
    }
    if (preClose > 0) {
      baselineRef.current = price.createPriceLine({
        price: preClose,
        color: "#9ca3af",
        lineWidth: 1,
        lineStyle: 2,
        axisLabelVisible: true,
        title: "昨收",
      });
    }

    // 买卖点标注：做 T 模式=低吸(吸↑红)/高抛(抛↓绿)；趋势模式=均价线交叉 B/S
    const markers: SeriesMarker<Time>[] =
      mode === "daytrade"
        ? daytrade.signals.map((s) =>
            s.type === "dip_buy"
              ? {
                  time: parseIntradayTime(s.time),
                  position: "belowBar",
                  color: BUY_COLOR,
                  shape: "arrowUp",
                  text: "吸",
                }
              : {
                  time: parseIntradayTime(s.time),
                  position: "aboveBar",
                  color: SELL_COLOR,
                  shape: "arrowDown",
                  text: "抛",
                },
          )
        : trendSignals.map((s) =>
            s.type === "buy"
              ? {
                  time: parseIntradayTime(s.time),
                  position: "belowBar",
                  color: BUY_COLOR,
                  shape: "arrowUp",
                  text: "B",
                }
              : {
                  time: parseIntradayTime(s.time),
                  position: "aboveBar",
                  color: SELL_COLOR,
                  shape: "arrowDown",
                  text: "S",
                },
          );
    markersRef.current?.setMarkers(markers);

    chart.timeScale().fitContent();
  }, [points, preClose, metrics, daytrade, trendSignals, mode]);

  // 同步三个 pane 的浮动图例（含义标注 + 随 hover 的指标值）
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;

    const gray = "#9ca3af";
    const span = (text: string, color: string) =>
      `<span style="color:${color}">${text}</span>`;

    const ensureLegend = (
      ref: { current: HTMLDivElement | null },
      paneIndex: number,
    ) => {
      if (ref.current) return;
      const row = chart.panes()[paneIndex]?.getHTMLElement();
      const cell = row
        ? Array.from(row.querySelectorAll("td")).find((td) =>
            td.querySelector("canvas"),
          )
        : null;
      if (!cell) return;
      const el = document.createElement("div");
      el.style.cssText =
        "position:absolute;left:8px;top:4px;z-index:3;pointer-events:none;" +
        "font-size:11px;line-height:1.5;white-space:nowrap;" +
        "font-variant-numeric:tabular-nums;";
      cell.appendChild(el);
      ref.current = el;
    };

    let raf = 0;
    const sync = () => {
      ensureLegend(legendMainRef, 0);
      ensureLegend(legendVolRef, 1);
      ensureLegend(legendBiasRef, 2);

      if (legendMainRef.current) {
        if (display) {
          const price = toNumber(display.price);
          const change = price - preClose;
          const pct = preClose > 0 ? (change / preClose) * 100 : 0;
          const c = change > 0 ? BUY_COLOR : change < 0 ? SELL_COLOR : gray;
          const sign = change >= 0 ? "+" : "";
          legendMainRef.current.innerHTML = [
            span(`分时 ${display.time.slice(11, 16)}`, gray),
            span(`价 ${formatPrice(display.price)}`, c),
            span(`均价 ${formatPrice(display.avg_price)}`, AVG_COLOR),
            preClose > 0
              ? span(
                  `${sign}${formatPrice(change)} (${sign}${pct.toFixed(2)}%)`,
                  c,
                )
              : "",
          ].join("&nbsp;&nbsp;");
        } else {
          legendMainRef.current.innerHTML = "";
        }
      }

      if (legendVolRef.current) {
        if (display && displayMetric) {
          const thresh =
            displayMetric.cumAvgVolume * DEFAULT_SIGNAL_PARAMS.volumeRatio;
          const ratioOk =
            displayMetric.volumeRatio >= DEFAULT_SIGNAL_PARAMS.volumeRatio;
          const i = displayMetric.index;
          const prevPrice = i > 0 ? toNumber(points[i - 1].price) : preClose;
          const volColor =
            toNumber(display.price) >= prevPrice ? BUY_COLOR : SELL_COLOR;
          legendVolRef.current.innerHTML = [
            span("分时量", gray),
            span(formatVolume(display.volume), volColor),
            span(`门槛 ${formatVolume(thresh)}`, THRESHOLD_COLOR),
            span(
              `量比 ${displayMetric.volumeRatio.toFixed(2)}`,
              ratioOk ? BUY_COLOR : gray,
            ),
          ].join("&nbsp;&nbsp;");
        } else {
          legendVolRef.current.innerHTML = "";
        }
      }

      if (legendBiasRef.current) {
        if (displayMetric) {
          const bc =
            displayMetric.bias > 0
              ? BIAS_UP_COLOR
              : displayMetric.bias < 0
                ? BIAS_DOWN_COLOR
                : gray;
          const bsign = displayMetric.bias >= 0 ? "+" : "";
          legendBiasRef.current.innerHTML = [
            span("乖离 BIAS", gray),
            span(`${bsign}${displayMetric.bias.toFixed(2)}%`, bc),
            span(`阈值 ±${BIAS_THRESHOLD_PCT.toFixed(2)}%`, THRESHOLD_COLOR),
          ].join("&nbsp;&nbsp;");
        } else {
          legendBiasRef.current.innerHTML = "";
        }
      }

      if (
        !legendMainRef.current ||
        !legendVolRef.current ||
        !legendBiasRef.current
      ) {
        raf = requestAnimationFrame(sync);
      }
    };

    sync();
    return () => {
      if (raf) cancelAnimationFrame(raf);
    };
  }, [display, displayMetric, preClose, points]);

  return (
    <div>
      <div className="mb-2 flex items-center gap-1">
        <ModeButton
          active={mode === "daytrade"}
          onClick={() => setMode("daytrade")}
        >
          做 T（吸/抛）
        </ModeButton>
        <ModeButton active={mode === "trend"} onClick={() => setMode("trend")}>
          趋势（B/S）
        </ModeButton>
      </div>
      <div
        ref={containerRef}
        className="w-full"
        onMouseLeave={() => setHoverTime(null)}
      />
      {mode === "daytrade" ? (
        <>
          <DayTradePanel
            trend={trend}
            plan={plan}
            atrPct={baseline.atrPct}
            refPct={baseline.refPct}
            baselineDays={baseline.days}
            signals={daytrade.signals}
            preClose={preClose}
            maturity={maturity}
          />
          <DayTradeTuner params={params} onChange={setParams} />
        </>
      ) : (
        <TrendSignalPanel signals={trendSignals} preClose={preClose} />
      )}
    </div>
  );
};

const ModeButton = ({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) => (
  <button
    type="button"
    onClick={onClick}
    className={cn(
      "rounded px-2.5 py-1 text-xs font-medium transition-colors",
      active
        ? "bg-primary text-primary-foreground"
        : "bg-muted text-muted-foreground hover:bg-muted/70",
    )}
  >
    {children}
  </button>
);

const TREND_LABEL: Record<EDayTrendState, string> = {
  strong_up: "强多",
  weak_up: "偏多",
  range: "横盘",
  weak_down: "偏空",
  strong_down: "强空",
};

const MODE_LABEL: Record<EDayTradeMode, string> = {
  none: "不建议做 T",
  positive_t: "正 T（先吸后抛）",
  reverse_t: "倒 T（先抛后吸）",
  both: "双向网格",
};

const trendClass = (state: EDayTrendState): string => {
  if (state === "strong_up" || state === "weak_up")
    return "bg-red-600 text-white";
  if (state === "strong_down" || state === "weak_down")
    return "bg-green-600 text-white";
  return "bg-muted text-foreground";
};

const MATURITY_CLASS: Record<EDaySessionLevel, string> = {
  low: "bg-red-100 text-red-700",
  medium: "bg-amber-100 text-amber-700",
  high: "bg-sky-100 text-sky-700",
  complete: "bg-emerald-100 text-emerald-700",
};

const MATURITY_INFO =
  "盘中分时是逐分钟拉取的，研判可靠度随交易时长上升。早盘样本不足（趋势态会抖动、预期振幅偏低）；午后渐稳；接近/已收盘为全日完整。历史 ATR 等基准来自昨日数据，全程可靠。";

const TREND_INFO =
  "当日趋势态：由跳空、价相对均价、VWAP 斜率、日内位置、量价配合、与历史均线 6 因子加权得分（-100~+100），映射 强多/偏多/横盘/偏空/强空，决定做正 T 还是倒 T。盘中随数据更新。";

const MODE_INFO =
  "做 T 建议：由趋势态 + 预期振幅决定。强多→倒 T（先抛后吸）；偏多/强空→正 T（先吸后抛）；横盘→双向网格；振幅不足→不建议。";

const EXPAMP_INFO =
  "预期振幅 = max(历史ATR%, 今日已实现振幅%)，衡量日内可吃的价差空间。低于“振幅门槛”则不建议做 T（覆盖不了回转成本）。";

const ATR_INFO =
  "历史ATR% = 近 N 个交易日平均振幅 (最高−最低)/前收。做 T 的波动标尺，决定轨道单侧偏离 refPct=clamp(ATR%/2, 0.4%, 1.5%)。来自昨日及以前数据。";

const BAND_INFO =
  "高抛低吸轨道 = 均价(VWAP) ± refPct × 趋势自适应系数。触及下轨缩量企稳→低吸；触及上轨冲高滞涨→高抛。";

/** 今日做 T 研判面板：趋势态 + 适合度 + 高抛低吸信号明细 */
const DayTradePanel = ({
  trend,
  plan,
  atrPct,
  refPct,
  baselineDays,
  signals,
  preClose,
  maturity,
}: {
  trend: TDayTrend;
  plan: ReturnType<typeof computeDayTradePlan>;
  atrPct: number;
  refPct: number;
  baselineDays: number;
  signals: TDayTradeSignal[];
  preClose: number;
  maturity: TDaySessionMaturity;
}) => {
  return (
    <div className="mt-3 rounded-md border bg-muted/30 p-3 text-xs">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <span className="font-medium text-foreground">今日做 T 研判</span>
        <span className="text-muted-foreground">（仅供参考，不接交易）</span>
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium",
            MATURITY_CLASS[maturity.level],
          )}
        >
          可靠度 {maturity.label}（{maturity.elapsedMinutes}/
          {maturity.totalMinutes}分）
          <InfoHint text={MATURITY_INFO} />
        </span>
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium",
            trendClass(trend.state),
          )}
        >
          趋势 {TREND_LABEL[trend.state]}（{trend.score.toFixed(0)}）
          <InfoHint text={TREND_INFO} />
        </span>
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium",
            plan.suitable
              ? "bg-amber-500 text-white"
              : "bg-muted text-muted-foreground",
          )}
        >
          {MODE_LABEL[plan.mode]}
          <InfoHint text={MODE_INFO} />
        </span>
        <span className="inline-flex items-center gap-1 text-muted-foreground tabular-nums">
          预期振幅 {trend.expAmp.toFixed(2)}%
          <InfoHint text={EXPAMP_INFO} />
        </span>
        <span className="inline-flex items-center gap-1 text-muted-foreground tabular-nums">
          历史ATR {atrPct > 0 ? `${atrPct.toFixed(2)}%` : "—"}
          {baselineDays > 0 ? `（${baselineDays}日）` : "（无历史K线）"}
          <InfoHint text={ATR_INFO} />
        </span>
        <span className="inline-flex items-center gap-1 text-muted-foreground tabular-nums">
          轨道 均价±{refPct.toFixed(2)}%×自适应
          <InfoHint text={BAND_INFO} />
        </span>
      </div>

      {plan.reasons.length > 0 ? (
        <ul className="mt-1.5 flex flex-col gap-0.5 text-muted-foreground">
          {plan.reasons.map((r) => (
            <li key={r}>· {r}</li>
          ))}
        </ul>
      ) : null}

      <div className="mt-2 text-muted-foreground">
        触及<span className="text-green-600">下轨</span>缩量企稳→
        <span className="text-red-500">低吸 吸</span>；触及
        <span className="text-red-500">上轨</span>冲高滞涨→
        <span className="text-green-600">高抛 抛</span>
      </div>

      {signals.length === 0 ? (
        <p className="mt-2 text-muted-foreground">
          当前分时暂无触发的做 T 买卖点。
        </p>
      ) : (
        <ul className="mt-2 flex flex-col gap-1">
          {[...signals].reverse().map((s) => {
            const pct =
              preClose > 0 ? ((s.price - preClose) / preClose) * 100 : 0;
            return (
              <li
                key={`${s.type}-${s.index}`}
                className="flex flex-wrap items-center gap-x-3 tabular-nums"
              >
                <span className="w-10 text-muted-foreground">
                  {s.time.slice(11, 16)}
                </span>
                <DtSignalLabel type={s.type} />
                <span>{formatPrice(s.price)}</span>
                {preClose > 0 ? (
                  <span className={changeColor(pct)}>
                    {pct >= 0 ? "+" : ""}
                    {pct.toFixed(2)}%
                  </span>
                ) : null}
                <span className="text-muted-foreground">{s.reason}</span>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};

const DtSignalLabel = ({ type }: { type: TDayTradeSignal["type"] }) => (
  <span
    className={cn(
      "inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-medium text-white",
      type === "dip_buy" ? "bg-red-600" : "bg-green-600",
    )}
  >
    {type === "dip_buy" ? "低吸 吸" : "高抛 抛"}
  </span>
);

/** info 图标 + hover 气泡提示（无依赖，纯 CSS group-hover） */
const InfoHint = ({ text }: { text: string }) => (
  <span className="group relative inline-flex">
    <span
      tabIndex={0}
      className="flex h-3.5 w-3.5 cursor-help items-center justify-center rounded-full border border-current text-[9px] leading-none opacity-70"
    >
      i
    </span>
    <span
      className="pointer-events-none absolute bottom-full left-1/2 z-20 mb-1 hidden w-56 -translate-x-1/2 rounded-md border bg-popover p-2 text-[11px] font-normal leading-relaxed text-popover-foreground shadow-md group-hover:block group-focus-within:block"
    >
      {text}
    </span>
  </span>
);

type TTunerField = {
  key: keyof Required<TDayTradeParams>;
  label: string;
  unit: string;
  min: number;
  max: number;
  step: number;
  info: string;
};

const TUNER_FIELDS: TTunerField[] = [
  {
    key: "minAmp",
    label: "振幅门槛",
    unit: "%",
    min: 0.5,
    max: 5,
    step: 0.1,
    info: "判定“是否适合做 T”的下线。预期振幅（max(历史ATR, 今日已实现振幅)）低于此值则判“不建议做 T”。调高更严格，只在大波动日提示。",
  },
  {
    key: "bandScale",
    label: "轨道宽度",
    unit: "×",
    min: 0.5,
    max: 2.5,
    step: 0.1,
    info: "高抛低吸轨道整体宽窄系数（×refPct）。调大→轨道更宽，仅更极端的偏离才触发吸/抛，信号更精；调小→信号更勤但噪声多。",
  },
  {
    key: "cooldownMinutes",
    label: "信号冷却",
    unit: "分",
    min: 1,
    max: 60,
    step: 1,
    info: "两个做 T 信号之间的最小间隔分钟数。调大→信号更稀疏，抑制同一时段反复触发。",
  },
  {
    key: "volCalmMax",
    label: "低吸缩量上限",
    unit: "量比",
    min: 1,
    max: 4,
    step: 0.1,
    info: "低吸要求“缩量企稳”：当根量比需 ≤ 此值才算承接。调大→放量也算企稳，低吸点更多；调小→只在明显缩量处低吸。",
  },
  {
    key: "slopeWindow",
    label: "趋势斜率窗口",
    unit: "分",
    min: 5,
    max: 60,
    step: 1,
    info: "计算均价线(VWAP)斜率的回看分钟数，影响趋势态灵敏度。调小→更灵敏但更抖；调大→更平滑但更滞后。",
  },
  {
    key: "baselineDays",
    label: "历史回看",
    unit: "日",
    min: 5,
    max: 60,
    step: 1,
    info: "历史 ATR 振幅基准的回看交易日数（昨日及以前），决定轨道偏离标尺 refPct=clamp(ATR%/2, 0.4%, 1.5%)。",
  },
];

const clampNum = (v: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, v));

/** 做 T 调参面板：每项带 info 提示，改动实时作用于研判与轨道 */
const DayTradeTuner = ({
  params,
  onChange,
}: {
  params: Required<TDayTradeParams>;
  onChange: (p: Required<TDayTradeParams>) => void;
}) => {
  const isDefault = TUNER_FIELDS.every(
    (f) => params[f.key] === DEFAULT_DAYTRADE_PARAMS[f.key],
  );
  return (
    <div className="mt-2 rounded-md border bg-muted/30 p-3 text-xs">
      <div className="mb-2 flex items-center justify-between">
        <span className="font-medium text-foreground">做 T 调参</span>
        <button
          type="button"
          disabled={isDefault}
          onClick={() => onChange(DEFAULT_DAYTRADE_PARAMS)}
          className={cn(
            "rounded px-2 py-0.5 text-[11px]",
            isDefault
              ? "cursor-default text-muted-foreground/50"
              : "text-primary hover:underline",
          )}
        >
          重置默认
        </button>
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-2.5 sm:grid-cols-3">
        {TUNER_FIELDS.map((f) => (
          <label key={f.key} className="flex flex-col gap-1">
            <span className="flex items-center gap-1 text-muted-foreground">
              {f.label}
              <InfoHint text={f.info} />
            </span>
            <span className="flex items-center gap-1.5">
              <input
                type="range"
                min={f.min}
                max={f.max}
                step={f.step}
                value={params[f.key]}
                onChange={(e) =>
                  onChange({
                    ...params,
                    [f.key]: clampNum(Number(e.target.value), f.min, f.max),
                  })
                }
                className="h-1.5 flex-1 cursor-pointer accent-primary"
              />
              <span className="w-12 shrink-0 text-right tabular-nums text-foreground">
                {params[f.key]}
                {f.unit}
              </span>
            </span>
          </label>
        ))}
      </div>
    </div>
  );
};

/** 趋势买卖点提示面板（均价线交叉法）：方案说明 + 信号明细 */
const TrendSignalPanel = ({
  signals,
  preClose,
}: {
  signals: TIntradaySignal[];
  preClose: number;
}) => {
  const latest = signals.length > 0 ? signals[signals.length - 1] : null;

  return (
    <div className="mt-3 rounded-md border bg-muted/30 p-3 text-xs">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
        <span className="font-medium text-foreground">趋势买卖点（仅供参考）</span>
        <span>
          均价线交叉 + 量比≥{DEFAULT_SIGNAL_PARAMS.volumeRatio} + 乖离≥
          {(DEFAULT_SIGNAL_PARAMS.biasThreshold * 100).toFixed(2)}% + 冷却
          {DEFAULT_SIGNAL_PARAMS.cooldownMinutes}分钟
        </span>
        <span className="text-muted-foreground">
          ｜价格上穿均价为
          <span className="text-red-500">买点 B</span>，下穿为
          <span className="text-green-500">卖点 S</span>
        </span>
      </div>

      {signals.length === 0 ? (
        <p className="mt-2 text-muted-foreground">
          当前分时暂无满足过滤条件的买卖点信号。
        </p>
      ) : (
        <>
          {latest ? (
            <p className="mt-2">
              最新信号：
              <TrendSignalLabel type={latest.type} />
              <span className="ml-1 text-muted-foreground">
                {latest.time.slice(11, 16)} @ {formatPrice(latest.price)}
                （乖离 {latest.bias >= 0 ? "+" : ""}
                {latest.bias.toFixed(2)}%，量比 {latest.volumeRatio.toFixed(2)}）
              </span>
            </p>
          ) : null}
          <ul className="mt-2 flex flex-col gap-1">
            {[...signals].reverse().map((s) => {
              const pct =
                preClose > 0 ? ((s.price - preClose) / preClose) * 100 : 0;
              return (
                <li
                  key={`${s.type}-${s.index}`}
                  className="flex flex-wrap items-center gap-x-3 tabular-nums"
                >
                  <span className="w-10 text-muted-foreground">
                    {s.time.slice(11, 16)}
                  </span>
                  <TrendSignalLabel type={s.type} />
                  <span>{formatPrice(s.price)}</span>
                  {preClose > 0 ? (
                    <span className={changeColor(pct)}>
                      {pct >= 0 ? "+" : ""}
                      {pct.toFixed(2)}%
                    </span>
                  ) : null}
                  <span className="text-muted-foreground">
                    乖离 {s.bias >= 0 ? "+" : ""}
                    {s.bias.toFixed(2)}%
                  </span>
                  <span className="text-muted-foreground">
                    量比 {s.volumeRatio.toFixed(2)}
                  </span>
                </li>
              );
            })}
          </ul>
        </>
      )}
    </div>
  );
};

const TrendSignalLabel = ({ type }: { type: TIntradaySignal["type"] }) => (
  <span
    className={cn(
      "inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-medium text-white",
      type === "buy" ? "bg-red-600" : "bg-green-600",
    )}
  >
    {type === "buy" ? "买 B" : "卖 S"}
  </span>
);
