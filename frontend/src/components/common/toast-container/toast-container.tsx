import { cn } from "@/lib/cn";
import type { TToast } from "@/hooks/use-toast";

export const ToastContainer = ({
  toasts,
  onDismiss,
}: {
  toasts: TToast[];
  onDismiss: (id: string) => void;
}) => (
  <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2">
    {toasts.map((t) => (
      <div
        key={t.id}
        role="alert"
        className={cn(
          "min-w-[280px] rounded-lg border bg-background p-4 shadow-lg",
          t.variant === "destructive" && "border-destructive",
        )}
        onClick={() => onDismiss(t.id)}
      >
        <p className="font-medium">{t.title}</p>
        {t.description ? (
          <p className="mt-1 text-sm text-muted-foreground">{t.description}</p>
        ) : null}
      </div>
    ))}
  </div>
);
