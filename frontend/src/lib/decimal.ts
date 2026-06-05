/** 后端 decimal 字段为字符串；统一转 number 再计算/格式化 */
export const toNumber = (v: string | number | null | undefined): number =>
  v == null ? 0 : typeof v === "number" ? v : Number(v);

export const formatPct = (v: string | number): string =>
  `${toNumber(v).toFixed(2)}%`;

export const formatPrice = (v: string | number, digits = 2): string =>
  toNumber(v).toFixed(digits);

/** 成交量按万/亿压缩，贴合 A 股展示习惯 */
export const formatVolume = (v: string | number): string => {
  const n = toNumber(v);
  if (n >= 1e8) return `${(n / 1e8).toFixed(2)}亿`;
  if (n >= 1e4) return `${(n / 1e4).toFixed(2)}万`;
  return `${Math.round(n)}`;
};

/** A 股涨跌色：涨红跌绿 */
export const changeColor = (v: string | number): string => {
  const n = toNumber(v);
  if (n > 0) return "text-red-500";
  if (n < 0) return "text-green-500";
  return "text-muted-foreground";
};
