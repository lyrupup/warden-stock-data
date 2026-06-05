package model

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type Admin struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Username     string    `gorm:"size:64;not null;uniqueIndex:uni_admins_username" json:"username"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	Role         string    `gorm:"size:16;not null;default:admin" json:"role"`
	Status       int16     `gorm:"not null;default:1" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Admin) TableName() string { return "admins" }

type APICredential struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	SecretID        string     `gorm:"size:40;not null;uniqueIndex:uni_api_credentials_secret_id" json:"secret_id"`
	SecretKeyCipher string     `gorm:"type:text;not null" json:"-"`
	SecretKeyHash   string     `gorm:"size:128;not null" json:"-"`
	ConsumerName    string     `gorm:"size:128;not null" json:"consumer_name"`
	Scope           string     `gorm:"size:32;not null;default:read" json:"scope"`
	RateLimit       int        `gorm:"not null;default:20" json:"rate_limit"`
	DailyQuota      int        `gorm:"not null;default:100000" json:"daily_quota"`
	Status          int16      `gorm:"not null;default:1" json:"status"`
	ExpireAt        *time.Time `json:"expire_at,omitempty"`
	CreatedBy       uint       `gorm:"not null" json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (APICredential) TableName() string { return "api_credentials" }

type DataSource struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Source    string         `gorm:"size:32;not null" json:"source"`
	Market    string         `gorm:"size:8;not null;default:CN" json:"market"`
	Name      string         `gorm:"size:64;not null" json:"name"`
	Enabled   bool           `gorm:"not null;default:true" json:"enabled"`
	Priority  int            `gorm:"not null;default:0" json:"priority"`
	Config    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"config"`
	Health    string         `gorm:"size:16;not null;default:unknown" json:"health"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (DataSource) TableName() string { return "data_sources" }

type Security struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Market    string    `gorm:"size:8;not null;default:CN" json:"market"`
	Code      string    `gorm:"size:16;not null" json:"code"`
	Name      string    `gorm:"size:64;not null;default:''" json:"name"`
	Board     string    `gorm:"size:16;not null;default:''" json:"board"`
	Status    int16     `gorm:"not null;default:1" json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Security) TableName() string { return "securities" }

type IndexQuote struct {
	ID            uint            `gorm:"primarykey" json:"id"`
	Market        string          `gorm:"size:8;not null;default:CN" json:"market"`
	IndexCode     string          `gorm:"size:16;not null" json:"index_code"`
	IndexName     string          `gorm:"size:64;not null" json:"index_name"`
	Price         decimal.Decimal `gorm:"type:numeric(20,4)" json:"price"`
	ChangeAmount  decimal.Decimal `gorm:"type:numeric(20,4)" json:"change_amount"`
	ChangePercent decimal.Decimal `gorm:"type:numeric(10,4)" json:"change_percent"`
	Volume        decimal.Decimal `gorm:"type:numeric(20,0)" json:"volume"`
	Amount        decimal.Decimal `gorm:"type:numeric(24,4)" json:"amount"`
	TradeDate     time.Time       `gorm:"type:date;not null" json:"trade_date"`
	SnapshotAt    time.Time       `json:"snapshot_at"`
	Stale         bool            `gorm:"-" json:"stale,omitempty"`
}

func (IndexQuote) TableName() string { return "index_quotes" }

type StockQuote struct {
	ID            uint            `gorm:"primarykey" json:"id"`
	Market        string          `gorm:"size:8;not null;default:CN" json:"market"`
	StockCode     string          `gorm:"size:16;not null" json:"stock_code"`
	StockName     string          `gorm:"size:64;not null;default:''" json:"stock_name"`
	Price         decimal.Decimal `gorm:"type:numeric(20,4)" json:"price"`
	Open          decimal.Decimal `gorm:"type:numeric(20,4)" json:"open"`
	High          decimal.Decimal `gorm:"type:numeric(20,4)" json:"high"`
	Low           decimal.Decimal `gorm:"type:numeric(20,4)" json:"low"`
	PrevClose     decimal.Decimal `gorm:"type:numeric(20,4)" json:"prev_close"`
	ChangePercent decimal.Decimal `gorm:"type:numeric(10,4)" json:"change_percent"`
	Volume        decimal.Decimal `gorm:"type:numeric(20,0)" json:"volume"`
	Amount        decimal.Decimal `gorm:"type:numeric(24,4)" json:"amount"`
	TurnoverRate  decimal.Decimal `gorm:"type:numeric(10,4)" json:"turnover_rate"`
	TradeDate     time.Time       `gorm:"type:date;not null" json:"trade_date"`
	SnapshotAt    time.Time       `json:"snapshot_at"`
	Stale         bool            `gorm:"-" json:"stale,omitempty"`
}

func (StockQuote) TableName() string { return "stock_quotes" }

type StockDailyKline struct {
	ID        uint            `gorm:"primarykey" json:"id"`
	Market    string          `gorm:"size:8;not null;default:CN" json:"market"`
	Source    string          `gorm:"size:32;not null;default:gotdx" json:"source"`
	StockCode string          `gorm:"size:16;not null;index:idx_klines_code_date" json:"stock_code"`
	TradeDate time.Time       `gorm:"type:date;not null;index:idx_klines_code_date" json:"trade_date"`
	Open      decimal.Decimal `gorm:"type:numeric(20,4)" json:"open"`
	High      decimal.Decimal `gorm:"type:numeric(20,4)" json:"high"`
	Low       decimal.Decimal `gorm:"type:numeric(20,4)" json:"low"`
	Close     decimal.Decimal `gorm:"type:numeric(20,4)" json:"close"`
	Volume    decimal.Decimal `gorm:"type:numeric(20,0)" json:"volume"`
	Amount    decimal.Decimal `gorm:"type:numeric(24,4)" json:"amount"`
	Adjust    string          `gorm:"size:8;not null;default:qfq" json:"adjust"`
	CreatedAt time.Time       `json:"created_at"`
}

func (StockDailyKline) TableName() string { return "stock_daily_klines" }

// StockIntraday 当日分时走势（实时透传，不落库）。
// 数据源为 gotdx 分时图（每个交易分钟一条），用于前端分时价格线 + 均价线 + 分时量。
type StockIntraday struct {
	Market    string          `json:"market"`
	StockCode string          `json:"stock_code"`
	StockName string          `json:"stock_name,omitempty"`
	TradeDate string          `json:"trade_date"`         // 交易日 YYYY-MM-DD
	PreClose  decimal.Decimal `json:"pre_close"`          // 昨收，作为分时图涨跌基准线
	Points    []IntradayPoint `json:"points"`
}

// IntradayPoint 单个交易分钟的分时数据点。Volume 为该分钟的成交量（非累计）。
type IntradayPoint struct {
	Time     string          `json:"time"`      // RFC3339（Asia/Shanghai）分钟时间点
	Price    decimal.Decimal `json:"price"`     // 当前价
	AvgPrice decimal.Decimal `json:"avg_price"` // 当日均价
	Volume   decimal.Decimal `json:"volume"`    // 该分钟成交量
}

type StockIndicatorSnapshot struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Market    string         `gorm:"size:8;not null;default:CN" json:"market"`
	StockCode string         `gorm:"size:16;not null;index:idx_indi_snap_code_date" json:"stock_code"`
	TradeDate time.Time      `gorm:"type:date;not null;index:idx_indi_snap_code_date" json:"trade_date"`
	Values    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"values"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (StockIndicatorSnapshot) TableName() string { return "stock_indicator_snapshots" }

type UpdateJob struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	JobType     string    `gorm:"size:16;not null" json:"job_type"`
	Market      string    `gorm:"size:8;not null;default:CN" json:"market"`
	CronExpr    string    `gorm:"size:64;not null;default:'0 0 17 * * *'" json:"cron_expr"`
	BatchSize   int       `gorm:"not null;default:20" json:"batch_size"`
	Concurrency int       `gorm:"not null;default:10" json:"concurrency"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UpdateJob) TableName() string { return "update_jobs" }

type UpdateJobRun struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	JobID      uint       `gorm:"not null;index:idx_job_runs_job" json:"job_id"`
	JobType    string     `gorm:"size:16;not null;default:''" json:"job_type"`
	Market     string     `gorm:"size:8;not null;default:CN" json:"market"`
	Status     string     `gorm:"size:16;not null;default:running" json:"status"`
	Total      int        `gorm:"not null;default:0" json:"total"`
	Processed  int        `gorm:"not null;default:0" json:"processed"`
	Succeeded  int        `gorm:"not null;default:0" json:"succeeded"`
	Failed     int        `gorm:"not null;default:0" json:"failed"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ErrorMsg   string     `gorm:"type:text;not null;default:''" json:"error_msg"`
}

func (UpdateJobRun) TableName() string { return "update_job_runs" }

var MigrateModels = []interface{}{
	&Admin{},
	&APICredential{},
	&DataSource{},
	&Security{},
	&TradingCalendar{},
	&IndexQuote{},
	&StockQuote{},
	&StockDailyKline{},
	&StockIndicatorSnapshot{},
	&UpdateWatermark{},
	&UpdateJob{},
	&UpdateJobRun{},
	&CredentialAccessLog{},
}
