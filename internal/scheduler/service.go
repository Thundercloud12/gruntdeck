package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/queue"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/robfig/cron/v3"
)

type Service struct {
	repo     repository.ScheduleRepository
	producer *queue.Producer
	cron     *cron.Cron
	entryIDs map[string]cron.EntryID
	mu       sync.Mutex
}

func NewService(repo repository.ScheduleRepository, producer *queue.Producer) *Service {
	c := cron.New(cron.WithParser(cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)))

	return &Service{
		repo:     repo,
		producer: producer,
		cron:     c,
		entryIDs: make(map[string]cron.EntryID),
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.cron.Start()

	schedules, err := s.repo.ListSchedules(ctx)
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	activeCount := 0
	for _, sched := range schedules {
		if sched.Enabled {
			if err := s.AddScheduleToCron(sched); err != nil {
				log.Printf("⚠️ Failed to register schedule %s (%s): %v", sched.ID, sched.CronExpression, err)
			} else {
				activeCount++
			}
		}
	}

	log.Printf("⏰ Scheduler service active with %d registered cron schedule(s).", activeCount)
	return nil
}

func (s *Service) Stop() {
	s.cron.Stop()
}

func (s *Service) AddScheduleToCron(sched models.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldEntry, exists := s.entryIDs[sched.ID]; exists {
		s.cron.Remove(oldEntry)
		delete(s.entryIDs, sched.ID)
	}

	if !sched.Enabled {
		return nil
	}

	cronExpr := sched.CronExpression
	if sched.Timezone != "" && sched.Timezone != "UTC" {
		cronExpr = fmt.Sprintf("CRON_TZ=%s %s", sched.Timezone, sched.CronExpression)
	}

	entryID, err := s.cron.AddFunc(cronExpr, func() {
		log.Printf("⏰ Cron trigger fired for Schedule '%s' (Job '%s')", sched.ID, sched.JobID)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		execID, targets, err := s.producer.EnqueueExecutionWithOptions(ctx, sched.JobID, sched.Options)
		if err != nil {
			log.Printf("❌ Failed to enqueue scheduled execution for Job '%s': %v", sched.JobID, err)
		} else {
			log.Printf("🚀 Scheduled execution %s enqueued for Job '%s' across %d target(s)", execID, sched.JobID, targets)
		}
	})

	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
	}

	s.entryIDs[sched.ID] = entryID
	return nil
}

func (s *Service) RemoveScheduleFromCron(scheduleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.entryIDs[scheduleID]; exists {
		s.cron.Remove(entryID)
		delete(s.entryIDs, scheduleID)
	}
}
