package taskscheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Scheduler manages task distribution and execution
type Scheduler struct {
	workers        map[string]*Worker
	pendingTasks   []*Task
	runningTasks   map[string]*Task
	completedTasks map[string]*Task
	mu             sync.RWMutex

	// Channels for communication
	taskSubmissions chan *Task
	taskCompletions chan *Task
	workerUpdates   chan *Worker

	// Context for shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewScheduler creates a new task scheduler
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		workers:         make(map[string]*Worker),
		pendingTasks:    make([]*Task, 0),
		runningTasks:    make(map[string]*Task),
		completedTasks:  make(map[string]*Task),
		taskSubmissions: make(chan *Task, 100),
		taskCompletions: make(chan *Task, 100),
		workerUpdates:   make(chan *Worker, 100),
		ctx:             ctx,
		cancelFunc:      cancel,
	}

	go s.run()
	return s
}

// SubmitTask adds a new task to the scheduler
func (s *Scheduler) SubmitTask(task *Task) error {
	if task.ID == "" {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("failed to generate task ID: %w", err)
		}
		task.ID = id
	}

	task.Status = TaskStatusPending
	task.CreatedAt = time.Now()

	s.taskSubmissions <- task
	return nil
}

// RegisterWorker adds a new worker to the scheduler
func (s *Scheduler) RegisterWorker(worker *Worker) {
	if worker.ID == "" {
		id, err := generateID()
		if err != nil {
			log.Printf("Failed to generate worker ID: %v", err)
			return
		}
		worker.ID = id
	}

	worker.Status = WorkerStatusIdle
	worker.LastHeartbeat = time.Now()
	worker.CurrentTasks = make(map[string]*Task)

	s.workerUpdates <- worker
}

// UpdateWorker updates the status of a worker
func (s *Scheduler) UpdateWorker(worker *Worker) {
	s.workerUpdates <- worker
}

// CompleteTask marks a task as completed
func (s *Scheduler) CompleteTask(task *Task) {
	s.taskCompletions <- task
}

// GetTask retrieves a task by ID
func (s *Scheduler) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check pending tasks
	for _, task := range s.pendingTasks {
		if task.ID == id {
			return task, true
		}
	}

	// Check running tasks
	if task, ok := s.runningTasks[id]; ok {
		return task, true
	}

	// Check completed tasks
	if task, ok := s.completedTasks[id]; ok {
		return task, true
	}

	return nil, false
}

// GetWorker retrieves a worker by ID
func (s *Scheduler) GetWorker(id string) (*Worker, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	worker, ok := s.workers[id]
	return worker, ok
}

// ListTasks returns all tasks
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0, len(s.pendingTasks)+len(s.runningTasks)+len(s.completedTasks))

	// Add pending tasks
	result = append(result, s.pendingTasks...)

	// Add running tasks
	for _, task := range s.runningTasks {
		result = append(result, task)
	}

	// Add completed tasks
	for _, task := range s.completedTasks {
		result = append(result, task)
	}

	return result
}

// ListWorkers returns all workers
func (s *Scheduler) ListWorkers() []*Worker {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Worker, 0, len(s.workers))
	for _, worker := range s.workers {
		result = append(result, worker)
	}

	return result
}

// Shutdown stops the scheduler
func (s *Scheduler) Shutdown() {
	s.cancelFunc()
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return

		case task := <-s.taskSubmissions:
			s.mu.Lock()
			s.pendingTasks = append(s.pendingTasks, task)
			s.mu.Unlock()
			log.Printf("Task submitted: %s (%s)", task.Name, task.ID)

		case task := <-s.taskCompletions:
			s.mu.Lock()
			delete(s.runningTasks, task.ID)
			s.completedTasks[task.ID] = task

			// Update worker status if needed
			if worker, ok := s.workers[task.WorkerID]; ok {
				worker.mu.Lock()
				delete(worker.CurrentTasks, task.ID)
				if len(worker.CurrentTasks) == 0 {
					worker.Status = WorkerStatusIdle
				}
				worker.mu.Unlock()
			}

			s.mu.Unlock()
			log.Printf("Task completed: %s (%s)", task.Name, task.ID)

		case worker := <-s.workerUpdates:
			s.mu.Lock()
			s.workers[worker.ID] = worker
			s.mu.Unlock()
			log.Printf("Worker updated: %s", worker.ID)

		case <-ticker.C:
			s.scheduleTasks()
			s.checkWorkerHealth()
		}
	}
}

// scheduleTasks assigns pending tasks to available workers
func (s *Scheduler) scheduleTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pendingTasks) == 0 || len(s.workers) == 0 {
		return
	}

	// Sort tasks by priority (higher priority first)
	sort.Slice(s.pendingTasks, func(i, j int) bool {
		return s.pendingTasks[i].Priority > s.pendingTasks[j].Priority
	})

	for i := 0; i < len(s.pendingTasks); i++ {
		task := s.pendingTasks[i]

		// Check if dependencies are satisfied
		if !s.areDependenciesSatisfied(task) {
			continue
		}

		// Find suitable worker
		worker := s.findSuitableWorker(task)
		if worker == nil {
			continue
		}

		// Assign task to worker
		task.Status = TaskStatusRunning
		task.WorkerID = worker.ID
		now := time.Now()
		task.StartedAt = &now

		// Update worker state
		worker.mu.Lock()
		worker.CurrentTasks[task.ID] = task
		worker.Status = WorkerStatusBusy
		worker.mu.Unlock()

		// Move task from pending to running
		s.runningTasks[task.ID] = task
		s.pendingTasks = append(s.pendingTasks[:i], s.pendingTasks[i+1:]...)
		i-- // Adjust index after removal

		log.Printf("Assigned task %s (%s) to worker %s", task.Name, task.ID, worker.ID)
	}
}

// areDependenciesSatisfied checks if all dependencies of a task are completed
func (s *Scheduler) areDependenciesSatisfied(task *Task) bool {
	for _, depID := range task.Dependencies {
		depTask, ok := s.completedTasks[depID]
		if !ok || depTask.Status != TaskStatusCompleted {
			return false
		}
	}
	return true
}

// findSuitableWorker selects an appropriate worker for a task
func (s *Scheduler) findSuitableWorker(task *Task) *Worker {
	var bestWorker *Worker
	var minTasks int = -1

	for _, worker := range s.workers {
		// Skip offline workers
		if worker.Status == WorkerStatusOffline {
			continue
		}

		// Check if worker has required capabilities
		if !hasRequiredCapabilities(worker, task) {
			continue
		}

		// Select worker with fewest current tasks
		worker.mu.RLock()
		taskCount := len(worker.CurrentTasks)
		worker.mu.RUnlock()

		if minTasks == -1 || taskCount < minTasks {
			minTasks = taskCount
			bestWorker = worker
		}
	}

	return bestWorker
}

// hasRequiredCapabilities checks if a worker can handle a task
func hasRequiredCapabilities(worker *Worker, task *Task) bool {
	// For now, assume all workers can handle all tasks
	return true
}

// checkWorkerHealth monitors worker heartbeats
func (s *Scheduler) checkWorkerHealth() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	timeout := 30 * time.Second

	for id, worker := range s.workers {
		if now.Sub(worker.LastHeartbeat) > timeout {
			// Worker is considered offline
			worker.Status = WorkerStatusOffline

			// Reassign tasks from this worker
			for taskID, task := range worker.CurrentTasks {
				// Move task back to pending
				task.Status = TaskStatusPending
				task.WorkerID = ""
				s.pendingTasks = append(s.pendingTasks, task)

				// Remove from running tasks
				delete(s.runningTasks, taskID)
			}

			// Clear worker's tasks
			worker.mu.Lock()
			worker.CurrentTasks = make(map[string]*Task)
			worker.mu.Unlock()

			log.Printf("Worker %s marked as offline", id)
		}
	}
}

// generateID creates a random ID
func generateID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
