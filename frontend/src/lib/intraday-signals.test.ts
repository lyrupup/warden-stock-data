import { describe, expect, it } from "vitest";
import {
  computeIntradayMetrics,
  computeIntradaySignals,
  type TIntradaySignalInput,
} from "./intraday-signals";

/** 构造一根分时点 */
const pt = (
  time: string,
  price: number,
  avgPrice: number,
  volume: number,
): TIntradaySignalInput => ({ time, price: price, avgPrice, volume });

describe("computeIntradayMetrics", () => {
  it("逐点计算乖离率与量比、累计均量", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 10.1, 10.05, 3000),
    ];
    const m = computeIntradayMetrics(points);
    expect(m).toHaveLength(2);
    expect(m[0]?.bias).toBeCloseTo(0, 6);
    expect(m[0]?.volumeRatio).toBeCloseTo(1, 6); // 首根量比=量/自身均值=1
    // 第二根：乖离 (10.1-10.05)/10.05*100 ≈ 0.4975%
    expect(m[1]?.bias).toBeCloseTo(0.4975, 3);
    // 累计均量 (1000+3000)/2=2000，量比 3000/2000=1.5
    expect(m[1]?.cumAvgVolume).toBeCloseTo(2000, 6);
    expect(m[1]?.volumeRatio).toBeCloseTo(1.5, 6);
  });
});

describe("computeIntradaySignals", () => {
  it("首根不出信号", () => {
    const points = [pt("09:30", 10, 10, 1000)];
    expect(computeIntradaySignals(points)).toHaveLength(0);
  });

  it("上穿均价且量能、乖离达标 → 买点", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 9.99, 10.0, 1000), // 价在均价下方
      pt("09:32", 10.05, 10.0, 3000), // 上穿，乖离 0.5%，量比高
    ];
    const signals = computeIntradaySignals(points);
    expect(signals).toHaveLength(1);
    expect(signals[0]?.type).toBe("buy");
    expect(signals[0]?.index).toBe(2);
    expect(signals[0]?.bias).toBeCloseTo(0.5, 5);
  });

  it("下穿均价 → 卖点", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 10.05, 10.0, 1000), // 价在均价上方
      pt("09:32", 9.95, 10.0, 3000), // 下穿
    ];
    const signals = computeIntradaySignals(points);
    expect(signals).toHaveLength(1);
    expect(signals[0]?.type).toBe("sell");
  });

  it("乖离不足时过滤掉穿越", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 9.999, 10.0, 1000),
      pt("09:32", 10.001, 10.0, 5000), // 乖离仅 0.01% < 0.15%
    ];
    expect(computeIntradaySignals(points)).toHaveLength(0);
  });

  it("量比不足时过滤掉穿越", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 9.99, 10.0, 1000),
      pt("09:32", 10.05, 10.0, 800), // 量比 < 1.2
    ];
    expect(computeIntradaySignals(points)).toHaveLength(0);
  });

  it("冷却期内的二次穿越被抑制", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 9.99, 10.0, 1000),
      pt("09:32", 10.05, 10.0, 3000), // 买点
      pt("09:33", 9.95, 10.0, 3000), // 下穿，但距上一信号仅 1 分钟 < 5
    ];
    const signals = computeIntradaySignals(points);
    expect(signals).toHaveLength(1);
    expect(signals[0]?.type).toBe("buy");
  });

  it("冷却期外的反向穿越可再次触发", () => {
    const points = [
      pt("09:30", 10.0, 10.0, 1000),
      pt("09:31", 9.99, 10.0, 1000),
      pt("09:32", 10.05, 10.0, 3000), // 买点 idx2
      pt("09:33", 10.05, 10.02, 1000),
      pt("09:34", 10.05, 10.03, 1000),
      pt("09:35", 10.05, 10.04, 1000),
      pt("09:36", 10.05, 10.05, 1000),
      pt("09:37", 9.9, 10.04, 3000), // 下穿 idx7，距 idx2 = 5 分钟，可触发
    ];
    const signals = computeIntradaySignals(points);
    expect(signals).toHaveLength(2);
    expect(signals[1]?.type).toBe("sell");
  });

  it("可自定义过滤参数", () => {
    const points = [
      pt("09:30", 9.999, 10.0, 1000), // 价始终在均价下方，首根不出信号
      pt("09:31", 9.999, 10.0, 1000),
      pt("09:32", 10.001, 10.0, 1100), // 上穿，乖离 0.01%，量比 ~1.05
    ];
    const signals = computeIntradaySignals(points, {
      biasThreshold: 0,
      volumeRatio: 1,
      cooldownMinutes: 0,
    });
    expect(signals).toHaveLength(1);
    expect(signals[0]?.type).toBe("buy");
  });
});
