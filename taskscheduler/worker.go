package taskscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// WorkerNode represents a worker in the distributed system
type WorkerNode struct {
	ID           string
	Capabilities []string
	Resources    Resources
	SchedulerURL string // URL of the scheduler API
	tasks        map[string]*Task
	registry     *TaskHandlerRegistry
	mu           sync.RWMutex

	// Channels
	taskQueue   chan *Task
	resultQueue chan *Task

	// Context for shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewWorker creates a new worker node
func NewWorker(id string, schedulerURL string) *WorkerNode {
	ctx, cancel := context.WithCancel(context.Background())

	w := &WorkerNode{
		ID:           id,
		Capabilities: []string{"default"},
		Resources: Resources{
			CPU:    100,
			Memory: 1024,
			Disk:   10240,
		},
		SchedulerURL: schedulerURL,
		tasks:        make(map[string]*Task),
		registry:     NewTaskHandlerRegistry(),
		taskQueue:    make(chan *Task, 10),
		resultQueue:  make(chan *Task, 10),
		ctx:          ctx,
		cancelFunc:   cancel,
	}

	go w.run()
	go w.sendHeartbeats()

	return w
}

// RegisterHandler adds a task handler to the worker
func (w *WorkerNode) RegisterHandler(name string, handler TaskHandler) {
	w.registry.Register(name, handler)
}

// AssignTask assigns a task to this worker
func (w *WorkerNode) AssignTask(task *Task) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we already have this task
	if _, exists := w.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already assigned to this worker", task.ID)
	}

	// Check if we can handle this task
	if _, ok := w.registry.Get(task.Handler); !ok {
		return fmt.Errorf("worker does not support handler %s", task.Handler)
	}

	w.tasks[task.ID] = task
	w.taskQueue <- task
	return nil
}

// GetTaskStatus returns the status of a task
func (w *WorkerNode) GetTaskStatus(taskID string) (TaskStatus, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	task, ok := w.tasks[taskID]
	if !ok {
		return "", fmt.Errorf("task %s not found on this worker", taskID)
	}

	return task.Status, nil
}

// Shutdown stops the worker
func (w *WorkerNode) Shutdown() {
	w.cancelFunc()
}

// run is the main worker loop
func (w *WorkerNode) run() {
	for {
		select {
		case <-w.ctx.Done():
			return

		case task := <-w.taskQueue:
			w.executeTask(task)

		case result := <-w.resultQueue:
			w.sendResultToScheduler(result)
		}
	}
}

// executeTask runs a task and captures the result
func (w *WorkerNode) executeTask(task *Task) {
	log.Printf("Worker %s executing task %s (%s)", w.ID, task.Name, task.ID)

	// Update task status
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	// Get the handler
	handler, ok := w.registry.Get(task.Handler)
	if !ok {
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("handler %s not found", task.Handler)
		w.resultQueue <- task
		return
	}

	// Execute the task in a goroutine
	go func() {
		taskCtx, cancel := context.WithTimeout(w.ctx, 10*time.Minute)
		defer cancel()

		result, err := handler(taskCtx, task.Args)

		completedAt := time.Now()
		task.CompletedAt = &completedAt

		if err != nil {
			task.Status = TaskStatusFailed
			task.Error = err.Error()
		} else {
			task.Status = TaskStatusCompleted
			task.Result = result
		}

		w.mu.Lock()
		delete(w.tasks, task.ID)
		w.mu.Unlock()

		w.resultQueue <- task
	}()
}

// sendResultToScheduler reports task completion to the scheduler
func (w *WorkerNode) sendResultToScheduler(task *Task) {
	log.Printf("Worker %s completed task %s (%s) with status %s",
		w.ID, task.Name, task.ID, task.Status)

	// In a real implementation, this would make an HTTP request to the scheduler
	// For now, we'll just log the result
	log.Printf("Task result: %s", string(task.Result))
}

// sendHeartbeats periodically sends heartbeats to the scheduler
func (w *WorkerNode) sendHeartbeats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a single heartbeat to the scheduler
func (w *WorkerNode) sendHeartbeat() {
	w.mu.RLock()
	taskCount := len(w.tasks)
	w.mu.RUnlock()

	log.Printf("Worker %s sending heartbeat (tasks: %d)", w.ID, taskCount)

	// In a real implementation, this would make an HTTP request to the scheduler
}

// WorkerInfo represents the worker information sent to the scheduler
type WorkerInfo struct {
	ID           string
	Capabilities []string
	Resources    Resources
	TaskCount    int
	Status       WorkerStatus
}

// GetWorkerInfo returns information about this worker
func (w *WorkerNode) GetWorkerInfo() WorkerInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	status := WorkerStatusIdle
	if len(w.tasks) > 0 {
		status = WorkerStatusBusy
	}

	return WorkerInfo{
		ID:           w.ID,
		Capabilities: w.Capabilities,
		Resources:    w.Resources,
		TaskCount:    len(w.tasks),
		Status:       status,
	}
}

// MarshalJSON implements json.Marshaler for Task
func (t *Task) MarshalJSON() ([]byte, error) {
	type Alias Task
	return json.Marshal(&struct {
		*Alias
		StartedAt   string `json:"started_at,omitempty"`
		CompletedAt string `json:"completed_at,omitempty"`
	}{
		Alias: (*Alias)(t),
		StartedAt: func() string {
			if t.StartedAt != nil {
				return t.StartedAt.Format(time.RFC3339)
			}
			return ""
		}(),
		CompletedAt: func() string {
			if t.CompletedAt != nil {
				return t.CompletedAt.Format(time.RFC3339)
			}
			return ""
		}(),
	})
}
