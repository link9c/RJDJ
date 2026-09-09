package model

import "time"

// User 用户账号，用于登录鉴权
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Email        string    `gorm:"size:128" json:"email"`
	Role         string    `gorm:"size:32;default:user" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OkxConfig OKX API 配置（密钥加密存储，默认不返回明文）
type OkxConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	Name        string    `gorm:"size:64" json:"name"`         // 配置备注名，如"我的主账户"
	ApiKey      string    `gorm:"size:255;not null" json:"-"`  // 加密存储
	ApiSecret   string    `gorm:"size:512;not null" json:"-"`  // 加密存储
	Passphrase  string    `gorm:"size:255;not null" json:"-"`  // 加密存储
	BaseURL     string    `gorm:"size:255" json:"base_url"`    // 域名，默认 https://www.okx.com
	IsDefault   bool      `gorm:"default:false" json:"is_default"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SafeView 返回给前端的安全视图（不含密钥）
func (c *OkxConfig) SafeView() map[string]any {
	return map[string]any{
		"id":          c.ID,
		"user_id":     c.UserID,
		"name":        c.Name,
		"base_url":    c.BaseURL,
		"is_default":  c.IsDefault,
		"description": c.Description,
		"api_key":     maskSecret(c.ApiKey),
		"has_secret":  c.ApiSecret != "",
		"created_at":  c.CreatedAt,
		"updated_at":  c.UpdatedAt,
	}
}

func maskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

// ChatMessage AI 对话消息记录
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ConfigID  uint      `gorm:"index" json:"config_id"` // 对话关联的 OKX 配置
	Role      string    `gorm:"size:16;not null" json:"role"` // user / assistant
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// AppSetting 全局键值设置（AI 大模型配置等，可网页编辑入库）
type AppSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OkxBill 已拉取入缓存的历史账单流水（按 OKX billId 去重）。
// 用于「有历史数据时直接读库，不再重复拉 OKX 接口」。
type OkxBill struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ConfigID  uint      `gorm:"uniqueIndex:uk_cfg_bill;not null" json:"config_id"`
	BillID    string    `gorm:"size:64;uniqueIndex:uk_cfg_bill" json:"billId"` // OKX billId，同 config 内唯一去重
	InstType  string    `gorm:"size:16" json:"instType"`                        // SWAP / FUTURES
	InstID    string    `gorm:"size:64" json:"instId"`
	Ccy       string    `gorm:"size:16;index" json:"ccy"`
	Pnl       float64   `json:"pnl"`     // 已实现盈亏
	Fee       float64   `json:"fee"`     // 手续费(负)
	BalChg    float64   `json:"balChg"`  // 余额变动
	Type      string    `gorm:"size:8" json:"type"`
	SubType   string    `gorm:"size:16" json:"subType"`
	Ts        int64     `gorm:"index" json:"ts"` // 事件时间(毫秒)
	Raw       string    `gorm:"type:text" json:"-"` // 原始 JSON，便于日后扩展字段
	CreatedAt time.Time `json:"created_at"`
}
