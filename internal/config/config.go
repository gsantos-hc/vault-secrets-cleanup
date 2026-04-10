package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Vault      VaultConfig      `mapstructure:"vault"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
	Staleness  StalenessConfig  `mapstructure:"staleness"`
	Exclusions ExclusionsConfig `mapstructure:"exclusions"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type VaultConfig struct {
	Address string `mapstructure:"address"`
	Token   string `mapstructure:"token"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

type StalenessConfig struct {
	DefaultPeriod string            `mapstructure:"default_period"`
	Namespaces    map[string]string `mapstructure:"namespaces"`
}

type ExclusionsConfig struct {
	Namespaces []string `mapstructure:"namespaces"`
	Paths      []string `mapstructure:"paths"`
	Mounts     []string `mapstructure:"mounts"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load(configPath string, flagValues map[string]any) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	v.SetDefault("rate_limit.requests_per_second", 100.0)
	v.SetDefault("rate_limit.burst", 100)
	v.SetDefault("staleness.default_period", "365d")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	_ = v.BindEnv("vault.address", "VAULT_ADDR")
	_ = v.BindEnv("vault.token", "VAULT_TOKEN")
	v.AutomaticEnv()

	for k, val := range flagValues {
		if val == nil {
			continue
		}
		if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		v.Set(k, val)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Vault.Address) == "" {
		errs = append(errs, errors.New("vault.address is required (or set VAULT_ADDR)"))
	}
	if strings.TrimSpace(c.Vault.Token) == "" {
		errs = append(errs, errors.New("vault.token is required (or set VAULT_TOKEN)"))
	}
	if c.RateLimit.RequestsPerSecond <= 0 {
		errs = append(errs, errors.New("rate_limit.requests_per_second must be > 0"))
	}
	if _, err := parsePeriod(c.Staleness.DefaultPeriod); err != nil {
		errs = append(errs, fmt.Errorf("staleness.default_period must be valid duration: %w", err))
	}

	for ns, period := range c.Staleness.Namespaces {
		if _, err := parsePeriod(period); err != nil {
			errs = append(errs, fmt.Errorf("staleness.namespaces[%s] must be valid duration: %w", ns, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func parsePeriod(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("period cannot be empty")
	}

	if strings.HasSuffix(value, "d") {
		daysRaw := strings.TrimSuffix(value, "d")
		days, err := strconv.Atoi(daysRaw)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid day period %q", raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be > 0")
	}

	return duration, nil
}
