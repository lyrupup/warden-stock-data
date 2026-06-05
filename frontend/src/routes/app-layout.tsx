import {
  Activity,
  BarChart3,
  Database,
  Key,
  LayoutDashboard,
  LogOut,
  Moon,
  Sun,
} from "lucide-react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { useAuthStore } from "@/stores/auth-store";
import { useThemeStore } from "@/stores/theme-store";

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "概览", end: true },
  { to: "/credentials", icon: Key, label: "凭证管理", end: false },
  { to: "/market", icon: BarChart3, label: "行情中心", end: true },
  { to: "/market/quote", icon: Activity, label: "个股行情", end: false },
  { to: "/ops/datasources", icon: Database, label: "数据源", end: false },
  { to: "/ops/jobs", icon: Activity, label: "更新作业", end: false },
];

export const AppLayout = () => {
  const navigate = useNavigate();
  const admin = useAuthStore((s) => s.admin);
  const logout = useAuthStore((s) => s.logout);
  const { theme, toggleTheme } = useThemeStore();

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-56 shrink-0 border-r bg-card md:block">
        <div className="flex h-14 items-center border-b px-4 font-semibold">
          {import.meta.env.VITE_APP_TITLE ?? "守望者"}
        </div>
        <nav className="space-y-1 p-3">
          {navItems.map(({ to, icon: Icon, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                )
              }
            >
              <Icon className="h-4 w-4" />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="flex h-14 items-center justify-between border-b px-4 md:px-6">
          <div className="md:hidden font-semibold text-sm">
            {import.meta.env.VITE_APP_TITLE}
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <span className="hidden text-sm text-muted-foreground sm:inline">
              {admin?.username ?? "管理员"}
            </span>
            <Button variant="ghost" size="icon" onClick={toggleTheme}>
              {theme === "dark" ? (
                <Sun className="h-4 w-4" />
              ) : (
                <Moon className="h-4 w-4" />
              )}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => {
                logout();
                navigate("/login");
              }}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </header>
        <main className="flex-1 overflow-auto p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
};
