@echo off
REM ===== 一键启动 OKX 分析台（开发模式） =====
REM 打开两个终端分别运行，或直接运行本脚本（会同时启动前后端）

echo [1/2] 启动后端 (http://localhost:8080) ...
start "OKX-backend" cmd /k "cd /d %~dp0backend && go run ./cmd/server"

echo [2/2] 启动前端 (http://localhost:5173) ...
cd /d %~dp0frontend
call npm run dev
