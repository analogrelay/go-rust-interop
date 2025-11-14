package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	azurecosmos "github.com/analogrelay/go-rust-interop/go-wrapper"
	"github.com/spf13/cobra"
)

// pointReadCmd represents the pointRead command
var pointReadCmd = &cobra.Command{
	Use:   "pointRead",
	Short: "Benchmark point read operations against CosmosDB",
	Long: `Performs a benchmark that executes point read operations against a CosmosDB container.
Each iteration selects a random item ID and reads it from the RandomDocs container.
Measures and reports throughput and latency metrics.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := runPointReadBenchmark(cmd)
		if err != nil {
			fmt.Printf("Error running benchmark: %v\n", err)
			return
		}
	},
}

type BenchmarkResults struct {
	TotalOps     int           `json:"totalOps"`
	ElapsedTime  time.Duration `json:"elapsedTime"`
	OpsPerSecond float64       `json:"opsPerSecond"`
	LatencyMs    float64       `json:"latencyMs"`
}

func runPointReadBenchmark(cmd *cobra.Command) error {
	// Get configuration
	itemCount, err := cmd.Flags().GetInt("item-count")
	if err != nil {
		return fmt.Errorf("failed to get item-count: %w", err)
	}

	duration, err := cmd.Flags().GetDuration("duration")
	if err != nil {
		return fmt.Errorf("failed to get duration: %w", err)
	}

	partitionCount, err := cmd.Flags().GetInt("partition-count")
	if err != nil {
		return fmt.Errorf("failed to get partition-count: %w", err)
	}

	workers, err := cmd.Flags().GetInt("workers")
	if err != nil {
		return fmt.Errorf("failed to get workers: %w", err)
	}

	containerName, err := cmd.Flags().GetString("container")
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
	}

	// Create Cosmos client and get container
	client, err := createCosmosClient(cmd)
	if err != nil {
		return fmt.Errorf("failed to create Cosmos client: %w", err)
	}
	defer client.Close()

	dbClient, err := getTestDbClient(cmd, client)
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	containerClient, err := dbClient.ContainerClient(containerName)
	if err != nil {
		return fmt.Errorf("failed to get container client: %w", err)
	}
	defer containerClient.Close()

	fmt.Printf("Starting point read benchmark...\n")
	fmt.Printf("Item count: %d\n", itemCount)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Partition count: %d\n", partitionCount)
	fmt.Printf("Workers: %d\n", workers)
	fmt.Printf("Container: %s\n", containerName)
	fmt.Println()

	// Run benchmark
	results, err := executeBenchmark(cmd.Context(), containerClient, itemCount, partitionCount, workers, duration)
	if err != nil {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	// Print results
	printResults(results)
	return nil
}

func executeBenchmark(ctx context.Context, container *azurecosmos.ContainerClient, itemCount, partitionCount, workers int, duration time.Duration) (*BenchmarkResults, error) {
	startTime := time.Now()
	endTime := startTime.Add(duration)

	fmt.Printf("Benchmark started at %v with %d workers\n", startTime.Format("15:04:05.000"), workers)

	// Shared counters for all workers
	var totalOps int64
	var totalLatency int64

	// Create a context that will be canceled when the benchmark duration expires
	benchCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// WaitGroup to wait for all workers to complete
	var wg sync.WaitGroup

	// Channel to signal workers when to stop (for clean shutdown)
	stopChan := make(chan struct{})

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerBenchmark(benchCtx, container, itemCount, partitionCount, &totalOps, &totalLatency, stopChan, workerID)
		}(i)
	}

	// Progress reporting goroutine
	progressTicker := time.NewTicker(5 * time.Second)
	defer progressTicker.Stop()

	go func() {
		for {
			select {
			case <-progressTicker.C:
				currentOps := atomic.LoadInt64(&totalOps)
				elapsed := time.Since(startTime)
				currentOpsPerSec := float64(currentOps) / elapsed.Seconds()
				remaining := time.Until(endTime)
				if remaining > 0 {
					fmt.Printf("Progress: %d ops, %.1f ops/sec, %v remaining\n",
						currentOps, currentOpsPerSec, remaining.Round(time.Second))
				}
			case <-benchCtx.Done():
				return
			}
		}
	}()

	// Wait for benchmark duration or context cancellation
	<-benchCtx.Done()

	// Signal all workers to stop
	close(stopChan)

	// Wait for all workers to finish
	wg.Wait()

	actualElapsed := time.Since(startTime)
	finalOps := atomic.LoadInt64(&totalOps)
	finalLatency := atomic.LoadInt64(&totalLatency)

	if finalOps == 0 {
		return nil, fmt.Errorf("no operations completed")
	}

	results := &BenchmarkResults{
		TotalOps:     int(finalOps),
		ElapsedTime:  actualElapsed,
		OpsPerSecond: float64(finalOps) / actualElapsed.Seconds(),
		LatencyMs:    float64(finalLatency) / float64(finalOps) / 1e6, // Convert to ms
	}

	return results, nil
}

func workerBenchmark(ctx context.Context, container *azurecosmos.ContainerClient, itemCount, partitionCount int, totalOps, totalLatency *int64, stopChan chan struct{}, workerID int) {
	// Create a local random source for this worker to avoid contention
	localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopChan:
			return
		default:
			// Select random item ID
			itemIndex := localRand.Intn(itemCount)
			itemID := fmt.Sprintf("item%d", itemIndex)
			partitionKey := fmt.Sprintf("partition%d", itemIndex%partitionCount)

			// Measure point read latency
			opStart := time.Now()

			_, err := container.ReadItem(itemID, partitionKey)

			opEnd := time.Now()
			opLatency := opEnd.Sub(opStart)

			if err != nil {
				// Log error but don't stop the benchmark for individual failures
				fmt.Printf("Worker %d: Error reading item %s: %v\n", workerID, itemID, err)
				continue
			}

			// Atomically update counters
			atomic.AddInt64(totalOps, 1)
			atomic.AddInt64(totalLatency, opLatency.Nanoseconds())
		}
	}
}

func printResults(results *BenchmarkResults) {
	fmt.Printf("\n=== Benchmark Results ===\n")
	fmt.Printf("Total ops: %d\n", results.TotalOps)
	fmt.Printf("Total elapsed time: %v\n", results.ElapsedTime.Round(time.Millisecond))
	fmt.Printf("Ops/sec: %.2f\n", results.OpsPerSecond)
	fmt.Printf("Latency (mean): %.2f ms\n", results.LatencyMs)
	fmt.Printf("========================\n")

	// Print markdown table for README
	fmt.Printf("\n=== Markdown Table (Point Read Benchmark) ===\n")
	fmt.Printf("| Implementation | Total Ops | Duration (ms) | Ops/sec | Latency (ms) |\n")
	fmt.Printf("|---------------|-----------|---------------|---------|--------------|\n")
	fmt.Printf("| Go Wrapper | %d | %d | %.2f | %.2f |\n",
		results.TotalOps,
		results.ElapsedTime.Milliseconds(),
		results.OpsPerSecond,
		results.LatencyMs)
	fmt.Printf("============================================\n")
}

func init() {
	rootCmd.AddCommand(pointReadCmd)

	// Add benchmark-specific flags
	pointReadCmd.Flags().IntP("item-count", "i", 10000, "Total number of items in the database")
	pointReadCmd.Flags().DurationP("duration", "t", 60*time.Second, "Duration to run the benchmark")
	pointReadCmd.Flags().IntP("partition-count", "p", 10, "Number of partitions the items are distributed across")
	pointReadCmd.Flags().IntP("workers", "w", runtime.NumCPU(), "Number of concurrent workers")
	pointReadCmd.Flags().StringP("container", "c", "RandomDocs", "Container name")
}
