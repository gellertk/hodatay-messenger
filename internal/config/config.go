package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string         `yaml:"env" json:"-"`
	DatabaseDSN string         `yaml:"database_dsn" env:"DATABASE_URL" env-required:"true" json:"-"`
	HTTPServer  HTTPServer     `yaml:"http_server" json:"-"`
	App         AppConfig      `yaml:"app" json:"app"`
	Messages    MessagesConfig `yaml:"messages" json:"messages"`
	Uploads     UploadsConfig  `yaml:"uploads" json:"uploads"`
}

type AppConfig struct {
	BaseURL string `yaml:"base_url" json:"base_url"`
}

type MessagesConfig struct {
	MaxAttachments int `yaml:"max_attachments" json:"max_attachments"`
}

type UploadsConfig struct {
	MaxImageSize       int64 `yaml:"max_image_size" json:"max_image_size"`
	MaxVoiceSize       int64 `yaml:"max_voice_size" json:"max_voice_size"`
	MaxVideoSize       int64 `yaml:"max_video_size" json:"max_video_size"`
	MaxDocumentSize    int64 `yaml:"max_document_size" json:"max_document_size"`
	MaxVoiceDurationMs int64 `yaml:"max_voice_duration_ms" json:"max_voice_duration_ms"`

	PresignTTL PresignTTLConfig `yaml:"presign_ttl" json:"presign_ttl"`
}

type PresignTTLConfig struct {
	VoiceSec    int `yaml:"voice_sec" json:"voice_sec"`
	ImageSec    int `yaml:"image_sec" json:"image_sec"`
	VideoSec    int `yaml:"video_sec" json:"video_sec"`
	DocumentSec int `yaml:"document_sec" json:"document_sec"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8082" json:"-"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s" json:"-"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s" json:"-"`
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
