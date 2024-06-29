package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	cfgStr := `
	[server]
	run_mode = "debug"
	bind = ":5000"
	read_timeout = "60s"
	write_timeout = "5m"

    [log]
    level = "debug"
    `
	cfg, err := loadConfigFromBytes([]byte(cfgStr))
	assert.Nil(t, err)
	assert.Equal(t, cfg.Server, ServerConfig{
		RunMode:      "debug",
		Bind:         ":5000",
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 5 * time.Minute,
	})

	_, err = LoadTestConfig()
	assert.Nil(t, err)
}

func TestLoadDefaultConfig(t *testing.T) {
	cfgStr := `
	[server]
    [log]
    `
	cfg, err := loadConfigFromBytes([]byte(cfgStr))
	assert.Nil(t, err)
	assert.Equal(t, cfg.GlobalTimeout, 5*time.Minute)
	assert.Equal(t, cfg.Server.RunMode, "release")
	assert.Equal(t, cfg.MaxConcurrency, 10000)
}
