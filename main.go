package main
import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-app/taskscheduler"
)

func main() {
	// Parse command-line flags
	nodeType := flag.String("type", "scheduler", "Node type: scheduler or worker")
	nodeID := flag.String("id", "", "Node ID (optional)")
	schedulerAddr := flag.String("scheduler", "http://localhost:8080", "Scheduler URL (for worker nodes)")
	listenAddr := flag.String("listen", ":8080", "Listen address")
	flag.Parse()

	log.Printf("Starting %s node", *nodeType)

	// Create a context that will be canceled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down", sig)
		cancel()
	}()

	// Run the appropriate node type
	if *nodeType == "scheduler" {
		runScheduler(ctx, *listenAddr)
	} else if *nodeType == "worker" {
		runWorker(ctx, *nodeID, *schedulerAddr)
	} else {
		log.Fatalf("Unknown node type: %s", *nodeType)
	}
}

func runScheduler(ctx context.Context, listenAddr string) {
	// Create the scheduler
	scheduler := taskscheduler.NewScheduler()

	// Create and start the API server
	apiServer := taskscheduler.NewAPIServer(scheduler, listenAddr)

	// Start the API server in a goroutine
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("API server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown the API server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down API server: %v", err)
	}

	// Shutdown the scheduler
	scheduler.Shutdown()

	log.Println("Scheduler node shut down")
}

func runWorker(ctx context.Context, id string, schedulerURL string) {
	// Create the worker
	worker := taskscheduler.NewWorker(id, schedulerURL)

	// Register sample task handlers
	for name, handler := range taskscheduler.SampleHandlers() {
		worker.RegisterHandler(name, handler)
	}

	// Register with the scheduler
	// In a real implementation, this would make an HTTP request to the scheduler
	log.Printf("Worker %s registered with scheduler at %s", worker.ID, schedulerURL)

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown the worker
	worker.Shutdown()

	log.Println("Worker node shut down")
}
