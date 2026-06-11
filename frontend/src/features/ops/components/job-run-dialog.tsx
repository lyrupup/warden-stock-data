import { Check, Copy, RotateCw, Search, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useStockSearch } from "@/features/market/hooks/use-market";
import { AppError } from "@/core/http-client";
import type { TJobRun, TJobType } from "@/types/admin";
import { useRunJob } from "../hooks/use-ops";

export type TRunDialogJob = {
  id: number;
  name: string;
  job_type: TJobType;
};

/**
 * 解析 run 中以逗号分隔的代码列表（failed_codes / skipped_codes 共用格式）；
 * 超量时尾部附 "…(共 N 个)" 标注，这里剔除标注 token，返回纯代码数组。
 */
export const parseRunCodes = (raw?: string): string[] => {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s !== "" && !s.startsWith("…") && !s.includes("共"));
};

/** @deprecated 使用 parseRunCodes */
export const parseFailedCodes = parseRunCodes;

/** 把用户手输的代码串（逗号 / 空格 / 换行分隔）规整为去重代码数组。 */
const parseManualCodes = (raw: string): string[] =>
  raw
    .split(/[\s,，、]+/)
    .map((s) => s.trim())
    .filter(Boolean);

const dedupe = (arr: string[]) => Array.from(new Set(arr));

type TScope = "all" | "codes";

export const JobRunDialog = ({
  job,
  initialScope = "all",
  initialCodes = [],
  onClose,
  onSubmitted,
}: {
  job: TRunDialogJob | null;
  initialScope?: TScope;
  initialCodes?: string[];
  onClose: () => void;
  onSubmitted: (runId: number) => void;
}) => {
  const runJob = useRunJob();
  const [scope, setScope] = useState<TScope>(initialScope);
  const [codes, setCodes] = useState<string[]>(initialCodes);
  const [kw, setKw] = useState("");
  const [manual, setManual] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [error, setError] = useState<string | null>(null);

  const { data: results } = useStockSearch(kw.trim());

  // 交易日历作业：只按日期区间拉取，不涉及股票代码。
  const isCalendar = job?.job_type === "calendar";
  // 周级 baostock 对齐作业：按指定区间用 baostock 覆盖该区间 K 线（source 翻 baostock）+刷因子。
  const isFactors = job?.job_type === "factors";
  // 股票范围仅对逐只处理的作业有意义；日历作业无股票维度。
  const supportsScope = !isCalendar;
  // 日期区间对日 K 采集类作业（全量 / 增量 / 周级对齐）与交易日历作业均有意义。
  const supportsDateRange =
    job?.job_type === "full" ||
    job?.job_type === "incremental" ||
    isFactors ||
    isCalendar;

  // 打开（job 变化）时按入参重置表单状态。
  useEffect(() => {
    if (job) {
      setScope(initialScope);
      setCodes(initialCodes);
      setKw("");
      setManual("");
      setFromDate("");
      setToDate("");
      setError(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [job?.id, initialScope, initialCodes.join(",")]);

  const addCode = (code: string) => {
    setCodes((prev) => dedupe([...prev, code]));
  };
  const removeCode = (code: string) => {
    setCodes((prev) => prev.filter((c) => c !== code));
  };
  const addManual = () => {
    const parsed = parseManualCodes(manual);
    if (parsed.length) {
      setCodes((prev) => dedupe([...prev, ...parsed]));
      setManual("");
    }
  };

  const submit = async () => {
    if (!job) return;
    setError(null);
    if (supportsScope && scope === "codes" && codes.length === 0) {
      setError("请至少选择或输入一个股票代码");
      return;
    }
    const from = fromDate.trim();
    const to = toDate.trim();
    if (supportsDateRange && from && to && from > to) {
      setError("结束日期不能早于起始日期");
      return;
    }
    try {
      const result = await runJob.mutateAsync({
        id: job.id,
        type: job.job_type,
        codes: supportsScope && scope === "codes" ? codes : undefined,
        fromDate: supportsDateRange && from ? from : undefined,
        toDate: supportsDateRange && to ? to : undefined,
      });
      onSubmitted(result.runId);
      onClose();
    } catch (e) {
      setError(e instanceof AppError ? e.message : "触发失败，请重试");
    }
  };

  return (
    <Dialog
      open={!!job}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        {job ? (
          <>
            <DialogHeader>
              <DialogTitle>触发作业 · {job.name}</DialogTitle>
              <DialogDescription>
                {isCalendar
                  ? "同步 baostock 官方交易日历入库。可选拉取日期区间；留空默认从最早历史拉到今年年底。"
                  : isFactors
                    ? "周级 baostock 对齐：按所选日期区间用 baostock 重拉该区间全市场日 K，覆盖 gotdx 数据（行 source 翻为 baostock）并刷新复权因子、证券列表。区间留空默认对齐最近 7 个交易日。"
                    : "选择触发范围：全量股票按证券列表逐只处理；也可指定个别代码单独补数。"}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              {supportsScope ? (
                <div className="grid gap-2">
                  <Label>触发范围</Label>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      size="sm"
                      variant={scope === "all" ? "default" : "outline"}
                      onClick={() => setScope("all")}
                    >
                      全量股票
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant={scope === "codes" ? "default" : "outline"}
                      onClick={() => setScope("codes")}
                    >
                      指定股票代码
                    </Button>
                  </div>
                </div>
              ) : null}

              {supportsScope && scope === "codes" ? (
                <div className="space-y-3">
                  <div className="grid gap-2">
                    <Label htmlFor="job-code-search">搜索添加</Label>
                    <div className="relative">
                      <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                      <Input
                        id="job-code-search"
                        className="pl-8"
                        placeholder="输入代码或名称（≥2 字）搜索"
                        value={kw}
                        onChange={(e) => setKw(e.target.value)}
                      />
                    </div>
                    {kw.trim().length >= 2 && results && results.length > 0 ? (
                      <div className="max-h-40 overflow-auto rounded-md border">
                        {results.map((s) => (
                          <button
                            key={s.stock_code}
                            type="button"
                            className="flex w-full items-center justify-between px-3 py-1.5 text-left text-sm hover:bg-muted"
                            onClick={() => addCode(s.stock_code)}
                          >
                            <span>
                              <span className="font-medium">{s.stock_code}</span>{" "}
                              <span className="text-muted-foreground">
                                {s.stock_name}
                              </span>
                            </span>
                            {codes.includes(s.stock_code) ? (
                              <Check className="h-4 w-4 text-green-600" />
                            ) : null}
                          </button>
                        ))}
                      </div>
                    ) : null}
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="job-code-manual">批量粘贴</Label>
                    <div className="flex gap-2">
                      <Input
                        id="job-code-manual"
                        placeholder="多个代码用逗号 / 空格分隔，回车添加"
                        value={manual}
                        onChange={(e) => setManual(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            addManual();
                          }
                        }}
                      />
                      <Button type="button" variant="outline" onClick={addManual}>
                        添加
                      </Button>
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <Label>已选 {codes.length} 个</Label>
                    {codes.length ? (
                      <div className="flex max-h-32 flex-wrap gap-1.5 overflow-auto rounded-md border p-2">
                        {codes.map((c) => (
                          <Badge key={c} variant="secondary" className="gap-1">
                            {c}
                            <button
                              type="button"
                              aria-label={`移除 ${c}`}
                              onClick={() => removeCode(c)}
                            >
                              <X className="h-3 w-3" />
                            </button>
                          </Badge>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-muted-foreground">
                        尚未选择任何代码
                      </p>
                    )}
                  </div>
                </div>
              ) : null}

              {supportsDateRange ? (
                <div className="grid gap-2">
                  <Label>
                    {isCalendar
                      ? "拉取日期区间（可选）"
                      : isFactors
                        ? "对齐日期区间（可选）"
                        : "回补日期区间（可选）"}
                  </Label>
                  <div className="flex items-center gap-2">
                    <Input
                      type="date"
                      aria-label="起始日期"
                      value={fromDate}
                      onChange={(e) => setFromDate(e.target.value)}
                    />
                    <span className="text-muted-foreground">至</span>
                    <Input
                      type="date"
                      aria-label="结束日期"
                      value={toDate}
                      onChange={(e) => setToDate(e.target.value)}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {isCalendar
                      ? "留空：起点取最早历史（BACKFILL_START_DATE），终点默认到今年年底（拉全当年节假日）。超出 baostock 已发布范围的未来日期会被自动忽略。"
                      : isFactors
                        ? "留空默认对齐最近 7 个交易日（当天为交易日则含当天）。也可指定要对齐的某段区间。将对全市场（或指定代码）用 baostock 重拉该区间日 K 覆盖 gotdx 数据并刷因子。baostock 串行，全市场约数小时，建议非交易时段执行。"
                        : "留空：增量按各自水位起点、全量按默认起始日。指定区间后将对所选范围内全部代码按该区间回补（跳过「仅落后标的」预筛）。如只拉某一交易日，把起止设为同一天。"}
                  </p>
                </div>
              ) : null}

              {error ? <p className="text-sm text-red-600">{error}</p> : null}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={onClose}>
                取消
              </Button>
              <Button disabled={runJob.isPending} onClick={() => void submit()}>
                确认触发
              </Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
};

export const FailedCodesDialog = ({
  run,
  onClose,
  onRerun,
}: {
  run: TJobRun | null;
  onClose: () => void;
  onRerun: (run: TJobRun, codes: string[]) => void;
}) => {
  const [copied, setCopied] = useState(false);
  const codes = useMemo(() => parseRunCodes(run?.failed_codes), [run]);

  useEffect(() => {
    setCopied(false);
  }, [run]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(codes.join(","));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <Dialog
      open={!!run}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        {run ? (
          <>
            <DialogHeader>
              <DialogTitle>未成功代码 · Run #{run.id}</DialogTitle>
              <DialogDescription>
                以下标的本次未成功或未完整处理，可复制后排查，或一键仅针对这些代码重跑补数。
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                共 {codes.length} 个失败代码
                {run.failed_codes?.includes("共") ? "（已截断展示）" : ""}
              </p>
              <div className="max-h-60 overflow-auto rounded-md border bg-muted/40 p-3 text-sm leading-relaxed">
                {codes.length ? codes.join(", ") : "无"}
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => void copy()}>
                {copied ? (
                  <Check className="mr-1 h-4 w-4" />
                ) : (
                  <Copy className="mr-1 h-4 w-4" />
                )}
                {copied ? "已复制" : "复制代码"}
              </Button>
              <Button
                disabled={codes.length === 0}
                onClick={() => onRerun(run, codes)}
              >
                <RotateCw className="mr-1 h-4 w-4" />
                重跑这些代码
              </Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
};

export const SkippedCodesDialog = ({
  run,
  onClose,
}: {
  run: TJobRun | null;
  onClose: () => void;
}) => {
  const [copied, setCopied] = useState(false);
  const codes = useMemo(() => parseRunCodes(run?.skipped_codes), [run]);

  useEffect(() => {
    setCopied(false);
  }, [run]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(codes.join(","));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <Dialog
      open={!!run}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        {run ? (
          <>
            <DialogHeader>
              <DialogTitle>无行情代码 · Run #{run.id}</DialogTitle>
              <DialogDescription>
                以下标的在数据源中既无历史日 K、又无有效实时快照（量价全 0），
                属于「已登记代码但暂无任何成交行情」的正常状态（典型为新股尚未上市 /
                长期停牌无量价）。本次作业已跳过，不计入失败。
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                共 {codes.length} 个无行情代码
                {run.skipped_codes?.includes("共") ? "（已截断展示）" : ""}
              </p>
              <div className="max-h-60 overflow-auto rounded-md border bg-muted/40 p-3 text-sm leading-relaxed">
                {codes.length ? codes.join(", ") : "无"}
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => void copy()}>
                {copied ? (
                  <Check className="mr-1 h-4 w-4" />
                ) : (
                  <Copy className="mr-1 h-4 w-4" />
                )}
                {copied ? "已复制" : "复制代码"}
              </Button>
              <Button onClick={onClose}>关闭</Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
};
