import {
  CandlestickSeries,
  ColorType,
  createChart,
  LineSeries,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type UTCTimestamp,
} from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { TKline } from "@/types/market";
import { toNumber } from "@/lib/decimal";

type TKlineChartProps = {
  klines: TKline[];
  maLines?: Record<string, LineData<UTCTimestamp>[]>;
  height?: number;
};

const parseTime = (date: string): UTCTimestamp =>
  (new Date(date).getTime() / 1000) as UTCTimestamp;

export const KlineChart = ({
  klines,
  maLines = {},
  height = 400,
}: TKlineChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const lineRefs = useRef<ISeriesApi<"Line">[]>([]);

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
      upColor: "#ef4444",
      downColor: "#22c55e",
      borderUpColor: "#ef4444",
      borderDownColor: "#22c55e",
      wickUpColor: "#ef4444",
      wickDownColor: "#22c55e",
    });

    chartRef.current = chart;
    candleRef.current = candleSeries;

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) {
        chart.applyOptions({ width: entry.contentRect.width });
      }
    });
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      lineRefs.current.forEach((s) => chart.removeSeries(s));
      lineRefs.current = [];
      chart.remove();
      chartRef.current = null;
      candleRef.current = null;
    };
  }, [height]);

  useEffect(() => {
    if (!candleRef.current || klines.length === 0) return;

    candleRef.current.setData(
      klines.map((k) => ({
        time: parseTime(k.date),
        open: toNumber(k.open),
        high: toNumber(k.high),
        low: toNumber(k.low),
        close: toNumber(k.close),
      })),
    );

    const chart = chartRef.current;
    if (!chart) return;

    lineRefs.current.forEach((s) => chart.removeSeries(s));
    lineRefs.current = [];

    const colors: Record<string, string> = {
      ma5: "#f59e0b",
      ma10: "#3b82f6",
      ma20: "#8b5cf6",
      ma30: "#ec4899",
      ma60: "#14b8a6",
    };

    Object.entries(maLines).forEach(([key, data]) => {
      if (data.length === 0) return;
      const line = chart.addSeries(LineSeries, {
        color: colors[key] ?? "#6b7280",
        lineWidth: 1,
        priceLineVisible: false,
        lastValueVisible: false,
      });
      line.setData(data);
      lineRefs.current.push(line);
    });

    chart.timeScale().fitContent();
  }, [klines, maLines]);

  return <div ref={containerRef} className="w-full" />;
};
