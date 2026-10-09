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

	cfg := Config{
		DatabaseURL:     requiredString("DATABASE_URL", &errs),
		HTTPPort:        requiredString("HTTP_PORT", &errs),
		JWTSecret:       requiredString("JWT_SECRET", &errs),
		AccessTokenTTL:  requiredDuration("ACCESS_TOKEN_TTL", &errs),
		RefreshTokenTTL: requiredDuration("REFRESH_TOKEN_TTL", &errs),
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

func requiredString(key string, errs *[]error) string {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func requiredDuration(key string, errs *[]error) time.Duration {
	v := requiredString(key, errs)
	if v == "" {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration like 15m", key))
	}
	return d
}
