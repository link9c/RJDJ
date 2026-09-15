package tui

// Theme 一套配色。全部使用 256 色索引，兼容绝大部分终端。
type Theme struct {
	Name     string
	Bg       Color
	PanelBg  Color
	Text     Color
	TextDim  Color
	Border   Color
	HeaderBg Color
	HeaderFg Color
	SelBg    Color
	SelFg    Color
	Accent   Color
	Rise     Color // 上涨/占用高（A 股习惯：红）
	Fall     Color // 下跌/释放（绿）
	Warn     Color
	BarBg    Color
	BarFill  Color
}

// darkTheme 仿 Windows 11 任务管理器的深色模式。
var darkTheme = Theme{
	Name:     "dark",
	Bg:       234, // #1C1C1C
	PanelBg:  235, // #262626
	Text:     253, // #DADADA
	TextDim:  244, // #808080
	Border:   239, // #4E4E4E
	HeaderBg: 237, // #3A3A3A
	HeaderFg: 251, // #C6C6C6
	SelBg:    24,  // #005F87
	SelFg:    231, // #FFFFFF
	Accent:   81,  // #5FD7FF
	Rise:     210, // #FF8787
	Fall:     114, // #87D787
	Warn:     221, // #FFD75F
	BarBg:    238, // #444444
	BarFill:  75,  // #5FAFFF
}

// lightTheme 仿 Windows 10/11 任务管理器的浅色模式。
var lightTheme = Theme{
	Name:     "light",
	Bg:       231, // #FFFFFF
	PanelBg:  255, // #EEEEEE
	Text:     235, // #262626
	TextDim:  243, // #767676
	Border:   250, // #BCBCBC
	HeaderBg: 252, // #D0D0D0
	HeaderFg: 236, // #303030
	SelBg:    153, // #AFD7FF
	SelFg:    233, // #121212
	Accent:   25,  // #005FAF
	Rise:     160, // #D70000
	Fall:     28,  // #008700
	Warn:     130, // #AF5F00
	BarBg:    251, // #C6C6C6
	BarFill:  32,  // #0087D7
}

// loadColor 按 CPU 强度返回告警配色。
func (t Theme) loadColor(pct float64) Color {
	switch {
	case pct >= 60:
		return t.Rise
	case pct >= 25:
		return t.Warn
	default:
		return t.Text
	}
}
