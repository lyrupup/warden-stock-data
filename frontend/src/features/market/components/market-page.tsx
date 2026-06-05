import { Link } from "react-router-dom";
import { PageHeader } from "@/components/common/page-header";
import { QuoteCell } from "@/components/common/quote-cell";
import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { changeColor, formatPrice } from "@/lib/decimal";
import { cn } from "@/lib/cn";
import { useIndices } from "../hooks/use-market";

/** 大盘指数置顶展示顺序（其余指数按后端目录原序排在后面） */
const INDEX_ORDER = ["000001", "399001", "399006", "000680", "000688", "000300"];

const indexRank = (code: string): number => {
  const i = INDEX_ORDER.indexOf(code);
  return i === -1 ? INDEX_ORDER.length : i;
};

export const MarketPage = () => {
  const { data, isLoading, isError } = useIndices();

  const sortedIndices = data
    ? [...data].sort((a, b) => indexRank(a.index_code) - indexRank(b.index_code))
    : [];

  return (
    <>
      <PageHeader
        title="行情中心"
        description="大盘指数概览"
        actions={
          <Button asChild variant="outline">
            <Link to="/market/quote">个股行情 →</Link>
          </Button>
        }
      />

      {isLoading ? (
        <p className="text-muted-foreground">加载中…</p>
      ) : isError ? (
        <EmptyState message="无法加载指数行情，请确认后端服务已启动" />
      ) : !data?.length ? (
        <EmptyState message="暂无指数数据" />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {sortedIndices.map((idx) => (
            <Card key={idx.index_code}>
              <CardHeader className="pb-1">
                <CardTitle className="flex items-baseline justify-between text-sm font-medium">
                  <span>{idx.index_name}</span>
                  <span className="text-xs font-normal text-muted-foreground">
                    {idx.index_code}
                  </span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className={cn("text-2xl font-bold", changeColor(idx.change_percent))}>
                  {formatPrice(idx.price)}
                </div>
                <div className="mt-1 flex items-baseline justify-between text-sm">
                  <QuoteCell value={idx.change_amount} signed />
                  <QuoteCell value={idx.change_percent} type="percent" signed />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </>
  );
};
