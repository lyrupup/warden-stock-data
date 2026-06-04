import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { TAdmin } from "@/types/admin";

const TOKEN_KEY = "warden_admin_token";

interface IAuthState {
  token: string | null;
  admin: TAdmin | null;
  login: (token: string, admin?: TAdmin | null) => void;
  setAdmin: (admin: TAdmin) => void;
  logout: () => void;
}

export const useAuthStore = create<IAuthState>()(
  persist(
    (set) => ({
      token: null,
      admin: null,
      login: (token, admin = null) => set({ token, admin }),
      setAdmin: (admin) => set({ admin }),
      logout: () => {
        set({ token: null, admin: null });
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
      },
    }),
    {
      name: TOKEN_KEY,
      partialize: (s) => ({ token: s.token, admin: s.admin }),
    },
  ),
);
