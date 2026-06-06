import { toNumber } from "./decimal";
import type { TIntradayMetric } from "./intraday-signals";

/**
 * 做 T（T+0 日内回转）研判与买卖点。四层：
 * 1. 历史基准：用昨日及以前的日 K 定振幅标尺与支撑压力。
 * 2. 当日趋势态：分时 + 昨收 + 历史，多因子合成 TrendScore。
 * 3. 是否适合做 T：振幅门槛 × 趋势态 × 流动性 → 正T/倒T/双向/不建议。
 * 4. 高抛低吸 B/S：围绕均价线 VWAP 的动态轨道，逆势低吸/高抛。
 *
 * 全部纯函数，便于单测；不接交易，信号仅供参考。
 */

export type TDayKline = {
  trade_date: string;
  open: string | number;
  high: string | number;
  low: string | number;
  close: string | number;
  volume: string | number;
};

/** 历史基准（不含今日） */
export type TDayTradeBaseline = {
  /** 近 N 日平均振幅（百分比） */
  atrPct: number;
  /** 单侧目标偏离（百分比）= clamp(atrPct/2, 0.4, 1.5) */
  refPct: number;
  /** 历史最近一根的 MA5 / MA20（收盘） */
  ma5: number;
  ma20: number;
  /** 近 N 日最低收盘 / 最高收盘（硬支撑/压力参考） */
  histLow: number;
  histHigh: number;
  /** 参与计算的历史交易日数 */
  days: number;
};

export type EDayTrendState =
  | "strong_up"
  | "weak_up"
  | "range"
  | "weak_down"
  | "strong_down";

export type TDayTrend = {
  score: number;
  state: EDayTrendState;
  /** 跳空幅度（百分比） */
  gapPct: number;
  /** 预期振幅（百分比）= max(历史 ATR%, 今日已实现振幅) */
  expAmp: number;
};

export type EDayTradeMode = "none" | "positive_t" | "reverse_t" | "both";

export type TDayTradePlan = {
  mode: EDayTradeMode;
  suitable: boolean;
  reasons: string[];
};

export type TDayTradeSignalType = "dip_buy" | "rally_sell";

export type TDayTradeSignal = {
  index: number;
  time: string;
  type: TDayTradeSignalType;
  price: number;
  vwap: number;
  /** 相对均价乖离（百分比） */
  bias: number;
  /** 触发说明 */
  reason: string;
};

export type TDayTradeBandPoint = {
  time: string;
  vwap: number;
  upper: number;
  lower: number;
};

export type TDayTradeParams = {
  /** 历史基准回看天数 */
  baselineDays?: number;
  /** 适合做 T 的最小预期振幅（百分比） */
  minAmp?: number;
  /** 趋势均价斜率回看分钟数 */
  slopeWindow?: number;
  /** 低吸量比上限（缩量承接才算企稳） */
  volCalmMax?: number;
  /** 信号冷却分钟数 */
  cooldownMinutes?: number;
  /** 高抛低吸轨道整体宽窄系数（×refPct） */
  bandScale?: number;
};

export const DEFAULT_DAYTRADE_PARAMS: Required<TDayTradeParams> = {
  baselineDays: 20,
  minAmp: 2.0,
  slopeWindow: 15,
  volCalmMax: 2.0,
  cooldownMinutes: 30,
  bandScale: 1.0,
};

const clamp = (v: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, v));

const mean = (xs: number[]) =>
  xs.length === 0 ? 0 : xs.reduce((a, b) => a + b, 0) / xs.length;

const maOf = (closes: number[], period: number): number => {
  if (closes.length < period) return 0;
  return mean(closes.slice(closes.length - period));
};

/** 历史基准：排除今日，取近 N 日。 */
export const computeDayTradeBaseline = (
  klines: TDayKline[],
  todayDate: string,
  params: TDayTradeParams = {},
): TDayTradeBaseline => {
  const days = params.baselineDays ?? DEFAULT_DAYTRADE_PARAMS.baselineDays;
  const hist = klines.filter((k) => k.trade_date < todayDate);
  const recent = hist.slice(Math.max(0, hist.length - days));

  const amps: number[] = [];
  for (let i = 1; i < recent.length; i++) {
    const prevClose = toNumber(recent[i - 1].close);
    if (prevClose <= 0) continue;
    amps.push(
      ((toNumber(recent[i].high) - toNumber(recent[i].low)) / prevClose) * 100,
    );
  }
  const atrPct = amps.length > 0 ? mean(amps) : 0;
  const closes = hist.map((k) => toNumber(k.close));
  const recentCloses = recent.map((k) => toNumber(k.close));

  return {
    atrPct,
    refPct: clamp(atrPct / 2, 0.4, 1.5),
    ma5: maOf(closes, 5),
    ma20: maOf(closes, 20),
    histLow: recentCloses.length > 0 ? Math.min(...recentCloses) : 0,
    histHigh: recentCloses.length > 0 ? Math.max(...recentCloses) : 0,
    days: recent.length,
  };
};

const stateOf = (score: number): EDayTrendState => {
  if (score >= 60) return "strong_up";
  if (score >= 20) return "weak_up";
  if (score > -20) return "range";
  if (score > -60) return "weak_down";
  return "strong_down";
};

/** 当日趋势态：多因子合成 [-100,100]。 */
export const computeDayTrend = (
  metrics: TIntradayMetric[],
  preClose: number,
  baseline: TDayTradeBaseline,
  params: TDayTradeParams = {},
): TDayTrend => {
  const slopeWindow = params.slopeWindow ?? DEFAULT_DAYTRADE_PARAMS.slopeWindow;
  if (metrics.length === 0 || preClose <= 0) {
    return { score: 0, state: "range", gapPct: 0, expAmp: baseline.atrPct };
  }

  const open = metrics[0].price;
  const last = metrics[metrics.length - 1];
  const price = last.price;
  const vwap = last.avgPrice;
  const prices = metrics.map((m) => m.price);
  const dayHigh = Math.max(...prices);
  const dayLow = Math.min(...prices);

  const gapPct = ((open - preClose) / preClose) * 100;

  // 因子归一化到 [-1,1]
  const fGap = clamp(gapPct / 2, -1, 1);
  const fPricePos = clamp(last.bias / 1, -1, 1);

  const slopeIdx = Math.max(0, metrics.length - 1 - slopeWindow);
  const vwapPast = metrics[slopeIdx].avgPrice;
  const fSlope =
    vwapPast > 0 ? clamp((((vwap - vwapPast) / vwapPast) * 100) / 0.5, -1, 1) : 0;

  const fDayPos =
    dayHigh > dayLow ? ((price - dayLow) / (dayHigh - dayLow)) * 2 - 1 : 0;

  let upVol = 0;
  let downVol = 0;
  for (let i = 1; i < metrics.length; i++) {
    const v = metrics[i].volume;
    if (metrics[i].price >= metrics[i - 1].price) upVol += v;
    else downVol += v;
  }
  const fVol = upVol + downVol > 0 ? (upVol - downVol) / (upVol + downVol) : 0;

  let fMa = 0;
  if (baseline.ma5 > 0) fMa += price >= baseline.ma5 ? 0.5 : -0.5;
  if (baseline.ma20 > 0) fMa += price >= baseline.ma20 ? 0.5 : -0.5;

  const score =
    100 *
    (0.15 * fGap +
      0.2 * fPricePos +
      0.2 * fSlope +
      0.15 * fDayPos +
      0.15 * fVol +
      0.15 * fMa);

  const todayAmp = ((dayHigh - dayLow) / preClose) * 100;
  const expAmp = Math.max(baseline.atrPct, todayAmp);

  return { score, state: stateOf(score), gapPct, expAmp };
};

/** 是否适合做 T，及建议模式。 */
export const computeDayTradePlan = (
  trend: TDayTrend,
  params: TDayTradeParams = {},
): TDayTradePlan => {
  const minAmp = params.minAmp ?? DEFAULT_DAYTRADE_PARAMS.minAmp;
  const reasons: string[] = [];

  if (trend.expAmp < minAmp) {
    reasons.push(
      `预期振幅 ${trend.expAmp.toFixed(2)}% < ${minAmp}%，波动不足以覆盖回转成本`,
    );
    return { mode: "none", suitable: false, reasons };
  }
  reasons.push(`预期振幅 ${trend.expAmp.toFixed(2)}%，具备做 T 空间`);

  let mode: EDayTradeMode;
  switch (trend.state) {
    case "strong_up":
      mode = "reverse_t";
      reasons.push("强多单边：宜倒 T（冲高用底仓减、回踩接回），禁裸追");
      break;
    case "strong_down":
      mode = "positive_t";
      reasons.push("强空单边：仅正 T 抢反弹，超跌低吸、反弹即抛并严格止损");
      break;
    case "weak_up":
      mode = "positive_t";
      reasons.push("偏多震荡：以正 T 为主，浅回调低吸、冲高高抛");
      break;
    case "weak_down":
      mode = "reverse_t";
      reasons.push("偏空震荡：偏倒 T / 轻仓，反弹高抛、深跌再吸");
      break;
    default:
      mode = "both";
      reasons.push("横盘震荡：双向网格高抛低吸最佳");
  }
  return { mode, suitable: true, reasons };
};

/**
 * 趋势态对低吸/高抛轨道宽度的自适应系数。
 * 偏精信号取向：整体拉宽，仅在更极端的偏离处触发。
 */
const bandFactors = (state: EDayTrendState): { kBuy: number; kSell: number } => {
  switch (state) {
    case "weak_up":
      return { kBuy: 0.9, kSell: 1.4 };
    case "weak_down":
      return { kBuy: 1.5, kSell: 0.9 };
    case "strong_up":
      return { kBuy: 0.7, kSell: 1.2 };
    case "strong_down":
      return { kBuy: 1.2, kSell: 0.7 };
    default:
      return { kBuy: 1.2, kSell: 1.2 };
  }
};

export type TDayTradeResult = {
  band: TDayTradeBandPoint[];
  signals: TDayTradeSignal[];
};

/** 高抛低吸轨道 + B/S 信号。 */
export const computeDayTradeSignals = (
  metrics: TIntradayMetric[],
  baseline: TDayTradeBaseline,
  trend: TDayTrend,
  plan: TDayTradePlan,
  params: TDayTradeParams = {},
): TDayTradeResult => {
  const volCalmMax = params.volCalmMax ?? DEFAULT_DAYTRADE_PARAMS.volCalmMax;
  const cooldown = params.cooldownMinutes ?? DEFAULT_DAYTRADE_PARAMS.cooldownMinutes;
  const bandScale = params.bandScale ?? DEFAULT_DAYTRADE_PARAMS.bandScale;
  const { kBuy, kSell } = bandFactors(trend.state);
  const off = (baseline.refPct / 100) * bandScale;

  const band: TDayTradeBandPoint[] = metrics.map((m) => ({
    time: m.time,
    vwap: m.avgPrice,
    upper: m.avgPrice * (1 + kSell * off),
    lower: m.avgPrice * (1 - kBuy * off),
  }));

  const signals: TDayTradeSignal[] = [];
  if (!plan.suitable) return { band, signals };

  // 硬支撑：历史近低与 MA20 取较低者（破位则不低吸并预警）
  const support = Math.min(
    baseline.histLow > 0 ? baseline.histLow : Number.POSITIVE_INFINITY,
    baseline.ma20 > 0 ? baseline.ma20 : Number.POSITIVE_INFINITY,
  );

  let lastIdx = Number.NEGATIVE_INFINITY;
  metrics.forEach((m, i) => {
    if (i < 2) return;
    if (m.avgPrice <= 0) return;
    const b = band[i];
    const price = m.price;
    const p1 = metrics[i - 1].price;
    const p2 = metrics[i - 2].price;
    if (i - lastIdx < cooldown) return;

    // 低吸：触及下轨 + 拐头企稳(不再创新低) + 缩量承接 + 未破硬支撑
    const touchLower = price <= b.lower;
    const turnedUp = price >= p1 && p1 <= p2;
    const calm = m.volumeRatio <= volCalmMax;
    const aboveSupport = !(price < support);
    if (
      (plan.mode === "positive_t" || plan.mode === "both") &&
      touchLower &&
      turnedUp &&
      calm &&
      aboveSupport
    ) {
      signals.push({
        index: i,
        time: m.time,
        type: "dip_buy",
        price,
        vwap: m.avgPrice,
        bias: m.bias,
        reason: `触及低吸轨(乖离${m.bias.toFixed(2)}%)且缩量企稳`,
      });
      lastIdx = i;
      return;
    }

    // 高抛：触及上轨 + 滞涨(不再创新高)
    const touchUpper = price >= b.upper;
    const stalled = price <= p1 && p1 >= p2;
    if (
      (plan.mode === "reverse_t" ||
        plan.mode === "both" ||
        plan.mode === "positive_t") &&
      touchUpper &&
      stalled
    ) {
      signals.push({
        index: i,
        time: m.time,
        type: "rally_sell",
        price,
        vwap: m.avgPrice,
        bias: m.bias,
        reason: `触及高抛轨(乖离+${m.bias.toFixed(2)}%)且冲高滞涨`,
      });
      lastIdx = i;
    }
  });

  return { band, signals };
};

export type EDaySessionLevel = "low" | "medium" | "high" | "complete";

export type TDaySessionMaturity = {
  /** 已走过的交易分钟（按 A 股 09:30-11:30 / 13:00-15:00 计） */
  elapsedMinutes: number;
  totalMinutes: number;
  ratio: number;
  level: EDaySessionLevel;
  label: string;
};

/** 时分（HH:MM）→ 当日已交易分钟（0~240）。 */
const elapsedTradingMinutes = (hh: number, mm: number): number => {
  const m = hh * 60 + mm;
  const OPEN = 570; // 09:30
  const MORNING_END = 690; // 11:30
  const NOON = 780; // 13:00
  const CLOSE = 900; // 15:00
  if (m <= OPEN) return 0;
  if (m <= MORNING_END) return m - OPEN;
  if (m <= NOON) return 120;
  if (m <= CLOSE) return 120 + (m - NOON);
  return 240;
};

/**
 * 分时数据成熟度：盘中逐步拉取时，研判可靠度随已走过的交易时长变化。
 * 早盘样本不足→低；午后→较高；接近/已收盘→全日完整。
 */
export const computeSessionMaturity = (
  points: { time: string }[],
): TDaySessionMaturity => {
  const total = 240;
  let elapsed = 0;
  const last = points[points.length - 1];
  if (last) {
    const mt = last.time.match(/T(\d{2}):(\d{2})/);
    if (mt) elapsed = elapsedTradingMinutes(+mt[1], +mt[2]);
  }
  const ratio = elapsed / total;

  let level: EDaySessionLevel;
  let label: string;
  if (elapsed >= 235) {
    level = "complete";
    label = "全日完整";
  } else if (elapsed >= 120) {
    level = "high";
    label = "午后较稳";
  } else if (elapsed >= 30) {
    level = "medium";
    label = "上午渐明";
  } else {
    level = "low";
    label = "早盘样本不足";
  }

  return { elapsedMinutes: elapsed, totalMinutes: total, ratio, level, label };
};
