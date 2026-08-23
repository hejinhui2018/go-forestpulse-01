package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is deliberately small and explicit so the service can be started by
// an operator without a configuration framework or hidden global state.
type Config struct {
	HTTPAddr       string
	DataPath       string
	RecoveryEvery  time.Duration
	RequestTimeout time.Duration
	MaxBatchSize   int
}

func Default() Config {
	return Config{HTTPAddr: ":8090", DataPath: "./data/forestpulse.json", RecoveryEvery: 2 * time.Second, RequestTimeout: 3 * time.Second, MaxBatchSize: 256}
}

func FromEnv(getenv func(string) string) (Config, error) {
	c := Default()
	if v := getenv("FORESTPULSE_HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := getenv("FORESTPULSE_DATA_PATH"); v != "" {
		c.DataPath = v
	}
	if v := getenv("FORESTPULSE_RECOVERY_EVERY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("recovery interval: %w", err)
		}
		c.RecoveryEvery = d
	}
	if v := getenv("FORESTPULSE_REQUEST_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("request timeout: %w", err)
		}
		c.RequestTimeout = d
	}
	if v := getenv("FORESTPULSE_MAX_BATCH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return c, fmt.Errorf("max batch: %w", err)
		}
		c.MaxBatchSize = n
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if c.HTTPAddr == "" || c.DataPath == "" {
		return fmt.Errorf("http address and data path are required")
	}
	if c.RecoveryEvery <= 0 || c.RequestTimeout <= 0 {
		return fmt.Errorf("durations must be positive")
	}
	if c.MaxBatchSize <= 0 || c.MaxBatchSize > 10000 {
		return fmt.Errorf("max batch must be between 1 and 10000")
	}
	return nil
}

func Load() (Config, error) { return FromEnv(os.Getenv) }
