package config

import (
	"bytes"
	_ "embed"
	"errors"
	"html/template"
	"os"
	"strings"
)

var (
	//go:embed templates/ceph.conf
	cephConfigStr string
	cephConfigTpl *template.Template
	//go:embed templates/ceph.client.eru.keyring
	cephKeyringStr string
	cephKeyringTpl *template.Template
	//go:embed templates/rbdmap
	rbdMapTplStr string
)

func (cfg *Config) PrepareCephConfig() error {
	if err := os.MkdirAll("/etc/ceph", 0755); err != nil {
		return err
	}

	if err := cfg.writeCephConfig(); err != nil {
		return err
	}
	if err := cfg.writeCephKeyring(); err != nil {
		return err
	}
	return cfg.writeRBDMap()
}

func (cfg *Config) writeCephConfig() (err error) {
	fname := "/etc/ceph/ceph.conf"
	if _, err := os.Stat(fname); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if cephConfigTpl == nil {
		if cephConfigTpl, err = template.New("ceph_config").Parse(cephConfigStr); err != nil {
			return err
		}
	}
	var monHosts []string //nolint
	parts := strings.Split(cfg.RBD.MonHost, ",")
	for _, p := range parts {
		hp := strings.Split(p, ":")
		monHosts = append(monHosts, hp[0])
	}
	d := map[string]any{
		"fsid":     cfg.RBD.FSID,
		"mon_host": monHosts,
	}
	var buf bytes.Buffer
	if err = cephConfigTpl.Execute(&buf, d); err != nil {
		return err
	}
	return os.WriteFile(fname, buf.Bytes(), 0644) //nolint
}

func (cfg *Config) writeCephKeyring() (err error) {
	fname := "/etc/ceph/ceph.client.eru.keyring"
	if _, err := os.Stat(fname); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if cephKeyringTpl == nil {
		if cephKeyringTpl, err = template.New("ceph_keyring").Parse(cephKeyringStr); err != nil {
			return
		}
	}

	d := map[string]any{
		"key": cfg.RBD.Key,
	}
	var buf bytes.Buffer
	if err = cephKeyringTpl.Execute(&buf, d); err != nil {
		return
	}
	return os.WriteFile(fname, buf.Bytes(), 0644) //nolint
}

func (cfg *Config) writeRBDMap() error {
	fname := "/etc/ceph/rbdmap"
	if _, err := os.Stat(fname); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(fname, []byte(rbdMapTplStr), 0644) //nolint
}
