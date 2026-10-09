package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPPort        string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load reports every missing or invalid variable at once, so one failed start is enough to fix them all.
func Load() (Config, error) {
	var errs []error

	str := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	dur := func(key string) time.Duration {
		v := str(key)
		if v == "" {
			return 0
		}
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration like 15m", key))
		}
		return d
	}

	cfg := Config{
		DatabaseURL:     str("DATABASE_URL"),
		HTTPPort:        str("HTTP_PORT"),
		JWTSecret:       str("JWT_SECRET"),
		AccessTokenTTL:  dur("ACCESS_TOKEN_TTL"),
		RefreshTokenTTL: dur("REFRESH_TOKEN_TTL"),
	}

	// HS256 needs a key at least as long as its 256-bit hash.
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 characters"))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}
