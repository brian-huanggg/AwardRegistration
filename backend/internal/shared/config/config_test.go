package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := config.Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Profile != "local" || c.PublicAddr != ":8080" || c.ManagementAddr != ":8081" ||
		c.LogLevel != slog.LevelInfo || c.ShutdownTimeout != 20*time.Second {
		t.Fatalf("%+v", c)
	}
}

func TestOverrides(t *testing.T) {
	c, err := config.Load(env(map[string]string{
		"ESG_PROFILE": "dev", "ESG_LOG_LEVEL": "debug", "ESG_SHUTDOWN_TIMEOUT": "5s", "ESG_READY_DRAIN_DELAY": "1s",
	}))
	if err != nil || c.Profile != "dev" || c.LogLevel != slog.LevelDebug || c.ReadyDrainDelay != time.Second {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestInvalid(t *testing.T) {
	for name, m := range map[string]map[string]string{
		"profile":      {"ESG_PROFILE": "prod"},
		"log level":    {"ESG_LOG_LEVEL": "loud"},
		"timeout":      {"ESG_SHUTDOWN_TIMEOUT": "soon"},
		"zero timeout": {"ESG_SHUTDOWN_TIMEOUT": "0s"},
		"drain":        {"ESG_READY_DRAIN_DELAY": "x"},
		"same address": {"ESG_PUBLIC_ADDR": ":9", "ESG_MANAGEMENT_ADDR": ":9"},
	} {
		if _, err := config.Load(env(m)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
