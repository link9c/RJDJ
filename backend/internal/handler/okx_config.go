package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"RJDJ/backend/internal/config"
	"RJDJ/backend/internal/middleware"
	"RJDJ/backend/internal/model"
	"RJDJ/backend/internal/okx"
	"RJDJ/backend/internal/util"
)

// OkxConfigHandler OKX 配置管理
type OkxConfigHandler struct {
	db    *gorm.DB
	cfg   *config.Config
}

func NewOkxConfigHandler(db *gorm.DB, cfg *config.Config) *OkxConfigHandler {
	return &OkxConfigHandler{db: db, cfg: cfg}
}

type okxConfigReq struct {
	Name        string `json:"name"`
	ApiKey      string `json:"api_key"`
	ApiSecret   string `json:"api_secret"`
	Passphrase  string `json:"passphrase"`
	BaseURL     string `json:"base_url"`
	IsDefault   bool   `json:"is_default"`
	Description string `json:"description"`
}

func normalizeBaseURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "https://www.okx.com"
	}
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return strings.TrimRight(u, "/")
}

// List 列出当前用户的配置（安全视图）
func (h *OkxConfigHandler) List(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	var cfgs []model.OkxConfig
	if err := h.db.Where("user_id = ?", uid).Order("is_default desc, id asc").Find(&cfgs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	views := make([]gin.H, 0, len(cfgs))
	for _, cg := range cfgs {
		views = append(views, cg.SafeView())
	}
	c.JSON(http.StatusOK, gin.H{"list": views})
}

// Create 新增配置
func (h *OkxConfigHandler) Create(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	var req okxConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.ApiKey == "" || req.ApiSecret == "" || req.Passphrase == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key / api_secret / passphrase 均必填"})
		return
	}
	encKey, err := util.EncryptSecret(h.cfg.EncryptKey, req.ApiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}
	encSecret, err := util.EncryptSecret(h.cfg.EncryptKey, req.ApiSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}
	encPhrase, err := util.EncryptSecret(h.cfg.EncryptKey, req.Passphrase)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}

	// 若设为默认则先清除旧的默认
	if req.IsDefault {
		h.db.Model(&model.OkxConfig{}).Where("user_id = ?", uid).Update("is_default", false)
	}
	cg := model.OkxConfig{
		UserID:      uid,
		Name:        req.Name,
		ApiKey:      encKey,
		ApiSecret:   encSecret,
		Passphrase:  encPhrase,
		BaseURL:     normalizeBaseURL(req.BaseURL),
		IsDefault:   req.IsDefault,
		Description: req.Description,
	}
	if err := h.db.Create(&cg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功", "config": cg.SafeView()})
}

// Update 更新配置
func (h *OkxConfigHandler) Update(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var cg model.OkxConfig
	if err := h.db.Where("id = ? AND user_id = ?", id, uid).First(&cg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	var req okxConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]any{
		"name":        req.Name,
		"base_url":    normalizeBaseURL(req.BaseURL),
		"description": req.Description,
		"is_default":  req.IsDefault,
	}
	// 仅当填写了新密钥才更新（允许留空表示不修改）
	if req.ApiKey != "" {
		if v, err := util.EncryptSecret(h.cfg.EncryptKey, req.ApiKey); err == nil {
			updates["api_key"] = v
		}
	}
	if req.ApiSecret != "" {
		if v, err := util.EncryptSecret(h.cfg.EncryptKey, req.ApiSecret); err == nil {
			updates["api_secret"] = v
		}
	}
	if req.Passphrase != "" {
		if v, err := util.EncryptSecret(h.cfg.EncryptKey, req.Passphrase); err == nil {
			updates["passphrase"] = v
		}
	}
	if req.IsDefault {
		h.db.Model(&model.OkxConfig{}).Where("user_id = ? AND id != ?", uid, id).Update("is_default", false)
	}
	if err := h.db.Model(&cg).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	h.db.First(&cg, id)
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "config": cg.SafeView()})
}

// Delete 删除配置
func (h *OkxConfigHandler) Delete(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	res := h.db.Where("id = ? AND user_id = ?", id, uid).Delete(&model.OkxConfig{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// TestConnection 测试 OKX 配置连通性（拉取账户余额）
func (h *OkxConfigHandler) TestConnection(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	cred, err := h.loadCredential(uid, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	client := h.buildOKXClient(cred)
	bal, err := client.GetAccountBalance()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接成功", "total_eq_usd": bal.TotalEq, "coins": len(bal.Details)})
}

// loadCredential 读取并解密配置，返回可直接使用的凭证结构
func (h *OkxConfigHandler) loadCredential(uid uint, id int) (*okx.Credentials, error) {
	var cg model.OkxConfig
	if err := h.db.Where("id = ? AND user_id = ?", id, uid).First(&cg).Error; err != nil {
		return nil, err
	}
	key, err := util.DecryptSecret(h.cfg.EncryptKey, cg.ApiKey)
	if err != nil {
		return nil, err
	}
	secret, err := util.DecryptSecret(h.cfg.EncryptKey, cg.ApiSecret)
	if err != nil {
		return nil, err
	}
	phrase, err := util.DecryptSecret(h.cfg.EncryptKey, cg.Passphrase)
	if err != nil {
		return nil, err
	}
	return &okx.Credentials{
		ApiKey:     key,
		ApiSecret:  secret,
		Passphrase: phrase,
		BaseURL:    normalizeBaseURL(cg.BaseURL),
	}, nil
}

func (h *OkxConfigHandler) buildOKXClient(cred *okx.Credentials) *okx.Client {
	return okx.NewClient(*cred)
}
