import { toNumber } from "./decimal";

export type TIntradaySignalType = "buy" | "sell";

/** 分时买卖点信号 */
export type TIntradaySignal = {
  /** 在分时点数组中的下标（A 股分时为 1 分钟一根，下标差即分钟差） */
  index: number;
  time: string;
  type: TIntradaySignalType;
  price: number;
  avgPrice: number;
  /** 价格相对均价的乖离率（百分比，正=价在均价上方） */
  bias: number;
  /** 当根量比 = 成交量 / 截至当前的累计平均量 */
  volumeRatio: number;
};

/** 信号过滤参数（默认值贴合日内噪声过滤经验） */
export type TIntradaySignalParams = {
  /** 乖离阈值（小数，0.0015 = 0.15%），过滤贴着均价的无效穿越 */
  biasThreshold?: number;
  /** 量比阈值，确认穿越时有量能配合 */
  volumeRatio?: number;
  /** 冷却分钟数，抑制同方向频繁触发 */
  cooldownMinutes?: number;
};

export type TIntradaySignalInput = {
  time: string;
  price: number | string;
  avgPrice: number | string;
  volume: number | string;
};

/** 逐分钟派生指标，供图表叠加与买卖点判定共用（同一数据源，保证一致） */
export type TIntradayMetric = {
  index: number;
  time: string;
  price: number;
  avgPrice: number;
  /** 价−均价 */
  diff: number;
  /** 乖离率（百分比，正=价在均价上方） */
  bias: number;
  volume: number;
  /** 截至当前的累计平均量 */
  cumAvgVolume: number;
  /** 当根量比 = volume / cumAvgVolume */
  volumeRatio: number;
};

export const DEFAULT_SIGNAL_PARAMS: Required<TIntradaySignalParams> = {
  biasThreshold: 0.0015,
  volumeRatio: 1.2,
  cooldownMinutes: 5,
};

/**
 * 计算逐分钟派生指标（乖离率、量比、累计均量）。
 * 图表副图与买卖点判定共用此结果，确保"看到的曲线=判定的依据"。
 */
export const computeIntradayMetrics = (
  points: TIntradaySignalInput[],
): TIntradayMetric[] => {
  let cumVolume = 0;
  return points.map((p, i) => {
    const price = toNumber(p.price);
    const avgPrice = toNumber(p.avgPrice);
    const volume = toNumber(p.volume);
    cumVolume += volume;
    const cumAvgVolume = cumVolume / (i + 1);
    const diff = avgPrice > 0 ? price - avgPrice : 0;
    return {
      index: i,
      time: p.time,
      price,
      avgPrice,
      diff,
      bias: avgPrice > 0 ? (diff / avgPrice) * 100 : 0,
      volume,
      cumAvgVolume,
      volumeRatio: cumAvgVolume > 0 ? volume / cumAvgVolume : 0,
    };
  });
};

/**
 * 价格线上穿/下穿均价线（VWAP）判定买卖点，并用乖离、量比、冷却三重过滤降噪。
 *
 * - 上穿（价格由均价下方升到上方）→ 买点 B
 * - 下穿（价格由均价上方落到下方）→ 卖点 S
 * - 仅当 |乖离| >= 阈值、当根量比 >= 阈值，且距上一信号 >= 冷却分钟时才确认。
 *
 * 纯函数，便于单测；首根无参照不出信号。
 */
export const computeIntradaySignals = (
  points: TIntradaySignalInput[],
  params: TIntradaySignalParams = {},
): TIntradaySignal[] => {
  const biasThreshold = params.biasThreshold ?? DEFAULT_SIGNAL_PARAMS.biasThreshold;
  const minVolumeRatio = params.volumeRatio ?? DEFAULT_SIGNAL_PARAMS.volumeRatio;
  const cooldown = params.cooldownMinutes ?? DEFAULT_SIGNAL_PARAMS.cooldownMinutes;

  const signals: TIntradaySignal[] = [];
  let prevDiff = 0;
  let lastSignalIndex = Number.NEGATIVE_INFINITY;

  computeIntradayMetrics(points).forEach((m) => {
    const { index: i, avgPrice, diff } = m;

    // 首根或均价无效：仅记录参照，不出信号
    if (i === 0 || avgPrice <= 0) {
      prevDiff = diff;
      return;
    }

    const crossedUp = prevDiff <= 0 && diff > 0;
    const crossedDown = prevDiff >= 0 && diff < 0;
    prevDiff = diff;

    if (!crossedUp && !crossedDown) return;
    if (Math.abs(m.bias) / 100 < biasThreshold) return;
    if (m.volumeRatio < minVolumeRatio) return;
    if (i - lastSignalIndex < cooldown) return;

    lastSignalIndex = i;
    signals.push({
      index: i,
      time: m.time,
      type: crossedUp ? "buy" : "sell",
      price: m.price,
      avgPrice: m.avgPrice,
      bias: m.bias,
      volumeRatio: m.volumeRatio,
    });
  });

  return signals;
};
