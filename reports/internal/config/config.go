package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"

	"github.com/ilyakaznacheev/cleanenv"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
)

type Config struct {
	Auth     auth.Config `yaml:"auth"`
	Database struct {
		Host     string `yaml:"host" env:"DATABASE_HOST" env-required:"true"`
		Port     string `yaml:"port" env:"DATABASE_PORT" env-default:"5432"`
		Username string `yaml:"username" env:"DATABASE_USERNAME" env-required:"true"`
		Password string `yaml:"password" env:"DATABASE_PASSWORD" env-required:"true"`
		Database string `yaml:"database" env:"DATABASE_DATABASE" env-default:"reports"`
		SSLMode  string `yaml:"ssl_mode" env:"DATABASE_SSL_MODE" env-default:"disable"`
	} `yaml:"database"`
	HTTP struct {
		Host string `yaml:"host" env:"HTTP_HOST"`
		Port string `yaml:"port" env:"HTTP_PORT"`
	} `yaml:"http"`
	EmployeesClient struct {
		Host string `yaml:"host" env:"EMPLOYEES_CLIENT_HOST"`
		Port string `yaml:"port" env:"EMPLOYEES_CLIENT_PORT"`
	} `yaml:"employees_client"`
	Queue struct {
		Brokers string `yaml:"brokers" env:"QUEUE_BROKERS" env-required:"true"`
		Topic   string `yaml:"topic" env:"QUEUE_TOPIC" env-default:"reports.files"`
	} `yaml:"queue"`
}

func MustLoad() *Config {
	var cfg Config

	if err := cleanenv.ReadConfig("config.yaml", &cfg); err == nil {
		return &cfg
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}
	return &cfg
}

func (c *Config) EmployeesClientURL() string {
	return fmt.Sprintf("http://%s:%s", c.EmployeesClient.Host, c.EmployeesClient.Port)
}

func (c *Config) DSN() string {
	u := url.URL{
		Scheme: "postgres", User: url.UserPassword(c.Database.Username, c.Database.Password),
		Host: net.JoinHostPort(c.Database.Host, c.Database.Port), Path: c.Database.Database,
	}
	q := url.Values{"sslmode": {c.Database.SSLMode}, "connect_timeout": {"5"}}
	u.RawQuery = q.Encode()
	return u.String()
}
