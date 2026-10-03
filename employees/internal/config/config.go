package config

import (
	"fmt"
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Database struct {
		Host     string `yaml:"host" env:"DATABASE_HOST"`
		Port     string `yaml:"port" env:"DATABASE_PORT"`
		Username string `yaml:"username" env:"DATABASE_USERNAME"`
		Password string `yaml:"password" env:"DATABASE_PASSWORD"`
		Database string `yaml:"database" env:"DATABASE_DATABASE"`
		SSLMode  string `yaml:"ssl_mode" env:"DATABASE_SSL_MODE"`
	} `yaml:"database"`
	HTTP struct {
		Host          string `yaml:"host" env:"HTTP_HOST"`
		Port          string `yaml:"port" env:"HTTP_PORT"`
		SwaggerPrefix string `yaml:"swagger_prefix" env:"HTTP_SWAGGER_PREFIX"`
	} `yaml:"http"`
	ReportsClient struct {
		Host string `yaml:"host" env:"REPORTS_CLIENT_HOST"`
		Port string `yaml:"port" env:"REPORTS_CLIENT_PORT"`
	} `yaml:"reports_client"`
}

func MustLoad() *Config {
	var cfg Config

	if err := cleanenv.ReadConfig("config.yaml", &cfg); err == nil {
		return &cfg
	}

	if err := cleanenv.ReadEnv(&cfg); err == nil {
		return &cfg
	}

	log.Fatal("config not found")
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.Database.Username, c.Database.Password,
		c.Database.Host, c.Database.Port,
		c.Database.Database, c.Database.SSLMode)
}

func (c *Config) ReportsClientURL() string {
	return fmt.Sprintf("http://%s:%s", c.ReportsClient.Host, c.ReportsClient.Port)
}
