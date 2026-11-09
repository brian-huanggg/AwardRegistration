package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
)

const key = "0123456789abcdef0123456789abcdef"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := config.Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Profile != "local" || c.PublicAddr != ":8080" || c.ManagementAddr != ":8081" ||
		c.LogLevel != slog.LevelInfo || c.ShutdownTimeout != 20*time.Second || c.CSRFKey != config.DefaultCSRFKey || c.ProxyHops != 1 ||
		c.MinIO != (config.MinIO{Endpoint: "localhost:9000", AccessKey: "minioadmin", SecretKey: "minioadmin", Bucket: "esg-slides"}) {
		t.Fatalf("%+v", c)
	}
}

func TestOverrides(t *testing.T) {
	c, err := config.Load(env(map[string]string{
		"ESG_PROFILE": "dev", "ESG_CSRF_KEY": key, "ESG_CSRF_KEY_PREVIOUS": key + "2", "ESG_LOG_LEVEL": "debug", "ESG_SHUTDOWN_TIMEOUT": "5s", "ESG_READY_DRAIN_DELAY": "1s", "ESG_TRUSTED_PROXY_HOPS": "2",
		"ESG_MINIO_ENDPOINT": "minio:9000", "ESG_MINIO_ACCESS_KEY": "a", "ESG_MINIO_SECRET_KEY": "s", "ESG_MINIO_BUCKET": "b", "ESG_MINIO_SECURE": "true",
	}))
	if err != nil || c.Profile != "dev" || c.LogLevel != slog.LevelDebug || c.ReadyDrainDelay != time.Second || c.ProxyHops != 2 ||
		c.MinIO != (config.MinIO{Endpoint: "minio:9000", AccessKey: "a", SecretKey: "s", Bucket: "b", Secure: true}) {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestInvalid(t *testing.T) {
	for name, m := range map[string]map[string]string{
		"profile":              {"ESG_PROFILE": "prod"},
		"log level":            {"ESG_LOG_LEVEL": "loud"},
		"timeout":              {"ESG_SHUTDOWN_TIMEOUT": "soon"},
		"zero timeout":         {"ESG_SHUTDOWN_TIMEOUT": "0s"},
		"drain":                {"ESG_READY_DRAIN_DELAY": "x"},
		"short csrf key":       {"ESG_CSRF_KEY": "short"},
		"short previous key":   {"ESG_CSRF_KEY": key, "ESG_CSRF_KEY_PREVIOUS": "short"},
		"dev needs csrf key":   {"ESG_PROFILE": "dev"},
		"k3s needs csrf key":   {"ESG_PROFILE": "k3s"},
		"hops text":            {"ESG_TRUSTED_PROXY_HOPS": "one"},
		"negative hops":        {"ESG_TRUSTED_PROXY_HOPS": "-1"},
		"too many hops":        {"ESG_TRUSTED_PROXY_HOPS": "11"},
		"same address":         {"ESG_PUBLIC_ADDR": ":9", "ESG_MANAGEMENT_ADDR": ":9"},
		"dev needs minio keys": {"ESG_PROFILE": "dev", "ESG_CSRF_KEY": key},
		"minio secure":         {"ESG_MINIO_SECURE": "maybe"},
		"admin file off local": {"ESG_PROFILE": "dev", "ESG_CSRF_KEY": key, "ESG_MINIO_ACCESS_KEY": "a", "ESG_MINIO_SECRET_KEY": "b", "ESG_ADMIN_FILE": "/x.json"},
	} {
		if _, err := config.Load(env(m)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
