package taskscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// SampleHandlers returns a map of sample task handlers
func SampleHandlers() map[string]TaskHandler {
	return map[string]TaskHandler{
		"echo":                EchoHandler,
		"sleep":               SleepHandler,
		"fibonacci":           FibonacciHandler,
		"prime_factorization": PrimeFactorizationHandler,
	}
}

// EchoHandler simply returns the input data
func EchoHandler(ctx context.Context, args []byte) ([]byte, error) {
	return args, nil
}

// SleepArgs defines the arguments for the sleep handler
type SleepArgs struct {
	Duration time.Duration `json:"duration"`
}

// SleepHandler sleeps for the specified duration
func SleepHandler(ctx context.Context, args []byte) ([]byte, error) {
	var sleepArgs SleepArgs
	if err := json.Unmarshal(args, &sleepArgs); err != nil {
		return nil, fmt.Errorf("invalid sleep arguments: %w", err)
	}

	select {
	case <-time.After(sleepArgs.Duration):
		return []byte(fmt.Sprintf("Slept for %s", sleepArgs.Duration)), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// FibonacciArgs defines the arguments for the Fibonacci handler
type FibonacciArgs struct {
	N int `json:"n"`
}

// FibonacciHandler calculates the Nth Fibonacci number
func FibonacciHandler(ctx context.Context, args []byte) ([]byte, error) {
	var fibArgs FibonacciArgs
	if err := json.Unmarshal(args, &fibArgs); err != nil {
		return nil, fmt.Errorf("invalid fibonacci arguments: %w", err)
	}

	if fibArgs.N < 0 {
		return nil, fmt.Errorf("fibonacci argument must be non-negative")
	}

	result := calculateFibonacci(fibArgs.N)

	return []byte(fmt.Sprintf("Fibonacci(%d) = %d", fibArgs.N, result)), nil
}

// calculateFibonacci calculates the Nth Fibonacci number
func calculateFibonacci(n int) int {
	if n <= 1 {
		return n
	}

	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}

	return b
}

// PrimeFactorizationArgs defines the arguments for the prime factorization handler
type PrimeFactorizationArgs struct {
	N int64 `json:"n"`
}

// PrimeFactorizationHandler calculates the prime factors of a number
func PrimeFactorizationHandler(ctx context.Context, args []byte) ([]byte, error) {
	var primeArgs PrimeFactorizationArgs
	if err := json.Unmarshal(args, &primeArgs); err != nil {
		return nil, fmt.Errorf("invalid prime factorization arguments: %w", err)
	}

	if primeArgs.N <= 1 {
		return nil, fmt.Errorf("number must be greater than 1")
	}

	factors := findPrimeFactors(primeArgs.N)

	result, err := json.Marshal(map[string]interface{}{
		"number":  primeArgs.N,
		"factors": factors,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return result, nil
}

// findPrimeFactors finds all prime factors of a number
func findPrimeFactors(n int64) []int64 {
	factors := []int64{}

	// Check for divisibility by 2
	for n%2 == 0 {
		factors = append(factors, 2)
		n /= 2
	}

	// Check for divisibility by odd numbers
	for i := int64(3); i <= int64(math.Sqrt(float64(n))); i += 2 {
		for n%i == 0 {
			factors = append(factors, i)
			n /= i
		}
	}

	// If n is a prime number greater than 2
	if n > 2 {
		factors = append(factors, n)
	}

	return factors
}
