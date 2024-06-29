package config

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	_ "embed"

	"github.com/mcuadros/go-defaults"

	"github.com/pelletier/go-toml"
)

var (
	cfg *Config
)

type Config struct {
	GlobalTimeout  time.Duration `toml:"global_timeout" default:"5m"`
	MaxConcurrency int           `toml:"max_concurrency" default:"10000"`
	Server         ServerConfig  `toml:"server"`
	RBD            RBDConfig     `toml:"rbd"`
	Log            LogConfig     `toml:"log"`
	Redis          RedisConfig   `toml:"redis"`
	Mysql          MysqlConfig   `toml:"mysql"`
	Storage        StorageConfig `toml:"storage"`
	JWT            JWTConfig     `toml:"jwt"`
}

type ServerConfig struct {
	RunMode      string        `toml:"run_mode" default:"release"`
	Bind         string        `toml:"bind" default:":8080"`
	ReadTimeout  time.Duration `toml:"read_timeout" default:"5m"`
	WriteTimeout time.Duration `toml:"write_timeout" default:"5m"`
}

type RedisConfig struct {
	Addr          string   `toml:"addr"`
	SentinelAddrs []string `toml:"sentinel_addrs"`
	MasterName    string   `toml:"master_name"`
	Username      string   `toml:"username"`
	Password      string   `toml:"password"`
	DB            int      `toml:"db"`
	Expire        uint     `toml:"expire"`
}

func (src *RedisConfig) CopyToIfEmpty(dest *RedisConfig) {
	if dest.Addr == "" {
		dest.Addr = src.Addr
	}
	if dest.SentinelAddrs == nil {
		dest.SentinelAddrs = src.SentinelAddrs
	}
	if dest.MasterName == "" {
		dest.MasterName = src.MasterName
	}
	if dest.Username == "" {
		dest.Username = src.Username
	}
	if dest.Password == "" {
		dest.Password = src.Password
	}
	if dest.DB == 0 {
		dest.DB = src.DB
	}
	if dest.Expire == 0 {
		dest.Expire = src.Expire
	}
}

type BackoffConfig struct {
	InitialInterval time.Duration `toml:"initial_interval" default:"30s"`
	MaxInterval     time.Duration `toml:"max_interval" default:"60m"`
	MaxElapsedTime  time.Duration `toml:"max_elapsed_time" default:"2h"`
}
type MysqlConfig struct {
	DSN          string `toml:"dsn"`
	MaxOpenConns int    `toml:"max_open_connections"`
	MaxIdleConns int    `toml:"max_idle_connections"`
}

type StorageConfig struct {
	Type  string              `toml:"type"`
	Local *LocalStorageConfig `toml:"local"`
	S3    *S3Config           `toml:"s3"`
}

type RBDConfig struct {
	Username string `toml:"username" json:"username"`
	Pool     string `toml:"pool" json:"pool"`
	QosBPS   int64  `toml:"qos_bps" json:"qosBps"`
	QosIOPS  int64  `toml:"qos_iops" json:"qosIops"`
	FSID     string `toml:"fsid" json:"fsid"`
	Key      string `toml:"key" json:"key"`
	MonHost  string `toml:"mon_host" json:"monHost"`
}

type LocalStorageConfig struct {
	BaseDir string `toml:"base_dir"`
}

type S3Config struct {
	Endpoint  string `toml:"endpoint"`
	AccessKey string `toml:"access_key"`
	SecretKey string `toml:"secret_key"`
	Bucket    string `toml:"bucket"`
	BaseDir   string `toml:"base_dir"`
}

type LogConfig struct {
	Level     string `toml:"level" default:"info"`
	UseJSON   bool   `toml:"use_json"`
	SentryDSN string `toml:"sentry_dsn"`
	// for file log
	Filename   string `toml:"filename"`
	MaxSize    int    `toml:"maxsize" default:"500"`
	MaxAge     int    `toml:"max_age" default:"28"`
	MaxBackups int    `toml:"max_backups" default:"3"`
}

// JWTConfig JWT signingKey info
type JWTConfig struct {
	SigningKey string `toml:"key"`
}

func (c *Config) String() string {
	bs, _ := json.MarshalIndent(c, "", "  ")
	return string(bs)
}

func loadConfigFromBytes(cfgBytes []byte) (*Config, error) {
	cfg := new(Config)
	defaults.SetDefaults(cfg)
	err := toml.Unmarshal(cfgBytes, cfg)
	if err != nil {
		return nil, err
	}

	err = checkConfig(cfg)
	return cfg, err
}

func Init(p string) (*Config, error) {
	cfgBytes, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if cfg, err = loadConfigFromBytes(cfgBytes); err != nil {
		return nil, err
	}
	if err := cfg.PrepareCephConfig(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func GetCfg() *Config {
	return cfg
}

func checkConfig(cfg *Config) error {
	// check run mode
	values := map[string]bool{
		"debug":   true,
		"test":    true,
		"release": true,
	}
	_, ok := values[cfg.Server.RunMode]
	if !ok {
		return errors.New("invalid value for run mode, only debug, test and release are allowed")
	}
	// check log config
	return nil
}

var (
	//go:embed config.example.toml
	testConfigStr string
)

func LoadTestConfig() (*Config, error) {
	var err error
	cfg, err = loadConfigFromBytes([]byte(testConfigStr))
	cfg.Log.Level = "debug"
	return cfg, err
}
