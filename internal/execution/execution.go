package execution

import (
	"time"
)

// Status represents the state of a run.
type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusAborted   Status = "aborted"
	StatusTimedOut  Status = "timed_out"
)

// StepExecutionState tracks the status of a specific workflow step.
type StepExecutionState struct {
	StepID      string    `json:"stepId"`
	Status      Status    `json:"status"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
}

// Execution represents a single run instance of a job or manual command.
type Execution struct {
	ID            string               `json:"id"`
	JobID         string               `json:"jobId,omitempty"` // empty if ad-hoc command
	Project       string               `json:"project"`
	Status        Status               `json:"status"`
	User          string               `json:"user"`
	StartedAt     time.Time            `json:"startedAt"`
	FinishedAt    time.Time            `json:"finishedAt,omitempty"`
	StepsState    []StepExecutionState `json:"stepsState,omitempty"`
	AdHocCommand  string               `json:"adHocCommand,omitempty"`
}

// Runner is the interface responsible for launching workflows and ad-hoc executions.
type Runner interface {
	RunJob(jobID string, options map[string]string, triggeredBy string) (*Execution, error)
	RunAdHoc(project string, command string, nodeFilter string, triggeredBy string) (*Execution, error)
	Abort(executionID string, abortedBy string) error
	GetExecution(id string) (*Execution, error)
}
