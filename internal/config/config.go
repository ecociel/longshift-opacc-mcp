package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAddr      = ":8080"
	DefaultTransport = "http"
	DefaultScale     = time.Hour
	DefaultWindow    = 90 * 24 * time.Hour
	DefaultSeed      = int64(1)
	DefaultTick      = time.Second
)

type Config struct {
	Addr      string
	Transport string
	TimeScale time.Duration
	Window    time.Duration
	Seed      int64
	Tick      time.Duration
	Now       time.Time
}

func FromEnv() (Config, error) {
	cfg := Config{
		Addr:      envOr("OPACC_MCP_ADDR", DefaultAddr),
		Transport: strings.ToLower(envOr("OPACC_MCP_TRANSPORT", DefaultTransport)),
		TimeScale: DefaultScale,
		Window:    DefaultWindow,
		Seed:      DefaultSeed,
		Tick:      DefaultTick,
	}

	var err error
	if raw := os.Getenv("OPACC_MCP_TIME_SCALE"); raw != "" {
		cfg.TimeScale, err = parseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("OPACC_MCP_TIME_SCALE: %w", err)
		}
		if cfg.TimeScale <= 0 {
			return Config{}, fmt.Errorf("OPACC_MCP_TIME_SCALE must be > 0")
		}
	}
	if raw := os.Getenv("OPACC_MCP_WINDOW"); raw != "" {
		cfg.Window, err = parseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("OPACC_MCP_WINDOW: %w", err)
		}
		if cfg.Window <= 0 {
			return Config{}, fmt.Errorf("OPACC_MCP_WINDOW must be > 0")
		}
	}
	if raw := os.Getenv("OPACC_MCP_SEED"); raw != "" {
		cfg.Seed, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("OPACC_MCP_SEED: %w", err)
		}
	}
	if raw := os.Getenv("OPACC_MCP_TICK"); raw != "" {
		cfg.Tick, err = time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("OPACC_MCP_TICK: %w", err)
		}
		if cfg.Tick <= 0 {
			return Config{}, fmt.Errorf("OPACC_MCP_TICK must be > 0")
		}
	}
	if raw := strings.TrimSpace(os.Getenv("OPACC_MCP_NOW")); raw != "" {
		cfg.Now, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return Config{}, fmt.Errorf("OPACC_MCP_NOW must be RFC3339: %w", err)
		}
	}

	switch cfg.Transport {
	case "http", "stdio":
	default:
		return Config{}, fmt.Errorf("OPACC_MCP_TRANSPORT must be http or stdio, got %q", cfg.Transport)
	}
	return cfg, nil
}

func parseDuration(raw string) (time.Duration, error) {
	s := strings.TrimSpace(raw)
	if n, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, fmt.Errorf("days: %w", err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
