package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config содержит все настройки приложения
type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`

	Backends []string `yaml:"backends"`

	HealthCheck struct {
		Endpoint string        `yaml:"endpoint"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"healthcheck"`

	Balancer struct {
		Algorithm string `yaml:"algorithm"`
	} `yaml:"balancer"`

	RateLimit struct {
		Default struct {
			Capacity   int     `yaml:"capacity"`
			RefillRate float64 `yaml:"refill_rate"`
		} `yaml:"default"`
	} `yaml:"ratelimit"`

	Storage struct {
		Type     string `yaml:"type"` // "memory" или "postgres"
		Postgres struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			DBName   string `yaml:"dbname"`
			SSLMode  string `yaml:"sslmode"`
		} `yaml:"postgres"`
	} `yaml:"storage"`
}

// LoadConfig загружает конфигурацию из файла
func LoadConfig(path string) (*Config, error) {
	// Проверяем на переменные окружения
	envConfig := os.Getenv("CONFIG")
	if envConfig != "" {
		path = envConfig
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла конфигурации: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации: %v", err)
	}

	// Валидация конфигурации
	if config.Server.Port == "" {
		config.Server.Port = "8080" // Порт по умолчанию
	}

	if len(config.Backends) == 0 {
		return nil, fmt.Errorf("не указаны бэкенд-серверы")
	}

	if config.HealthCheck.Endpoint == "" {
		config.HealthCheck.Endpoint = "/health" // Эндпоинт по умолчанию
	}

	if config.HealthCheck.Interval == 0 {
		config.HealthCheck.Interval = 5 * time.Second // Интервал по умолчанию
	}

	if config.Balancer.Algorithm == "" {
		config.Balancer.Algorithm = "round-robin" // Алгоритм по умолчанию
	}

	if config.RateLimit.Default.Capacity == 0 {
		config.RateLimit.Default.Capacity = 100 // Емкость по умолчанию
	}

	if config.RateLimit.Default.RefillRate == 0 {
		config.RateLimit.Default.RefillRate = 10 // Скорость пополнения по умолчанию
	}

	// Настройки хранилища
	if config.Storage.Type == "" {
		config.Storage.Type = "memory"
	}

	// Валидация настроек PostgreSQL
	if config.Storage.Type == "postgres" {
		if config.Storage.Postgres.Host == "" {
			config.Storage.Postgres.Host = "localhost"
		}
		if config.Storage.Postgres.Port == 0 {
			config.Storage.Postgres.Port = 5432
		}
		if config.Storage.Postgres.User == "" {
			config.Storage.Postgres.User = "postgres"
		}
		if config.Storage.Postgres.DBName == "" {
			config.Storage.Postgres.DBName = "loadbalancer"
		}
		if config.Storage.Postgres.SSLMode == "" {
			config.Storage.Postgres.SSLMode = "disable"
		}
	}

	return &config, nil
}
