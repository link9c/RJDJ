# RJDJ 项目长期记忆

## 是什么
OKX 交易数据看板 + AI 分析平台（自托管）
- 位置 F:/projects/RJDJ
- 前端 React18 + Vite + Tailwind + ECharts，端口 5173
- 后端 Go 1.27 + Gin + GORM + glebarez/sqlite（纯 Go 免 CGO），端口 8080
- AI 对话：OpenAI 兼容 + Function Calling 调用 OKX 实时数据

## 关键决策
- goex 库调研结论：高层封装字段精简、URI 注入耦合、持仓仅合约；
  本项目自研 OKX v5 REST 客户端（HMAC-SHA256 签名对齐 goex 规范），
  支持完整 Endpoint(域名) 可配 + 原生字段返回，便于 AI 灵活调用。
- sqlite 选 glebarez/sqlite 而非 mattn/go-sqlite3，原因：本机无 gcc。
- OKX 密钥 AES-256-GCM 加密存库；接口只回掩码。

## 启动
- 后端：cd backend && go run ./cmd/server  （或 ./bin/server.exe）
- 前端：cd frontend && npm run dev
- 默认管理员 admin / admin123
- AI 需在 backend/.env 配置 LLM_API_KEY / LLM_BASE_URL / LLM_MODEL

## 端口与代理
- 前端 vite.config.ts 已配 proxy /api → :8080
- 生产部署可在 backend/.env 设 STATIC_DIR=../frontend/dist 单端口服务

## 重要配置入口
- AI 大模型配置：在网页「AI 分析」页 → 右上「AI 模型配置」按钮（GET/PUT /api/settings/ai）。
  入库 AppSetting 表（key=ai.provider/base_url/model/api_key），保存时自动 ping 测试连通性。
  仍可通过 backend/.env 提供默认值（DB 优先）。
- OKX 配置：网页「OKX 配置」页 + 侧边栏下拉切换账户。

## 我的策略/Trading Bot
OKX v5 REST 把 bot 接口放在 `/api/v5/tradingBot/` 下：
- grid（现货网格 / 合约网格）
- recurring（定投 / 与 martingale 同类的 DCA 机器人）
- signal（信号机器人）
- 策略委托/条件单（止盈止损/oco/trailing）在 `/api/v5/trade/orders-algo-pending|history`
- 注意：马丁格尔(Martingale)在官方文档被描述为 DCA bot，但其底层 REST 是否单独开放了
  `/api/v5/tradingBot/martingale/*` 未明确。前端展示网格+定投+信号+条件单四类，若马丁格尔存在，
  通常在 recurring 或 algo 类别下。
goex 未封装 bot 接口，本项目自研 strategy.go 补齐。

## 可复用经验
- Windows 装 Go 用 winget：winget install -e --id GoLang.Go
- GOPATH 必须绝对路径：C:/Users/link.chen/go
- Go 字符串内含 ASCII " 容易截断，工具描述文案避免
- LLM tool-call 消息 Content 用 *string 发 null 更兼容（DeepSeek/OpenAI）
- agent-browser 在本沙箱环境 daemon 跨命令不稳定，用 playwright-core + 已下 chrome 直接跑一次性脚本更可靠