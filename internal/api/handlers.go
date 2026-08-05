package api

import (
	"encoding/json"
	"net/http"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/queue"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/Thundercloud12/gruntdeck/internal/scheduler"
	"github.com/google/uuid"
)

type Server struct {
	producer   *queue.Producer
	jobs       repository.JobRepository
	executions repository.ExecutionRepository
	logs       repository.LogRepository
	inventory  repository.InventoryRepository
	schedules  repository.ScheduleRepository
	scheduler  *scheduler.Service
}

type RunJobRequest struct {
	Options map[string]string `json:"options"`
}

func NewRouter(
	producer *queue.Producer,
	jobs repository.JobRepository,
	executions repository.ExecutionRepository,
	logs repository.LogRepository,
	inventory repository.InventoryRepository,
	schedules repository.ScheduleRepository,
	schedService *scheduler.Service,
) http.Handler {
	s := &Server{
		producer:   producer,
		jobs:       jobs,
		executions: executions,
		logs:       logs,
		inventory:  inventory,
		schedules:  schedules,
		scheduler:  schedService,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/run", s.handleRunJob)
	mux.HandleFunc("GET /api/v1/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/executions/{id}", s.handleGetExecution)
	mux.HandleFunc("GET /api/v1/executions/{id}/logs", s.handleGetLogs)
	mux.HandleFunc("GET /api/v1/targets", s.handleListTargets)
	mux.HandleFunc("GET /api/v1/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/v1/schedules", s.handleCreateSchedule)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleDeleteSchedule)

	return enableCORS(mux)
}

// GET /api/v1/jobs
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobsList, err := s.jobs.ListJobs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, jobsList)
}

// GET /api/v1/jobs/{id}
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	job, err := s.jobs.GetJobByID(r.Context(), jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// POST /api/v1/jobs/{id}/run
func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	var req RunJobRequest
	if r.Header.Get("Content-Type") == "application/json" && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	executionID, targetCount, err := s.producer.EnqueueExecutionWithOptions(r.Context(), jobID, req.Options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"execution_id": executionID,
		"job_id":       jobID,
		"status":       "queued",
		"targets":      targetCount,
	})
}

// GET /api/v1/executions
func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	executionsList, err := s.executions.ListExecutions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, executionsList)
}

// GET /api/v1/executions/{id}
func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	executionID := r.PathValue("id")
	execution, err := s.executions.GetExecutionByID(r.Context(), executionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if execution == nil {
		http.Error(w, "execution not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, execution)
}

// GET /api/v1/executions/{id}/logs
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	executionID := r.PathValue("id")
	logEntries, err := s.logs.GetLogsByExecutionID(r.Context(), executionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, logEntries)
}

// GET /api/v1/targets
func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	targetsList, err := s.inventory.ListTargets(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, targetsList)
}

// GET /api/v1/schedules
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if s.schedules == nil {
		writeJSON(w, http.StatusOK, []models.Schedule{})
		return
	}
	scheds, err := s.schedules.ListSchedules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, scheds)
}

// POST /api/v1/schedules
func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var sched models.Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if sched.JobID == "" || sched.CronExpression == "" {
		http.Error(w, "job_id and cron_expression are required", http.StatusBadRequest)
		return
	}

	if sched.ID == "" {
		sched.ID = uuid.New().String()
	}
	if sched.Timezone == "" {
		sched.Timezone = "UTC"
	}
	sched.Enabled = true

	if err := s.schedules.CreateSchedule(r.Context(), sched); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.scheduler != nil {
		if err := s.scheduler.AddScheduleToCron(sched); err != nil {
			http.Error(w, "schedule saved but failed to register cron: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	writeJSON(w, http.StatusCreated, sched)
}

// DELETE /api/v1/schedules/{id}
func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing schedule id", http.StatusBadRequest)
		return
	}

	if err := s.schedules.DeleteSchedule(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.scheduler != nil {
		s.scheduler.RemoveScheduleFromCron(id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
