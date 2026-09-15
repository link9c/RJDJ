// Package okx 是 OKX v5 REST 的只读精简客户端。
//
// 从 RJDJ/backend/internal/okx 抽取，去掉了业务归一化与 AI 相关逻辑，
// 只保留签名鉴权、请求发送与持仓/策略/行情三类读取。
// 签名遵循 OKX 官方规范（HMAC-SHA256 + OK-ACCESS-* 请求头），与 goex 实现一致。
package okx

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Credentials OKX API 凭证。建议只授予「读取」权限。
type Credentials struct {
	APIKey     string
	APISecret  string
	Passphrase string
	BaseURL    string // 留空使用官方域名
	Simulated  bool   // 模拟盘
}

// Client OKX v5 REST 客户端。
type Client struct {
	cred    Credentials
	http    *http.Client
	baseURL string
}

// NewClient 创建客户端。
func NewClient(cred Credentials) *Client {
	base := strings.TrimSpace(cred.BaseURL)
	if base == "" {
		base = "https://www.okx.com"
	}
	return &Client{
		cred:    cred,
		http:    &http.Client{Timeout: 20 * time.Second},
		baseURL: strings.TrimRight(base, "/"),
	}
}

func (c *Client) signParam(method, reqPath string, body []byte) (sign, ts string) {
	ts = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	payload := ts + method + reqPath
	if len(body) > 0 {
		payload += string(body)
	}
	mac := hmac.New(sha256.New, []byte(c.cred.APISecret))
	mac.Write([]byte(payload))
	sign = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return
}

// flexibleString 兼容 OKX 偶尔把 code 返回成数字 0 的情况。
type flexibleString string

func (f *flexibleString) UnmarshalJSON(b []byte) error {
	*f = flexibleString(strings.Trim(string(b), `"`))
	return nil
}

// okxResp 统一响应体。data 用 RawMessage 承接：OKX 部分接口在无数据时
// 会把 data 返回成对象 {} 而非数组，直接按 []T 反序列化会失败。
type okxResp struct {
	Code flexibleString  `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// apiErr OKX 业务错误，携带原始 code/msg。
type apiErr struct {
	Code string
	Msg  string
}

func (e *apiErr) Error() string { return fmt.Sprintf("okx api error [%s]: %s", e.Code, e.Msg) }

// dataList 把 data 统一规整为列表；null / {} / 非数组一律按空处理。
func (r *okxResp) dataList() []json.RawMessage {
	raw := r.Data
	if len(raw) == 0 {
		return nil
	}
	i := 0
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\n' || raw[i] == '\r') {
		i++
	}
	if i >= len(raw) || raw[i] != '[' {
		return nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list
}

// Request 发送带鉴权的请求并返回 data 数组原始 JSON。
func (c *Client) Request(method, reqPath string, query url.Values, body any) ([]json.RawMessage, error) {
	full := c.baseURL + reqPath
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyBytes = b
	}

	signPath := reqPath
	if method == http.MethodGet && len(query) > 0 {
		enc := query.Encode()
		signPath = reqPath + "?" + enc
		full = full + "?" + enc
	}
	sign, ts := c.signParam(method, signPath, bodyBytes)

	req, err := http.NewRequest(method, full, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("OK-ACCESS-KEY", c.cred.APIKey)
	req.Header.Set("OK-ACCESS-SIGN", sign)
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.cred.Passphrase)
	req.Header.Set("Content-Type", "application/json")
	if c.cred.Simulated {
		req.Header.Set("x-simulated-trading", "1")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wrap okxResp
	if err := json.Unmarshal(respBytes, &wrap); err != nil {
		return nil, fmt.Errorf("okx 响应解析失败: %w", err)
	}
	if string(wrap.Code) != "0" {
		return nil, &apiErr{Code: string(wrap.Code), Msg: wrap.Msg}
	}
	return wrap.dataList(), nil
}

// Get 带鉴权的 GET。
func (c *Client) Get(reqPath string, query url.Values) ([]json.RawMessage, error) {
	return c.Request(http.MethodGet, reqPath, query, nil)
}

// PublicGet 无鉴权的公共 GET（行情等）。
func (c *Client) PublicGet(reqPath string, query url.Values) ([]json.RawMessage, error) {
	full := c.baseURL + reqPath
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wrap okxResp
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, fmt.Errorf("okx 响应解析失败: %w", err)
	}
	if string(wrap.Code) != "0" {
		return nil, &apiErr{Code: string(wrap.Code), Msg: wrap.Msg}
	}
	return wrap.dataList(), nil
}
