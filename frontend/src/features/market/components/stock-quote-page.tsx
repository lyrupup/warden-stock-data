import { Search } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { PageHeader } from "@/components/common/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useStockSearch } from "../hooks/use-market";

export const StockQuotePage = () => {
  const navigate = useNavigate();
  const [kw, setKw] = useState("");
  const [query, setQuery] = useState("");
  const [showResults, setShowResults] = useState(false);

  const { data: searchResults, isFetching: searching } = useStockSearch(query);

  const submitSearch = () => {
    const kwTrimmed = kw.trim();
    if (kwTrimmed.length < 2) return;
    setQuery(kwTrimmed);
    setShowResults(true);
  };

  return (
    <>
      <PageHeader title="个股行情" description="搜索标的查看快照、K 线与均线指标" />

      <div className="relative mb-6 max-w-md">
        <div className="flex gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="pl-9"
              placeholder="输入代码或名称，回车或点击搜索…"
              value={kw}
              onChange={(e) => {
                setKw(e.target.value);
                setShowResults(false);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") submitSearch();
              }}
            />
          </div>
          <Button onClick={submitSearch} disabled={kw.trim().length < 2}>
            <Search className="mr-1 h-4 w-4" />
            搜索
          </Button>
        </div>
        {showResults ? (
          <div className="absolute z-10 mt-1 w-full rounded-md border bg-background shadow-lg">
            {searching ? (
              <div className="px-4 py-2 text-sm text-muted-foreground">
                搜索中…
              </div>
            ) : searchResults && searchResults.length > 0 ? (
              searchResults.map((s) => (
                <button
                  key={s.stock_code}
                  type="button"
                  className="flex w-full items-center justify-between px-4 py-2 text-left text-sm hover:bg-muted"
                  onClick={() => navigate(`/market/quote/${s.stock_code}`)}
                >
                  <span className="font-mono">{s.stock_code}</span>
                  <span className="text-muted-foreground">{s.stock_name}</span>
                </button>
              ))
            ) : (
              <div className="px-4 py-2 text-sm text-muted-foreground">
                未找到匹配标的
              </div>
            )}
          </div>
        ) : null}
      </div>

      <p className="text-muted-foreground">请搜索并选择一只股票查看行情</p>
    </>
  );
};
