package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	HTTP     HTTPConfig     `yaml:"http"`
	GRPC     GRPCConfig     `yaml:"grpc"`
	Auth     AuthConfig     `yaml:"auth"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type HTTPConfig struct {
	Address string `yaml:"address"`
}

type GRPCConfig struct {
	Address string `yaml:"address"`
}

type AuthConfig struct {
	WorkerPassword string `yaml:"worker_password"`
	AdminPassword  string `yaml:"admin_password"`
}

func Load() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Expand environment variables in the config file
	expandedData := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, err
	}

	// Override with environment variables if set (legacy support)
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		cfg.Database.Password = dbPass
	}
	if workerPass := os.Getenv("AUTH_WORKER_PASSWORD"); workerPass != "" {
		cfg.Auth.WorkerPassword = workerPass
	}
	if adminPass := os.Getenv("AUTH_ADMIN_PASSWORD"); adminPass != "" {
		cfg.Auth.AdminPassword = adminPass
	}
	// Legacy env var names
	if workerPass := os.Getenv("WORKER_PASSWORD"); workerPass != "" {
		cfg.Auth.WorkerPassword = workerPass
	}
	if adminPass := os.Getenv("ADMIN_PASSWORD"); adminPass != "" {
		cfg.Auth.AdminPassword = adminPass
	}

	return &cfg, nil
}
