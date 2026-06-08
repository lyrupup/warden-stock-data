import { create } from "zustand";

export type TToast = {
  id: string;
  title: string;
  description?: string;
  variant?: "default" | "destructive";
};

type TToastInput = Omit<TToast, "id">;

interface IToastState {
  toasts: TToast[];
  toast: (opts: TToastInput) => void;
  dismiss: (id: string) => void;
}

let toastId = 0;
/** 居中黑底 toast 通常 2s 自动消失 */
const AUTO_DISMISS_MS = 2000;

export const useToastStore = create<IToastState>()((set) => ({
  toasts: [],
  toast: (opts) => {
    const id = String(++toastId);
    // 居中 toast 同时只展示一条，新消息覆盖旧消息
    set({ toasts: [{ ...opts, id }] });
    setTimeout(() => {
      set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
    }, AUTO_DISMISS_MS);
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));

/** 全局 toast，任意模块可调用（由 App 挂载 ToastContainer 展示） */
export const toast = (opts: TToastInput) => useToastStore.getState().toast(opts);
