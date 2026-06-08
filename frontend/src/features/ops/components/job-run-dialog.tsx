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
  const [error, setError] = useState<string | null>(null);

  const { data: results } = useStockSearch(kw.trim());

  // 打开（job 变化）时按入参重置表单状态。
  useEffect(() => {
    if (job) {
      setScope(initialScope);
      setCodes(initialCodes);
      setKw("");
      setManual("");
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
    if (scope === "codes" && codes.length === 0) {
      setError("请至少选择或输入一个股票代码");
      return;
    }
    try {
      const result = await runJob.mutateAsync({
        id: job.id,
        type: job.job_type,
        codes: scope === "codes" ? codes : undefined,
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
                选择触发范围：全量股票按证券列表逐只处理；也可指定个别代码单独补数。
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
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

              {scope === "codes" ? (
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
