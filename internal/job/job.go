package job

import "time"

// Option defines a parameter that can be passed to a job.
type Option struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	DefaultValue string   `json:"defaultValue,omitempty"`
	Required     bool     `json:"required"`
	Values       []string `json:"values,omitempty"`
}

// Step represents a single task in a job workflow.
type Step struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`        // "node-step" or "workflow-step"
	PluginType  string            `json:"pluginType"`  // "exec", "script", "command", etc.
	Configuration map[string]string `json:"configuration"`
	KeepGoingOnSuccess bool          `json:"keepGoingOnSuccess,omitempty"`
}

// Workflow defines a list of steps to execute and their execution strategy.
type Workflow struct {
	Steps            []Step `json:"steps"`
	KeepGoing        bool   `json:"keepGoing"`        // Keep executing subsequent steps if a step fails
	Strategy         string `json:"strategy"`         // "node-first" or "step-first"
}

// DispatchConfig contains node filtering and parallel execution parameters.
type DispatchConfig struct {
	NodeFilter        string `json:"nodeFilter,omitempty"`
	ThreadCount       int    `json:"threadCount"`
	KeepGoing         bool   `json:"keepGoing"`         // Keep going on remaining nodes if one node fails
	ExcludePrecedence bool   `json:"excludePrecedence"`
}

// Schedule holds cron scheduling details.
type Schedule struct {
	CronExpression string `json:"cronExpression,omitempty"`
	Enabled        bool   `json:"enabled"`
}

// Job represents a runnable job definition (ScheduledExecution).
type Job struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	GroupPath        string         `json:"groupPath,omitempty"`
	Description      string         `json:"description,omitempty"`
	Project          string         `json:"project"`
	Options          []Option       `json:"options,omitempty"`
	Workflow         Workflow       `json:"workflow"`
	DispatchConfig   DispatchConfig `json:"dispatchConfig"`
	Schedule         Schedule       `json:"schedule"`
	ExecutionEnabled bool           `json:"executionEnabled"`
	Retry            int            `json:"retry,omitempty"`
	RetryDelay       time.Duration  `json:"retryDelay,omitempty"`
	Timeout          time.Duration  `json:"timeout,omitempty"`
	DateCreated      time.Time      `json:"dateCreated"`
	LastUpdated      time.Time      `json:"lastUpdated"`
}

// Store defines persistence methods for Job definitions.
type Store interface {
	Save(j *Job) error
	Get(id string) (*Job, error)
	List(project string) ([]*Job, error)
	Delete(id string) error
}
