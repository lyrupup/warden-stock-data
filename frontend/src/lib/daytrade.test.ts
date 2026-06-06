import { describe, expect, it } from "vitest";
import {
  computeDayTradeBaseline,
  computeDayTradePlan,
  computeDayTradeSignals,
  computeDayTrend,
  computeSessionMaturity,
  type TDayKline,
  type TDayTradeBaseline,
  type TDayTrend,
} from "./daytrade";
import { computeIntradayMetrics } from "./intraday-signals";

const kline = (
  date: string,
  high: number,
  low: number,
  close: number,
): TDayKline => ({
  trade_date: date,
  open: close,
  high,
  low,
  close,
  volume: 10000,
});

const metricsOf = (rows: Array<[string, number, number, number]>) =>
  computeIntradayMetrics(
    rows.map(([time, price, avgPrice, volume]) => ({
      time,
      price,
      avgPrice,
      volume,
    })),
  );

describe("computeDayTradeBaseline", () => {
  it("排除今日并计算近 N 日平均振幅与均线", () => {
    const klines = [
      kline("2026-06-01", 10.2, 9.8, 10.0),
      kline("2026-06-02", 10.4, 10.0, 10.2),
      kline("2026-06-03", 10.6, 10.2, 10.4),
      kline("2026-06-04", 10.0, 9.6, 9.8), // 今日，应排除
    ];
    const b = computeDayTradeBaseline(klines, "2026-06-04", { baselineDays: 20 });
    expect(b.days).toBe(3); // 仅 06-01~06-03
    // 振幅: (10.4-10.0)/10.0, (10.6-10.2)/10.2 的均值 ≈ (4% + 3.92%)/2
    expect(b.atrPct).toBeCloseTo((4 + (0.4 / 10.2) * 100) / 2, 2);
    expect(b.refPct).toBeGreaterThanOrEqual(0.4);
    expect(b.refPct).toBeLessThanOrEqual(1.5);
    expect(b.histHigh).toBeCloseTo(10.4, 5);
    expect(b.histLow).toBeCloseTo(10.0, 5);
  });

  it("refPct 受 [0.4,1.5] 夹取", () => {
    const big = [
      kline("2026-06-01", 11, 9, 10),
      kline("2026-06-02", 13, 9, 11), // 振幅约 40%
    ];
    expect(
      computeDayTradeBaseline(big, "2026-06-03").refPct,
    ).toBe(1.5);
  });
});

const baseline: TDayTradeBaseline = {
  atrPct: 2,
  refPct: 1.0,
  ma5: 9,
  ma20: 9,
  histLow: 9,
  histHigh: 11,
  days: 20,
};

describe("computeDayTrend", () => {
  it("价稳站均价上方、VWAP 上行、放量上涨 → 偏多/强多", () => {
    const m = metricsOf([
      ["09:30", 10.0, 10.0, 1000],
      ["09:31", 10.1, 10.02, 3000],
      ["09:32", 10.2, 10.05, 3000],
      ["09:33", 10.3, 10.1, 3000],
    ]);
    const t = computeDayTrend(m, 10, baseline);
    expect(t.score).toBeGreaterThan(20);
    expect(["weak_up", "strong_up"]).toContain(t.state);
  });

  it("价在均价下方、VWAP 下行、放量下跌 → 偏空/强空", () => {
    const m = metricsOf([
      ["09:30", 10.0, 10.0, 1000],
      ["09:31", 9.9, 9.98, 3000],
      ["09:32", 9.8, 9.95, 3000],
      ["09:33", 9.7, 9.9, 3000],
    ]);
    const t = computeDayTrend(m, 10, baseline);
    expect(t.score).toBeLessThan(-20);
    expect(["weak_down", "strong_down"]).toContain(t.state);
  });
});

describe("computeDayTradePlan", () => {
  const mk = (state: TDayTrend["state"], expAmp: number): TDayTrend => ({
    score: 0,
    state,
    gapPct: 0,
    expAmp,
  });

  it("振幅不足 → 不建议", () => {
    const p = computeDayTradePlan(mk("range", 1.0));
    expect(p.mode).toBe("none");
    expect(p.suitable).toBe(false);
  });

  it("横盘震荡 → 双向", () => {
    expect(computeDayTradePlan(mk("range", 3)).mode).toBe("both");
  });

  it("强多 → 倒 T，强空 → 正 T", () => {
    expect(computeDayTradePlan(mk("strong_up", 3)).mode).toBe("reverse_t");
    expect(computeDayTradePlan(mk("strong_down", 3)).mode).toBe("positive_t");
  });
});

describe("computeDayTradeSignals", () => {
  const trend: TDayTrend = {
    score: 0,
    state: "range",
    gapPct: 0,
    expAmp: 3,
  };
  const plan = computeDayTradePlan(trend);

  it("触及下轨且缩量企稳拐头 → 低吸", () => {
    const m = metricsOf([
      ["09:30", 10.0, 10.0, 1000],
      ["09:31", 9.88, 10.0, 1000],
      ["09:32", 9.85, 10.0, 1000],
      ["09:33", 9.86, 10.0, 1000],
    ]);
    const { signals } = computeDayTradeSignals(m, baseline, trend, plan);
    expect(signals.some((s) => s.type === "dip_buy" && s.index === 3)).toBe(
      true,
    );
  });

  it("触及上轨且冲高滞涨 → 高抛", () => {
    const m = metricsOf([
      ["09:30", 10.0, 10.0, 1000],
      ["09:31", 10.12, 10.0, 1000],
      ["09:32", 10.15, 10.0, 1000],
      ["09:33", 10.13, 10.0, 1000],
    ]);
    const { signals } = computeDayTradeSignals(m, baseline, trend, plan);
    expect(signals.some((s) => s.type === "rally_sell" && s.index === 3)).toBe(
      true,
    );
  });

  it("不适合做 T 时只给轨道、不给信号", () => {
    const notSuit = computeDayTradePlan({ ...trend, expAmp: 1 });
    const m = metricsOf([
      ["09:30", 10.0, 10.0, 1000],
      ["09:31", 9.88, 10.0, 1000],
      ["09:32", 9.85, 10.0, 1000],
      ["09:33", 9.86, 10.0, 1000],
    ]);
    const res = computeDayTradeSignals(m, baseline, trend, notSuit);
    expect(res.signals).toHaveLength(0);
    expect(res.band.length).toBe(m.length);
  });

  it("轨道宽度系数调大后更难触发", () => {
    const m = metricsOf([
      ["09:30", 10.0, 10.0, 1000],
      ["09:31", 9.88, 10.0, 1000],
      ["09:32", 9.85, 10.0, 1000],
      ["09:33", 9.86, 10.0, 1000],
    ]);
    const wide = computeDayTradeSignals(m, baseline, trend, plan, {
      bandScale: 3,
    });
    expect(wide.signals).toHaveLength(0);
  });
});

describe("computeSessionMaturity", () => {
  const at = (hhmm: string) => [{ time: `2026-06-05T${hhmm}:00+08:00` }];

  it("早盘样本不足 → low", () => {
    expect(computeSessionMaturity(at("09:45")).level).toBe("low");
  });

  it("上午渐明 → medium", () => {
    expect(computeSessionMaturity(at("10:30")).level).toBe("medium");
  });

  it("午后较稳 → high", () => {
    const m = computeSessionMaturity(at("14:00"));
    expect(m.level).toBe("high");
    expect(m.elapsedMinutes).toBe(180); // 120 + 60
  });

  it("接近收盘 → complete", () => {
    expect(computeSessionMaturity(at("15:00")).level).toBe("complete");
  });
});
