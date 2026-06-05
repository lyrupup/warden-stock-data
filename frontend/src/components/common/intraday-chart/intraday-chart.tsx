import {
  ColorType,
  createChart,
  HistogramSeries,
  LineSeries,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type UTCTimestamp,
} from "lightweight-charts";
import { useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/cn";
import { changeColor, formatPrice, formatVolume, toNumber } from "@/lib/decimal";
import type { TStockIntraday } from "@/types/market";

/** A 股配色：涨红跌绿 */
const PRICE_COLOR = "#2563eb";
const AVG_COLOR = "#ea580c";

/** 成交量柱用半透明涨跌色 */
const UP_VOLUME_COLOR = "rgba(220,38,38,0.5)";
const DOWN_VOLUME_COLOR = "rgba(22,163,74,0.5)";

/**
 * 后端分时点时间为 Asia/Shanghai 的 RFC3339（含 +08:00）。lightweight-charts 按 UTC
 * 渲染坐标轴，这里取墙钟时分作为 UTC 时间戳，使横轴直接显示 09:30~15:00。
 */
const parseIntradayTime = (s: string): UTCTimestamp => {
  const m = s.match(/(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/);
  if (!m) return Math.floor(new Date(s).getTime() / 1000) as UTCTimestamp;
  const [, y, mo, d, hh, mm] = m;
  return Math.floor(
    Date.UTC(+y, +mo - 1, +d, +hh, +mm) / 1000,
  ) as UTCTimestamp;
};

type TIntradayChartProps = {
  intraday: TStockIntraday;
  height?: number;
};

/** 分时图（lightweight-charts v5）：价格线 + 均价线 + 昨收基准 + 分时量副图 */
export const IntradayChart = ({
  intraday,
  height = 320,
}: TIntradayChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const priceRef = useRef<ISeriesApi<"Line"> | null>(null);
  const avgRef = useRef<ISeriesApi<"Line"> | null>(null);
  const volumeRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const baselineRef = useRef<IPriceLine | null>(null);

  const [hoverTime, setHoverTime] = useState<number | null>(null);

  const preClose = toNumber(intraday.pre_close);
  const points = intraday.points;

  const byTime = useMemo(() => {
    const m = new Map<number, TStockIntraday["points"][number]>();
    points.forEach((p) => m.set(parseIntradayTime(p.time), p));
    return m;
  }, [points]);

  const latest = points.length > 0 ? points[points.length - 1] : null;
  const display =
    (hoverTime != null ? byTime.get(hoverTime) : undefined) ?? latest;

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
      timeScale: { borderVisible: false, timeVisible: true, secondsVisible: false },
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
    priceRef.current = priceSeries;
    avgRef.current = avgSeries;
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
      chart.remove();
      chartRef.current = null;
      priceRef.current = null;
      avgRef.current = null;
      volumeRef.current = null;
      baselineRef.current = null;
    };
  }, [height]);

  useEffect(() => {
    const price = priceRef.current;
    const avg = avgRef.current;
    const volume = volumeRef.current;
    const chart = chartRef.current;
    if (!price || !avg || !volume || !chart || points.length === 0) return;

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
    volume.setData(
      points.map((p) => ({
        time: parseIntradayTime(p.time),
        value: toNumber(p.volume),
        color:
          toNumber(p.price) >= preClose ? UP_VOLUME_COLOR : DOWN_VOLUME_COLOR,
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

    chart.timeScale().fitContent();
  }, [points, preClose]);

  return (
    <div>
      {display ? (
        <IntradayStrip
          p={display}
          preClose={preClose}
          isHover={hoverTime !== null}
        />
      ) : null}
      <div
        ref={containerRef}
        className="w-full"
        onMouseLeave={() => setHoverTime(null)}
      />
    </div>
  );
};

const IntradayStrip = ({
  p,
  preClose,
  isHover,
}: {
  p: TStockIntraday["points"][number];
  preClose: number;
  isHover: boolean;
}) => {
  const price = toNumber(p.price);
  const change = price - preClose;
  const color = changeColor(change);
  const pct = preClose === 0 ? 0 : (change / preClose) * 100;
  const sign = change >= 0 ? "+" : "";

  return (
    <div className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs tabular-nums">
      <span
        className={cn(
          "text-muted-foreground",
          isHover && "font-medium text-foreground",
        )}
      >
        {p.time.slice(11, 16)}
      </span>
      <span>
        <span className="text-muted-foreground">价 </span>
        <span className={cn("font-medium", color)}>{formatPrice(p.price)}</span>
      </span>
      <span style={{ color: AVG_COLOR }} className="font-medium">
        均 {formatPrice(p.avg_price)}
      </span>
      {preClose > 0 ? (
        <span className={color}>
          {sign}
          {formatPrice(change)} ({sign}
          {pct.toFixed(2)}%)
        </span>
      ) : null}
      <span>
        <span className="text-muted-foreground">量 </span>
        <span className="font-medium text-foreground">
          {formatVolume(p.volume)}
        </span>
      </span>
    </div>
  );
};
