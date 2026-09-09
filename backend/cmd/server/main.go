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
	gdb, err := db.Init(cfg.DBPath)
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
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

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
		auth.GET("/okx/strategies", dataH.Strategies)
		auth.GET("/okx/strategies/grid-positions", dataH.GridPositions)

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

	log.Printf("OKX Analytics backend listening on %s", cfg.ServerAddr())
	if err := r.Run(cfg.ServerAddr()); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
