package queue

type ExecuteJobArgs struct {
	ExecutionID string            `json:"execution_id"`
	JobID       string            `json:"job_id"`
	TargetID    string            `json:"target_id"`
	Options     map[string]string `json:"options"`
}

func (ExecuteJobArgs) Kind() string {
	return "execute_job"
}
