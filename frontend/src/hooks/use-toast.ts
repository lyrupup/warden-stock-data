import { useCallback, useState } from "react";

export type TToast = {
  id: string;
  title: string;
  description?: string;
  variant?: "default" | "destructive";
};

let toastId = 0;

export const useToast = () => {
  const [toasts, setToasts] = useState<TToast[]>([]);

  const toast = useCallback(
    (opts: Omit<TToast, "id">) => {
      const id = String(++toastId);
      setToasts((prev) => [...prev, { ...opts, id }]);
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
      }, 4000);
    },
    [],
  );

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return { toasts, toast, dismiss };
};
