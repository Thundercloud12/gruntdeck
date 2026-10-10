package internal

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

type Task struct {
	ID        int    `json:"id"`
	Title     string `json:"string"`
	Completed bool   `json:"completed"`
}

var tasks []Task

func listTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, tasks)
}

func createTask(w http.ResponseWriter, r *http.Request) {
	var req Task
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", 400)
		return
	}

	req = Task{
		ID:        req.ID,
		Title:     req.Title,
		Completed: req.Completed,
	}

	tasks = append(tasks, req)

	writeJSON(w, 200, map[string]any{
		"ID":        req.ID,
		"Title":     req.Title,
		"Completed": req.Completed,
	})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /tasks", listTasks)
	mux.HandleFunc("POST /tasks", createTask)
}
