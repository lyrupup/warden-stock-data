import { Link } from "react-router-dom";
import { PageHeader } from "@/components/common/page-header";
import { QuoteCell } from "@/components/common/quote-cell";
import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatPrice } from "@/lib/decimal";
import { useIndices } from "../hooks/use-market";

export const MarketPage = () => {
  const { data, isLoading, isError } = useIndices();

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
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.map((idx) => (
            <Card key={idx.index_code}>
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-medium">
                  {idx.index_name}
                  <span className="ml-2 text-xs text-muted-foreground">
                    {idx.index_code}
                  </span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-3xl font-bold">{formatPrice(idx.price)}</div>
                <div className="mt-2 flex gap-4 text-sm">
                  <QuoteCell value={idx.change_amount} />
                  <QuoteCell value={idx.change_percent} type="percent" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </>
  );
};
