import { create } from "zustand";
import { persist } from "zustand/middleware";

type TTheme = "light" | "dark";

interface IThemeState {
  theme: TTheme;
  setTheme: (theme: TTheme) => void;
  toggleTheme: () => void;
}

const applyTheme = (theme: TTheme) => {
  const root = document.documentElement;
  root.classList.toggle("dark", theme === "dark");
};

export const useThemeStore = create<IThemeState>()(
  persist(
    (set, get) => ({
      theme: "light",
      setTheme: (theme) => {
        applyTheme(theme);
        set({ theme });
      },
      toggleTheme: () => {
        const next = get().theme === "light" ? "dark" : "light";
        applyTheme(next);
        set({ theme: next });
      },
    }),
    {
      name: "warden_theme",
      onRehydrateStorage: () => (state) => {
        if (state) applyTheme(state.theme);
      },
    },
  ),
);
