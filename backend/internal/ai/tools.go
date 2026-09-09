package ai

import "encoding/json"

// BuildAccountTools 返回 OKX 数据分析可用的工具定义
func BuildAccountTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_account_overview",
				Description: "获取 OKX 账户资产总览，包含总权益(totalEq)、各币种余额、以及当前所有合约/币对持仓信息。用于回答我的账户资产多少/总权益/持仓等问题。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_ticker",
				Description: "获取某个交易产品的实时行情价格。例如获取 BTC-USDT 的现价、24h涨跌幅。instId 传币对标识，如 BTC-USDT、ETH-USDT。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"inst_id": map[string]any{"type": "string", "description": "产品ID，如 BTC-USDT"},
					},
					"required": []string{"inst_id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_positions",
				Description: "获取当前全部合约持仓(含方向、数量、开仓均价、未实现盈亏upl、杠杆)。用于回答持仓相关、盈亏分析问题。",
				Parameters: map[string]any{
					"type":     "object",
					"properties": map[string]any{
						"inst_type": map[string]any{"type": "string", "enum": []string{"", "SPOT", "SWAP", "FUTURES", "OPTION"}, "description": "产品类型，留空获取全部"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_candles",
				Description: "获取K线行情数据(可含时间序列 OHLCV)，用于技术分析、走势判断。返回数据格式每行 [时间戳,开,高,低,收,成交量]。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"inst_id": map[string]any{"type": "string", "description": "产品ID，如 BTC-USDT"},
						"bar":     map[string]any{"type": "string", "enum": []string{"1m", "5m", "15m", "1H", "4H", "1D"}, "description": "K线周期，默认1H"},
						"limit":   map[string]any{"type": "integer", "description": "返回根数，默认100，最大300"},
					},
					"required": []string{"inst_id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_strategies",
				Description: "获取用户的交易策略/机器人列表(含运行中和历史)，涵盖马丁格尔DCA(dca，分现货spot_dca与合约contract_dca)、网格(grid，分现货与合约)、定投(recurring)、信号(signal)、策略委托条件单/止盈止损(algo)。返回里每条含 algoOrdType(如 contract_dca=合约马丁) 与 instType(SPOT/SWAP)。参数 history=0 只查运行中，1 查历史。用于回答'我的策略/我的马丁格尔/我有哪些策略/策略运行情况'等问题。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"history": map[string]any{"type": "integer", "enum": []int{0, 1}, "description": "0=运行中，1=历史，默认0"},
					},
				},
			},
		},
	}
}

// MarshalJSON 便于打印
func (t Tool) Pretty() string {
	b, _ := json.MarshalIndent(t, "", "  ")
	return string(b)
}
