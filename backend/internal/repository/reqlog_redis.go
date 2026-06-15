package repository

import (
	"crypto/tls"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

type ReqLogRedisClient struct {
	Client    *redis.Client
	dedicated bool
}

func ProvideReqLogRedis(cfg *config.Config, main *redis.Client) *ReqLogRedisClient {
	if cfg == nil || main == nil || !cfg.Ops.RequestLog.Redis.UseDedicated {
		return &ReqLogRedisClient{Client: main}
	}
	r := cfg.Ops.RequestLog.Redis
	merged := config.RedisConfig{
		Host:                fallbackString(r.Host, cfg.Redis.Host),
		Port:                fallbackInt(r.Port, cfg.Redis.Port),
		Password:            fallbackString(r.Password, cfg.Redis.Password),
		DB:                  r.DB,
		DialTimeoutSeconds:  fallbackInt(r.DialTimeoutSeconds, cfg.Redis.DialTimeoutSeconds),
		ReadTimeoutSeconds:  fallbackInt(r.ReadTimeoutSeconds, cfg.Redis.ReadTimeoutSeconds),
		WriteTimeoutSeconds: fallbackInt(r.WriteTimeoutSeconds, cfg.Redis.WriteTimeoutSeconds),
		PoolSize:            fallbackInt(r.PoolSize, cfg.Redis.PoolSize),
		MinIdleConns:        fallbackInt(r.MinIdleConns, cfg.Redis.MinIdleConns),
		EnableTLS:           r.EnableTLS,
	}
	opts := buildRedisOptionsFromRedisConfig(merged)
	if merged.EnableTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: merged.Host}
	}
	return &ReqLogRedisClient{Client: redis.NewClient(opts), dedicated: true}
}

func (c *ReqLogRedisClient) Close() error {
	if c == nil || c.Client == nil || !c.dedicated {
		return nil
	}
	return c.Client.Close()
}

func (c *ReqLogRedisClient) String() string {
	if c == nil || c.Client == nil {
		return "reqlog redis <nil>"
	}
	mode := "shared"
	if c.dedicated {
		mode = "dedicated"
	}
	return fmt.Sprintf("reqlog redis %s", mode)
}

func fallbackString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func fallbackInt(v, fallback int) int {
	if v != 0 {
		return v
	}
	return fallback
}
