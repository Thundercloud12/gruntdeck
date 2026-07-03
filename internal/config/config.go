package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Target struct{
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	User    string `yaml:"user"`
	KeyPath string `yaml:"key_path"`
}

type Config struct{
	Target []Target `yaml:"targets"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	return &cfg, nil
}

