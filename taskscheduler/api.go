package taskscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// APIServer provides HTTP endpoints for interacting with the scheduler
type APIServer struct {
	scheduler *Scheduler
	server    *http.Server
	addr      string
}

// NewAPIServer creates a new API server for the scheduler
func NewAPIServer(scheduler *Scheduler, addr string) *APIServer {
	return &APIServer{
		scheduler: scheduler,
		addr:      addr,
	}
}

// Start begins listening for HTTP requests
func (s *APIServer) Start() error {
	mux := http.NewServeMux()

	// Task endpoints
	mux.HandleFunc("POST /tasks", s.handleSubmitTask)
	mux.HandleFunc("GET /tasks", s.handleListTasks)
	mux.HandleFunc("GET /tasks/{id}", s.handleGetTask)

	// Worker endpoints
	mux.HandleFunc("POST /workers", s.handleRegisterWorker)
	mux.HandleFunc("GET /workers", s.handleListWorkers)
	mux.HandleFunc("GET /workers/{id}", s.handleGetWorker)
	mux.HandleFunc("PUT /workers/{id}/heartbeat", s.handleWorkerHeartbeat)
	mux.HandleFunc("POST /workers/{id}/tasks/{taskId}/complete", s.handleTaskCompletion)

	// System endpoints
	mux.HandleFunc("GET /health", s.handleHealthCheck)

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("API server listening on %s", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown stops the API server
func (s *APIServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleSubmitTask handles task submission requests
func (s *APIServer) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if task.Name == "" || task.Handler == "" {
		http.Error(w, "Task name and handler are required", http.StatusBadRequest)
		return
	}

	if err := s.scheduler.SubmitTask(&task); err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit task: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     task.ID,
		"status": string(task.Status),
	})
}

// handleListTasks returns all tasks
func (s *APIServer) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.scheduler.ListTasks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// handleGetTask returns a specific task
func (s *APIServer) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	task, found := s.scheduler.GetTask(taskID)
	if !found {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// handleRegisterWorker registers a new worker
func (s *APIServer) handleRegisterWorker(w http.ResponseWriter, r *http.Request) {
	var workerInfo struct {
		ID           string    `json:"id"`
		Capabilities []string  `json:"capabilities"`
		Resources    Resources `json:"resources"`
	}

	if err := json.NewDecoder(r.Body).Decode(&workerInfo); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	worker := &Worker{
		ID:            workerInfo.ID,
		Capabilities:  workerInfo.Capabilities,
		Resources:     workerInfo.Resources,
		LastHeartbeat: time.Now(),
		Status:        WorkerStatusIdle,
		CurrentTasks:  make(map[string]*Task),
	}

	s.scheduler.RegisterWorker(worker)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     worker.ID,
		"status": string(worker.Status),
	})
}

// handleListWorkers returns all workers
func (s *APIServer) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	workers := s.scheduler.ListWorkers()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workers)
}

// handleGetWorker returns a specific worker
func (s *APIServer) handleGetWorker(w http.ResponseWriter, r *http.Request) {
	workerID := r.PathValue("id")
	if workerID == "" {
		http.Error(w, "Worker ID is required", http.StatusBadRequest)
		return
	}

	worker, found := s.scheduler.GetWorker(workerID)
	if !found {
		http.Error(w, "Worker not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(worker)
}

// handleWorkerHeartbeat updates a worker's heartbeat
func (s *APIServer) handleWorkerHeartbeat(w http.ResponseWriter, r *http.Request) {
	workerID := r.PathValue("id")
	if workerID == "" {
		http.Error(w, "Worker ID is required", http.StatusBadRequest)
		return
	}

	worker, found := s.scheduler.GetWorker(workerID)
	if !found {
		http.Error(w, "Worker not found", http.StatusNotFound)
		return
	}

	worker.LastHeartbeat = time.Now()
	s.scheduler.UpdateWorker(worker)

	w.WriteHeader(http.StatusNoContent)
}

// handleTaskCompletion marks a task as completed
func (s *APIServer) handleTaskCompletion(w http.ResponseWriter, r *http.Request) {
	workerID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	if workerID == "" || taskID == "" {
		http.Error(w, "Worker ID and Task ID are required", http.StatusBadRequest)
		return
	}

	var taskResult struct {
		Status TaskStatus `json:"status"`
		Result []byte     `json:"result,omitempty"`
		Error  string     `json:"error,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&taskResult); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	task, found := s.scheduler.GetTask(taskID)
	if !found {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if task.WorkerID != workerID {
		http.Error(w, "Task not assigned to this worker", http.StatusForbidden)
		return
	}

	task.Status = taskResult.Status
	task.Result = taskResult.Result
	task.Error = taskResult.Error
	now := time.Now()
	task.CompletedAt = &now

	s.scheduler.CompleteTask(task)

	w.WriteHeader(http.StatusNoContent)
}

// handleHealthCheck returns the health status of the scheduler
func (s *APIServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
