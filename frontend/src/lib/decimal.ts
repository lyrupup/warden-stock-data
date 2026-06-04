/** 后端 decimal 字段为字符串；统一转 number 再计算/格式化 */
export const toNumber = (v: string | number | null | undefined): number =>
  v == null ? 0 : typeof v === "number" ? v : Number(v);

export const formatPct = (v: string | number): string =>
  `${toNumber(v).toFixed(2)}%`;

export const formatPrice = (v: string | number, digits = 2): string =>
  toNumber(v).toFixed(digits);

/** A 股涨跌色：涨红跌绿 */
export const changeColor = (v: string | number): string => {
  const n = toNumber(v);
  if (n > 0) return "text-red-500";
  if (n < 0) return "text-green-500";
  return "text-muted-foreground";
};
