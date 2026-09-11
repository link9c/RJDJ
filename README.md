# OKX 资产分析台 (RJDJ)

一个自托管的 **OKX 交易数据看板 + AI 分析** 平台。

> 后端 Go + MySQL 存储与对接 OKX API（签名鉴权参考 [goex](https://github.com/nntaoli-project/goex) 的 OKX 实现规范）；
> 前端 React (Vite + Tailwind)，中后台界面风格参考工时填报系统。

## ✨ 功能

- 🔐 **账号密码登录**：本地注册/登录，JWT 鉴权（默认管理员 `admin / admin123`）
- 🔑 **可配置的 OKX API Key**：api_key / secret / passphrase / **域名(BaseURL)** 均可配置，
  支持多账户、默认账户、一键测试连接。密钥经 **AES-256-GCM** 加密后存入 MySQL，接口永不回传明文。
- 📊 **资产看板**：账户总权益、各币种余额、资产占比饼图、持仓明细（方向/开仓价/浮动盈亏/杠杆）
- 🤖 **AI 对话分析**：接入 OpenAI 兼容大模型，AI 通过 **Function Calling** 自动调用 OKX 接口
  拉取真实数据（账户/持仓/行情/K线），再给出行情解读、盈亏分析、持仓建议。

## 🏗️ 技术栈

| 层 | 技术 |
|---|---|
| 前端 | React 18 · Vite · TypeScript · Tailwind · ECharts · zustand · react-router |
| 后端 | Go 1.27 · Gin · GORM · MySQL 8 · golang-jwt |
| OKX 对接 | 自研轻量 OKX v5 REST 客户端（HMAC-SHA256 签名，域名可配），规范对齐 goex |
| AI | OpenAI 兼容 Chat Completions + Function Calling（OpenAI / DeepSeek / 其他） |
| 部署 | Docker Compose（MySQL + 后端 + Nginx 前端） |

## 📁 目录结构

```
RJDJ/
├── docker-compose.yml       # 一键部署
├── .env.example             # Compose 环境变量模板
├── backend/                 # Go 后端
│   ├── Dockerfile
│   ├── cmd/server/main.go   # 入口
│   ├── internal/
│   │   ├── config/          # 配置加载 (.env)
│   │   ├── db/              # MySQL 初始化 + 种子
│   │   ├── model/           # 数据模型
│   │   ├── util/            # 密码哈希 / AES 加解密
│   │   ├── middleware/      # JWT
│   │   ├── okx/             # OKX v5 REST 客户端（参考 goex 规范）
│   │   ├── ai/              # LLM 客户端 + 工具定义
│   │   └── handler/         # HTTP 处理器
│   └── .env.example
└── frontend/                # React 前端
    ├── Dockerfile
    ├── nginx.conf           # /api 反代到后端
    └── src/
        ├── api/             # HTTP 封装 + 类型
        ├── store/           # zustand
        ├── components/      # 布局
        └── pages/           # 登录 / 看板 / AI / 配置
```

## 🐳 Docker Compose 部署（推荐）

需要已安装 Docker 与 Docker Compose。

```bash
cp .env.example .env
# 编辑 .env：修改 MYSQL_PASSWORD / JWT_SECRET / ENCRYPT_KEY（生产务必改）
# 可选填入 LLM_API_KEY / LLM_BASE_URL / LLM_MODEL

docker compose up -d --build
```

浏览器打开 http://localhost（默认 `APP_PORT=80`），用 `admin / admin123` 登录。

常用命令：

```bash
docker compose ps
docker compose logs -f backend
docker compose down          # 停服务，保留数据卷
docker compose down -v       # 停服务并删除 MySQL 数据
```

服务组成：

| 服务 | 说明 |
|---|---|
| `mysql` | MySQL 8，数据卷 `mysql_data`（默认不映射到宿主机，避免端口冲突） |
| `backend` | Go API，内部端口 8080，经前端 Nginx 反代 |
| `frontend` | Nginx 托管静态资源，并把 `/api` 转到后端 |

## 🚀 本地开发

先有一台可达的 MySQL（可用 Compose 只起数据库；默认不映射 3306 到宿主机）。
本地 `go run` 需要直连时，在 `docker-compose.yml` 的 `mysql` 服务下增加：

```yaml
ports:
  - "127.0.0.1:3306:3306"
```

然后：

```bash
docker compose up -d mysql
```

### 1. 启动后端（端口 8080）

```bash
cd backend

cp .env.example .env
# 编辑 .env，确认 DB_* 与上面 MySQL 账号一致
# AI 对话需填 LLM_API_KEY / LLM_BASE_URL / LLM_MODEL（也可登录后在网页配置）

go mod tidy
go run ./cmd/server
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

`backend/.env` 或项目根目录 `.env` 使用 **OpenAI 兼容** 协议：

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
- 生产环境请修改 `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD` / `JWT_SECRET` / `ENCRYPT_KEY`

## ❗ 免责声明

本项目仅供学习与技术交流，不构成任何投资建议。加密货币交易有高风险，
请妥善保管你的 API Key（建议仅开通「读取」权限，不要开启提现权限）。
