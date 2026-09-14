import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  LayoutDashboard,
  Settings2,
  Bot,
  LogOut,
  TrendingUp,
  RefreshCw,
  Layers,
  Menu,
  X,
} from "lucide-react";
import { useAuthStore } from "@/store/auth";
import { useConfigStore } from "@/store/config";

const navItems = [
  { to: "/", label: "资产总览", icon: LayoutDashboard, end: true },
  { to: "/strategies", label: "我的策略", icon: Layers },
  { to: "/analyze", label: "AI 分析", icon: Bot },
  { to: "/config", label: "OKX 配置", icon: Settings2 },
];

export default function Layout() {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const { list, activeId, setActive, fetch } = useConfigStore();
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    fetch();
  }, [fetch]);

  // 路由切换后自动收起抽屉
  useEffect(() => {
    setMenuOpen(false);
  }, [location.pathname]);

  // 抽屉打开时锁定背景滚动
  useEffect(() => {
    document.body.style.overflow = menuOpen ? "hidden" : "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [menuOpen]);

  // 已登录但无 user（如 localStorage 损坏）重新拉取
  const handleLogout = () => {
    setMenuOpen(false);
    logout();
    navigate("/login");
  };

  return (
    <div className="flex h-[100dvh] overflow-hidden bg-slate-100">
      {/* ===== 手机端遮罩 ===== */}
      {menuOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/50 lg:hidden"
          onClick={() => setMenuOpen(false)}
        />
      )}

      {/* ===== 侧边栏：lg 以上常驻，以下抽屉 ===== */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 max-w-[82vw] bg-slate-900 text-slate-200 flex flex-col transform transition-transform duration-200 lg:static lg:z-auto lg:w-60 lg:max-w-none lg:translate-x-0 lg:transition-none ${
          menuOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <div className="flex items-center gap-2.5 px-5 h-16 border-b border-white/10 pt-safe">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-brand-600">
            <TrendingUp size={20} className="text-white" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-[15px] font-semibold text-white leading-tight">
              OKX 分析台
            </div>
            <div className="text-[11px] text-slate-400">资产 · 行情 · AI</div>
          </div>
          <button
            className="lg:hidden p-2 -mr-2 rounded-lg text-slate-400 hover:text-white hover:bg-white/10"
            onClick={() => setMenuOpen(false)}
            aria-label="关闭菜单"
          >
            <X size={18} />
          </button>
        </div>

        <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors ${
                  isActive
                    ? "bg-brand-600 text-white"
                    : "text-slate-300 hover:bg-white/10 hover:text-white"
                }`
              }
            >
              <item.icon size={18} />
              {item.label}
            </NavLink>
          ))}
        </nav>

        {/* 当前账户选择器 */}
        <div className="px-3 pb-2 border-t border-white/10 pt-3">
          {list.length > 0 ? (
            <select
              value={activeId ?? ""}
              onChange={(e) => setActive(Number(e.target.value) || null)}
              className="w-full rounded-lg bg-white/5 border border-white/10 text-sm text-slate-200 px-2.5 py-2 outline-none focus:border-brand-500"
            >
              {list.map((c) => (
                <option key={c.id} value={c.id} className="text-slate-800">
                  {c.is_default ? "⭐ " : ""}
                  {c.name || `配置${c.id}`} ({c.base_url?.replace("https://", "")})
                </option>
              ))}
            </select>
          ) : (
            <button
              onClick={() => navigate("/config")}
              className="w-full rounded-lg border border-dashed border-white/20 text-slate-400 text-xs px-2 py-2 hover:text-white hover:border-white/40 transition-colors"
            >
              尚未配置 OKX 密钥 → 去添加
            </button>
          )}
        </div>

        <div className="px-3 py-3 border-t border-white/10 flex items-center gap-2.5 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-700 text-sm font-medium text-white">
            {(user?.nickname || user?.username || "U").charAt(0).toUpperCase()}
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-sm text-white truncate">
              {user?.nickname || user?.username}
            </div>
            <div className="text-[11px] text-slate-400 truncate">
              {user?.username}
            </div>
          </div>
          <button
            onClick={handleLogout}
            title="退出登录"
            className="p-2 rounded-lg text-slate-400 hover:text-red-400 hover:bg-white/10"
          >
            <LogOut size={17} />
          </button>
        </div>
      </aside>

      {/* ===== 主区域 ===== */}
      <div className="flex-1 flex flex-col overflow-hidden min-w-0">
        <header className="h-16 shrink-0 bg-white border-b border-slate-200 flex items-center gap-3 px-4 sm:px-6">
          <button
            className="lg:hidden -ml-1 p-2 rounded-lg text-slate-600 hover:bg-slate-100"
            onClick={() => setMenuOpen(true)}
            aria-label="打开菜单"
          >
            <Menu size={20} />
          </button>
          <div className="min-w-0">
            <h1 className="text-base font-semibold text-slate-800 truncate">
              {pageTitle(location.pathname)}
            </h1>
            <p className="text-xs text-slate-400 mt-0.5 hidden sm:block">
              OKX API 数据 · 由 AI 驱动分析
            </p>
          </div>
          <div className="flex-1" />
          <button
            onClick={() => fetch()}
            title="刷新配置"
            className="btn-ghost !px-2.5"
          >
            <RefreshCw size={16} className="text-slate-500" />
            <span className="text-xs">刷新</span>
          </button>
        </header>
        <main className="flex-1 overflow-y-auto p-3 sm:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function pageTitle(path: string): string {
  if (path === "/" || path === "") return "资产总览";
  if (path.startsWith("/strategies")) return "我的策略";
  if (path.startsWith("/analyze")) return "AI 对话分析";
  if (path.startsWith("/config")) return "OKX 接口配置";
  return "OKX 分析台";
}
