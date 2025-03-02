package taskscheduler

import (
	"context"
	"sync"
	"time"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Task represents a unit of work to be executed
type Task struct {
	ID           string
	Name         string
	Handler      string   // Name of the handler function
	Args         []byte   // Serialized arguments
	Dependencies []string // IDs of dependent tasks
	Priority     int
	Status       TaskStatus
	Result       []byte
	Error        string
	WorkerID     string
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Metadata     map[string]string
}

// Worker represents a node that can execute tasks
type Worker struct {
	ID            string
	Capabilities  []string
	CurrentTasks  map[string]*Task
	LastHeartbeat time.Time
	Resources     Resources
	Status        WorkerStatus
	mu            sync.RWMutex
}

// Resources represents available computational resources
type Resources struct {
	CPU    int // percentage
	Memory int // MB
	Disk   int // MB
}

// WorkerStatus represents the current state of a worker
type WorkerStatus string

const (
	WorkerStatusIdle    WorkerStatus = "idle"
	WorkerStatusBusy    WorkerStatus = "busy"
	WorkerStatusOffline WorkerStatus = "offline"
)

// TaskHandler is a function that can execute a task
type TaskHandler func(ctx context.Context, args []byte) ([]byte, error)

// TaskHandlerRegistry stores available task handlers
type TaskHandlerRegistry struct {
	handlers map[string]TaskHandler
	mu       sync.RWMutex
}

// NewTaskHandlerRegistry creates a new registry for task handlers
func NewTaskHandlerRegistry() *TaskHandlerRegistry {
	return &TaskHandlerRegistry{
		handlers: make(map[string]TaskHandler),
	}
}

// Register adds a new task handler to the registry
func (r *TaskHandlerRegistry) Register(name string, handler TaskHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Get retrieves a task handler by name
func (r *TaskHandlerRegistry) Get(name string) (TaskHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[name]
	return handler, ok
}
