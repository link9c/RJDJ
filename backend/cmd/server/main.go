package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"RJDJ/backend/internal/config"
	"RJDJ/backend/internal/db"
	"RJDJ/backend/internal/handler"
	"RJDJ/backend/internal/middleware"
)

func main() {
	cfg := config.Load()

	// 初始化数据库
	gdb, err := db.Init(cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	middleware.InitJWT(cfg.JWTSecret)

	// 构建 handler
	authH := handler.NewAuthHandler(gdb)
	cfgH := handler.NewOkxConfigHandler(gdb, cfg)
	dataH := handler.NewOkxDataHandler(cfgH)
	settingsH := handler.NewSettingsHandler(gdb)
	aiH := handler.NewAIHandler(gdb, cfg, settingsH, dataH)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	// CORS：CORS_ORIGINS 为 "*" 时放行所有来源（回显 Origin，兼容局域网 IP/主机名访问），
	// 否则按逗号分隔的白名单精确匹配
	corsCfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}
	if len(cfg.CORSOrigins) == 1 && cfg.CORSOrigins[0] == "*" {
		corsCfg.AllowOriginFunc = func(origin string) bool { return true }
	} else {
		corsCfg.AllowOrigins = cfg.CORSOrigins
	}
	r.Use(cors.New(corsCfg))

	// ========== API 路由 ==========
	api := r.Group("/api")

	// 公开
	api.POST("/auth/login", authH.Login)
	api.POST("/auth/register", authH.Register)

	// 需鉴权
	auth := api.Group("")
	auth.Use(middleware.AuthRequired())
	{
		auth.GET("/auth/me", authH.Me)

		// OKX 配置
		auth.GET("/okx/configs", cfgH.List)
		auth.POST("/okx/configs", cfgH.Create)
		auth.PUT("/okx/configs/:id", cfgH.Update)
		auth.DELETE("/okx/configs/:id", cfgH.Delete)
		auth.POST("/okx/configs/:id/test", cfgH.TestConnection)

		// OKX 数据
		auth.GET("/okx/overview", dataH.Overview)
		auth.GET("/okx/positions", dataH.Positions)
		auth.GET("/okx/ticker", dataH.Ticker)
		auth.GET("/okx/tickers", dataH.Tickers)
		auth.GET("/okx/candles", dataH.Candles)
		auth.GET("/okx/pnl/daily", dataH.PnlDaily)
		auth.POST("/okx/pnl/daily/start", dataH.StartPnlFetch)
		auth.GET("/okx/pnl/daily/status", dataH.PnlFetchStatus)
		auth.GET("/okx/strategies", dataH.Strategies)
		auth.GET("/okx/strategies/grid-positions", dataH.GridPositions)
		auth.GET("/okx/strategies/detail", dataH.StrategyDetail)

		// AI 对话
		auth.POST("/ai/chat", aiH.Chat)
		auth.GET("/ai/history", aiH.History)

		// 系统设置（AI 大模型配置入库）
		auth.GET("/settings/ai", settingsH.GetAI)
		auth.PUT("/settings/ai", settingsH.UpdateAI)
	}

	// 健康检查
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// ========== 静态资源（可选：服务前端打包产物） ==========
	if cfg.StaticDir != "" {
		if _, err := os.Stat(cfg.StaticDir); err == nil {
			r.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				full := filepath.Join(cfg.StaticDir, path)
				if _, err := os.Stat(full); err == nil {
					c.File(full)
					return
				}
				// SPA fallback
				c.File(filepath.Join(cfg.StaticDir, "index.html"))
			})
			log.Printf("[static] serving %s", cfg.StaticDir)
		}
	}

	handler.StartPnlTaskJanitor()
	log.Printf("OKX Analytics backend listening on %s", cfg.ServerAddr())
	if err := r.Run(cfg.ServerAddr()); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
