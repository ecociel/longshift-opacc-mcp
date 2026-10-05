package config

import (
	"testing"
	"time"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("OPACC_MCP_ADDR", "")
	t.Setenv("OPACC_MCP_TRANSPORT", "")
	t.Setenv("OPACC_MCP_TIME_SCALE", "")
	t.Setenv("OPACC_MCP_WINDOW", "")
	t.Setenv("OPACC_MCP_SEED", "")
	t.Setenv("OPACC_MCP_TICK", "")
	t.Setenv("OPACC_MCP_NOW", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != DefaultAddr || cfg.Transport != "http" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.TimeScale != DefaultScale || cfg.Window != DefaultWindow || cfg.Seed != DefaultSeed {
		t.Fatalf("%+v", cfg)
	}
	if !cfg.Now.IsZero() {
		t.Fatalf("now should be unset: %v", cfg.Now)
	}
}

func TestFromEnvOverridesAndValidation(t *testing.T) {
	t.Setenv("OPACC_MCP_ADDR", "127.0.0.1:9")
	t.Setenv("OPACC_MCP_TRANSPORT", "stdio")
	t.Setenv("OPACC_MCP_TIME_SCALE", "24h")
	t.Setenv("OPACC_MCP_WINDOW", "60d")
	t.Setenv("OPACC_MCP_SEED", "42")
	t.Setenv("OPACC_MCP_TICK", "250ms")
	t.Setenv("OPACC_MCP_NOW", "2026-07-01T08:00:00Z")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9" || cfg.Transport != "stdio" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.TimeScale != 24*time.Hour || cfg.Window != 60*24*time.Hour || cfg.Seed != 42 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.Tick != 250*time.Millisecond {
		t.Fatalf("tick %v", cfg.Tick)
	}
	if !cfg.Now.Equal(time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("now %v", cfg.Now)
	}

	t.Setenv("OPACC_MCP_TRANSPORT", "unix")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected transport error")
	}
}
