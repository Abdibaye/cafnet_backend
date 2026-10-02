package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

// ServerConfig unifies HTTP transport, security, and database connectivity settings.
type ServerConfig struct {
	HTTPAddr          string
	Environment       string
	AllowedOrigins    []string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	RequestTimeout    time.Duration
	MaxBodyBytes      int64

	Auth    AuthConfig
	DB      db.Config
	Storage StorageConfig
}

// LoadServerConfig reads environment configuration with production-hardened defaults.
func LoadServerConfig() (*ServerConfig, error) {
	authCfg, err := LoadAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load auth config: %w", err)
	}

	dbCfg := db.LoadConfigFromEnv()
	storageCfg := LoadStorageConfig()

	env := authCfg.Environment

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		port := os.Getenv("PORT")
		if port != "" {
			httpAddr = ":" + port
		} else {
			httpAddr = ":8080"
		}
	}

	readTimeout := parseDurationEnv("HTTP_READ_TIMEOUT", 15*time.Second)
	readHeaderTimeout := parseDurationEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	writeTimeout := parseDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second)
	idleTimeout := parseDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	requestTimeout := parseDurationEnv("HTTP_REQUEST_TIMEOUT", 30*time.Second)
	maxBodyBytes := int64(parseIntEnv("HTTP_MAX_BODY_BYTES", 1024*1024)) // Default: 1 MB

	// Origins: Inherit from AuthConfig's allowed origins or override via APP_ALLOWED_ORIGINS
	origins := authCfg.AllowedOrigins
	if appOriginsStr := os.Getenv("APP_ALLOWED_ORIGINS"); appOriginsStr != "" {
		origins = nil
		for _, o := range strings.Split(appOriginsStr, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
	}

	cfg := &ServerConfig{
		HTTPAddr:          httpAddr,
		Environment:       env,
		AllowedOrigins:    origins,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		RequestTimeout:    requestTimeout,
		MaxBodyBytes:      maxBodyBytes,
		Auth:              *authCfg,
		DB:                dbCfg,
		Storage:           storageCfg,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks critical server settings.
func (c *ServerConfig) Validate() error {
	if c.ReadTimeout < 1*time.Second {
		return fmt.Errorf("HTTP_READ_TIMEOUT must be >= 1s")
	}
	if c.WriteTimeout < 1*time.Second {
		return fmt.Errorf("HTTP_WRITE_TIMEOUT must be >= 1s")
	}
	if c.MaxBodyBytes < 1024 {
		return fmt.Errorf("HTTP_MAX_BODY_BYTES must be >= 1024 bytes")
	}
	if (c.Environment == "production" || c.Environment == "prod") && len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("production requires at least one configured APP_ALLOWED_ORIGINS")
	}
	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("storage config invalid: %w", err)
	}
	return nil
}
