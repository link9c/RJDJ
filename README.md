# OKX 资产分析台 (RJDJ)

一个自托管的 **OKX 交易数据看板 + AI 分析** 平台。

> 后端 Go + SQLite 存储与对接 OKX API（签名鉴权参考 [goex](https://github.com/nntaoli-project/goex) 的 OKX 实现规范）；
> 前端 React (Vite + Tailwind)，中后台界面风格参考工时填报系统。

## ✨ 功能

- 🔐 **账号密码登录**：本地注册/登录，JWT 鉴权（默认管理员 `admin / admin123`）
- 🔑 **可配置的 OKX API Key**：api_key / secret / passphrase / **域名(BaseURL)** 均可配置，
  支持多账户、默认账户、一键测试连接。密钥经 **AES-256-GCM** 加密后存入 SQLite，接口永不回传明文。
- 📊 **资产看板**：账户总权益、各币种余额、资产占比饼图、持仓明细（方向/开仓价/浮动盈亏/杠杆）
- 🤖 **AI 对话分析**：接入 OpenAI 兼容大模型，AI 通过 **Function Calling** 自动调用 OKX 接口
  拉取真实数据（账户/持仓/行情/K线），再给出行情解读、盈亏分析、持仓建议。

## 🏗️ 技术栈

| 层 | 技术 |
|---|---|
| 前端 | React 18 · Vite · TypeScript · Tailwind · ECharts · zustand · react-router |
| 后端 | Go 1.27 · Gin · GORM · SQLite(纯 Go, 免 CGO) · golang-jwt |
| OKX 对接 | 自研轻量 OKX v5 REST 客户端（HMAC-SHA256 签名，域名可配），规范对齐 goex |
| AI | OpenAI 兼容 Chat Completions + Function Calling（OpenAI / DeepSeek / 其他） |

## 📁 目录结构

```
RJDJ/
├── backend/                 # Go 后端
│   ├── cmd/server/main.go   # 入口
│   ├── internal/
│   │   ├── config/          # 配置加载 (.env)
│   │   ├── db/              # SQLite 初始化 + 种子
│   │   ├── model/           # 数据模型
│   │   ├── util/            # 密码哈希 / AES 加解密
│   │   ├── middleware/      # JWT
│   │   ├── okx/             # OKX v5 REST 客户端（参考 goex 规范）
│   │   ├── ai/              # LLM 客户端 + 工具定义
│   │   └── handler/         # HTTP 处理器
│   ├── .env.example
│   └── data/app.db          # SQLite 数据库（自动生成）
└── frontend/                # React 前端
    └── src/
        ├── api/             # HTTP 封装 + 类型
        ├── store/           # zustand
        ├── components/      # 布局
        └── pages/           # 登录 / 看板 / AI / 配置
```

## 🚀 本地运行

### 1. 启动后端（端口 8080）

```bash
cd backend

# 首次：安装 Go 依赖
go mod tidy

# 配置环境变量（AI 必须填 LLM_API_KEY 才能对话）
cp .env.example .env
# 编辑 .env，填入 LLM_API_KEY / LLM_BASE_URL / LLM_MODEL

# 运行
go run ./cmd/server
# 或编译后运行： go build -o bin/server.exe ./cmd/server && ./bin/server.exe
```

### 2. 启动前端（端口 5173，已代理 /api 到 8080）

```bash
cd frontend
npm install
npm run dev
```

打开 http://localhost:5173 用 `admin / admin123` 登录。

> 若只想用单个端口运行打包产物，可让后端直接托管前端：
> ```bash
> cd frontend && npm run build
> # backend/.env 增加 STATIC_DIR=../frontend/dist
> # 然后访问 http://localhost:8080
> ```

## ⚙️ AI 大模型配置（.env）

`backend/.env` 使用 **OpenAI 兼容** 协议：

```dotenv
LLM_PROVIDER=deepseek            # 备注用，仅作标识
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=sk-xxxx
LLM_MODEL=deepseek-chat
```

- **DeepSeek**：`https://api.deepseek.com` + `deepseek-chat`
- **OpenAI**：`https://api.openai.com/v1` + `gpt-4o-mini`（注意带 `/v1`）
- 其它 OpenAI 兼容网关（Moonshot、通义 dashscope、本地 vLLM/Ollama）同理。

AI 可调用的工具（自动）：`get_account_overview`（账户+持仓）、`get_ticker`（行情）、
`get_positions`（持仓）、`get_candles`（K线）。

## 🧩 OKX 域名（BaseURL）说明

- 默认 `https://www.okx.com`（官方现货+合约统一账户）
- 使用 **模拟盘** 时可填对应 demo 域名（需使用模拟盘创建的 API Key）
- 自建代理/网关同理，在此配置任意可达域名

## 🔒 安全说明

- 用户密码：bcrypt 哈希
- OKX 密钥：AES-256-GCM 加密后落库，密钥来自 `.env` 的 `ENCRYPT_KEY`（**务必修改默认值**）
- 对外接口只返回掩码后的 key，不返回 secret/passphrase
- 默认种子管理员 `admin/admin123`，首次部署后请立即修改或禁用

## ❗ 免责声明

本项目仅供学习与技术交流，不构成任何投资建议。加密货币交易有高风险，
请妥善保管你的 API Key（建议仅开通「读取」权限，不要开启提现权限）。
