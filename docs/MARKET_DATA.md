# 行情数据拉取技术文档（K 线 + 分时）

本文档梳理 **个股 K 线（日/周/月）** 与 **当日分时走势** 两类行情数据的获取链路、分层职责、数据源接口、存储策略与关键实现细节，供后续维护与扩展参考。

> 阅读前置：`docs/BACKEND.md`（后端架构）、`docs/openapi.yaml`（接口契约）。

---

## 一、总览对比

| 维度 | 日 K 线 | 分时走势 |
|------|---------|----------|
| 对外接口 | `GET /open/v1/stocks/{code}/kline` | `GET /open/v1/stocks/{code}/intraday` |
| 管理后台镜像 | `GET /admin/market/stocks/{code}/kline` | `GET /admin/market/stocks/{code}/intraday` |
| 数据源 | gotdx → 通达信（TDX）行情节点 | gotdx → 通达信（TDX）行情节点 |
| gotdx 方法 | `GetSecurityBars` | `StockTickChart` / `StockHistoryTickChart` + 辅助 |
| PostgreSQL 落库 | ✅ `stock_daily_klines`（`period=day` 时） | ❌ 不落库（实时透传） |
| Redis 缓存 | ❌（依赖库优先） | ❌ |
| 读取策略 | **读库优先**，缺失/非日 K 才回源 TDX | **每次实时拉 TDX** |
| 增量落库 | ✅ 盘后定时任务按水位增量写入 | — |
| 时间粒度 | 交易日（`date`） | 交易分钟（240 点 / 日） |
| 主要用途 | 图表展示 + 指标计算 + 回测底座 | 个股详情页分时图展示 |

**一句话**：日 K 是「**先查库、按需回源、盘后增量落库**」的持久化数据；分时是「**每次实时从 TDX 拉取、不落库**」的透传数据。

---

## 二、涉及文件与分层

两条链路共用同一套适配器分层（适配器模式，上层只依赖 `IMarketProvider` 接口）：

| 分层 | 文件 | 职责 |
|------|------|------|
| 路由 | `backend/internal/router/router.go` | 注册 admin / open 两组只读路由 |
| Handler | `backend/internal/handler/open/market_handler.go` | 解析 query 参数、统一响应 |
| Service（K 线） | `backend/internal/service/kline_service.go` | 读库优先 + 区间/数量过滤 |
| Service（分时/快照） | `backend/internal/service/quote_service.go` | 透传 provider + 补全股票名 |
| Service（增量落库） | `backend/internal/service/update_service.go` | 盘后增量写日 K + 推进水位 |
| Repository | `backend/internal/repository/market_repo.go` | `KlineRepository`：查询/批量 upsert |
| Provider 接口 | `backend/internal/integration/market/provider.go` | `IMarketProvider` 抽象 |
| gotdx 实现 | `backend/internal/integration/market/gotdx_provider.go` | 调 gotdx 拉数 |
| 字段映射 | `backend/internal/integration/market/gotdx_mapper.go` | gotdx 原始结构 → 领域模型 |
| 连接池 | `backend/internal/integration/market/gotdx_pool.go` | 复用 TDX TCP 连接 |
| 工厂 | `backend/internal/integration/market/factory.go` | 创建 gotdx provider |
| 领域模型 | `backend/internal/model/models.go` | `StockDailyKline` / `StockIntraday` |
| 前端图表 | `frontend/src/components/common/intraday-chart/intraday-chart.tsx` | 价/均价/量柱 + 乖离副图、量能门槛线、做 T 高抛低吸轨道带与 B/S、研判面板 |
| 前端信号 | `frontend/src/lib/intraday-signals.ts` | `computeIntradayMetrics`（逐点乖离/量比）+ `computeIntradaySignals`（均价线交叉趋势 BS） |
| 前端做 T | `frontend/src/lib/daytrade.ts` | 历史基准/当日趋势态/可做 T 判定/高抛低吸 B/S/分时成熟度 纯函数 |

---

## 三、K 线数据链路

### 3.1 接口与参数

```
GET /open/v1/stocks/{code}/kline?period=day&adjust=qfq&limit=120
GET /open/v1/stocks/{code}/kline?from=2025-01-01&to=2025-12-31   # 区间查询（回测用）
GET /open/v1/stocks/{code}/kline?indicators=ma5,macd_bar,kdj_k   # 带逐 bar 指标（绘图用）
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `period` | `day` | `day` / `week` / `month`（分钟级未实现） |
| `adjust` | `qfq` | `qfq`(前复权) / `hfq`(后复权) / `""`(不复权) |
| `limit` | `120` | 最近 N 根；与 `from`/`to` 互斥（区间优先） |
| `from` / `to` | — | 交易日区间（含端点）；传入后 `limit` 置 0 |
| `indicators` | — | 逗号分隔指标类型；传入则返回 `{bars, indicators}`（逐 bar 指标，与 bars 按 `trade_date` 对齐） |
| `market` | `CN` | 市场维度（当前仅 CN） |

Handler 解析逻辑（`market_handler.go`）：传入 `from` 或 `to` 时会把 `limit` 置 0，转为区间查询；传入 `indicators` 时调 `IndicatorService.KlineIndicators`（快照优先 + 实时补齐）附带逐 bar 指标，响应体由 bars 数组变为 `{bars, indicators}` 对象（不传则保持数组，向后兼容）。

> **逐 bar 指标策略**：日 K + 前复权且指标属默认快照集合 → 读 `stock_indicator_snapshots` 区间快照（含完整历史预热）；周/月 K、后复权、非默认指标（如 `ma120`）或快照缺口 → 在返回 bars 上逐 bar `bars[:i+1]` point-in-time 实时计算。前端 K 线图的 MA/BOLL 叠加与 MACD/KDJ/RSI/ATR/动量副图全部读此接口，**不再在前端手算**。

### 3.2 Service 读库优先逻辑

`KlineService.Kline`（`kline_service.go`）核心规则：

```go
// 仅 period == "day" 且接入了 klineRepo 时走数据库
if s.klineRepo != nil && q.Period == "day" {
    bars, err := s.klineRepo.List(ctx, marketCode, q.Code, adjust, q.From, q.To, q.Limit)
    if err == nil && len(bars) > 0 {
        return bars, nil          // 命中库内日 K，直接返回
    }
}
// 库空 / 周 K / 月 K → 回源数据源
bars, err := s.provider.Kline(ctx, q.Code, q.Period, adjust)
// 升序排序后，按 from/to 区间或 limit 过滤
```

要点：
- **只有日 K（`period=day`）走 PostgreSQL**；周 K / 月 K 总是实时回源 TDX（库内只存日 K）。
- 库内日 K 命中即返回，**不再访问 TDX**（这是日 K 与分时的本质差异）。
- 回源后在内存里升序排序，再做 `from`/`to` 区间过滤或 `limit` 截断（取末尾 N 根）。

### 3.3 Repository 查询

`KlineRepository.List`（`market_repo.go`）：
- **limit 模式**（无 from/to）：用子查询 `ORDER BY trade_date DESC LIMIT N` 取最近 N 根，再外层升序，避免全表扫描。
- **区间模式**：`WHERE trade_date BETWEEN from AND to ORDER BY trade_date ASC`。

`UpsertBatch`：按唯一键 `(market, stock_code, trade_date, adjust)` 做 `ON CONFLICT DO UPDATE`，分批 200 条写入。

### 3.4 gotdx 拉取与周期映射

`GotdxProvider.Kline`（`gotdx_provider.go`）：

```go
reply, err := c.GetSecurityBars(klineType, mkt, code, 0, 600)
```

- 调 gotdx `GetSecurityBars`，单次取 **600 根**（约 2.5 年日线）。
- **count 上限注意**：实测 `count=800` 会触发 TDX 返回异常短帧导致 gotdx 解析 panic，`<=600` 稳定。

周期映射 `klineTypeOf`（`gotdx_mapper.go`）：

| period | gotdx 类型常量 |
|--------|----------------|
| `""` / `day` | `KLINE_TYPE_RI_K` |
| `week` | `KLINE_TYPE_WEEKLY` |
| `month` | `KLINE_TYPE_MONTHLY` |
| 其他 | 报错 `unsupported kline period` |

字段映射：`Open/High/Low/Close` 经 `priceFromFloat` 转 decimal，`Vol` 经 `volumeFromFloat` 转 decimal；交易日取 `b.DateTime`，缺失时由 `Year/Month/Day` 组装。

### 3.5 增量落库（盘后定时）

库内日 K 由 `UpdateService.IncrementalOne`（`update_service.go`）写入，由调度器盘后触发（默认 17:00）：

```
读个股水位 wm(last_trade_date)
  → provider.Kline(code, "day", "qfq")    # 实时拉 TDX
  → filterAfterWatermark(bars, wm)         # 仅保留水位之后的新交易日
  → klineRepo.UpsertBatch(newBars)         # 有新数据才写
  → calRepo.UpsertInferred(dates)          # 用实际成交日反推校正交易日历
  → 推进 watermark 到 max(数据源最新, 全市场最新交易日)
```

水位机制的两个关键设计：
- **`PendingCodes` 预筛**：增量更新前，用「库内 K 线 `MAX(trade_date)`」为基准，仅挑「无水位（新股）或水位落后」的标的发起 TDX 请求，盘后多数已最新时请求量可从全市场约 5200 次降到个位/十位数。
- **水位对齐避免反复拉取**：即便本轮无新 K 线（停牌/次新股），也把水位推进到全市场最新交易日，否则这些股票会因永远落后而每轮被反复无效拉取。

> 复权口径：增量落库固定以 **`qfq`（前复权）** 存储，作为指标计算与全市场扫描的统一底座。

---

## 四、分时数据链路

### 4.1 接口

```
GET /open/v1/stocks/{code}/intraday?market=CN
```

无分页/周期参数。返回结构（`model.StockIntraday`）：

```json
{
  "market": "CN",
  "stock_code": "600519",
  "stock_name": "贵州茅台",
  "trade_date": "2026-06-05",
  "pre_close": "1680.0000",
  "points": [
    { "time": "2026-06-05T09:30:00+08:00", "price": "1690.30", "avg_price": "1689.50", "volume": "1234" }
  ]
}
```

- `pre_close`：昨收，作为分时图涨跌基准线。
- `trade_date`：**实际返回数据所属交易日**（非交易日回退时会是上一交易日）。
- `points`：逐交易分钟数据点，每点含 价格 / 均价 / 该分钟成交量。

### 4.2 Service 透传

`QuoteService.Intraday`（`quote_service.go`）：直接调 `provider.Intraday`，**不读写任何行情表**；仅查 `securities` 表补全 `stock_name`（gotdx 分时接口不带名称）。失败统一映射为 `ErrProvider` 错误码。

### 4.3 gotdx 接口（单次请求最多 4 个调用）

`GotdxProvider.Intraday`（`gotdx_provider.go`）：

| 调用 | gotdx 方法 | 底层协议 | 用途 |
|------|-----------|----------|------|
| ① | `GetSecurityBars`(日K, 2 根) | K 线协议 | 推断**最近交易日** |
| ② | `StockTickChart` | `GetTickChart` / `GetMinuteTimeData` | **当日分时图** |
| ③ | `StockHistoryTickChart` | `GetHistoryTickChart` | **历史分时**（②为空时回退） |
| ④ | `StockQuotesDetail` | 五档/盘口行情 | 取**昨收** `pre_close` |

gotdx 分时点原始结构（`proto.MinuteTimeData`）只有三个字段，**无时间戳**：

```go
type MinuteTimeData struct {
    Price float64  // 成交价
    Avg   float64  // 均价
    Vol   int      // 该分钟成交量
}
```

### 4.4 交易分钟推算

由于分时点无时间戳，`gotdx_mapper.go` 的 `intradayMinute` 按数组下标推算 A 股交易分钟：

- 上午：09:30–11:30，120 个点（下标 0–119）
- 下午：13:00–15:00，120 个点（下标 120–239）
- 合计 240 点，时区固定东八区（`cnLoc = FixedZone("CST", 8*3600)`，避免容器缺 tzdata）。

`mapHistoryIntradayPoints` 复用 `mapIntradayPoints`：历史分时结构与当日一致，仅类型不同，转换后复用同一映射逻辑（DRY）。

### 4.5 字段映射与增减

gotdx 分时返回是**点数组**（`[]MinuteTimeData`），每点仅 3 个字段、**无时间戳，也无任何元信息**（代码、日期、昨收均不带）。后端做了**字段重命名 + 类型转换（float → decimal 字符串）**，并在点级与外层**新增**若干字段。

**点级字段（`points[]`）**：

| gotdx `MinuteTimeData` | 后端 `IntradayPoint` | 处理 |
|------------------------|----------------------|------|
| _(无)_ | `time` | **新增**：按下标推算 `09:30 + i` 分钟，RFC3339（东八区） |
| `Price` | `price` | 重命名；`float64` → decimal（JSON 字符串） |
| `Avg` | `avg_price` | 重命名；`float64` → decimal |
| `Vol` | `volume` | 重命名；`int` → decimal |

> 点级**无删减**（原本就只有 3 个字段），也**没有**成交额 `amount`（gotdx 分时协议本身不返回）。

**外层包装字段（`StockIntraday`）**：

| 字段 | 来源 | 说明 |
|------|------|------|
| `market` | Provider 写死 `"CN"` | gotdx 分时无 |
| `stock_code` | 请求路径参数 | gotdx 分时无 |
| `stock_name` | **PostgreSQL `securities` 表** | Service 层补全，gotdx 分时不带名称 |
| `trade_date` | **日 K 最后一根日期** | Provider 推算，gotdx 分时无 |
| `pre_close` | **`StockQuotesDetail` 盘口接口** | Provider 另拉一次，gotdx 分时无 |
| `points` | 上述映射 | — |

**关键点**：价格 / 均价 / 成交量**原样透传**，后端不做二次聚合或口径重算；仅把数据包装成 REST 友好结构（snake_case + decimal 字符串），并补齐展示用的时间与元信息。前端 `TStockIntraday` / `TIntradayPoint` 与该 JSON 一一对应。

### 4.6 非交易日 / 盘前回退

```
latestTradeDate = 日K最后一根的日期（取不到则退回当前东八区日期）
当日分时 StockTickChart
  ├─ points 非空 → 使用当日分时
  └─ points 为空（非交易日 / 盘前）
        → StockHistoryTickChart(latestTradeDate) 历史分时
响应 trade_date = latestTradeDate
```

- 「最近交易日」来自 **gotdx 日 K 最后一根**，不查 `trading_calendars` 表。
- 前端个股详情页标题旁展示 `trade_date`，用户可见当前为哪一天的分时（如周末显示上一交易日）。
- 前端 `useStockIntraday` 每 **60 秒** 轮询一次。

### 4.7 为什么不落库

设计上**有意透传**（`StockIntraday` 是纯领域对象，无 GORM 表，`init.sql` 也无分时表）：
- 分时是分钟级、盘中频繁变化的展示数据，与日 K（盘后增量、批量算指标、回测底座）用途不同。
- 仅服务详情页展示，实时拉取实现最简单。

### 4.8 前端量柱着色口径（对齐同花顺）

分时图量柱红绿由**前端**计算（gotdx / 后端不提供颜色），位于 `intraday-chart.tsx`。当前口径为 **环比上一分钟**，与同花顺一致：

| 量柱 | 规则 |
|------|------|
| 第 2 根起 | 本分钟 `price` ≥ **上一分钟** `price` → 红，否则绿 |
| 第 1 根（无上一分钟） | 退回与**昨收** `pre_close` 比较 |

```ts
const prev = i > 0 ? toNumber(points[i - 1].price) : preClose;
color: toNumber(p.price) >= prev ? UP_VOLUME_COLOR : DOWN_VOLUME_COLOR;
```

**两种着色口径的含义区别**（历史上曾用「相对昨收」，现已改为「相对上一分钟」）：

| 口径 | 比较对象 | 颜色含义 | 读图视角 |
|------|----------|----------|----------|
| 相对昨收（旧） | `pre_close` | 当前处于昨收之上/之下 | 当日整体强弱；横盘在昨收上方时几乎全红 |
| **相对上一分钟（现状）** | 上一分钟 `price` | 这一分钟涨还是跌 | 盘中短线多空节奏，红绿交替，贴近同花顺/通达信 |

> 注意区分：**K 线图**量柱用的是另一套口径——当日 `close ≥ open`（这根 K 线阴阳），与分时的环比口径不同（见 `kline-chart.tsx`）。

> 口径差异提示：分时量柱仅改变**着色**，不改变数值。与同花顺逐分钟对比时若仍有差异，多来自数据源口径——① 第一根 09:30 通常含 **09:25 集合竞价**成交，量偏大；② TDX 与同花顺对「某一分钟归属哪个时间标签」存在约 1 分钟错位。这些属于数据源语义，非 bug。

### 4.9 前端分时买卖点（均价线交叉法，趋势向）

> 说明：分时图左上角有**模式切换**（「做 T（吸/抛）」/「趋势（B/S）」），默认 **做 T**（见 4.10）。本节的「均价线交叉趋势 BS」(`computeIntradaySignals`) 在「趋势」模式展示，并作为逐点指标基座保留（`computeIntradayMetrics` 被做 T 复用），单测 `intraday-signals.test.ts` 继续维护。切到「趋势」模式时高抛低吸轨道带会隐藏、面板切换为趋势信号明细。

分时买卖点由**前端**纯计算，不依赖后端新增字段。计算逻辑集中在 `frontend/src/lib/intraday-signals.ts` 的纯函数 `computeIntradaySignals`，并配有单测 `intraday-signals.test.ts`。

**判定主线**：价格线相对**均价线（VWAP）**的穿越。

| 事件 | 含义 | 标注 |
|------|------|------|
| 价格**上穿**均价（上一分钟在均价下方/持平，本分钟升到上方） | 多头转强 | 价格线下方红色向上箭头 **B（买）** |
| 价格**下穿**均价（上一分钟在均价上方/持平，本分钟落到下方） | 空头转强 | 价格线上方绿色向下箭头 **S（卖）** |

**三重过滤降噪**（默认值见 `DEFAULT_SIGNAL_PARAMS`）：

| 过滤项 | 默认阈值 | 作用 |
|--------|----------|------|
| 乖离 BIAS | `|price − avg| / avg ≥ 0.15%` | 过滤贴着均价反复擦边的无效穿越 |
| 量比 | `当根量 / 截至当前累计均量 ≥ 1.2` | 要求穿越时有量能配合，避免缩量假突破 |
| 冷却 | 距上一信号 `≥ 5 分钟`（分时 1 根=1 分钟，按下标差计） | 抑制同一时段反复抖动触发 |

> 首根（09:30）无上一分钟参照，不出信号；均价 ≤ 0 的异常点跳过。

**判定数据全部可视化**（图上看到的曲线即判定依据，逐点指标由 `computeIntradayMetrics` 统一计算，与信号判定同源）：

| 图层 | 位置 | 对应判定要素 |
|------|------|--------------|
| 价格线 / 均价线 | 主图 | **穿越**：价格线穿过均价线 |
| 乖离 BIAS 曲线 + `±0.15%` 阈值线 | 乖离副图（上红下绿，零轴=穿越点） | **乖离过滤**：曲线越过阈值带才算有效 |
| 量能门槛线 = `1.2 × 累计均量` | 量副图（叠加在量柱上，同刻度） | **量比过滤**：量柱高过门槛线即达标 |
| B/S 箭头 | 主图价格线 | 最终确认的买卖点 |

**前端展示**：
- 主图价格线上打 B/S 箭头。
- 三个 pane 各自左上角有浮动图例（通过 `IPaneApi.getHTMLElement()` 注入到 pane 绘图单元格）：主图「价/均价/涨跌」、量副图「量/门槛/量比」、乖离副图「乖离%/阈值」。图例既**标注每个副图的含义**，又随 hover **同步显示该分钟各自的指标值**（hover 任意位置，三个 pane 的图例一起更新；未 hover 时显示最新值）。
- 图表下方面板 `SignalPanel` 给出方案说明（阈值）、最新信号与全部信号明细（时间、买/卖、价格、相对昨收涨跌幅、乖离、量比）。

> 实现注记：lightweight-charts 用表格布局，`getHTMLElement()` 返回的是 `<tr>`，图例 `<div>` 必须挂到其中含 canvas 的绘图单元格（`<td>`）上；副图 canvas 异步挂载，故图例创建用 `requestAnimationFrame` 自愈重试直到三个 pane 都就绪。

> 定位：这是**轻量级日内辅助提示**，仅基于价/均价/量的形态，不含基本面与多周期确认，文案已标注「仅供参考」，不构成投资建议。阈值如需调整，传入 `computeIntradaySignals(points, { biasThreshold, volumeRatio, cooldownMinutes })` 覆盖默认值即可。

### 4.10 单日做 T 研判与高抛低吸 B/S

做 T（T+0 日内回转，持底仓低吸高抛、不丢筹码）与 4.9 的「趋势 BS」目标相反——做 T 是**逆势的高抛低吸**。计算集中在 `frontend/src/lib/daytrade.ts`（纯函数 + 单测 `daytrade.test.ts`），图表与研判面板在 `intraday-chart.tsx`。历史基准用**昨日及以前的日线前复权**（详情页固定 `useStockKline(code,"day","qfq")` 注入，独立于上方 K 线周期选择器），日内量基于今日分时实时计算。

四层闭环：

1. **历史基准** `computeDayTradeBaseline`（排除今日）：
   - `atrPct` = 近 20 日 `mean((high−low)/prevClose)`；`refPct = clamp(atrPct/2, 0.4%, 1.5%)` 作单侧轨道偏离；
   - `ma5/ma20`、近 20 日收盘高低 → 硬支撑/压力。
2. **当日趋势态** `computeDayTrend` → `TrendScore ∈ [-100,100]`，6 因子加权（跳空 15% / 价相对均价 20% / 均价线斜率 20% / 日内位置 15% / 量价配合 15% / 与历史均线 15%），映射 5 态：`strong_up / weak_up / range / weak_down / strong_down`。
3. **是否适合做 T** `computeDayTradePlan`：预期振幅 `expAmp = max(ATR%, 今日已实现振幅)`，`< 2.0%` → 不建议；否则按趋势态给模式：强多→倒 T、强空→正 T 抢反弹、偏多→正 T、偏空→倒 T、横盘→双向网格。
4. **高抛低吸 B/S** `computeDayTradeSignals`：以均价线 VWAP 为中枢建轨道 `upper = vwap×(1+kSell·refPct)`、`lower = vwap×(1−kBuy·refPct)`，`kBuy/kSell` 按趋势态自适应（偏精取向、整体拉宽：偏多 0.9/1.4、横盘 1.2/1.2、偏空 1.5/0.9、强多 0.7/1.2、强空 1.2/0.7）。
   - **低吸 吸**：触及下轨 + 拐头企稳(不再创新低) + 缩量承接(量比≤2) + 未破硬支撑；
   - **高抛 抛**：触及上轨 + 冲高滞涨(不再创新高)；
   - 冷却 30 分钟，按下标差计。

| 图层 | 位置 | 含义 |
|------|------|------|
| 价格线 / 均价线(VWAP) | 主图 | 中枢 |
| 高抛低吸轨道带（上轨红虚线/下轨绿虚线） | 主图，与价格同刻度 | 低吸/高抛触发轨道，宽度随趋势态自适应 |
| 吸↑(红) / 抛↓(绿) 箭头 | 主图价格线 | 做 T 买卖点 |
| 「今日做 T 研判」面板 | 图表下方 | 趋势态+评分、建议模式、预期振幅/历史 ATR、轨道宽度、信号明细 |
| 模式切换 | 主图左上角 | 「做 T（吸/抛）」/「趋势（B/S）」一键切换标注与下方面板 |
| 做 T 调参面板 | 研判面板下方 | 6 项参数滑块（振幅门槛/轨道宽度/信号冷却/低吸缩量上限/趋势斜率窗口/历史回看），每项带 info 提示，改动**实时**重算研判与轨道；可一键重置默认 |

**盘中可靠度**：分时是逐分钟拉取的，研判可靠度随已走过的交易时长上升。`computeSessionMaturity(points)` 按 A 股 09:30-11:30 / 13:00-15:00（共 240 分钟）折算已用交易分钟，分四档并在研判面板以徽标标注：

| 已用交易分钟 | 档位 | 含义 |
|------|------|------|
| < 30 | 早盘样本不足 | 趋势态会抖动、今日已实现振幅偏低，结论权重最低 |
| 30 ~ 120 | 上午渐明 | 趋势渐清 |
| 120 ~ 235 | 午后较稳 | 研判较可靠 |
| ≥ 235 | 全日完整 | 接近/已收盘 |

> 重要：方法本身**无未来函数**（逐点指标只用「当前及之前」数据），盘中实时算与收盘后算在同一时刻结论一致；历史 ATR 等基准来自昨日数据全程可靠。主要风险是早盘**样本不足**，故以可靠度徽标提示。唯一的轻微前视：轨道 k 值取自「当前最新趋势态」回贴历史 bar，仅影响收盘复盘画轨道、不影响盘中决策，**不可当回测引擎用**。

> 局限与风险：历史只到昨日，不能预知当日变盘，趋势态盘中会切换需实时重判；分时 60s 刷新，属分钟级确认；做 T 依赖真实底仓，本系统只给**信号与研判、不接交易**，文案标注「仅供参考」。参数可经 `DEFAULT_DAYTRADE_PARAMS` 覆盖（`baselineDays/minAmp/slopeWindow/volCalmMax/cooldownMinutes`）。

---

## 五、连接池 GotdxPool

两条链路的 TDX 请求都经 `GotdxPool`（`gotdx_pool.go`）借用复用连接：

- TDX 单次握手实测约 **1–4s**，连接池复用避免每请求重连。
- 容量 `maxConn`（默认 10，可由环境变量 `MARKET_GOTDX_MAX_CONN` 覆盖）。
- 借出/归还基于 token 配额，严格限制并发连接数 ≤ maxConn。
- 连接发生错误/panic 即丢弃重建（TDX 节点不稳定，坏连接续用会导致读写错位）。
- `Acquire` 支持 ctx 取消/超时，避免连接全借出时无限阻塞。

---

## 六、数据存储与缓存现状

| 数据 | PostgreSQL | Redis |
|------|-----------|-------|
| 日 K 线 | `stock_daily_klines`（前复权落库） | 无 |
| 周/月 K 线 | 不落库（实时回源） | 无 |
| 分时走势 | 不落库 | 无 |
| 个股快照 | `stock_quotes`（历史快照） | 30s 缓存 |
| 指数快照 | `index_quotes` | 5min 缓存 |

> 注意区分：`stock_quotes` 是个股**当日单点汇总快照**（现价/开高低收/总量），与分时的**逐分钟 240 点序列**不是同一张表、不是同一条链路。

---

## 七、扩展点（当前未实现）

- **分钟 K 线（1m/5m/...）**：`openapi.yaml` 开放 API 的 `period` 枚举声明了 `1m/5m/15m/30m/60m`，但后端 `klineTypeOf` 未映射，传入会报错；需补充 gotdx 分钟类型常量映射。
- **分时缓存 / 落库**：如需降低 TDX 压力或盘后加速，可加 Redis 短 TTL 缓存（30–60s），或盘后落 `stock_intraday_ticks` 表。
- **多数据源**：`factory.go` 的 `providerName` 已作多源扩展占位；`FallbackProvider` 支持主备降级链。
