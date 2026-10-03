package config

import (
	"fmt"
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTP struct {
		Host string `yaml:"host" env:"HTTP_HOST"`
		Port string `yaml:"port" env:"HTTP_PORT"`
	} `yaml:"http"`
	EmployeesClient struct {
		Host string `yaml:"host" env:"EMPLOYEES_CLIENT_HOST"`
		Port string `yaml:"port" env:"EMPLOYEES_CLIENT_PORT"`
	} `yaml:"employees_client"`
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
