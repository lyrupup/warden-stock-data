import { changeColor, formatPct, formatPrice, toNumber } from "@/lib/decimal";
import { cn } from "@/lib/cn";

type TQuoteCellProps = {
  value: string | number;
  type?: "price" | "percent";
  /** 为正值补前导 "+"，强化涨跌方向（涨跌额/涨跌幅展示用） */
  signed?: boolean;
  className?: string;
};

export const QuoteCell = ({
  value,
  type = "price",
  signed = false,
  className,
}: TQuoteCellProps) => {
  const text = type === "percent" ? formatPct(value) : formatPrice(value);
  const prefix = signed && toNumber(value) > 0 ? "+" : "";
  return (
    <span className={cn(changeColor(value), className)}>
      {prefix}
      {text}
    </span>
  );
};
