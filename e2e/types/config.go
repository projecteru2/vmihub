package types

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/mcuadros/go-defaults"
)

type Config struct {
	URL      string `toml:"url"`
	BaseDir  string `toml:"base_dir"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

func LoadConfig(fname string) (*Config, error) {
	cfgBytes, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	cfg := new(Config)
	defaults.SetDefaults(cfg)
	err = toml.Unmarshal(cfgBytes, cfg)
	if err != nil {
		return nil, err
	}
	err = checkConfig(cfg)
	return cfg, err
}

func checkConfig(cfg *Config) error {
	if cfg.Username == "" {
		return errors.New("username is required")
	}
	if cfg.Password == "" {
		return errors.New("password is required")
	}
	if cfg.URL == "" {
		return errors.New("url is required")
	}
	return nil
}
