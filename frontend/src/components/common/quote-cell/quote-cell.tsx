import { changeColor, formatPct, formatPrice } from "@/lib/decimal";
import { cn } from "@/lib/cn";

type TQuoteCellProps = {
  value: string | number;
  type?: "price" | "percent";
  className?: string;
};

export const QuoteCell = ({
  value,
  type = "price",
  className,
}: TQuoteCellProps) => (
  <span className={cn(changeColor(value), className)}>
    {type === "percent" ? formatPct(value) : formatPrice(value)}
  </span>
);
