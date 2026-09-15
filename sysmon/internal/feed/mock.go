package feed

import "time"

// DemoSample 返回一份内置演示快照，供界面预览与测试使用。
func DemoSample() Snapshot { return mockSnapshot() }

// mockSnapshot 离线演示数据。用于未配置 API Key 或开启 okx.mock 时渲染界面，
// 方便先调整外观/映射，再去接真实账户。
func mockSnapshot() Snapshot {
	return Snapshot{
		Time:    time.Now(),
		TotalEq: 24680.55,
		Rows: []Row{
			{InstID: "BTC-USDT-SWAP", Kind: "position", Side: "long", State: "live",
				Lever: 10, UplRatio: 0.0352, Upl: 412.66, Margin: 1240.5, Notional: 12405,
				AvgPx: 61230.5, MarkPx: 63385.0, Pos: 2.02, Chg24h: 0.0183, Ccy: "USDT", Note: "逐仓"},

			{InstID: "ETH-USDT-SWAP", Kind: "position", Side: "short", State: "live",
				Lever: 5, UplRatio: -0.0127, Upl: -96.4, Margin: 760.0, Notional: 7600,
				AvgPx: 2540.8, MarkPx: 2573.1, Pos: 29.8, Chg24h: -0.0091, Ccy: "USDT", Note: "全仓"},

			{InstID: "SOL-USDT-SWAP", Kind: "position", Side: "long", State: "live",
				Lever: 3, UplRatio: 0.0684, Upl: 214.9, Margin: 980.0, Notional: 2940,
				AvgPx: 138.62, MarkPx: 148.1, Pos: 21.2, Chg24h: 0.0412, Ccy: "USDT", Note: "逐仓"},

			{InstID: "BTC-USDT", Kind: "grid", AlgoID: "3125501", Side: "neutral", State: "running",
				UplRatio: 0.0213, Upl: 63.9, Margin: 3000, Notional: 3000, AvgPx: 62410, MarkPx: 63385,
				Chg24h: 0.0183, Ccy: "USDT", Note: "现货网格"},

			{InstID: "ETH-USDT-SWAP", Kind: "dca", AlgoID: "3125502", Side: "long", State: "running",
				Lever: 5, UplRatio: -0.0431, Upl: -129.3, Margin: 3000, Notional: 3000,
				AvgPx: 2648.2, MarkPx: 2573.1, Chg24h: -0.0091, Ccy: "USDT", Note: "合约马丁"},

			{InstID: "BTC-USDT", Kind: "recurring", AlgoID: "3125503", Side: "long", State: "running",
				UplRatio: 0.0086, Upl: 12.4, Margin: 1440, Notional: 1440,
				AvgPx: 62870, MarkPx: 63385, Chg24h: 0.0183, Ccy: "USDT", Note: "定投"},

			{InstID: "BTC-USDT-SWAP", Kind: "signal", AlgoID: "3125504", Side: "long", State: "running",
				Lever: 5, UplRatio: 0.0271, Upl: 108.4, Margin: 4000, Notional: 4000,
				AvgPx: 61810, MarkPx: 63385, Chg24h: 0.0183, Ccy: "USDT", Note: "信号"},

			{InstID: "ETH-USDT-SWAP", Kind: "algo", AlgoID: "3125505", Side: "short", State: "effective",
				UplRatio: 0.0, Upl: 0, Margin: 0, Notional: 0,
				MarkPx: 2573.1, Chg24h: -0.0091, Ccy: "USDT", Note: "条件单"},
		},
	}
}
