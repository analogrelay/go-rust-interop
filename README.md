# Go/Rust Interop Benchmarks

This repository contains tests to benchmark performance of the Go SDK, the Rust SDK, and a potential Go wrapper around the Rust SDK for Cosmos DB.

## Point Reads

This table compares the performance of point reads across the three implementations.

| Implementation | Total Ops | Duration (ms) | Ops/sec | Latency (ms) |
|---------------|-----------|---------------|---------|--------------|
| Rust | 16973 | 60003 | 282.87 | 70.63 |
| Go | 17693 | 60000 | 294.88 | 67.79 |