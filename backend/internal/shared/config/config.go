// Package config loads the process configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config is the validated process configuration.
type Config struct {
	Profile         string // local | k3s | dev
	PublicAddr      string // public http.Server
	ManagementAddr  string // probes and metrics; never routed by the ingress
	LogLevel        slog.Level
	ShutdownTimeout time.Duration // must stay below terminationGracePeriodSeconds
	ReadyDrainDelay time.Duration // time readiness stays false before Shutdown starts
}

// Load reads ESG_* variables through getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	get := func(k, def string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return def
	}
	c := Config{
		Profile:        get("ESG_PROFILE", "local"),
		PublicAddr:     get("ESG_PUBLIC_ADDR", ":8080"),
		ManagementAddr: get("ESG_MANAGEMENT_ADDR", ":8081"),
	}
	switch c.Profile {
	case "local", "k3s", "dev":
	default:
		return Config{}, fmt.Errorf("ESG_PROFILE %q: want local, k3s or dev", c.Profile)
	}
	if err := c.LogLevel.UnmarshalText([]byte(strings.ToUpper(get("ESG_LOG_LEVEL", "info")))); err != nil {
		return Config{}, fmt.Errorf("ESG_LOG_LEVEL: %w", err)
	}
	var err error
	if c.ShutdownTimeout, err = time.ParseDuration(get("ESG_SHUTDOWN_TIMEOUT", "20s")); err != nil {
		return Config{}, fmt.Errorf("ESG_SHUTDOWN_TIMEOUT: %w", err)
	}
	if c.ReadyDrainDelay, err = time.ParseDuration(get("ESG_READY_DRAIN_DELAY", "0s")); err != nil {
		return Config{}, fmt.Errorf("ESG_READY_DRAIN_DELAY: %w", err)
	}
	if c.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("ESG_SHUTDOWN_TIMEOUT must be positive")
	}
	if c.PublicAddr == c.ManagementAddr {
		return Config{}, fmt.Errorf("public and management addresses must differ")
	}
	return c, nil
}

// OS loads the configuration from the real environment.
func OS() (Config, error) { return Load(os.Getenv) }
