# Distributed Task Scheduler

A scalable, distributed task scheduler built with Go, designed to handle task distribution across multiple worker nodes.

## Features

- **Distributed Architecture**: Separate scheduler and worker nodes
- **RESTful API**: HTTP API for task submission and management
- **Task Prioritization**: Tasks can be assigned priorities
- **Task Dependencies**: Support for task dependencies
- **Worker Health Monitoring**: Automatic detection of offline workers
- **Extensible Task Handlers**: Easy to add new task types
- **Fault Tolerance**: Tasks from failed workers are automatically reassigned

## Architecture

The system consists of two main components:

1. **Scheduler Node**: Manages task distribution, worker registration, and task scheduling
2. **Worker Nodes**: Execute tasks and report results back to the scheduler

## Getting Started

### Prerequisites

- Go 1.22 or later

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/go-app.git
   cd go-app
   ```

2. Build the application:
   ```bash
   go build
   ```

### Running the Scheduler

```bash
./go-app -type=scheduler -listen=:8080
```

Options:
- `-listen`: Address to listen on (default: `:8080`)

### Running a Worker

```bash
./go-app -type=worker -id=worker1 -scheduler=http://localhost:8080
```

Options:
- `-id`: Worker ID (optional, will be generated if not provided)
- `-scheduler`: URL of the scheduler (default: `http://localhost:8080`)

## API Reference

### Task Management

#### Submit a Task

```
POST /tasks
```

Request body:
```json
{
  "name": "Task Name",
  "handler": "handler_name",
  "args": "base64_encoded_arguments",
  "dependencies": ["task_id1", "task_id2"],
  "priority": 10
}
```

Response:
```json
{
  "id": "task_id",
  "status": "pending"
}
```

#### List All Tasks

```
GET /tasks
```

#### Get Task Details

```
GET /tasks/{id}
```

### Worker Management

#### Register a Worker

```
POST /workers
```

Request body:
```json
{
  "id": "worker_id",
  "capabilities": ["capability1", "capability2"],
  "resources": {
    "cpu": 100,
    "memory": 1024,
    "disk": 10240
  }
}
```

#### List All Workers

```
GET /workers
```

#### Get Worker Details

```
GET /workers/{id}
```

#### Send Worker Heartbeat

```
PUT /workers/{id}/heartbeat
```

#### Complete a Task

```
POST /workers/{id}/tasks/{taskId}/complete
```

Request body:
```json
{
  "status": "completed",
  "result": "base64_encoded_result",
  "error": "error_message_if_any"
}
```

### System

#### Health Check

```
GET /health
```

## Built-in Task Handlers

The system comes with several built-in task handlers:

1. **echo**: Simply returns the input data
2. **sleep**: Sleeps for the specified duration
3. **fibonacci**: Calculates the Nth Fibonacci number
4. **prime_factorization**: Finds the prime factors of a number

### Example: Submitting a Fibonacci Task

1. Encode the arguments in base64:
   ```bash
   echo -n '{"n":10}' | base64
   # Output: eyJuIjoxMH0=
   ```

2. Submit the task:
   ```bash
   curl -X POST http://localhost:8080/tasks \
     -H "Content-Type: application/json" \
     -d '{"name":"Calculate Fibonacci","handler":"fibonacci","args":"eyJuIjoxMH0="}'
   ```

## Extending with Custom Task Handlers

You can add custom task handlers by implementing the `TaskHandler` interface and registering them with the worker:

```go
// Define your handler
func MyCustomHandler(ctx context.Context, args []byte) ([]byte, error) {
    // Parse arguments
    var myArgs struct {
        Param1 string `json:"param1"`
        Param2 int    `json:"param2"`
    }
    if err := json.Unmarshal(args, &myArgs); err != nil {
        return nil, err
    }
    
    // Process the task
    result := doSomething(myArgs.Param1, myArgs.Param2)
    
    // Return the result
    return json.Marshal(result)
}

// Register your handler with the worker
worker.RegisterHandler("my_custom_handler", MyCustomHandler)
```

## Architecture Details

### Task Lifecycle

1. **Submission**: Tasks are submitted to the scheduler via the API
2. **Scheduling**: The scheduler assigns tasks to available workers based on priority and dependencies
3. **Execution**: Workers execute the tasks and report results back to the scheduler
4. **Completion**: The scheduler marks tasks as completed or failed

### Worker Lifecycle

1. **Registration**: Workers register with the scheduler
2. **Heartbeat**: Workers periodically send heartbeats to the scheduler
3. **Task Assignment**: The scheduler assigns tasks to workers
4. **Task Execution**: Workers execute tasks and report results
5. **Offline Detection**: The scheduler detects offline workers and reassigns their tasks

## Future Improvements

- Persistent storage for tasks and state
- Authentication and authorization
- Monitoring and metrics
- Task result storage and retrieval
- More sophisticated scheduling algorithms
- Network communication between components
- Proper serialization/deserialization of task arguments and results

## License

This project is licensed under the MIT License - see the LICENSE file for details. 