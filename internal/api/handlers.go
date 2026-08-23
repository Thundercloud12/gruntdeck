package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/queue"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/Thundercloud12/gruntdeck/internal/scheduler"
	"github.com/Thundercloud12/gruntdeck/internal/secrets"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	producer    *queue.Producer
	projects    repository.ProjectRepository
	jobs        repository.JobRepository
	executions  repository.ExecutionRepository
	logs        repository.LogRepository
	inventory   repository.InventoryRepository
	schedules   repository.ScheduleRepository
	credentials repository.CredentialRepository
	users       repository.UserRepository
	sessions    repository.SessionRepository
	scheduler   *scheduler.Service
	secSvc      *secrets.Service
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RunJobRequest struct {
	Options map[string]string `json:"options"`
}

type JobRequest struct {
	ID           string             `json:"id"`
	ProjectID    string             `json:"project_id"`
	Name         string             `json:"name"`
	TargetFilter []string           `json:"target_filter"`
	Steps        []models.JobStep   `json:"steps"`
	Options      []models.JobOption `json:"options"`
}

var validStepTypes = map[string]bool{
	"command":   true,
	"script":    true,
	"file-copy": true,
	"job-ref":   true,
}

type CreateCredentialRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "ssh_key", "password", "token"
	Payload string `json:"payload"`
}

type CreateTargetRequest struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"project_id"`
	Host         string   `json:"host"`
	Port         string   `json:"port"`
	User         string   `json:"user"`
	KeyPath      string   `json:"key_path"`
	CredentialID string   `json:"credential_id"`
	Tags         []string `json:"tags"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type JobStats struct {
	JobID              string  `json:"job_id"`
	TotalExecutions    int     `json:"total_executions"`
	SucceededCount     int     `json:"succeeded_count"`
	FailedCount        int     `json:"failed_count"`
	SuccessRate        float64 `json:"success_rate"`
	AvgDurationSeconds float64 `json:"avg_duration_seconds"`
}

func NewServer(
	producer *queue.Producer,
	projects repository.ProjectRepository,
	jobs repository.JobRepository,
	executions repository.ExecutionRepository,
	logs repository.LogRepository,
	inventory repository.InventoryRepository,
	schedules repository.ScheduleRepository,
	credentials repository.CredentialRepository,
	users repository.UserRepository,
	sessions repository.SessionRepository,
	schedService *scheduler.Service,
	secSvc *secrets.Service,
) *Server {
	s := &Server{
		producer:    producer,
		projects:    projects,
		jobs:        jobs,
		executions:  executions,
		logs:        logs,
		inventory:   inventory,
		schedules:   schedules,
		credentials: credentials,
		users:       users,
		sessions:    sessions,
		scheduler:   schedService,
		secSvc:      secSvc,
	}

	// Bootstrap initial admin user if none exists
	s.bootstrapAdminUser(context.Background())
	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public Auth Endpoints
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleGetMe)

	// Projects Endpoints
	mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)
	mux.HandleFunc("GET /api/v1/projects/{id}", s.handleGetProject)
	mux.HandleFunc("DELETE /api/v1/projects/{id}", s.handleDeleteProject)

	// Protected Application API Endpoints
	mux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	mux.HandleFunc("POST /api/v1/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("PUT /api/v1/jobs/{id}", s.handleUpdateJob)
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", s.handleDeleteJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}/stats", s.handleGetJobStats)
	mux.HandleFunc("GET /api/v1/jobs/{id}/executions", s.handleGetJobExecutions)
	mux.HandleFunc("POST /api/v1/jobs/{id}/run", s.handleRunJob)
	mux.HandleFunc("GET /api/v1/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/executions/{id}", s.handleGetExecution)
	mux.HandleFunc("GET /api/v1/executions/{id}/logs", s.handleGetLogs)
	mux.HandleFunc("GET /api/v1/targets", s.handleListTargets)
	mux.HandleFunc("POST /api/v1/targets", s.handleCreateTarget)
	mux.HandleFunc("PUT /api/v1/targets/{id}", s.handleUpdateTarget)
	mux.HandleFunc("DELETE /api/v1/targets/{id}", s.handleDeleteTarget)
	mux.HandleFunc("GET /api/v1/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/v1/schedules", s.handleCreateSchedule)
	mux.HandleFunc("PUT /api/v1/schedules/{id}", s.handleUpdateSchedule)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleDeleteSchedule)
	mux.HandleFunc("GET /api/v1/credentials", s.handleListCredentials)
	mux.HandleFunc("POST /api/v1/credentials", s.handleCreateCredential)
	mux.HandleFunc("PUT /api/v1/credentials/{id}", s.handleUpdateCredential)
	mux.HandleFunc("DELETE /api/v1/credentials/{id}", s.handleDeleteCredential)

	return mux
}

func (s *Server) getProjectContext(r *http.Request) string {
	if p := r.URL.Query().Get("project_id"); p != "" {
		return p
	}
	if p := r.Header.Get("X-Project-ID"); p != "" {
		return p
	}
	return ""
}

func (s *Server) bootstrapAdminUser(ctx context.Context) {
	if s.users == nil {
		return
	}
	existing, err := s.users.ListUsers(ctx)
	if err == nil && len(existing) > 0 {
		return // Admin user already exists
	}

	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "admin"
	}
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD environment variable is required to bootstrap the initial admin user")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return
	}

	_ = s.users.CreateUser(ctx, models.User{
		ID:           uuid.New().String(),
		Username:     adminUsername,
		PasswordHash: string(hash),
		Role:         "admin",
	})
}

// Authentication Middleware
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Public static assets & auth endpoint bypassing middleware
		if path == "/login.html" || path == "/style.css" || path == "/app.js" || path == "/api/v1/auth/login" || strings.HasPrefix(path, "/favicon") {
			next.ServeHTTP(w, r)
			return
		}

		if s.sessions == nil {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("gruntdeck_session")
		if err != nil || cookie.Value == "" {
			s.rejectUnauthenticated(w, r)
			return
		}

		sess, err := s.sessions.GetSessionByToken(r.Context(), cookie.Value)
		if err != nil || sess == nil {
			s.rejectUnauthenticated(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) rejectUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login.html", http.StatusFound)
}

// POST /api/v1/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := s.users.GetUserByUsername(r.Context(), req.Username)
	if err != nil || user == nil {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	sessionToken := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	sess := models.Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: expiresAt,
	}

	if err := s.sessions.CreateSession(r.Context(), sess); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gruntdeck_session",
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "authenticated",
		"username": user.Username,
		"role":     user.Role,
	})
}

// POST /api/v1/auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("gruntdeck_session")
	if err == nil && cookie.Value != "" {
		if s.sessions != nil {
			_ = s.sessions.DeleteSession(r.Context(), cookie.Value)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gruntdeck_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// GET /api/v1/auth/me
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("gruntdeck_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sess, err := s.sessions.GetSessionByToken(r.Context(), cookie.Value)
	if err != nil || sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := s.users.GetUserByID(r.Context(), sess.UserID)
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ==========================================
// Projects Handlers
// ==========================================

// GET /api/v1/projects
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeJSON(w, http.StatusOK, []models.Project{})
		return
	}
	projectsList, err := s.projects.ListProjects(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, projectsList)
}

// POST /api/v1/projects
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "project name is required", http.StatusBadRequest)
		return
	}

	id := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))

	project := models.Project{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.projects.CreateProject(r.Context(), project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// GET /api/v1/projects/{id}
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := s.projects.GetProjectByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

// DELETE /api/v1/projects/{id}
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing project id", http.StatusBadRequest)
		return
	}

	if err := s.projects.DeleteProject(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/v1/jobs
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	projectID := s.getProjectContext(r)
	jobsList, err := s.jobs.ListJobs(r.Context(), projectID)
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

// buildJobFromRequest validates a job payload and fills in generated IDs/defaults.
func buildJobFromRequest(req JobRequest, projectID string) (models.Job, error) {
	if req.Name == "" {
		return models.Job{}, fmt.Errorf("job name is required")
	}
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.ProjectID == "" {
		req.ProjectID = projectID
	}
	if req.ProjectID == "" {
		req.ProjectID = "default"
	}
	if req.TargetFilter == nil {
		req.TargetFilter = []string{}
	}

	for i := range req.Steps {
		if !validStepTypes[req.Steps[i].Type] {
			return models.Job{}, fmt.Errorf("invalid step type %q at step %d", req.Steps[i].Type, i+1)
		}
		if req.Steps[i].ID == "" {
			req.Steps[i].ID = uuid.New().String()
		}
		req.Steps[i].JobID = req.ID
		req.Steps[i].StepOrder = i
		if len(req.Steps[i].Attributes) == 0 {
			req.Steps[i].Attributes = json.RawMessage("{}")
		} else if !json.Valid(req.Steps[i].Attributes) {
			return models.Job{}, fmt.Errorf("invalid attributes JSON at step %d", i+1)
		}
	}

	for i := range req.Options {
		if req.Options[i].Name == "" {
			return models.Job{}, fmt.Errorf("option name is required at option %d", i+1)
		}
		if req.Options[i].ID == "" {
			req.Options[i].ID = uuid.New().String()
		}
		req.Options[i].JobID = req.ID
		if req.Options[i].Type == "" {
			req.Options[i].Type = "string"
		}
		if req.Options[i].Choices == nil {
			req.Options[i].Choices = []string{}
		}
	}

	return models.Job{
		ID:           req.ID,
		ProjectID:    req.ProjectID,
		Name:         req.Name,
		TargetFilter: req.TargetFilter,
		Steps:        req.Steps,
		Options:      req.Options,
	}, nil
}

// POST /api/v1/jobs
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	job, err := buildJobFromRequest(req, s.getProjectContext(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.jobs.AddJob(r.Context(), job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

// PUT /api/v1/jobs/{id}
func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.ID = id

	job, err := buildJobFromRequest(req, s.getProjectContext(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.jobs.UpdateJob(r.Context(), job); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// DELETE /api/v1/jobs/{id}
func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	if err := s.jobs.DeleteJob(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/v1/jobs/{id}/stats
func (s *Server) handleGetJobStats(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	allExecs, err := s.executions.ListExecutions(r.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := JobStats{
		JobID: jobID,
	}

	var totalDuration float64
	var completedCount int

	for _, ex := range allExecs {
		if ex.JobID == jobID {
			stats.TotalExecutions++
			st := strings.ToLower(ex.Status)
			if st == "succeeded" || st == "completed" {
				stats.SucceededCount++
			} else if st == "failed" {
				stats.FailedCount++
			}

			if !ex.StartedAt.IsZero() && !ex.EndedAt.IsZero() {
				dur := ex.EndedAt.Sub(ex.StartedAt).Seconds()
				if dur > 0 {
					totalDuration += dur
					completedCount++
				}
			}
		}
	}

	if stats.TotalExecutions > 0 {
		stats.SuccessRate = math.Round((float64(stats.SucceededCount)/float64(stats.TotalExecutions)*100)*10) / 10
	}
	if completedCount > 0 {
		stats.AvgDurationSeconds = math.Round((totalDuration/float64(completedCount))*10) / 10
	}

	writeJSON(w, http.StatusOK, stats)
}

// GET /api/v1/jobs/{id}/executions
func (s *Server) handleGetJobExecutions(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	allExecs, err := s.executions.ListExecutions(r.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jobExecs := make([]models.Execution, 0)
	for _, ex := range allExecs {
		if ex.JobID == jobID {
			jobExecs = append(jobExecs, ex)
		}
	}

	writeJSON(w, http.StatusOK, jobExecs)
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
	projectID := s.getProjectContext(r)
	executionsList, err := s.executions.ListExecutions(r.Context(), projectID)
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
	projectID := s.getProjectContext(r)
	targetsList, err := s.inventory.ListTargets(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, targetsList)
}

// POST /api/v1/targets
func (s *Server) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	var req CreateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Host == "" || req.User == "" {
		http.Error(w, "host and user are required", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Port == "" {
		req.Port = "22"
	}
	if req.ProjectID == "" {
		req.ProjectID = s.getProjectContext(r)
	}
	if req.ProjectID == "" {
		req.ProjectID = "default"
	}

	target := models.Target{
		ID:           req.ID,
		ProjectID:    req.ProjectID,
		Host:         req.Host,
		Port:         req.Port,
		User:         req.User,
		KeyPath:      req.KeyPath,
		CredentialID: req.CredentialID,
		Tags:         req.Tags,
	}

	if err := s.inventory.AddTarget(r.Context(), target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, target)
}

// PUT /api/v1/targets/{id}
func (s *Server) handleUpdateTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing target id", http.StatusBadRequest)
		return
	}

	var req CreateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Host == "" || req.User == "" {
		http.Error(w, "host and user are required", http.StatusBadRequest)
		return
	}

	if req.Port == "" {
		req.Port = "22"
	}
	if req.ProjectID == "" {
		req.ProjectID = s.getProjectContext(r)
	}
	if req.ProjectID == "" {
		req.ProjectID = "default"
	}

	target := models.Target{
		ID:           id,
		ProjectID:    req.ProjectID,
		Host:         req.Host,
		Port:         req.Port,
		User:         req.User,
		KeyPath:      req.KeyPath,
		CredentialID: req.CredentialID,
		Tags:         req.Tags,
	}

	if err := s.inventory.UpdateTarget(r.Context(), target); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, target)
}

// DELETE /api/v1/targets/{id}
func (s *Server) handleDeleteTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing target id", http.StatusBadRequest)
		return
	}

	if err := s.inventory.DeleteTarget(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/v1/schedules
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if s.schedules == nil {
		writeJSON(w, http.StatusOK, []models.Schedule{})
		return
	}
	projectID := s.getProjectContext(r)
	scheds, err := s.schedules.ListSchedules(r.Context(), projectID)
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
	if sched.ProjectID == "" {
		sched.ProjectID = s.getProjectContext(r)
	}
	if sched.ProjectID == "" {
		sched.ProjectID = "default"
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

// PUT /api/v1/schedules/{id}
func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing schedule id", http.StatusBadRequest)
		return
	}

	var sched models.Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if sched.JobID == "" || sched.CronExpression == "" {
		http.Error(w, "job_id and cron_expression are required", http.StatusBadRequest)
		return
	}

	sched.ID = id
	if sched.Timezone == "" {
		sched.Timezone = "UTC"
	}
	if sched.ProjectID == "" {
		sched.ProjectID = s.getProjectContext(r)
	}
	if sched.ProjectID == "" {
		sched.ProjectID = "default"
	}

	if err := s.schedules.UpdateSchedule(r.Context(), sched); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.scheduler != nil {
		if err := s.scheduler.AddScheduleToCron(sched); err != nil {
			http.Error(w, "schedule saved but failed to register cron: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	writeJSON(w, http.StatusOK, sched)
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

// GET /api/v1/credentials
func (s *Server) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	if s.credentials == nil {
		writeJSON(w, http.StatusOK, []models.Credential{})
		return
	}
	creds, err := s.credentials.ListCredentials(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, creds)
}

// POST /api/v1/credentials
func (s *Server) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var req CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Payload == "" {
		http.Error(w, "name and payload are required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = "ssh_key"
	}

	secSvc := s.secSvc
	if secSvc == nil {
		secSvc = secrets.NewService()
	}

	encrypted, nonce, err := secSvc.Encrypt([]byte(req.Payload))
	if err != nil {
		http.Error(w, "encryption failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cred := models.Credential{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Type:          req.Type,
		EncryptedData: encrypted,
		Nonce:         nonce,
	}

	if err := s.credentials.CreateCredential(r.Context(), cred); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":   cred.ID,
		"name": cred.Name,
		"type": cred.Type,
	})
}

// PUT /api/v1/credentials/{id}
// Rotating a credential in place (rather than delete+recreate) preserves its ID, so any
// Target.credential_id already pointing at it keeps working.
func (s *Server) handleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing credential id", http.StatusBadRequest)
		return
	}

	var req CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	existing, err := s.credentials.GetCredentialByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	cred := models.Credential{
		ID:            id,
		Name:          req.Name,
		Type:          req.Type,
		EncryptedData: existing.EncryptedData,
		Nonce:         existing.Nonce,
	}
	if cred.Type == "" {
		cred.Type = existing.Type
	}

	// Only re-encrypt when a new secret payload was actually supplied; an empty
	// payload means "rename/retype only", not "wipe the stored secret".
	if req.Payload != "" {
		secSvc := s.secSvc
		if secSvc == nil {
			secSvc = secrets.NewService()
		}
		encrypted, nonce, err := secSvc.Encrypt([]byte(req.Payload))
		if err != nil {
			http.Error(w, "encryption failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		cred.EncryptedData = encrypted
		cred.Nonce = nonce
	}

	if err := s.credentials.UpdateCredential(r.Context(), cred); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":   cred.ID,
		"name": cred.Name,
		"type": cred.Type,
	})
}

// DELETE /api/v1/credentials/{id}
func (s *Server) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing credential id", http.StatusBadRequest)
		return
	}

	if err := s.credentials.DeleteCredential(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Project-ID")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
