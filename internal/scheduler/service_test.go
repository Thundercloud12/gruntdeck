package scheduler_test

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestCronParser(t *testing.T) {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	expr := "0 0 2 * * *" // 2 AM every day
	sched, err := parser.Parse(expr)
	if err != nil {
		t.Fatalf("Failed to parse valid cron expression %q: %v", expr, err)
	}

	nextRun := sched.Next(time.Now())
	if nextRun.IsZero() {
		t.Errorf("Expected valid next run time, got zero time")
	}
}
