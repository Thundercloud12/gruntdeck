package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Step struct {
	Type       string   `yaml:"type"`                  // "command", "script", "file-copy", "job-ref"
	Value      string   `yaml:"value,omitempty"`       // Used for command type
	SourcePath string   `yaml:"source_path,omitempty"` // Used for script & file-copy
	DestPath   string   `yaml:"dest_path,omitempty"`   // Used for file-copy
	Args       []string `yaml:"args,omitempty"`        // Used for script arguments
	JobID      string   `yaml:"job_id,omitempty"`      // Used for job-ref
}

type JobConfig struct {
	Jobs []Jobs `yaml:"jobs"`
}

type Jobs struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	TargetFilter []string `yaml:"target_filter"`
	Steps        []Step   `yaml:"steps"`
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