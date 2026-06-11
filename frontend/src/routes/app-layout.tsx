import * as DialogPrimitive from "@radix-ui/react-dialog";
import {
  Activity,
  BarChart3,
  BookOpen,
  Database,
  Key,
  LayoutDashboard,
  LogOut,
  Menu,
  Moon,
  Sun,
} from "lucide-react";
import { useState } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
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

const appTitle = import.meta.env.VITE_APP_TITLE ?? "守望者";

/** 全局应用图标（与浏览器 favicon 同源 /favicon.png），统一以图片展示 logo。 */
const AppLogo = ({ className }: { className?: string }) => (
  <img
    src="/favicon.png"
    alt="logo"
    className={cn("h-6 w-6 shrink-0", className)}
  />
);

/** 桌面侧边栏与移动端浮层共用同一份导航；onNavigate 用于移动端点击后关闭浮层。 */
const NavItems = ({ onNavigate }: { onNavigate?: () => void }) => (
  <nav className="space-y-1 p-3">
    {navItems.map(({ to, icon: Icon, label, end }) => (
      <NavLink
        key={to}
        to={to}
        end={end}
        onClick={onNavigate}
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
);

export const AppLayout = () => {
  const navigate = useNavigate();
  const admin = useAuthStore((s) => s.admin);
  const logout = useAuthStore((s) => s.logout);
  const { theme, toggleTheme } = useThemeStore();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-56 shrink-0 border-r bg-card md:block">
        <div className="flex h-14 items-center gap-2 border-b px-4 font-semibold">
          <AppLogo />
          {appTitle}
        </div>
        <NavItems />
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="flex h-14 items-center justify-between border-b px-4 md:px-6">
          <div className="flex items-center gap-2 md:hidden">
            <DialogPrimitive.Root
              open={mobileNavOpen}
              onOpenChange={setMobileNavOpen}
            >
              <DialogPrimitive.Trigger asChild>
                <Button variant="ghost" size="icon" aria-label="打开导航菜单">
                  <Menu className="h-5 w-5" />
                </Button>
              </DialogPrimitive.Trigger>
              <DialogPrimitive.Portal>
                {/* 点击浮层外部（遮罩）由 Radix 触发 onOpenChange(false) 自动关闭 */}
                <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/60 duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=open]:fade-in data-[state=closed]:fade-out md:hidden" />
                <DialogPrimitive.Content
                  aria-describedby={undefined}
                  className="fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r bg-card shadow-lg duration-300 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=open]:slide-in-from-left data-[state=closed]:slide-out-to-left md:hidden"
                >
                  <DialogPrimitive.Title className="flex h-14 items-center gap-2 border-b px-4 font-semibold">
                    <AppLogo />
                    导航菜单
                  </DialogPrimitive.Title>
                  {/* 点击任一导航项跳转后关闭浮层 */}
                  <NavItems onNavigate={() => setMobileNavOpen(false)} />
                </DialogPrimitive.Content>
              </DialogPrimitive.Portal>
            </DialogPrimitive.Root>
            <span className="flex items-center gap-1.5 font-semibold text-sm">
              <AppLogo className="h-5 w-5" />
              {appTitle}
            </span>
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <Button asChild variant="outline" size="sm" className="gap-1.5">
              <Link to="/api-docs">
                <BookOpen className="h-4 w-4" />
                <span className="hidden sm:inline">API 文档</span>
              </Link>
            </Button>
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
