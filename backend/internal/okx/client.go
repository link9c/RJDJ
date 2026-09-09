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

// Credentials OKX API 凭证
type Credentials struct {
	ApiKey     string
	ApiSecret  string
	Passphrase string
	BaseURL    string // 域名，默认 https://www.okx.com
}

// Client OKX v5 REST 客户端
// 参考 goex (github.com/nntaoli-project/goex/v2) 对 OKX 的鉴权签名实现，
// 遵循 OKX 官方 v5 REST 规范（HMAC-SHA256 + OK-ACCESS-* 请求头）。
type Client struct {
	cred     Credentials
	http     *http.Client
	baseURL  string
	simulated bool
}

// NewClient 创建客户端，baseURL 为空则用官方域名
func NewClient(cred Credentials) *Client {
	base := cred.BaseURL
	if base == "" {
		base = "https://www.okx.com"
	}
	base = strings.TrimRight(base, "/")
	return &Client{
		cred:    cred,
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: base,
	}
}

func (c *Client) signParam(method, reqPath string, body []byte) (sign, ts string) {
	ts = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	payload := ts + method + reqPath
	if len(body) > 0 {
		payload += string(body)
	}
	mac := hmac.New(sha256.New, []byte(c.cred.ApiSecret))
	mac.Write([]byte(payload))
	sign = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return
}

// flexibleString 既能解析 JSON 字符串又能解析 JSON 数字（OKX 部分接口 code 返回数字 0）
type flexibleString string

func (f *flexibleString) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*f = flexibleString(s)
	return nil
}

// okxResp OKX 统一响应体。data 用 json.RawMessage 承接，因为 OKX 部分接口（如
// bills-history 在无对应类型流水时）会把 data 返回为单个对象 {} 而非数组 []，
// 直接用 []json.RawMessage 反序列化会报 "cannot unmarshal object ... of type []jsontext.Value"。
type okxResp struct {
	Code flexibleString `json:"code"`
	Msg  string         `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// apiErr OKX 业务错误，携带原始 code/msg，便于上层退避/重试判断。
type apiErr struct {
	Code string
	Msg  string
}

func (e *apiErr) Error() string { return fmt.Sprintf("okx api error [%s]: %s", e.Code, e.Msg) }

// IsRateLimit 判断错误是否为 OKX 限流（code 50011 Too Many Requests），用于退避重试。
func IsRateLimit(err error) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*apiErr); ok {
		return ae.Code == "50011"
	}
	return strings.Contains(err.Error(), "[50011]")
}

// dataList 将 okxResp.Data 统一规整为 []json.RawMessage：
//   - null / 缺失 / 单对象 → 返回空列表（单对象不是正常的列表型数据，上层按其为空处理，避免解析崩溃）
//   - 数组 → 原样返回
func (r *okxResp) dataList() []json.RawMessage {
	raw := r.Data
	if len(raw) == 0 {
		return nil
	}
	// 跳过空白，判断首字符
	i := 0
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\n' || raw[i] == '\r') {
		i++
	}
	if i >= len(raw) || raw[i] != '[' {
		return nil // null、{} 或其它非数组 → 空列表
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list
}

// Request 发送一个带鉴权的请求。method: GET/POST; reqPath: 以 /api/v5/... 开头;
// query: URL 查询参数(仅GET); body: POST 请求体(可为 nil)
// 返回 OKX data 数组的原始 JSON（[]json.RawMessage），供上层解析。
func (c *Client) Request(method, reqPath string, query url.Values, body any) ([]json.RawMessage, error) {
	full := c.baseURL + reqPath
	var (
		bodyBytes []byte
		err       error
	)
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	// 构建用于签名的 path
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
	req.Header.Set("OK-ACCESS-KEY", c.cred.ApiKey)
	req.Header.Set("OK-ACCESS-SIGN", sign)
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.cred.Passphrase)
	req.Header.Set("Content-Type", "application/json")
	if method == http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("invalid okx response: %w", err)
	}
	if string(wrap.Code) != "0" {
		return nil, &apiErr{Code: string(wrap.Code), Msg: wrap.Msg}
	}
	return wrap.dataList(), nil
}

// Private GET helper
func (c *Client) Get(reqPath string, query url.Values) ([]json.RawMessage, error) {
	return c.Request(http.MethodGet, reqPath, query, nil)
}

// Private POST helper
func (c *Client) Post(reqPath string, body any) ([]json.RawMessage, error) {
	return c.Request(http.MethodPost, reqPath, nil, body)
}

// PublicRequest 发送无鉴权的公共请求（行情等）
func (c *Client) PublicRequest(reqPath string, query url.Values) ([]json.RawMessage, error) {
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
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wrap okxResp
	if err := json.Unmarshal(respBytes, &wrap); err != nil {
		return nil, fmt.Errorf("invalid okx response: %w", err)
	}
	if string(wrap.Code) != "0" {
		return nil, fmt.Errorf("okx public error [%s]: %s", string(wrap.Code), wrap.Msg)
	}
	return wrap.dataList(), nil
}
