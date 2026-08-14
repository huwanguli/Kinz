// Package kconf holds the Kinz runtime configuration. Loading follows the
// chain defaults -> YAML file (optional, never panics when missing) -> KINZ_*
// environment variables. A missing file is not an error; an invalid file or
// environment value is.
package kconf

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from YAML either as a string
// like "10s" or as an integer number of nanoseconds.
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw any
	if err := node.Decode(&raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case int:
		*d = Duration(time.Duration(v))
		return nil
	case string:
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("kinz: invalid duration %q: %w", v, err)
		}
		*d = Duration(parsed)
		return nil
	default:
		return fmt.Errorf("kinz: invalid duration value %v", raw)
	}
}

// Config holds all runtime configuration with production-safe defaults.
type Config struct {
	Name              string   `yaml:"Name"`
	Host              string   `yaml:"Host"`
	Port              int      `yaml:"Port"`
	MaxConn           int      `yaml:"MaxConn"`
	MaxPacketSize     uint32   `yaml:"MaxPacketSize"`
	WorkerPoolSize    uint32   `yaml:"WorkerPoolSize"`
	MaxWorkerTaskLen  uint32   `yaml:"MaxWorkerTaskLen"`
	HeartbeatInterval Duration `yaml:"HeartbeatInterval"`
	HeartbeatTimeout  Duration `yaml:"HeartbeatTimeout"`
	WriteQueueSize    int      `yaml:"WriteQueueSize"`
	WriteTimeout      Duration `yaml:"WriteTimeout"`
	ReadIdleTimeout   Duration `yaml:"ReadIdleTimeout"`
}

// Default returns the built-in defaults.
func Default() *Config {
	return &Config{
		Name:              "KinzServer",
		Host:              "0.0.0.0",
		Port:              8999,
		MaxConn:           1024,
		MaxPacketSize:     4096,
		WorkerPoolSize:    10,
		MaxWorkerTaskLen:  1024,
		HeartbeatInterval: Duration(10 * time.Second),
		HeartbeatTimeout:  Duration(30 * time.Second),
		WriteQueueSize:    256,
		WriteTimeout:      Duration(5 * time.Second),
		ReadIdleTimeout:   Duration(0),
	}
}

// Load builds a Config: defaults, then the YAML file at path (when non-empty
// and present), then KINZ_* environment variables. A missing file is ignored;
// an unreadable/parse-invalid file or invalid env value returns an error.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("kinz: parse config %s: %w", path, err)
			}
		case !os.IsNotExist(err):
			return nil, fmt.Errorf("kinz: read config %s: %w", path, err)
		}
	}

	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) error {
	get := func(key string) (string, bool) { return os.LookupEnv(key) }
	if v, ok := get("KINZ_HOST"); ok {
		cfg.Host = v
	}
	if v, ok := get("KINZ_PORT"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("kinz: invalid KINZ_PORT %q", v)
		}
		cfg.Port = n
	}
	if v, ok := get("KINZ_MAXCONN"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("kinz: invalid KINZ_MAXCONN %q", v)
		}
		cfg.MaxConn = n
	}
	if v, ok := get("KINZ_MAXPACKETSIZE"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("kinz: invalid KINZ_MAXPACKETSIZE %q", v)
		}
		cfg.MaxPacketSize = uint32(n)
	}
	if v, ok := get("KINZ_WORKERPOOLSIZE"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("kinz: invalid KINZ_WORKERPOOLSIZE %q", v)
		}
		cfg.WorkerPoolSize = uint32(n)
	}
	if v, ok := get("KINZ_HEARTBEATINTERVAL"); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("kinz: invalid KINZ_HEARTBEATINTERVAL %q", v)
		}
		cfg.HeartbeatInterval = Duration(d)
	}
	return nil
}
