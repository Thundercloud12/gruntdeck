package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/execution"
	"github.com/Thundercloud12/gruntdeck/internal/job"
	"github.com/Thundercloud12/gruntdeck/internal/node"
	"github.com/Thundercloud12/gruntdeck/internal/project"
)

// Server encapsulates the HTTP api handlers and internal state.
type Server struct {
	mu         sync.RWMutex
	projects   map[string]*project.Project
	jobs       map[string]*job.Job
	executions map[string]*execution.Execution
}

// NewServer initializes an in-memory Gruntdeck API server.
func NewServer() *Server {
	s := &Server{
		projects:   make(map[string]*project.Project),
		jobs:       make(map[string]*job.Job),
		executions: make(map[string]*execution.Execution),
	}

	// Seed with a default project
	s.projects["default"] = &project.Project{
		Name:        "default",
		Description: "Default Gruntdeck Project",
		Config:      map[string]string{"executor.default": "local"},
	}

	return s
}

// Router sets up the HTTP router with Go 1.22 path value routing patterns.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)
	
	mux.HandleFunc("GET /api/v1/projects/{project}/nodes", s.handleListNodes)

	mux.HandleFunc("GET /api/v1/projects/{project}/jobs", s.handleListJobs)
	mux.HandleFunc("POST /api/v1/projects/{project}/jobs", s.handleCreateJob)

	mux.HandleFunc("POST /api/v1/jobs/{jobId}/run", s.handleRunJob)
	mux.HandleFunc("GET /api/v1/executions/{executionId}", s.handleGetExecution)

	// Health check endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	return s.loggingMiddleware(mux)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

// JSON utility helpers
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

// Projects handlers
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*project.Project, 0, len(s.projects))
	for _, p := range s.projects {
		list = append(list, p)
	}
	respondWithJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req project.Project
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Project name is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.projects[req.Name]; exists {
		respondWithError(w, http.StatusConflict, "Project already exists")
		return
	}

	s.projects[req.Name] = &req
	respondWithJSON(w, http.StatusCreated, req)
}

// Nodes handler
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("project")

	s.mu.RLock()
	_, exists := s.projects[projectName]
	s.mu.RUnlock()

	if !exists {
		respondWithError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Mocking node registry response
	nodes := []*node.Node{
		{
			Name:     "localhost",
			Hostname: "127.0.0.1",
			Username: "grunt",
			OSFamily: "unix",
			OSName:   "Linux",
			Tags:     []string{"local", "dev"},
		},
	}
	respondWithJSON(w, http.StatusOK, nodes)
}

// Jobs handlers
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("project")

	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*job.Job, 0)
	for _, j := range s.jobs {
		if j.Project == projectName {
			list = append(list, j)
		}
	}
	respondWithJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("project")

	s.mu.RLock()
	_, projExists := s.projects[projectName]
	s.mu.RUnlock()

	if !projExists {
		respondWithError(w, http.StatusNotFound, "Project not found")
		return
	}

	var req job.Job
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Job name is required")
		return
	}

	// Assign an ID and project
	req.ID = fmt.Sprintf("job_%d", time.Now().UnixNano())
	req.Project = projectName
	req.DateCreated = time.Now()
	req.LastUpdated = time.Now()

	s.mu.Lock()
	s.jobs[req.ID] = &req
	s.mu.Unlock()

	respondWithJSON(w, http.StatusCreated, req)
}

// Execution handlers
func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")

	s.mu.Lock()
	j, exists := s.jobs[jobID]
	if !exists {
		s.mu.Unlock()
		respondWithError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Triggering a simulated run
	execID := fmt.Sprintf("exec_%d", time.Now().UnixNano())
	exec := &execution.Execution{
		ID:        execID,
		JobID:     j.ID,
		Project:   j.Project,
		Status:    execution.StatusRunning,
		User:      "admin",
		StartedAt: time.Now(),
	}
	s.executions[execID] = exec
	s.mu.Unlock()

	// Simulate background completion
	go func(id string) {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		if e, ok := s.executions[id]; ok {
			e.Status = execution.StatusSucceeded
			e.FinishedAt = time.Now()
		}
		s.mu.Unlock()
	}(execID)

	respondWithJSON(w, http.StatusAccepted, exec)
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	execID := r.PathValue("executionId")

	s.mu.RLock()
	exec, exists := s.executions[execID]
	s.mu.RUnlock()

	if !exists {
		respondWithError(w, http.StatusNotFound, "Execution not found")
		return
	}

	respondWithJSON(w, http.StatusOK, exec)
}
