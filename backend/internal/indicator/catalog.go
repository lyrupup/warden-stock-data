package indicator

import "sort"

// DefaultSnapshotTypes 为全市场扫描 / 逐日指标快照默认落库的指标集合。
// 在均线基础上纳入回测高频使用的经典技术指标（MACD/KDJ/RSI/BOLL/ATR）与中长期动量，
// 使接入方可按交易日直接读 point-in-time 历史指标，无需重算（回测友好）。
// 指标值统一存 stock_indicator_snapshots.values（JSONB），新增指标无需改表结构。
// 该集合由指标包统一维护，service 层引用，避免重复定义。
var DefaultSnapshotTypes = []string{
	"ma5", "ma10", "ma20", "ma30", "ma60",
	"macd_dif", "macd_dea", "macd_bar",
	"kdj_k", "kdj_d", "kdj_j",
	"rsi6", "rsi12", "rsi24",
	"boll_mid", "boll_upper", "boll_lower",
	"atr14", "atr20",
	"pct_change20", "pct_change60",
}

func snapshotTypeSet() map[string]bool {
	set := make(map[string]bool, len(DefaultSnapshotTypes))
	for _, t := range DefaultSnapshotTypes {
		set[t] = true
	}
	return set
}

// InDefaultSnapshot 判断某指标类型是否在默认逐日快照集合中（决定能否走快照读取）。
func InDefaultSnapshot(typ string) bool {
	for _, t := range DefaultSnapshotTypes {
		if t == typ {
			return true
		}
	}
	return false
}

// catalogMeta 为指标的静态展示元数据，作为「指标定义 ↔ 计算引擎 ↔ 前端/接入方」的单一事实源。
type catalogMeta struct {
	name      string
	group     string
	valueType string                 // number / bool
	params    map[string]interface{} // 计算参数，便于接入方理解口径
	order     int                    // 目录展示顺序
}

// metaTable 描述各指标的元数据。implemented / snapshot 字段不在此声明，
// 而是分别由「是否已注册到 registry」与「是否在 DefaultSnapshotTypes」动态派生，保证一致。
var metaTable = map[string]catalogMeta{
	"ma5":   {name: "MA5", group: "均线", valueType: "number", params: map[string]interface{}{"period": 5}, order: 10},
	"ma10":  {name: "MA10", group: "均线", valueType: "number", params: map[string]interface{}{"period": 10}, order: 11},
	"ma20":  {name: "MA20", group: "均线", valueType: "number", params: map[string]interface{}{"period": 20}, order: 12},
	"ma30":  {name: "MA30", group: "均线", valueType: "number", params: map[string]interface{}{"period": 30}, order: 13},
	"ma60":  {name: "MA60", group: "均线", valueType: "number", params: map[string]interface{}{"period": 60}, order: 14},
	"ma120": {name: "MA120", group: "均线", valueType: "number", params: map[string]interface{}{"period": 120}, order: 15},

	"macd_dif": {name: "MACD-DIF(快线)", group: "MACD", valueType: "number", params: macdParams(), order: 20},
	"macd_dea": {name: "MACD-DEA(信号线)", group: "MACD", valueType: "number", params: macdParams(), order: 21},
	"macd_bar": {name: "MACD柱", group: "MACD", valueType: "number", params: macdParams(), order: 22},

	"kdj_k": {name: "KDJ-K", group: "KDJ", valueType: "number", params: kdjParams(), order: 30},
	"kdj_d": {name: "KDJ-D", group: "KDJ", valueType: "number", params: kdjParams(), order: 31},
	"kdj_j": {name: "KDJ-J", group: "KDJ", valueType: "number", params: kdjParams(), order: 32},

	"rsi6":  {name: "RSI6", group: "RSI", valueType: "number", params: map[string]interface{}{"period": 6}, order: 40},
	"rsi12": {name: "RSI12", group: "RSI", valueType: "number", params: map[string]interface{}{"period": 12}, order: 41},
	"rsi24": {name: "RSI24", group: "RSI", valueType: "number", params: map[string]interface{}{"period": 24}, order: 42},

	"boll_mid":   {name: "BOLL中轨", group: "BOLL", valueType: "number", params: bollParams(), order: 50},
	"boll_upper": {name: "BOLL上轨", group: "BOLL", valueType: "number", params: bollParams(), order: 51},
	"boll_lower": {name: "BOLL下轨", group: "BOLL", valueType: "number", params: bollParams(), order: 52},

	"atr14": {name: "ATR14(真实波幅)", group: "波动率", valueType: "number", params: map[string]interface{}{"period": 14}, order: 60},
	"atr20": {name: "ATR20(真实波幅)", group: "波动率", valueType: "number", params: map[string]interface{}{"period": 20}, order: 61},

	"bias5":  {name: "乖离率5", group: "乖离", valueType: "number", params: map[string]interface{}{"period": 5}, order: 70},
	"bias10": {name: "乖离率10", group: "乖离", valueType: "number", params: map[string]interface{}{"period": 10}, order: 71},
	"bias20": {name: "乖离率20", group: "乖离", valueType: "number", params: map[string]interface{}{"period": 20}, order: 72},

	"ma_align": {name: "均线多头排列", group: "形态", valueType: "bool", params: map[string]interface{}{"periods": []int{5, 10, 20, 30, 60}}, order: 80},

	"amplitude":        {name: "振幅", group: "波动率", valueType: "number", order: 90},
	"amplitude_streak": {name: "连续振幅", group: "波动率", valueType: "number", params: map[string]interface{}{"threshold": 5}, order: 91},

	"pct_change":   {name: "涨跌幅", group: "动量", valueType: "number", params: map[string]interface{}{"days": 1}, order: 100},
	"pct_change5":  {name: "5日涨跌幅", group: "动量", valueType: "number", params: map[string]interface{}{"days": 5}, order: 101},
	"pct_change20": {name: "20日涨跌幅(动量)", group: "动量", valueType: "number", params: map[string]interface{}{"days": 20}, order: 102},
	"pct_change60": {name: "60日涨跌幅(动量)", group: "动量", valueType: "number", params: map[string]interface{}{"days": 60}, order: 103},

	"vol_ratio": {name: "量比", group: "量价", valueType: "number", params: map[string]interface{}{"period": 5}, order: 110},

	"close":  {name: "收盘价", group: "原始字段", valueType: "number", order: 120},
	"volume": {name: "成交量", group: "原始字段", valueType: "number", order: 121},
}

func macdParams() map[string]interface{} {
	return map[string]interface{}{"fast": macdFast, "slow": macdSlow, "signal": macdSignal}
}

func kdjParams() map[string]interface{} {
	return map[string]interface{}{"n": kdjN, "k": kdjK, "d": kdjD}
}

func bollParams() map[string]interface{} {
	return map[string]interface{}{"period": bollPeriod, "mult": bollMult}
}

// Catalog 由 registry 动态派生指标目录：凡已注册的指标类型均出现（注册即可见），
// implemented 恒为 true（已注册即可计算），snapshot 标识该指标是否进默认逐日快照
// （决定能否经 /open/v1/indicators 批量按日读取，回测构建策略的关键依据）。
func Catalog() []map[string]interface{} {
	inSnapshot := snapshotTypeSet()
	out := make([]map[string]interface{}, 0, len(registry))
	for typ := range registry {
		m := metaTable[typ]
		name := m.name
		if name == "" {
			name = typ
		}
		valueType := m.valueType
		if valueType == "" {
			valueType = "number"
		}
		item := map[string]interface{}{
			"type":        typ,
			"name":        name,
			"group":       m.group,
			"value_type":  valueType,
			"snapshot":    inSnapshot[typ],
			"implemented": true,
			"_order":      m.order,
		}
		if m.params != nil {
			item["params"] = m.params
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		oi := out[i]["_order"].(int)
		oj := out[j]["_order"].(int)
		if oi != oj {
			if oi == 0 {
				return false
			}
			if oj == 0 {
				return true
			}
			return oi < oj
		}
		return out[i]["type"].(string) < out[j]["type"].(string)
	})
	for _, item := range out {
		delete(item, "_order")
	}
	return out
}
