// Package config loads the process configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the validated process configuration.
type Config struct {
	Profile         string // local | k3s | dev
	PublicAddr      string // public http.Server
	ManagementAddr  string // probes and metrics; never routed by the ingress
	DBDSN           string // go-sql-driver DSN; must carry the pins of Backend.md §2.3
	LogLevel        slog.Level
	ShutdownTimeout time.Duration // must stay below terminationGracePeriodSeconds
	ReadyDrainDelay time.Duration // time readiness stays false before Shutdown starts
	CSRFKey         string        // HMAC key that signs CSRF tokens (>= 32 bytes)
	CSRFKeyPrevious string        // optional: still verifies for 8 h after a rotation
	ProxyHops       int           // X-Forwarded-For proxies to trust for the client IP; 0 = TCP peer (PRD §6.5)
	MinIO           MinIO
}

// MinIO configures the slide store. The bucket must exist; the application
// never creates it.
type MinIO struct {
	Endpoint  string // host:port
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool // TLS to MinIO
}

// DefaultMinIOKey is the access and secret key of docker-compose.yml, used by
// the local profile only; every other profile must set both keys.
const DefaultMinIOKey = "minioadmin"

// DefaultCSRFKey is the throwaway key of the local profile only; every other
// profile must set ESG_CSRF_KEY.
const DefaultCSRFKey = "local-dev-csrf-key-not-a-secret-0000"

const (
	minCSRFKeyBytes = 32
	maxProxyHops    = 10
)

// DefaultDSN matches docker-compose.yml (throwaway local credentials). The query
// pins UTC and matched-row counts (Backend.md §2.3).
const DefaultDSN = "esg:esg@tcp(localhost:3306)/esg?parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27&clientFoundRows=true"

// Load reads ESG_* variables through getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	get := func(k, def string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return def
	}
	c := Config{
		Profile:         get("ESG_PROFILE", "local"),
		PublicAddr:      get("ESG_PUBLIC_ADDR", ":8080"),
		ManagementAddr:  get("ESG_MANAGEMENT_ADDR", ":8081"),
		DBDSN:           get("ESG_DB_DSN", DefaultDSN),
		CSRFKey:         getenv("ESG_CSRF_KEY"),
		CSRFKeyPrevious: getenv("ESG_CSRF_KEY_PREVIOUS"),
		MinIO: MinIO{
			Endpoint:  get("ESG_MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getenv("ESG_MINIO_ACCESS_KEY"),
			SecretKey: getenv("ESG_MINIO_SECRET_KEY"),
			Bucket:    get("ESG_MINIO_BUCKET", "esg-slides"),
		},
	}
	switch c.Profile {
	case "local", "k3s", "dev":
	default:
		return Config{}, fmt.Errorf("ESG_PROFILE %q: want local, k3s or dev", c.Profile)
	}
	if c.CSRFKey == "" && c.Profile == "local" {
		c.CSRFKey = DefaultCSRFKey
	}
	if c.Profile == "local" {
		if c.MinIO.AccessKey == "" {
			c.MinIO.AccessKey = DefaultMinIOKey
		}
		if c.MinIO.SecretKey == "" {
			c.MinIO.SecretKey = DefaultMinIOKey
		}
	}
	if c.MinIO.AccessKey == "" || c.MinIO.SecretKey == "" {
		return Config{}, fmt.Errorf("ESG_MINIO_ACCESS_KEY and ESG_MINIO_SECRET_KEY are required outside the local profile")
	}
	if len(c.CSRFKey) < minCSRFKeyBytes {
		return Config{}, fmt.Errorf("ESG_CSRF_KEY: need at least %d bytes", minCSRFKeyBytes)
	}
	if c.CSRFKeyPrevious != "" && len(c.CSRFKeyPrevious) < minCSRFKeyBytes {
		return Config{}, fmt.Errorf("ESG_CSRF_KEY_PREVIOUS: need at least %d bytes", minCSRFKeyBytes)
	}
	if err := c.LogLevel.UnmarshalText([]byte(strings.ToUpper(get("ESG_LOG_LEVEL", "info")))); err != nil {
		return Config{}, fmt.Errorf("ESG_LOG_LEVEL: %w", err)
	}
	var err error
	if c.ProxyHops, err = strconv.Atoi(get("ESG_TRUSTED_PROXY_HOPS", "1")); err != nil || c.ProxyHops < 0 || c.ProxyHops > maxProxyHops {
		return Config{}, fmt.Errorf("ESG_TRUSTED_PROXY_HOPS: want an integer 0..%d", maxProxyHops)
	}
	if c.MinIO.Secure, err = strconv.ParseBool(get("ESG_MINIO_SECURE", "false")); err != nil {
		return Config{}, fmt.Errorf("ESG_MINIO_SECURE: %w", err)
	}
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
