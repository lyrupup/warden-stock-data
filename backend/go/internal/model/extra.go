package model

import "time"

type TradingCalendar struct {
	ID      uint      `gorm:"primarykey" json:"id"`
	Market  string    `gorm:"size:8;not null;default:CN" json:"market"`
	CalDate time.Time `gorm:"type:date;not null;uniqueIndex:uni_trading_calendars_market_date" json:"cal_date"`
	IsOpen  bool      `gorm:"not null;default:false" json:"is_open"`
	Source  string    `gorm:"size:16;not null;default:manual" json:"source"`
}

func (TradingCalendar) TableName() string { return "trading_calendars" }

type UpdateWatermark struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	Market        string     `gorm:"size:8;not null;default:CN" json:"market"`
	StockCode     string     `gorm:"size:16;not null;uniqueIndex:uni_watermark_market_code" json:"stock_code"`
	LastTradeDate *time.Time `gorm:"type:date" json:"last_trade_date"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (UpdateWatermark) TableName() string { return "update_watermarks" }

type CredentialAccessLog struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	CredentialID uint       `gorm:"not null;uniqueIndex:uni_access_log_cred_date" json:"credential_id"`
	StatDate     time.Time  `gorm:"type:date;not null;uniqueIndex:uni_access_log_cred_date" json:"stat_date"`
	CallCount    int64      `gorm:"not null;default:0" json:"call_count"`
	ErrorCount   int64      `gorm:"not null;default:0" json:"error_count"`
	LastAccessAt *time.Time `json:"last_access_at"`
}

func (CredentialAccessLog) TableName() string { return "credential_access_logs" }
