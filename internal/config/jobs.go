package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type JobConfig struct{
	Jobs []Jobs `yaml:"jobs"`
}

type Jobs struct{
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	TargetFilter []string `yaml:"target_filter"`
	Steps        []string `yaml:"steps"`
}

func LoadJobs(path string) (*JobConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read jobs file: %w", err)
	}

	var cfg JobConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse jobs yaml: %w", err)
	}

	return &cfg, nil
}