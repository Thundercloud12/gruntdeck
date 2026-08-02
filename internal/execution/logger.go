package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/google/uuid"
)

func (s *Service) recordLog(ctx context.Context, executionID, targetHost, stepID, level, msg string, err error) {
	if s.logs == nil || executionID == "" {
		return
	}
	if err != nil {
		level = "error"
		msg = fmt.Sprintf("%s | Error: %v", msg, err)
	}
	_ = s.logs.AddLogEntry(ctx, models.LogEntry{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		TargetID:    targetHost,
		StepID:      stepID,
		Timestamp:   time.Now(),
		Level:       level,
		Message:     msg,
	})
}
