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

**实测踩坑（2026-09-15 实盘验证）**——各列表端点必传的「子类型参数」并不统一，
漏传/传错会直接报错，且**容错为零**：
- `tradingBot/grid/orders-algo-pending` → `algoOrdType` 必填，取值 `grid` / `contract_grid`
- `tradingBot/dca/ongoing-list` → `algoOrdType`，`spot_dca` / `contract_dca`
- `tradingBot/recurring/orders-algo-pending` → `algoOrdType=recurring`
- `tradingBot/signal/orders-algo-pending` → `algoOrdType=`**`contract`**（合约信号）。
  传 `signal` 会报 `51000 Parameter algoOrdType error`，空着报 `50014`
- `trade/orders-algo-pending`（条件单）→ 参数名是 **`ordType`**（不是 algoOrdType），
  取值 `conditional` / `oco` / `trigger` / `move_order_stop`；传错名字报 `51000`
- 信号策略的 margin/upl 字段名与网格不同：`frozenBal`（占用保证金）、`floatPnl`（浮动盈亏）、
  `pnlRatio`、`totalPnl`，取值链要兼容

## sysmon（子工具：持仓伪装成任务管理器）
目录 `sysmon/`，独立 go module，零第三方依赖，`sysmon.exe`。
界面仿 Windows 任务管理器（进程/性能/详细信息三页签），把持仓与策略伪装成系统进程。
**五个监控指标全部 1:1，不做任何倍率换算**（2026-09-15 用户要求，系数仅为调节手段保留，默认全 1）：
CPU%←|收益率%|、内存←保证金(U)→MB、磁盘←|浮盈(U)|→MB/s、网络←|24h涨跌%|→Mbps、GPU%←杠杆倍。
界面上那一列的数字就是真实数值本身，不用心算。
- **`mem_total_gb` 默认 32**（不是 16）：内存列 1:1 等于保证金，要装得下所有持仓保证金之和
  （总保证金 ÷ 1024 = 最小 GB），否则性能页内存占比会贴 100%
- 1:1 的副作用：磁盘列会显得很忙（几百 U 浮盈 = 几百 MB/s），介意就让该条加 `disk_scale: 0.1`
- 配置 `config.json`（支持 // 注释）；`processes[]` 定义 name/inst_id/source 映射
- **盈亏方向由独立的「趋势」列承载，红 `+` 盈 / 绿 `-` 亏**。指标本身都取了绝对值
  （负 CPU 会穿帮），不能从数值正负判断方向。符号用 ASCII `+ -` 不用 ▲▼——后者是
  Unicode Ambiguous 宽度，中文终端会当两列渲染导致列错位。
- **进程页最后两列是行情，可以直接读实时价格**（2026-09-15 加）：
  - **提交大小 ← 最新价**、**页面错误 ← 持仓均价**，都是任务管理器「详细信息」页的真实列名，
    取值区间本来就很宽，价格放进去不用额外伪装。两列物理量不同，数字接近也不会被交叉验算。
  - 默认系数都是 **0.1**，读法统一为「该列数字 × 10 = 价格」。
    **不要改成 1**：BTC 现价会原样显示成 61.9 GB 的提交大小，而 `mem_total_gb` 写的是 16，一眼假。
  - `fmtCommit` **固定以 MB 显示、不自动升 GB**（升 GB 就得再乘 1024，破坏读法）
  - 这两列**不加抖动**（其余指标照旧正弦漂移），抖了就读不出准确价格
  - 策略类记录多无开仓均价 → 页面错误显示 `-`；价格极小的标的（0.05）要按条目把系数调大
  - 终端宽度 < 112 列自动隐藏；`ui.show_price` 只控制开关
- 进程页签列序：名称 趋势 PID 状态 CPU 内存 磁盘 网络 GPU 提交大小 页面错误（索引 0-10），
  改列必须同步 `sortCol` 默认值、`toggleSort` 按键（C/M/N/D/P/S/E）、`sortProcs` 三处，
  以及 `drawProcesses` 里 `fixed`/`ncol` 的列宽累加
- 行情拉取：`GetTickers` 要 **SWAP + SPOT 两份合并**，只拉 SWAP 时现货网格/现货定投标的没价格
- 参数：`-check` 配置自检并打印映射对照表（含现价/均价/伪装后取值，便于核对换算）、
  `-dump 118x30` 纯文本预览、`-c` 指定配置
- 未配 key 时自动用内置演示数据（feed/mock.go）
- 复用 backend 的 OKX 签名实现（okx/client.go），补了模拟盘 x-simulated-trading header
- 终端控制全走 kernel32（raw mode/尺寸/WriteConsoleW），不依赖 x/term

## 可复用经验
- Windows 装 Go 用 winget：winget install -e --id GoLang.Go
- GOPATH 必须绝对路径：C:/Users/link.chen/go
- Go 字符串内含 ASCII " 容易截断，工具描述文案避免
- LLM tool-call 消息 Content 用 *string 发 null 更兼容（DeepSeek/OpenAI）
- agent-browser 在本沙箱环境 daemon 跨命令不稳定，用 playwright-core + 已下 chrome 直接跑一次性脚本更可靠
- **Edit 工具：同一文件的多处修改绝不能放在同一条消息的多个 Edit 里**——后一次写入会基于
  陈旧内容，把前一次静默覆盖掉（报 success 但没落盘），只在下次编译时才暴露。
  一条一发，改完立刻 grep 校验。
- 本沙箱 bash 缺 `sed`/`head`/`dirname` 等基础命令，`find`/`grep` 也别用；
  改用 Read/Grep/Glob 工具，或 PowerShell 处理文本。