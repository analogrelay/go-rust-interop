use std::sync::Arc;
use std::sync::atomic::{AtomicI64, Ordering};
use std::time::{Duration, Instant};

use anyhow::Result;
use azure_core::credentials::Secret;
use azure_data_cosmos::CosmosClient;
use azure_data_cosmos::clients::ContainerClient;
use clap::Parser;
use rand::Rng;
use serde::{Deserialize, Serialize};
use tokio::time::sleep;

// Well-known Cosmos DB Emulator key, not a secret.
const EMULATOR_KEY: &str =
    "C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw==";

#[derive(Parser)]
#[command(name = "rust-bench")]
#[command(about = "Rust benchmarks for Cosmos DB SDK")]
struct Cli {
    /// Cosmos DB endpoint URL
    #[arg(
        short = 'e',
        long = "endpoint",
        default_value = "https://localhost:8080"
    )]
    endpoint: String,

    /// Cosmos DB primary key (if not specified, uses Azure CLI credentials)
    #[arg(short = 'k', long = "key", default_value = EMULATOR_KEY)]
    key: String,

    /// Benchmarking database name
    #[arg(short = 'd', long = "database", default_value = "sdk-bench-db")]
    database: String,

    #[command(subcommand)]
    command: Commands,
}

#[derive(clap::Subcommand)]
enum Commands {
    /// Benchmark point read operations against CosmosDB
    PointRead {
        /// Total number of items in the database
        #[arg(short = 'i', long = "item-count", default_value = "10000")]
        item_count: i32,

        /// Duration to run the benchmark (in seconds)
        #[arg(short = 't', long = "duration", default_value = "60")]
        duration_seconds: u64,

        /// Number of partitions the items are distributed across
        #[arg(short = 'p', long = "partition-count", default_value = "10")]
        partition_count: i32,

        /// Number of concurrent workers
        #[arg(short = 'w', long = "workers", default_value_t = num_cpus::get())]
        workers: usize,
    },
}

#[derive(Serialize, Deserialize)]
struct BenchmarkResults {
    total_ops: i64,
    elapsed_time_ms: u64,
    ops_per_second: f64,
    latency_ms: f64,
}

#[derive(Serialize, Deserialize)]
struct RandomDocsItem {
    id: String,
    #[serde(rename = "partitionKey")]
    partition_key: String,
    data: String,
    #[serde(rename = "randomNumber")]
    random_number: i32,
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::PointRead {
            item_count,
            duration_seconds,
            partition_count,
            workers,
        } => {
            run_point_read_benchmark(
                &cli.endpoint,
                &cli.key,
                &cli.database,
                item_count,
                duration_seconds,
                partition_count,
                workers,
            )
            .await?;
        }
    }

    Ok(())
}

async fn run_point_read_benchmark(
    endpoint: &str,
    key: &str,
    database_name: &str,
    item_count: i32,
    duration_seconds: u64,
    partition_count: i32,
    workers: usize,
) -> Result<()> {
    // Create Cosmos client
    let credential = Secret::from(key.to_string());
    let client = CosmosClient::with_key(endpoint, credential, None)?;
    let database = client.database_client(database_name);
    let container = database.container_client("RandomDocs");

    println!("Starting point read benchmark...");
    println!("Item count: {}", item_count);
    println!("Duration: {}s", duration_seconds);
    println!("Partition count: {}", partition_count);
    println!("Workers: {}", workers);
    println!();

    // Run benchmark
    let results = execute_benchmark(
        &container,
        item_count,
        partition_count,
        workers,
        Duration::from_secs(duration_seconds),
    )
    .await?;

    // Print results
    print_results(&results);

    Ok(())
}

async fn execute_benchmark(
    container: &ContainerClient,
    item_count: i32,
    partition_count: i32,
    workers: usize,
    duration: Duration,
) -> Result<BenchmarkResults> {
    let start_time = Instant::now();
    println!(
        "Benchmark started at {} with {} workers",
        chrono::Utc::now().format("%H:%M:%S%.3f"),
        workers
    );

    // Shared counters for all workers
    let total_ops = Arc::new(AtomicI64::new(0));
    let total_latency_ns = Arc::new(AtomicI64::new(0));

    // Create cancellation token for clean shutdown
    let cancel_token = tokio_util::sync::CancellationToken::new();

    // Start workers
    let mut worker_handles = Vec::new();
    for worker_id in 0..workers {
        let container_clone = container.clone();
        let total_ops_clone = total_ops.clone();
        let total_latency_clone = total_latency_ns.clone();
        let cancel_clone = cancel_token.clone();

        let handle = tokio::spawn(async move {
            worker_benchmark(
                container_clone,
                item_count,
                partition_count,
                total_ops_clone,
                total_latency_clone,
                cancel_clone,
                worker_id,
            )
            .await;
        });

        worker_handles.push(handle);
    }

    // Progress reporting task
    let total_ops_progress = total_ops.clone();
    let cancel_progress = cancel_token.clone();
    let progress_handle = tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_secs(5));
        loop {
            tokio::select! {
                _ = interval.tick() => {
                    let current_ops = total_ops_progress.load(Ordering::Relaxed);
                    let elapsed = start_time.elapsed();
                    let ops_per_sec = current_ops as f64 / elapsed.as_secs_f64();
                    let remaining = duration.saturating_sub(elapsed);

                    if remaining > Duration::ZERO {
                        println!(
                            "Progress: {} ops, {:.1} ops/sec, {}s remaining",
                            current_ops,
                            ops_per_sec,
                            remaining.as_secs()
                        );
                    }
                }
                _ = cancel_progress.cancelled() => {
                    break;
                }
            }
        }
    });

    // Wait for benchmark duration
    sleep(duration).await;

    // Cancel all workers
    cancel_token.cancel();

    // Wait for all workers to finish
    for handle in worker_handles {
        handle.await?;
    }

    // Cancel progress reporting
    progress_handle.abort();

    let actual_elapsed = start_time.elapsed();
    let final_ops = total_ops.load(Ordering::Relaxed);
    let final_latency_ns = total_latency_ns.load(Ordering::Relaxed);

    if final_ops == 0 {
        return Err(anyhow::anyhow!("No operations completed"));
    }

    let results = BenchmarkResults {
        total_ops: final_ops,
        elapsed_time_ms: actual_elapsed.as_millis() as u64,
        ops_per_second: final_ops as f64 / actual_elapsed.as_secs_f64(),
        latency_ms: (final_latency_ns as f64 / final_ops as f64) / 1_000_000.0, // Convert to ms
    };

    Ok(results)
}

async fn worker_benchmark(
    container: ContainerClient,
    item_count: i32,
    partition_count: i32,
    total_ops: Arc<AtomicI64>,
    total_latency_ns: Arc<AtomicI64>,
    cancel_token: tokio_util::sync::CancellationToken,
    worker_id: usize,
) {
    loop {
        tokio::select! {
            _ = cancel_token.cancelled() => {
                break;
            }
            _ = async {
                // Select random item ID
                let item_index = rand::rng().random_range(0..item_count);
                let item_id = format!("item{}", item_index);
                let partition_key = format!("partition{}", item_index % partition_count);

                // Measure point read latency
                let op_start = Instant::now();

                let result = container
                    .read_item::<RandomDocsItem>(&partition_key, &item_id, None)
                    .await;

                let op_latency = op_start.elapsed();

                match result {
                    Ok(_) => {
                        // Successfully read item
                        total_ops.fetch_add(1, Ordering::Relaxed);
                        total_latency_ns.fetch_add(op_latency.as_nanos() as i64, Ordering::Relaxed);
                    }
                    Err(e) => {
                        // Log error but don't stop the benchmark for individual failures
                        eprintln!("Worker {}: Error reading item {}: {}", worker_id, item_id, e);
                    }
                }

                Ok::<(), anyhow::Error>(())
            } => {}
        }
    }
}

fn print_results(results: &BenchmarkResults) {
    println!();
    println!("=== Benchmark Results ===");
    println!("Total ops: {}", results.total_ops);
    println!("Total elapsed time: {}ms", results.elapsed_time_ms);
    println!("Ops/sec: {:.2}", results.ops_per_second);
    println!("Latency (mean): {:.2} ms", results.latency_ms);
    println!("========================");

    // Print markdown table for README
    println!();
    println!("=== Markdown Table (Point Read Benchmark) ===");
    println!("| Implementation | Total Ops | Duration (ms) | Ops/sec | Latency (ms) |");
    println!("|---------------|-----------|---------------|---------|--------------|");
    println!(
        "| Rust | {} | {} | {:.2} | {:.2} |",
        results.total_ops, results.elapsed_time_ms, results.ops_per_second, results.latency_ms
    );
    println!("============================================");
}
