@echo off
cd /d "%~dp0"

echo [1/2] Starting backend http://localhost:8080 ...
start "OKX-backend" /D "%~dp0backend" cmd /k "go run ./cmd/server"

echo [2/2] Starting frontend http://localhost:5173 ...
cd /d "%~dp0frontend"
call npm run dev
