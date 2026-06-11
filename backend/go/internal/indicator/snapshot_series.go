package indicator

import (
	"context"
	"math"

	"github.com/shopspring/decimal"
)

// SnapshotEmit 流式输出单日指标快照；返回非 nil error 时立即停止（含 context.Canceled）。
type SnapshotEmit func(i int, vals map[string]decimal.Decimal) error

// StreamSnapshotSeries 单遍 O(n) 流式计算逐日指标，额外内存 O(1)（不缓存全长序列）。
func StreamSnapshotSeries(ctx context.Context, s Series, types []string, emit SnapshotEmit) error {
	n := len(s.Bars)
	if n == 0 || len(types) == 0 || emit == nil {
		return nil
	}
	want := make(map[string]bool, len(types))
	for _, t := range types {
		want[t] = true
	}

	eng := newSnapshotStreamEngine(s.Bars, want)
	for i := 0; i < n; i++ {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		vals := eng.step(i)
		if len(vals) == 0 {
			continue
		}
		if err := emit(i, vals); err != nil {
			return err
		}
	}
	return nil
}

// ComputeSnapshotSeries 供测试/小规模场景；生产回补请用 StreamSnapshotSeries。
func ComputeSnapshotSeries(s Series, types []string) []map[string]decimal.Decimal {
	n := len(s.Bars)
	out := make([]map[string]decimal.Decimal, n)
	_ = StreamSnapshotSeries(context.Background(), s, types, func(i int, vals map[string]decimal.Decimal) error {
		if len(vals) == 0 {
			return nil
		}
		cp := make(map[string]decimal.Decimal, len(vals))
		for k, v := range vals {
			cp[k] = v
		}
		out[i] = cp
		return nil
	})
	return out
}

type snapshotStreamEngine struct {
	bars []Bar
	want map[string]bool
	// emitBuf 在 emit 返回前有效，下一根 K 线 step 会覆盖，禁止异步持有引用。
	emitBuf map[string]decimal.Decimal

	ma map[int]*maStream

	emaFast, emaSlow, emaDea, dif decimal.Decimal
	macdFastInit, macdSlowInit, macdDeaInit bool
	kFast, kSlow, kSig                       decimal.Decimal
	oneMinusKFast, oneMinusKSlow, oneMinusKSig decimal.Decimal

	rsi map[int]*rsiStream

	prevK, prevD decimal.Decimal

	bollSum, bollSumSq decimal.Decimal

	atr map[int]*atrStream

	prevClose    decimal.Decimal
	hasPrevClose bool
}

func newSnapshotStreamEngine(bars []Bar, want map[string]bool) *snapshotStreamEngine {
	e := &snapshotStreamEngine{
		bars:    bars,
		want:    want,
		emitBuf: make(map[string]decimal.Decimal, len(want)),
		ma:      make(map[int]*maStream),
		rsi:     make(map[int]*rsiStream),
		atr:     make(map[int]*atrStream),
		prevK:   decimal.NewFromInt(50),
		prevD:   decimal.NewFromInt(50),
	}
	e.kFast = decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(macdFast) + 1))
	e.kSlow = decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(macdSlow) + 1))
	e.kSig = decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(macdSignal) + 1))
	e.oneMinusKFast = decimal.NewFromInt(1).Sub(e.kFast)
	e.oneMinusKSlow = decimal.NewFromInt(1).Sub(e.kSlow)
	e.oneMinusKSig = decimal.NewFromInt(1).Sub(e.kSig)

	for _, p := range []int{5, 10, 20, 30, 60} {
		if wantMA(want, p) {
			e.ma[p] = &maStream{period: p}
		}
	}
	for _, p := range []int{6, 12, 24} {
		if wantRSI(want, p) {
			e.rsi[p] = &rsiStream{period: p}
		}
	}
	for _, p := range []int{14, 20} {
		if wantATR(want, p) {
			e.atr[p] = &atrStream{period: p}
		}
	}
	return e
}

func wantMA(want map[string]bool, p int) bool {
	switch p {
	case 5:
		return want["ma5"]
	case 10:
		return want["ma10"]
	case 20:
		return want["ma20"]
	case 30:
		return want["ma30"]
	case 60:
		return want["ma60"]
	}
	return false
}

func wantRSI(want map[string]bool, p int) bool {
	switch p {
	case 6:
		return want["rsi6"]
	case 12:
		return want["rsi12"]
	case 24:
		return want["rsi24"]
	}
	return false
}

func wantATR(want map[string]bool, p int) bool {
	switch p {
	case 14:
		return want["atr14"]
	case 20:
		return want["atr20"]
	}
	return false
}

type maStream struct {
	period int
	sum    decimal.Decimal
}

type rsiStream struct {
	period      int
	avgGain     decimal.Decimal
	avgLoss     decimal.Decimal
	warmGain    decimal.Decimal
	warmLoss    decimal.Decimal
	barsSeen    int
	initialized bool
}

type atrStream struct {
	period      int
	atr         decimal.Decimal
	warmSum     decimal.Decimal
	warmCount   int
	initialized bool
}

type macdVals struct {
	dif, dea, bar decimal.Decimal
}

func (e *snapshotStreamEngine) step(i int) map[string]decimal.Decimal {
	bar := e.bars[i]
	close := bar.Close
	vals := e.emitBuf
	for k := range vals {
		delete(vals, k)
	}

	for p, st := range e.ma {
		if v, ok := st.add(close, i, e.bars); ok {
			vals[maType(p)] = v
		}
	}

	if e.want["macd_dif"] || e.want["macd_dea"] || e.want["macd_bar"] {
		if v, ok := e.macdStep(close, i); ok {
			if e.want["macd_dif"] {
				vals["macd_dif"] = v.dif
			}
			if e.want["macd_dea"] {
				vals["macd_dea"] = v.dea
			}
			if e.want["macd_bar"] {
				vals["macd_bar"] = v.bar
			}
		}
	}

	for p, st := range e.rsi {
		if v, ok := st.add(close, e.prevClose, e.hasPrevClose); ok {
			vals[rsiType(p)] = v
		}
	}

	if e.want["kdj_k"] || e.want["kdj_d"] || e.want["kdj_j"] {
		if k, d, j, ok := e.kdjStep(bar, i); ok {
			if e.want["kdj_k"] {
				vals["kdj_k"] = k
			}
			if e.want["kdj_d"] {
				vals["kdj_d"] = d
			}
			if e.want["kdj_j"] {
				vals["kdj_j"] = j
			}
		}
	}

	if e.want["boll_mid"] || e.want["boll_upper"] || e.want["boll_lower"] {
		if mid, upper, lower, ok := e.bollStep(close, i); ok {
			if e.want["boll_mid"] {
				vals["boll_mid"] = mid
			}
			if e.want["boll_upper"] {
				vals["boll_upper"] = upper
			}
			if e.want["boll_lower"] {
				vals["boll_lower"] = lower
			}
		}
	}

	for p, st := range e.atr {
		if v, ok := st.add(bar, e.prevClose, e.hasPrevClose, i); ok {
			vals[atrType(p)] = v
		}
	}

	hundred := decimal.NewFromInt(100)
	if e.want["pct_change20"] && i >= 20 {
		base := e.bars[i-20].Close
		if !base.IsZero() {
			vals["pct_change20"] = close.Sub(base).Div(base).Mul(hundred)
		}
	}
	if e.want["pct_change60"] && i >= 60 {
		base := e.bars[i-60].Close
		if !base.IsZero() {
			vals["pct_change60"] = close.Sub(base).Div(base).Mul(hundred)
		}
	}

	e.prevClose = close
	e.hasPrevClose = true
	return vals
}

func maType(p int) string {
	switch p {
	case 5:
		return "ma5"
	case 10:
		return "ma10"
	case 20:
		return "ma20"
	case 30:
		return "ma30"
	default:
		return "ma60"
	}
}

func rsiType(p int) string {
	switch p {
	case 6:
		return "rsi6"
	case 12:
		return "rsi12"
	default:
		return "rsi24"
	}
}

func atrType(p int) string {
	if p == 14 {
		return "atr14"
	}
	return "atr20"
}

func (st *maStream) add(close decimal.Decimal, i int, bars []Bar) (decimal.Decimal, bool) {
	p := st.period
	if i < p-1 {
		return decimal.Zero, false
	}
	if i == p-1 {
		sum := decimal.Zero
		for j := 0; j < p; j++ {
			sum = sum.Add(bars[j].Close)
		}
		st.sum = sum
	} else {
		st.sum = st.sum.Add(close).Sub(bars[i-p].Close)
	}
	pd := decimal.NewFromInt(int64(p))
	return st.sum.Div(pd), true
}

func (e *snapshotStreamEngine) macdStep(close decimal.Decimal, i int) (macdVals, bool) {
	if !e.macdFastInit {
		e.emaFast = close
		e.macdFastInit = true
	} else {
		e.emaFast = close.Mul(e.kFast).Add(e.emaFast.Mul(e.oneMinusKFast))
	}
	if !e.macdSlowInit {
		e.emaSlow = close
		e.macdSlowInit = true
	} else {
		e.emaSlow = close.Mul(e.kSlow).Add(e.emaSlow.Mul(e.oneMinusKSlow))
	}
	e.dif = e.emaFast.Sub(e.emaSlow)
	if !e.macdDeaInit {
		e.emaDea = e.dif
		e.macdDeaInit = true
	} else {
		e.emaDea = e.dif.Mul(e.kSig).Add(e.emaDea.Mul(e.oneMinusKSig))
	}
	two := decimal.NewFromInt(2)
	bar := e.dif.Sub(e.emaDea).Mul(two)
	if i < macdSlow+macdSignal-1 {
		return macdVals{}, false
	}
	return macdVals{dif: e.dif, dea: e.emaDea, bar: bar}, true
}

func (st *rsiStream) add(close, prevClose decimal.Decimal, hasPrev bool) (decimal.Decimal, bool) {
	if !hasPrev {
		return decimal.Zero, false
	}
	st.barsSeen++
	diff := close.Sub(prevClose)
	gain := decimal.Zero
	loss := decimal.Zero
	if diff.IsPositive() {
		gain = diff
	} else {
		loss = diff.Neg()
	}
	pd := decimal.NewFromInt(int64(st.period))
	pm1 := decimal.NewFromInt(int64(st.period) - 1)
	hundred := decimal.NewFromInt(100)

	if !st.initialized {
		st.warmGain = st.warmGain.Add(gain)
		st.warmLoss = st.warmLoss.Add(loss)
		if st.barsSeen < st.period {
			return decimal.Zero, false
		}
		st.avgGain = st.warmGain.Div(pd)
		st.avgLoss = st.warmLoss.Div(pd)
		st.initialized = true
	} else {
		st.avgGain = st.avgGain.Mul(pm1).Add(gain).Div(pd)
		st.avgLoss = st.avgLoss.Mul(pm1).Add(loss).Div(pd)
	}
	if st.avgLoss.IsZero() {
		return hundred, true
	}
	rs := st.avgGain.Div(st.avgLoss)
	return hundred.Sub(hundred.Div(decimal.NewFromInt(1).Add(rs))), true
}

func (e *snapshotStreamEngine) kdjStep(bar Bar, i int) (k, d, j decimal.Decimal, ok bool) {
	rsv := decimal.Zero
	if i >= kdjN-1 {
		high := bar.High
		low := bar.Low
		for idx := i - kdjN + 1; idx <= i; idx++ {
			if e.bars[idx].High.GreaterThan(high) {
				high = e.bars[idx].High
			}
			if e.bars[idx].Low.LessThan(low) {
				low = e.bars[idx].Low
			}
		}
		hundred := decimal.NewFromInt(100)
		rng := high.Sub(low)
		if rng.IsPositive() {
			rsv = bar.Close.Sub(low).Div(rng).Mul(hundred)
		}
	}
	kw := decimal.NewFromInt(int64(kdjK))
	dw := decimal.NewFromInt(int64(kdjD))
	e.prevK = e.prevK.Mul(kw.Sub(decimal.NewFromInt(1))).Add(rsv).Div(kw)
	e.prevD = e.prevD.Mul(dw.Sub(decimal.NewFromInt(1))).Add(e.prevK).Div(dw)
	if i < kdjN-1 {
		return decimal.Zero, decimal.Zero, decimal.Zero, false
	}
	jVal := e.prevK.Mul(decimal.NewFromInt(3)).Sub(e.prevD.Mul(decimal.NewFromInt(2)))
	return e.prevK, e.prevD, jVal, true
}

func (e *snapshotStreamEngine) bollStep(close decimal.Decimal, i int) (mid, upper, lower decimal.Decimal, ok bool) {
	if i < bollPeriod-1 {
		return decimal.Zero, decimal.Zero, decimal.Zero, false
	}
	sq := close.Mul(close)
	if i == bollPeriod-1 {
		sum := decimal.Zero
		sumSq := decimal.Zero
		for j := 0; j < bollPeriod; j++ {
			c := e.bars[j].Close
			sum = sum.Add(c)
			sumSq = sumSq.Add(c.Mul(c))
		}
		e.bollSum = sum
		e.bollSumSq = sumSq
	} else {
		old := e.bars[i-bollPeriod].Close
		e.bollSum = e.bollSum.Add(close).Sub(old)
		e.bollSumSq = e.bollSumSq.Add(sq).Sub(old.Mul(old))
	}
	pd := decimal.NewFromInt(int64(bollPeriod))
	mid = e.bollSum.Div(pd)
	meanSq := e.bollSumSq.Div(pd)
	variance := meanSq.Sub(mid.Mul(mid))
	if variance.IsNegative() {
		variance = decimal.Zero
	}
	vf, _ := variance.Float64()
	std := decimal.NewFromFloat(math.Sqrt(vf))
	band := std.Mul(decimal.NewFromInt(int64(bollMult)))
	return mid, mid.Add(band), mid.Sub(band), true
}

func (st *atrStream) add(bar Bar, prevClose decimal.Decimal, hasPrev bool, i int) (decimal.Decimal, bool) {
	tr := bar.High.Sub(bar.Low)
	if hasPrev {
		hc := bar.High.Sub(prevClose).Abs()
		lc := bar.Low.Sub(prevClose).Abs()
		tr = decimal.Max(tr, hc, lc)
	}
	if i == 0 {
		return decimal.Zero, false
	}
	pd := decimal.NewFromInt(int64(st.period))
	pm1 := decimal.NewFromInt(int64(st.period) - 1)

	if !st.initialized {
		st.warmSum = st.warmSum.Add(tr)
		st.warmCount++
		if st.warmCount < st.period {
			return decimal.Zero, false
		}
		st.atr = st.warmSum.Div(pd)
		st.initialized = true
		return st.atr, true
	}
	st.atr = st.atr.Mul(pm1).Add(tr).Div(pd)
	return st.atr, true
}
