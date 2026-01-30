package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string     `yaml:"env" env-default:"local"`
	DatabaseDSN string     `yaml:"database_dsn" env:"DATABASE_URL" env-required:"true"`
	HTTPServer  HTTPServer `yaml:"http_server"`
	AppConfig   AppConfig  `yaml:"app"`
}

type AppConfig struct {
	BaseURL        string `yaml:"base_url" json:"base_url"`
	MaxAttachments int    `yaml:"max_attachments" json:"max_attachments"`
	MaxUploadSize  int64  `yaml:"max_upload_size" json:"max_upload_size"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8082"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config %s", err)
	}

	return &cfg
}
