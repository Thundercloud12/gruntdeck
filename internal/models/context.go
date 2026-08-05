package models

// ExecutionContext aggregates all runtime metadata and options for an execution step.
type ExecutionContext struct {
	Execution Execution
	Job       Job
	Target    Target
	Options   map[string]string
	Data      map[string]string
}
