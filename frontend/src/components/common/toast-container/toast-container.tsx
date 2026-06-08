import { cn } from "@/lib/cn";
import type { TToast } from "@/hooks/use-toast";

/** 居中黑底 toast（类似移动端 / Vant Toast），非 shadcn 默认右下角卡片样式 */
export const ToastContainer = ({
  toasts,
  onDismiss,
}: {
  toasts: TToast[];
  onDismiss: (id: string) => void;
}) => {
  const latest = toasts.length > 0 ? toasts[toasts.length - 1] : null;
  if (!latest) return null;

  return (
    <div className="pointer-events-none fixed inset-0 z-[100] flex items-center justify-center">
      <div
        key={latest.id}
        role="status"
        aria-live="polite"
        className={cn(
          "pointer-events-auto max-w-[80vw] animate-in fade-in-0 zoom-in-95 rounded-lg px-5 py-2.5 text-center text-sm font-medium text-white shadow-lg duration-200",
          latest.variant === "destructive"
            ? "bg-destructive/90"
            : "bg-black/75",
        )}
        onClick={() => onDismiss(latest.id)}
      >
        <p>{latest.title}</p>
        {latest.description ? (
          <p className="mt-1 text-xs font-normal text-white/80">
            {latest.description}
          </p>
        ) : null}
      </div>
    </div>
  );
};
