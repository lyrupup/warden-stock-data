package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type KlineRepository struct{ db *gorm.DB }

func NewKlineRepository(db *gorm.DB) *KlineRepository { return &KlineRepository{db: db} }

func (r *KlineRepository) UpsertBatch(ctx context.Context, bars []model.StockDailyKline) error {
	if len(bars) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "market"}, {Name: "stock_code"}, {Name: "trade_date"}, {Name: "adjust"}},
		DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "amount", "source"}),
	}).CreateInBatches(bars, 200).Error
}

func (r *KlineRepository) List(ctx context.Context, market, code, adjust string, from, to *time.Time, limit int) ([]model.StockDailyKline, error) {
	q := r.db.WithContext(ctx).Where("market = ? AND stock_code = ? AND adjust = ?", market, code, adjust)
	if from != nil {
		q = q.Where("trade_date >= ?", from)
	}
	if to != nil {
		q = q.Where("trade_date <= ?", to)
	}
	q = q.Order("trade_date asc")
	if limit > 0 && from == nil && to == nil {
		var bars []model.StockDailyKline
		err := r.db.WithContext(ctx).Raw(`
			SELECT * FROM (
				SELECT * FROM stock_daily_klines
				WHERE market = ? AND stock_code = ? AND adjust = ?
				ORDER BY trade_date DESC LIMIT ?
			) t ORDER BY trade_date ASC`, market, code, adjust, limit).Scan(&bars).Error
		return bars, err
	}
	var bars []model.StockDailyKline
	err := q.Find(&bars).Error
	return bars, err
}

func (r *KlineRepository) LatestTradeDate(ctx context.Context, market string) (*time.Time, error) {
	var t sql.NullTime
	err := r.db.WithContext(ctx).Model(&model.StockDailyKline{}).
		Where("market = ?", market).Select("MAX(trade_date)").Scan(&t).Error
	if err != nil || !t.Valid {
		return nil, err
	}
	return &t.Time, nil
}

// EnsureIndexes 幂等创建 K 线表的「市场 + 交易日」复合索引。
// 使 MAX(trade_date)/最新交易日筛选走索引而非在数百万行上全表扫描（init.sql 仅
// 首次初始化执行，存量库启动时调用以补齐）。
func (r *KlineRepository) EnsureIndexes(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec(
		"CREATE INDEX IF NOT EXISTS idx_klines_market_trade_date ON stock_daily_klines (market, trade_date)",
	).Error
}

type QuoteRepository struct{ db *gorm.DB }

func NewQuoteRepository(db *gorm.DB) *QuoteRepository { return &QuoteRepository{db: db} }

func (r *QuoteRepository) Upsert(ctx context.Context, q *model.StockQuote) error {
	return r.db.WithContext(ctx).Create(q).Error
}

func (r *QuoteRepository) LatestByCodes(ctx context.Context, market string, codes []string) ([]model.StockQuote, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var quotes []model.StockQuote
	sub := r.db.WithContext(ctx).Model(&model.StockQuote{}).
		Select("stock_code, MAX(snapshot_at) as snapshot_at").
		Where("market = ? AND stock_code IN ?", market, codes).
		Group("stock_code")
	err := r.db.WithContext(ctx).Table("stock_quotes sq").
		Joins("INNER JOIN (?) latest ON sq.stock_code = latest.stock_code AND sq.snapshot_at = latest.snapshot_at", sub).
		Where("sq.market = ?", market).
		Find(&quotes).Error
	return quotes, err
}

func (r *QuoteRepository) SaveIndexQuotes(ctx context.Context, quotes []model.IndexQuote) error {
	if len(quotes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&quotes).Error
}

// LatestIndexQuotes 返回各指数最新一条快照，用于 Redis 未命中时快速回源。
func (r *QuoteRepository) LatestIndexQuotes(ctx context.Context, market string) ([]model.IndexQuote, error) {
	sub := r.db.WithContext(ctx).Model(&model.IndexQuote{}).
		Select("index_code, MAX(snapshot_at) as snapshot_at").
		Where("market = ?", market).
		Group("index_code")
	var quotes []model.IndexQuote
	err := r.db.WithContext(ctx).Table("index_quotes iq").
		Joins("INNER JOIN (?) latest ON iq.index_code = latest.index_code AND iq.snapshot_at = latest.snapshot_at", sub).
		Where("iq.market = ?", market).
		Find(&quotes).Error
	return quotes, err
}

type IndicatorRepository struct{ db *gorm.DB }

func NewIndicatorRepository(db *gorm.DB) *IndicatorRepository { return &IndicatorRepository{db: db} }

func (r *IndicatorRepository) UpsertSnapshot(ctx context.Context, market, code string, tradeDate time.Time, values map[string]decimal.Decimal) error {
	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	snap := model.StockIndicatorSnapshot{
		Market: market, StockCode: code, TradeDate: tradeDate,
		Values: datatypes.JSON(b), UpdatedAt: time.Now(),
	}
	// 冲突时按 JSONB 合并（已有 || 新值，同键以新值为准）而非整行覆盖：
	// 这样用更小的 types 子集再次扫描不会抹掉该日已落库的其它指标，新增指标也能增量补字段。
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "market"}, {Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"values":     gorm.Expr(`COALESCE(stock_indicator_snapshots."values", '{}'::jsonb) || excluded."values"`),
			"updated_at": time.Now(),
		}),
	}).Create(&snap).Error
}

func (r *IndicatorRepository) GetSnapshots(ctx context.Context, market string, codes []string, tradeDate *time.Time) ([]model.StockIndicatorSnapshot, error) {
	q := r.db.WithContext(ctx).Where("market = ? AND stock_code IN ?", market, codes)
	if tradeDate != nil {
		q = q.Where("trade_date = ?", tradeDate)
	} else {
		q = q.Where("trade_date = (SELECT MAX(trade_date) FROM stock_indicator_snapshots WHERE market = ?)", market)
	}
	var snaps []model.StockIndicatorSnapshot
	err := q.Find(&snaps).Error
	return snaps, err
}

// GetSnapshotsRange 取单只标的在 [from, to] 区间内逐交易日的指标快照（升序），
// 供 K 线接口按 bar 对齐附带 point-in-time 指标。
func (r *IndicatorRepository) GetSnapshotsRange(ctx context.Context, market, code string, from, to time.Time) ([]model.StockIndicatorSnapshot, error) {
	var snaps []model.StockIndicatorSnapshot
	err := r.db.WithContext(ctx).
		Where("market = ? AND stock_code = ? AND trade_date BETWEEN ? AND ?", market, code, from, to).
		Order("trade_date asc").
		Find(&snaps).Error
	return snaps, err
}

func (r *IndicatorRepository) LatestTradeDate(ctx context.Context, market string) (*time.Time, error) {
	var t sql.NullTime
	err := r.db.WithContext(ctx).Model(&model.StockIndicatorSnapshot{}).
		Where("market = ?", market).Select("MAX(trade_date)").Scan(&t).Error
	if err != nil || !t.Valid {
		return nil, err
	}
	return &t.Time, nil
}

// EarliestTradeDate 返回指标快照最早交易日，供运维查看快照历史覆盖区间。
func (r *IndicatorRepository) EarliestTradeDate(ctx context.Context, market string) (*time.Time, error) {
	var t sql.NullTime
	err := r.db.WithContext(ctx).Model(&model.StockIndicatorSnapshot{}).
		Where("market = ?", market).Select("MIN(trade_date)").Scan(&t).Error
	if err != nil || !t.Valid {
		return nil, err
	}
	return &t.Time, nil
}

// SnapshotStockCountAt 返回某交易日有指标快照的股票数（唯一键 market+code+date，行数即股数），
// 配合证券总数衡量「最新交易日指标快照的覆盖完整性」。
func (r *IndicatorRepository) SnapshotStockCountAt(ctx context.Context, market string, date time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.StockIndicatorSnapshot{}).
		Where("market = ? AND trade_date = ?", market, date).Count(&n).Error
	return n, err
}

type SecurityRepository struct{ db *gorm.DB }

func NewSecurityRepository(db *gorm.DB) *SecurityRepository { return &SecurityRepository{db: db} }

func (r *SecurityRepository) UpsertBatch(ctx context.Context, secs []model.Security) error {
	if len(secs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "market"}, {Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "board", "status", "updated_at"}),
	}).CreateInBatches(secs, 500).Error
}

// NamesByCodes 批量查询证券名称，供行情接口补全 stock_name。
func (r *SecurityRepository) NamesByCodes(ctx context.Context, market string, codes []string) (map[string]string, error) {
	if len(codes) == 0 {
		return map[string]string{}, nil
	}
	var secs []model.Security
	if err := r.db.WithContext(ctx).
		Select("code", "name").
		Where("market = ? AND code IN ?", market, codes).
		Find(&secs).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(secs))
	for _, s := range secs {
		if s.Name != "" {
			out[s.Code] = s.Name
		}
	}
	return out, nil
}

func (r *SecurityRepository) List(ctx context.Context, market string) ([]model.Security, error) {
	var list []model.Security
	err := r.db.WithContext(ctx).Where("market = ? AND status = 1", market).Order("code asc").Find(&list).Error
	return list, err
}

func (r *SecurityRepository) Count(ctx context.Context, market string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Security{}).
		Where("market = ? AND status = 1", market).Count(&count).Error
	return count, err
}

func (r *SecurityRepository) Search(ctx context.Context, market, kw string) ([]model.Security, error) {
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return nil, nil
	}
	// 转义 LIKE 通配符，避免用户输入的 % / _ 被当作模式匹配。
	pattern := "%" + likeEscape(kw) + "%"
	var list []model.Security
	err := r.db.WithContext(ctx).
		Where(`market = ? AND status = 1 AND (code ILIKE ? ESCAPE '\' OR name ILIKE ? ESCAPE '\')`, market, pattern, pattern).
		Order("code asc").Limit(50).Find(&list).Error
	if err != nil {
		return nil, err
	}
	// 命中优先级：精确代码 > 代码前缀 > 代码包含 > 仅名称命中。
	rankSecurities(list, strings.ToLower(kw))
	return list, nil
}

// EnsureSearchIndexes 幂等创建证券搜索所需的 pg_trgm 扩展与 GIN 索引，
// 使 code/name 的 ILIKE '%kw%' 模糊搜索走索引而非全表扫描。
// 针对已有库（init.sql 仅首次初始化时执行），启动时调用以补齐索引。
func (r *SecurityRepository) EnsureSearchIndexes(ctx context.Context) error {
	stmts := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE INDEX IF NOT EXISTS idx_securities_code_trgm ON securities USING gin (code gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_securities_name_trgm ON securities USING gin (name gin_trgm_ops)",
	}
	for _, s := range stmts {
		if err := r.db.WithContext(ctx).Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func rankSecurities(list []model.Security, kwLower string) {
	sort.SliceStable(list, func(i, j int) bool {
		return secRank(list[i], kwLower) < secRank(list[j], kwLower)
	})
}

func secRank(s model.Security, kwLower string) int {
	code := strings.ToLower(s.Code)
	switch {
	case code == kwLower:
		return 0
	case strings.HasPrefix(code, kwLower):
		return 1
	case strings.Contains(code, kwLower):
		return 2
	default:
		return 3
	}
}

type CalendarRepository struct{ db *gorm.DB }

func NewCalendarRepository(db *gorm.DB) *CalendarRepository { return &CalendarRepository{db: db} }

func (r *CalendarRepository) IsTradingDay(ctx context.Context, market string, date time.Time) (bool, error) {
	d := date.Truncate(24 * time.Hour)
	var cal model.TradingCalendar
	err := r.db.WithContext(ctx).Where("market = ? AND cal_date = ?", market, d).First(&cal).Error
	if err == gorm.ErrRecordNotFound {
		weekday := d.Weekday()
		return weekday != time.Saturday && weekday != time.Sunday, nil
	}
	if err != nil {
		return false, err
	}
	return cal.IsOpen, nil
}

func (r *CalendarRepository) UpsertInferred(ctx context.Context, market string, dates []time.Time) error {
	for _, d := range dates {
		cal := model.TradingCalendar{Market: market, CalDate: d.Truncate(24 * time.Hour), IsOpen: true, Source: "inferred"}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "market"}, {Name: "cal_date"}},
			DoUpdates: clause.AssignmentColumns([]string{"is_open", "source"}),
		}).Create(&cal).Error; err != nil {
			return err
		}
	}
	return nil
}

type WatermarkRepository struct{ db *gorm.DB }

func NewWatermarkRepository(db *gorm.DB) *WatermarkRepository { return &WatermarkRepository{db: db} }

func (r *WatermarkRepository) Get(ctx context.Context, market, code string) (*model.UpdateWatermark, error) {
	var wm model.UpdateWatermark
	err := r.db.WithContext(ctx).Where("market = ? AND stock_code = ?", market, code).First(&wm).Error
	if err == gorm.ErrRecordNotFound {
		return &model.UpdateWatermark{Market: market, StockCode: code}, nil
	}
	return &wm, err
}

// MapByMarket 一次性读取某市场全部股票水位，返回 code -> last_trade_date。
// 用于增量更新前批量判定哪些标的落后于全市场最新交易日，避免逐只查询。
func (r *WatermarkRepository) MapByMarket(ctx context.Context, market string) (map[string]time.Time, error) {
	var wms []model.UpdateWatermark
	if err := r.db.WithContext(ctx).Where("market = ?", market).Find(&wms).Error; err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(wms))
	for _, w := range wms {
		if w.LastTradeDate != nil {
			out[w.StockCode] = *w.LastTradeDate
		}
	}
	return out, nil
}

// CountByMarket 统计某市场已建立更新水位的股票数量。水位在 K 线增量/回补成功后
// 按 (market, code) 落一行，故其行数即「已入库行情数据的股票数」，且每股一行、
// 查询走唯一索引，远快于在 stock_daily_klines 数百万行上做 COUNT(DISTINCT)。
func (r *WatermarkRepository) CountByMarket(ctx context.Context, market string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UpdateWatermark{}).
		Where("market = ?", market).Count(&count).Error
	return count, err
}

func (r *WatermarkRepository) Upsert(ctx context.Context, market, code string, last time.Time) error {
	wm := model.UpdateWatermark{Market: market, StockCode: code, LastTradeDate: &last, UpdatedAt: time.Now()}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "market"}, {Name: "stock_code"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_trade_date", "updated_at"}),
	}).Create(&wm).Error
}

type AccessLogRepository struct{ db *gorm.DB }

func NewAccessLogRepository(db *gorm.DB) *AccessLogRepository { return &AccessLogRepository{db: db} }

func (r *AccessLogRepository) Incr(ctx context.Context, credentialID uint, isError bool) error {
	today := time.Now().Truncate(24 * time.Hour)
	now := time.Now()
	log := model.CredentialAccessLog{CredentialID: credentialID, StatDate: today, CallCount: 1, LastAccessAt: &now}
	if isError {
		log.ErrorCount = 1
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "credential_id"}, {Name: "stat_date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"call_count":     gorm.Expr("credential_access_logs.call_count + 1"),
			"error_count":    gorm.Expr("credential_access_logs.error_count + ?", boolToInt(isError)),
			"last_access_at": now,
		}),
	}).Create(&log).Error
}

func (r *AccessLogRepository) ListByCredential(ctx context.Context, credentialID uint, days int) ([]model.CredentialAccessLog, error) {
	since := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	var logs []model.CredentialAccessLog
	err := r.db.WithContext(ctx).Where("credential_id = ? AND stat_date >= ?", credentialID, since).
		Order("stat_date desc").Find(&logs).Error
	return logs, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
