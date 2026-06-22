package config

import (
	"os"

	"go.yaml.in/yaml/v4"
)

type Config struct {
	CA           CAConfig          `yaml:"ca"`
	Dirs         DirsConfig        `yaml:"dirs"`
	Nginx        NginxConfig       `yaml:"nginx"`
	Server       ServerConfig      `yaml:"server"`
	DNS          DNSConfig         `yaml:"dns"`
	RuntimePaths map[string]string `yaml:"runtime_paths"`
}

type CAConfig struct {
	Dir string `yaml:"dir"`
}

type DirsConfig struct {
	Certs string `yaml:"certs"`
	Envs  string `yaml:"envs"`
	Units string `yaml:"units"`
	Data  string `yaml:"data"`
}

type NginxConfig struct {
	SitesAvailable string `yaml:"sites_available"`
	SitesEnabled   string `yaml:"sites_enabled"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DNSConfig struct {
	DnsmasqConf string `yaml:"dnsmasq_conf"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}

	// defaults
	if cfg.CA.Dir == "" {
		cfg.CA.Dir = "/etc/lighthouse/ca"
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9000
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}

	if cfg.Dirs.Certs == "" {
		cfg.Dirs.Certs = "/etc/lighthouse/certs"
	}

	if cfg.Dirs.Envs == "" {
		cfg.Dirs.Envs = "/etc/lighthouse/envs"
	}

	if cfg.Dirs.Units == "" {
		cfg.Dirs.Units = "/etc/systemd/system"
	}

	if cfg.Dirs.Data == "" {
		cfg.Dirs.Data = "/var/lib/lighthouse"
	}

	if cfg.Nginx.SitesAvailable == "" {
		cfg.Nginx.SitesAvailable = "/etc/nginx/sites-available"
	}

	if cfg.Nginx.SitesEnabled == "" {
		cfg.Nginx.SitesEnabled = "/etc/nginx/sites-enabled"
	}

	if cfg.DNS.DnsmasqConf == "" {
		if path == "config.dev.yml" {
			cfg.DNS.DnsmasqConf = "./dev-data/dnsmasq.conf"
		} else {
			cfg.DNS.DnsmasqConf = "/etc/lighthouse/dnsmasq.conf"
		}
	}

	return cfg, nil
}
