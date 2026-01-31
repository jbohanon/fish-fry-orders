package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	WorkerPassword string `yaml:"worker"`
	AdminPassword  string `yaml:"admin"`
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

	// Load database config from secret mount if available
	if err := loadSecretConfig("/app/secrets/database", &cfg.Database); err != nil {
		// Not fatal - might be running locally without secrets
		_ = err
	}

	// Load auth config from secret mount if available
	if err := loadSecretConfig("/app/secrets/auth", &cfg.Auth); err != nil {
		// Not fatal - might be running locally without secrets
		_ = err
	}

	// Override with environment variables if set (fallback for local dev)
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

// loadSecretConfig reads all files in a secret mount directory and unmarshals them into the target struct.
// Kubernetes secrets mount each key as a separate file, so we read all files and merge them.
func loadSecretConfig(secretPath string, target any) error {
	// Check if the secret directory exists
	info, err := os.Stat(secretPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}

	// Read all files in the directory
	entries, err := os.ReadDir(secretPath)
	if err != nil {
		return err
	}

	// Build a map from all secret keys
	secretData := make(map[string]any)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Skip hidden files (like ..data symlinks in k8s secrets)
		if entry.Name()[0] == '.' {
			continue
		}

		filePath := filepath.Join(secretPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Store the content with the filename as key
		// Try to parse as int first for numeric fields like port
		strContent := strings.TrimSpace(string(content))
		if intVal, err := strconv.Atoi(strContent); err == nil {
			secretData[entry.Name()] = intVal
		} else {
			secretData[entry.Name()] = strContent
		}
	}

	// Marshal to YAML and unmarshal into target to merge
	yamlData, err := yaml.Marshal(secretData)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(yamlData, target)
}
