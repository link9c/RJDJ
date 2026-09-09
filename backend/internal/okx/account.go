package okx

import (
	"encoding/json"
	"net/url"
)

// ============= 账户相关（需鉴权） =============

// AccountBalance OKX 统一账户资产全览 /api/v5/account/balance
// 返回解析后的通用结构（保留原生字段，便于前端展示与 AI 分析）
type AccountBalance struct {
	TS       string `json:"ts"`
	Utime    string `json:"uTime"`
	TotalEq  string `json:"totalEq"` // 折算美元总权益
	IsoEq    string `json:"isoEq"`
	AdjEq    string `json:"adjEq"`
	Details  []CoinBalance `json:"details"`
}

type CoinBalance struct {
	Ccy        string `json:"ccy"`
	Eq         string `json:"eq"`          // 币种总权益
	CashBal    string `json:"cashBal"`     // 币种可用余额
	Utime      string `json:"uTime"`
	IsoEq      string `json:"isoEq"`
	EqUsd      string `json:"eqUsd"`       // 折算美元
	AvailBal   string `json:"availBal"`
	FrozenBal  string `json:"frozenBal"`
	MgnRatio   string `json:"mgnRatio"`
	AvailEq    string `json:"availEq"`
	OrdFrozen  string `json:"ordFrozen"`
}

// GetAccountBalance 获取账户资产全览
func (c *Client) GetAccountBalance() (*AccountBalance, error) {
	data, err := c.Get("/api/v5/account/balance", nil)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &AccountBalance{}, nil
	}
	var bal AccountBalance
	if err := json.Unmarshal(data[0], &bal); err != nil {
		return nil, err
	}
	return &bal, nil
}

// Position 持仓
type Position struct {
	InstType   string `json:"instType"`
	MgnMode    string `json:"mgnMode"`
	PosSide    string `json:"posSide"`
	Pos        string `json:"pos"`
	AvailPos   string `json:"availPos"`
	AvgPx      string `json:"avgPx"`
	Upl        string `json:"upl"`     // 未实现盈亏
	UplRatio   string `json:"uplRatio"`
	InstId     string `json:"instId"`
	Lever      string `json:"lever"`
	Margin     string `json:"margin"`
	LiqPx      string `json:"liqPx"`
	MarkPx     string `json:"markPx"`
	MgnRatio   string `json:"mgnRatio"`
	PosUptime  string `json:"uTime"`
}

// GetPositions 获取持仓列表（instType: SWAP/FUTURES/OPTION/SPOT，空则全部）
func (c *Client) GetPositions(instType string) ([]Position, error) {
	q := url.Values{}
	if instType != "" {
		q.Set("instType", instType)
	}
	data, err := c.Get("/api/v5/account/positions", q)
	if err != nil {
		return nil, err
	}
	positions := make([]Position, 0, len(data))
	for _, raw := range data {
		var p Position
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		positions = append(positions, p)
	}
	return positions, nil
}
