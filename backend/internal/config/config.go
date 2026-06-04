package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppPort int
	AppEnv  string

	JWTSecret   string
	JWTExpire   time.Duration
	EncKey      string
	SignSkewSec int
	NonceTTL    time.Duration

	PGHost     string
	PGPort     int
	PGUser     string
	PGPassword string
	PGDB       string
	PGSSLMode  string

	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int

	MarketProvider string
	RequestTimeout time.Duration
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")
	_ = v.ReadInConfig()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("JWT_SECRET", "change_me_dev_only")
	v.SetDefault("JWT_EXPIRE_HOURS", 24)
	v.SetDefault("CONFIG_ENC_KEY", "32_bytes_key_for_aes_gcm________")
	v.SetDefault("SIGN_TS_SKEW_SEC", 300)
	v.SetDefault("SIGN_NONCE_TTL_SEC", 300)
	v.SetDefault("PG_HOST", "localhost")
	v.SetDefault("PG_PORT", 5432)
	v.SetDefault("PG_USER", "postgres")
	v.SetDefault("PG_PASSWORD", "postgres")
	v.SetDefault("PG_DB", "warden_data")
	v.SetDefault("PG_SSLMODE", "disable")
	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("MARKET_PROVIDER", "stub")
	v.SetDefault("REQUEST_TIMEOUT_SEC", 30)

	cfg := &Config{
		AppPort:        v.GetInt("APP_PORT"),
		AppEnv:         v.GetString("APP_ENV"),
		JWTSecret:      v.GetString("JWT_SECRET"),
		JWTExpire:      time.Duration(v.GetInt("JWT_EXPIRE_HOURS")) * time.Hour,
		EncKey:         v.GetString("CONFIG_ENC_KEY"),
		SignSkewSec:    v.GetInt("SIGN_TS_SKEW_SEC"),
		NonceTTL:       time.Duration(v.GetInt("SIGN_NONCE_TTL_SEC")) * time.Second,
		PGHost:         v.GetString("PG_HOST"),
		PGPort:         v.GetInt("PG_PORT"),
		PGUser:         v.GetString("PG_USER"),
		PGPassword:     v.GetString("PG_PASSWORD"),
		PGDB:           v.GetString("PG_DB"),
		PGSSLMode:      v.GetString("PG_SSLMODE"),
		RedisHost:      v.GetString("REDIS_HOST"),
		RedisPort:      v.GetInt("REDIS_PORT"),
		RedisPassword:  v.GetString("REDIS_PASSWORD"),
		RedisDB:        v.GetInt("REDIS_DB"),
		MarketProvider: v.GetString("MARKET_PROVIDER"),
		RequestTimeout: time.Duration(v.GetInt("REQUEST_TIMEOUT_SEC")) * time.Second,
	}
	return cfg, nil
}
