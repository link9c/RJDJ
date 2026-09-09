package db

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite" // 纯 Go 实现，免 CGO
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"RJDJ/backend/internal/model"
	"RJDJ/backend/internal/util"
)

// Init 初始化数据库连接并自动建表
func Init(dbPath string) (*gorm.DB, error) {
	// 确保目录存在
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	// 启用 WAL 提高并发
	if sqlDB, err := gdb.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	if err := autoMigrate(gdb); err != nil {
		return nil, err
	}

	return gdb, nil
}

func autoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&model.User{},
		&model.OkxConfig{},
		&model.ChatMessage{},
		&model.AppSetting{},
	)
	if err != nil {
		return err
	}
	// 初始化默认管理员账号 admin / admin123
	if err := seedAdmin(db); err != nil {
		log.Println("[db] seed admin warn:", err)
	}
	return nil
}

func seedAdmin(db *gorm.DB) error {
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count > 0 {
		return nil
	}
	hash, err := util.HashPassword("admin123")
	if err != nil {
		return err
	}
	return db.Create(&model.User{
		Username:     "admin",
		PasswordHash: hash,
		Nickname:     "管理员",
		Role:         "admin",
	}).Error
}
