package okx

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// ============= 行情与公共数据（免鉴权） =============

type Ticker struct {
	InstId  string `json:"instId"`
	Last    string `json:"last"`
	LastSz  string `json:"lastSz"`
	AskPx   string `json:"askPx"`
	BidPx   string `json:"bidPx"`
	Open24h string `json:"open24h"`
	High24h string `json:"high24h"`
	Low24h  string `json:"low24h"`
	Vol24h  string `json:"vol24h"`
	VolCcy24h string `json:"volCcy24h"`
	Ts      string `json:"ts"`
}

// GetTicker 获取单个产品行情
func (c *Client) GetTicker(instId string) (*Ticker, error) {
	q := url.Values{}
	q.Set("instId", instId)
	data, err := c.PublicRequest("/api/v5/market/ticker", q)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &Ticker{}, nil
	}
	var t Ticker
	if err := json.Unmarshal(data[0], &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTickers 获取全部行情（instType 可选 SPOT/SWAP/FUTURES 等，空为全部现货）
func (c *Client) GetTickers(instType string) ([]Ticker, error) {
	q := url.Values{}
	if instType != "" {
		q.Set("instType", instType)
	}
	data, err := c.PublicRequest("/api/v5/market/tickers", q)
	if err != nil {
		return nil, err
	}
	tickers := make([]Ticker, 0, len(data))
	for _, raw := range data {
		var t Ticker
		if err := json.Unmarshal(raw, &t); err == nil {
			tickers = append(tickers, t)
		}
	}
	return tickers, nil
}

// Kline 单根K线
type Kline [6]string // [ts, o, h, l, c, vol]

// GetCandles 获取K线，bar: 1m/5m/15m/1H/4H/1D 等
func (c *Client) GetCandles(instId, bar string, limit int) ([][]string, error) {
	q := url.Values{}
	q.Set("instId", instId)
	if bar != "" {
		q.Set("bar", bar)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	data, err := c.PublicRequest("/api/v5/market/candles", q)
	if err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(data))
	for _, raw := range data {
		var arr []string
		if err := json.Unmarshal(raw, &arr); err == nil {
			out = append(out, arr)
		}
	}
	return out, nil
}
