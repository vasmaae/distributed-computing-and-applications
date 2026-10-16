package config

import (
	"fmt"
	"log"

	"github.com/ilyakaznacheev/cleanenv"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
)

type Config struct {
	Auth auth.Config `yaml:"auth"`
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

	if err := cleanenv.ReadEnv(&cfg); err == nil {
		return &cfg
	}

	log.Fatal("config not found")
	return nil
}

func (c *Config) EmployeesClientURL() string {
	return fmt.Sprintf("http://%s:%s", c.EmployeesClient.Host, c.EmployeesClient.Port)
}
