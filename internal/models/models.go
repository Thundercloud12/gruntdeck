package models

import (
	"encoding/json"
	"time"
)

type Target struct {
	ID      string   `json:"id"`
	Host    string   `json:"host"`
	Port    string   `json:"port"`
	User    string   `json:"user"`
	KeyPath string   `json:"key_path"`
	Tags    []string `json:"tags"`
}

type JobOption struct {
	ID           string   `json:"id"`
	JobID        string   `json:"job_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"` // "string", "int", "bool", "choice"
	Required     bool     `json:"required"`
	DefaultValue string   `json:"default_value"`
	Choices      []string `json:"choices"`
}

type Job struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	TargetFilter []string    `json:"target_filter"`
	Steps        []JobStep   `json:"steps"`
	Options      []JobOption `json:"options"`
}

type JobStep struct {
	ID         string          `json:"id"`
	JobID      string          `json:"job_id"`
	StepOrder  int             `json:"step_order"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type Execution struct {
	ID               string            `json:"id"`
	JobID            string            `json:"job_id"`
	Status           string            `json:"status"`
	Options          map[string]string `json:"options"`
	StartedAt        time.Time         `json:"started_at"`
	EndedAt          time.Time         `json:"ended_at"`
	TargetsTotal     int               `json:"targets_total"`
	TargetsSucceeded int               `json:"targets_succeeded"`
	TargetsFailed    int               `json:"targets_failed"`
}

type LogEntry struct {
	ID          string    `json:"id"`
	ExecutionID string    `json:"execution_id"`
	TargetID    string    `json:"target_id"`
	StepID      string    `json:"step_id"`
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
}
