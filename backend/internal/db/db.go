package db

import (
	"fmt"
	"log"
	"net"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"RJDJ/backend/internal/config"
	"RJDJ/backend/internal/model"
	"RJDJ/backend/internal/util"
)

// Init 初始化 MySQL 连接并自动建表
func Init(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DBDSN
	if dsn == "" {
		mc := mysqldriver.NewConfig()
		mc.User = cfg.DBUser
		mc.Passwd = cfg.DBPassword
		mc.Net = "tcp"
		mc.Addr = net.JoinHostPort(cfg.DBHost, cfg.DBPort)
		mc.DBName = cfg.DBName
		mc.Params = map[string]string{"charset": "utf8mb4"}
		mc.ParseTime = true
		mc.Loc = time.Local
		mc.AllowNativePasswords = true
		dsn = mc.FormatDSN()
	}

	var gdb *gorm.DB
	var err error
	for i := 1; i <= 30; i++ {
		gdb, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err == nil {
			if sqlDB, pingErr := gdb.DB(); pingErr == nil {
				if pingErr = sqlDB.Ping(); pingErr == nil {
					sqlDB.SetMaxIdleConns(10)
					sqlDB.SetMaxOpenConns(50)
					sqlDB.SetConnMaxLifetime(time.Hour)
					break
				}
				err = pingErr
			} else {
				err = pingErr
			}
		}
		log.Printf("[db] 等待 MySQL (%s:%s) 第 %d/30 次: %v", cfg.DBHost, cfg.DBPort, i, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
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
		&model.OkxBill{},
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
