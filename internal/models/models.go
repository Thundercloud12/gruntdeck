package models

import (
	"encoding/json"
	"time"
)

type Target struct {
	ID      string
	Host    string
	Port    string
	User    string
	KeyPath string
	Tags    []string
}

type Job struct {
	ID           string
	Name         string
	TargetFilter []string
	Steps        []JobStep
}

type JobStep struct {
	ID         string
	JobID      string
	StepOrder  int
	Type       string
	Atrributes json.RawMessage
}

type Execution struct {
	ID               string
	JobID            string
	Status           string
	StartedAt        time.Time
	EndedAt          time.Time
	TargetsTotal     int
	TargetsSucceeded int
	TargetsFailed    int
}
