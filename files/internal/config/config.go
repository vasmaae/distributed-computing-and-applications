package config

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ilyakaznacheev/cleanenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
)

type Config struct {
	Auth  auth.Config `yaml:"auth"`
	Queue struct {
		Brokers string `yaml:"brokers" env:"QUEUE_BROKERS" env-required:"true"`
		Topic   string `yaml:"topic" env:"QUEUE_TOPIC" env-default:"reports.files"`
		Group   string `yaml:"group" env:"QUEUE_GROUP" env-default:"files-service"`
	} `yaml:"queue"`
	HTTP struct {
		Host string `yaml:"host" env:"HTTP_HOST"`
		Port string `yaml:"port" env:"HTTP_PORT"`
	} `yaml:"http"`
	S3 struct {
		Endpoint        string `yaml:"endpoint" env:"S3_ENDPOINT"`
		Bucket          string `yaml:"bucket" env:"S3_BUCKET"`
		Region          string `yaml:"region" env:"S3_AWS_REGION"`
		AccessKeyId     string `yaml:"access_key_id" env:"S3_AWS_ACCESS_KEY_ID"`
		SecretAccessKey string `yaml:"secret_access_key" env:"S3_SECRET_ACCESS_KEY"`
	} `yaml:"s3"`
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

func (c *Config) S3Client() *awss3.Client {
	sdkCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(c.S3.Region),
		awsconfig.WithHTTPClient(&http.Client{Timeout: 10 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)}),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				c.S3.AccessKeyId,
				c.S3.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	return awss3.NewFromConfig(sdkCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(c.S3.Endpoint)
		o.UsePathStyle = true
	})
}
