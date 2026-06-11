import { useQuery } from "@tanstack/react-query";
import type { Components } from "react-markdown";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { PageHeader } from "@/components/common/page-header";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/cn";

/**
 * API 文档页：运行时 fetch public/api-guide.md（由 `npm run sync:docs` 从
 * docs/API_GUIDE.md 复制而来），并渲染为页面，供第三方接入说明查阅。
 */
const fetchApiGuide = async (): Promise<string> => {
  const res = await fetch(`${import.meta.env.BASE_URL}api-guide.md`, {
    headers: { Accept: "text/markdown,text/plain" },
  });
  if (!res.ok) throw new Error(`加载 API 文档失败（${res.status}）`);
  return res.text();
};

const markdownComponents: Components = {
  h1: ({ className, ...props }) => (
    <h1
      className={cn(
        "mt-8 mb-4 border-b pb-2 text-3xl font-bold tracking-tight first:mt-0",
        className,
      )}
      {...props}
    />
  ),
  h2: ({ className, ...props }) => (
    <h2
      className={cn("mt-8 mb-3 border-b pb-1.5 text-2xl font-semibold tracking-tight", className)}
      {...props}
    />
  ),
  h3: ({ className, ...props }) => (
    <h3 className={cn("mt-6 mb-2 text-xl font-semibold", className)} {...props} />
  ),
  h4: ({ className, ...props }) => (
    <h4 className={cn("mt-5 mb-2 text-base font-semibold", className)} {...props} />
  ),
  p: ({ className, ...props }) => (
    <p className={cn("my-3 leading-7 text-sm", className)} {...props} />
  ),
  ul: ({ className, ...props }) => (
    <ul className={cn("my-3 ml-6 list-disc space-y-1.5 text-sm", className)} {...props} />
  ),
  ol: ({ className, ...props }) => (
    <ol className={cn("my-3 ml-6 list-decimal space-y-1.5 text-sm", className)} {...props} />
  ),
  li: ({ className, ...props }) => (
    <li className={cn("leading-7", className)} {...props} />
  ),
  a: ({ className, ...props }) => (
    <a
      className={cn("font-medium text-primary underline underline-offset-4", className)}
      target="_blank"
      rel="noreferrer"
      {...props}
    />
  ),
  blockquote: ({ className, ...props }) => (
    <blockquote
      className={cn(
        "my-4 border-l-4 border-border bg-muted/50 py-1 pl-4 pr-2 text-sm text-muted-foreground",
        className,
      )}
      {...props}
    />
  ),
  hr: ({ className, ...props }) => (
    <hr className={cn("my-8 border-border", className)} {...props} />
  ),
  table: ({ className, ...props }) => (
    <div className="my-4 w-full overflow-x-auto">
      <table className={cn("w-full border-collapse text-sm", className)} {...props} />
    </div>
  ),
  thead: ({ className, ...props }) => (
    <thead className={cn("bg-muted", className)} {...props} />
  ),
  th: ({ className, ...props }) => (
    <th
      className={cn("border px-3 py-2 text-left font-semibold", className)}
      {...props}
    />
  ),
  td: ({ className, ...props }) => (
    <td className={cn("border px-3 py-2 align-top", className)} {...props} />
  ),
  code: ({ className, children, ...props }) => {
    const isBlock = /\blanguage-/.test(className ?? "");
    if (isBlock) {
      return (
        <code className={cn("font-mono text-sm", className)} {...props}>
          {children}
        </code>
      );
    }
    return (
      <code
        className={cn(
          "rounded bg-muted px-1.5 py-0.5 font-mono text-[0.85em] text-foreground",
          className,
        )}
        {...props}
      >
        {children}
      </code>
    );
  },
  pre: ({ className, ...props }) => (
    <pre
      className={cn(
        "my-4 overflow-x-auto rounded-lg border bg-muted p-4 text-sm leading-6",
        className,
      )}
      {...props}
    />
  ),
};

export const ApiDocsPage = () => {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["api-guide"],
    queryFn: fetchApiGuide,
    staleTime: 5 * 60 * 1000,
  });

  return (
    <>
      <PageHeader
        title="开放 API 接入文档"
        description="第三方接入说明：secretKey 使用与开放接口契约"
      />
      <Card>
        <CardContent className="py-6">
          {isLoading ? (
            <p className="text-sm text-muted-foreground">加载中…</p>
          ) : isError ? (
            <p className="text-sm text-destructive">
              {(error as Error)?.message ?? "加载 API 文档失败"}
            </p>
          ) : (
            <div className="max-w-none">
              <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                {data ?? ""}
              </Markdown>
            </div>
          )}
        </CardContent>
      </Card>
    </>
  );
};
