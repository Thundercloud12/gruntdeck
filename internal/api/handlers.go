package api

import (
	"context"
	"encoding/json"
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

type RunJobRequest struct {
	Options map[string]string `json:"options"`
}

type CreateCredentialRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "ssh_key", "password", "token"
	Payload string `json:"payload"`
}

type CreateTargetRequest struct {
	ID           string   `json:"id"`
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

func NewServer(
	producer *queue.Producer,
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

	// Protected Application API Endpoints
	mux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/run", s.handleRunJob)
	mux.HandleFunc("GET /api/v1/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/executions/{id}", s.handleGetExecution)
	mux.HandleFunc("GET /api/v1/executions/{id}/logs", s.handleGetLogs)
	mux.HandleFunc("GET /api/v1/targets", s.handleListTargets)
	mux.HandleFunc("POST /api/v1/targets", s.handleCreateTarget)
	mux.HandleFunc("DELETE /api/v1/targets/{id}", s.handleDeleteTarget)
	mux.HandleFunc("GET /api/v1/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/v1/schedules", s.handleCreateSchedule)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleDeleteSchedule)
	mux.HandleFunc("GET /api/v1/credentials", s.handleListCredentials)
	mux.HandleFunc("POST /api/v1/credentials", s.handleCreateCredential)
	mux.HandleFunc("DELETE /api/v1/credentials/{id}", s.handleDeleteCredential)

	return mux
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
		adminPassword = "admin"
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

	target := models.Target{
		ID:           req.ID,
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
