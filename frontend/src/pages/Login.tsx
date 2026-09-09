import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { TrendingUp, AlertCircle } from "lucide-react";
import { authApi } from "@/api";
import { useAuthStore } from "@/store/auth";

export default function LoginPage() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);
  const [mode, setMode] = useState<"login" | "register">("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [nickname, setNickname] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      if (mode === "login") {
        const resp = await authApi.login(username, password);
        setAuth(resp.data.token, resp.data.user);
        navigate("/");
      } else {
        await authApi.register({ username, password, nickname });
        setMode("login");
        setError("注册成功，请登录");
      }
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex">
      {/* 左侧品牌区 */}
      <div className="hidden lg:flex w-[45%] bg-slate-900 flex-col justify-between p-12 text-white relative overflow-hidden">
        <div className="absolute -right-20 -top-20 h-80 w-80 rounded-full bg-brand-600/20 blur-3xl" />
        <div className="absolute -left-16 bottom-0 h-72 w-72 rounded-full bg-indigo-500/20 blur-3xl" />
        <div className="relative flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand-600">
            <TrendingUp size={22} />
          </div>
          <span className="text-lg font-semibold">OKX 资产分析台</span>
        </div>
        <div className="relative">
          <h1 className="text-3xl font-bold leading-snug">
            你的 OKX 交易数据，
            <br />
            交给 AI 分析。
          </h1>
          <p className="mt-4 text-slate-400 leading-relaxed max-w-md">
            安全接入你的 OKX API 密钥，实时查看资产与持仓，
            并通过自然语言对话让 AI 帮你分析行情、解读盈亏。
          </p>
          <div className="mt-8 flex flex-wrap gap-2">
            {["资产实时总览", "多账户密钥管理", "AI 智能分析", "AES 加密存储"].map(
              (t) => (
                <span
                  key={t}
                  className="px-3 py-1.5 text-xs rounded-full bg-white/5 border border-white/10 text-slate-300"
                >
                  {t}
                </span>
              )
            )}
          </div>
        </div>
        <p className="relative text-xs text-slate-500">
          OKX API 参考 goex 生态对接规范 · 数据仅存于本机 SQLite
        </p>
      </div>

      {/* 右侧表单区 */}
      <div className="flex-1 flex items-center justify-center p-8 bg-slate-50">
        <div className="w-full max-w-md">
          <div className="mb-8 text-center lg:text-left">
            <h2 className="text-2xl font-bold text-slate-800">
              {mode === "login" ? "欢迎回来" : "创建账号"}
            </h2>
            <p className="text-sm text-slate-500 mt-1">
              {mode === "login"
                ? "登录以管理你的 OKX 账户"
                : "注册一个本地账号（数据存于本机）"}
            </p>
          </div>

          <div className="bg-white rounded-2xl shadow-card border border-slate-200 p-7">
            <div className="flex bg-slate-100 rounded-lg p-1 mb-6">
              <button
                className={`flex-1 py-2 text-sm font-medium rounded-md transition ${
                  mode === "login"
                    ? "bg-white shadow-sm text-slate-800"
                    : "text-slate-500"
                }`}
                onClick={() => {
                  setMode("login");
                  setError("");
                }}
              >
                登录
              </button>
              <button
                className={`flex-1 py-2 text-sm font-medium rounded-md transition ${
                  mode === "register"
                    ? "bg-white shadow-sm text-slate-800"
                    : "text-slate-500"
                }`}
                onClick={() => {
                  setMode("register");
                  setError("");
                }}
              >
                注册
              </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              {mode === "register" && (
                <div>
                  <label className="label">昵称（可选）</label>
                  <input
                    className="input"
                    value={nickname}
                    onChange={(e) => setNickname(e.target.value)}
                    placeholder="你的昵称"
                  />
                </div>
              )}
              <div>
                <label className="label">用户名</label>
                <input
                  className="input"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="请输入用户名"
                  required
                />
              </div>
              <div>
                <label className="label">密码</label>
                <input
                  type="password"
                  className="input"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={mode === "register" ? "至少 6 位" : "请输入密码"}
                  required
                  minLength={6}
                />
              </div>

              {error && (
                <div
                  className={`flex items-center gap-2 text-sm px-3 py-2.5 rounded-lg ${
                    error.includes("成功")
                      ? "bg-green-50 text-green-700"
                      : "bg-red-50 text-red-600"
                  }`}
                >
                  <AlertCircle size={15} />
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="btn-primary w-full !py-2.5"
              >
                {loading
                  ? "处理中..."
                  : mode === "login"
                  ? "登 录"
                  : "注 册"}
              </button>
            </form>

            {mode === "login" && (
              <p className="text-xs text-slate-400 text-center mt-4">
                默认账号 <code className="text-brand-600">admin</code> /{" "}
                <code className="text-brand-600">admin123</code>
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
