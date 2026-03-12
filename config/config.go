package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Email    EmailConfig
}

type ServerConfig struct {
	Origin string
	Port   string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	Path     string
}

type AuthConfig struct {
	Key string
}

type EmailConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

const configPath = "config/config.yaml"

func LoadConfig() (Config, error) {
	var cfg Config

	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		zap.L().Error(err.Error())
		return cfg, nil
	}

	if err := v.Unmarshal(&cfg); err != nil {
		zap.L().Error(err.Error())
		return cfg, nil
	}

	return cfg, nil
}
