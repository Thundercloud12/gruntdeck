package variables_test

import (
	"testing"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/variables"
)

func TestResolveTemplate(t *testing.T) {
	execCtx := models.ExecutionContext{
		Execution: models.Execution{ID: "exec-123"},
		Job:       models.Job{ID: "job-deploy", Name: "Deploy App"},
		Target:    models.Target{ID: "t-1", Host: "10.0.0.5", User: "ubuntu", Port: "22"},
		Options: map[string]string{
			"APP_VERSION": "v2.0.1",
			"ENVIRONMENT": "production",
		},
	}

	input := "Deploying ${option.APP_VERSION} to ${node.host} for ${job.name} (Exec: ${execution.id})"
	expected := "Deploying v2.0.1 to 10.0.0.5 for Deploy App (Exec: exec-123)"

	result := variables.Resolve(input, execCtx)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
