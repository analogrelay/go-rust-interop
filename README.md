# Go/Rust Interop Benchmarks

This repository contains tests to benchmark performance of the Go SDK, the Rust SDK, and a potential Go wrapper around the Rust SDK for Cosmos DB.

## Current Results

### Machine Configuration

Benchmarks were collected on an Azure VM under the following conditions:

* SKU: Standard_D2s_v3 (2 vCPUs, 8 GiB RAM)
* OS: Ubuntu 22.04 LTS
* Region: Canada Central
* Cosmos DB Account Region: Canada Central
* Accelerated Networking: Enabled
* Worker Count: 16 workers (tested to ensure average CPU load of 80% on each core in the Go/Rust native benchmarks)

### Point Reads

This table compares the performance of point reads across the three implementations.

| Implementation | Total Ops | Duration (ms) | Ops/sec | Latency (ms) | Rough CPU Utilization |
|---------------|-----------|---------------|---------|--------------|-------|
| Rust | 502760 | 60002 | 8378.97 | 1.91 | 75% per core |
| Go | 425800 | 60000 | 7096.59 | 2.25 | 75% per core |
| Go Wrapper | 440182 | 60004 | 7335.80 | 2.18 | 95% per core |

## Setup

### Prerequisites

- Go 1.25+ installed
- Rust toolchain installed (for building the native library)
- `pkg-config` installed
- Access to a Cosmos DB instance (emulator or Azure account)
- A Linux development environment, WSL2 on Windows or native Linux. Not tested on Windows or macOS at this time.

### Initial Setup

1. **Clone the Azure SDK for Rust repository**:
    (Temporary: Clone from my fork until the PR is merged)
   ```bash
   git clone https://github.com/analogrelay/azure-sdk-for-rust.git
   ```
   
   Note: This subdirectory is not tracked by git and will need to be cloned manually on fresh checkouts.

2. (Temporary) **Check out the `ashleyst/port-c-bindings` branch** of the Azure SDK for Rust to get the necessary C bindings:
   ```bash
   cd azure-sdk-for-rust
   git checkout ashleyst/port-c-bindings
   cd ..
   ```

2. **Build the native Cosmos library and set up the Go wrapper**:
   ```bash
   ./script/update-azurecosmos
   ```
   
   This script will:
   - Build the `azure_data_cosmos_native` package in release mode
   - Copy the static and shared libraries to `./rust-sdk/lib/`
   - Copy the header file to `./rust-sdk/include/`
   - Generate a `pkg-config` file for easy linking

3. **Set the PKG_CONFIG_PATH environment variable**:
   ```bash
   export PKG_CONFIG_PATH=$PWD/rust-sdk/lib:$PKG_CONFIG_PATH
   ```
   
   You may want to add this to your shell profile for persistence across sessions.

4. **Verify the setup**:
   ```bash
   pkg-config --cflags --libs azurecosmos
   ```
   
   This should output the compiler and linker flags needed to use the library.

## Running Benchmarks

All benchmarks assume you have a Cosmos DB instance running with test data. The benchmarks use the following defaults:
- **Endpoint**: `https://localhost:8080` (Cosmos DB Emulator)
- **Database**: `sdk-bench-db`
- **Container**: `RandomDocs`
- **Duration**: 60 seconds
- **Workers**: CPU count
- **Item count**: 10,000 items distributed across 10 partitions

### Rust Benchmark

```bash
cd rust-bench
cargo run --release -- --help  # See available options
cargo run --release -- --duration 60s --workers 8
```

### Go SDK Benchmark

```bash
cd go-bench
go run main.go pointRead --help  # See available options
go run main.go pointRead --duration 60s --workers 8
```

### Go Wrapper Benchmark

```bash
cd go-wrapper-bench
go run main.go pointRead --help  # See available options  
go run main.go pointRead --duration 60s --workers 8
```

### Common Options

All benchmarks support similar command-line options:

- `--endpoint, -e`: Cosmos DB endpoint URL
- `--key, -k`: Cosmos DB primary key (defaults to emulator key)
- `--database, -d`: Database name
- `--duration, -t`: Benchmark duration (e.g., `60s`, `5m`)
- `--workers, -w`: Number of concurrent workers
- `--item-count, -i`: Total number of items in the database
- `--partition-count, -p`: Number of partitions

### Example with Custom Parameters

```bash
# Run 30-second benchmark with 4 workers against Azure Cosmos DB
go run main.go pointRead \
  --endpoint "https://myaccount.documents.azure.com:443/" \
  --key "your-cosmos-key-here" \
  --database "my-test-db" \
  --duration 30s \
  --workers 4
```

## Troubleshooting

### Common Issues

**"pkg-config: command not found"**
- Install pkg-config: `brew install pkg-config` (macOS) or `apt-get install pkg-config` (Ubuntu)

**"Package azurecosmos was not found"**
- Ensure `PKG_CONFIG_PATH` is set correctly
- Verify the `pkg-config` file exists at `./rust-sdk/lib/azurecosmos.pc`
- Re-run `./script/update-azurecosmos`

**"libazurecosmos.so: cannot open shared object file"**
- The shared library path may not be in your system's library search path
- Try setting `LD_LIBRARY_PATH`: `export LD_LIBRARY_PATH=$PWD/rust-sdk/lib:$LD_LIBRARY_PATH`
- Alternatively, use static linking by modifying the cgo flags

**Go wrapper compilation errors**
- Ensure the Azure SDK for Rust is cloned and the update script has been run
- Verify that Rust toolchain is installed and `cargo` is in PATH
- Check that the build completed successfully without errors

### Performance Notes

- For best results, run benchmarks with `--workers` set to your CPU core count
- Ensure your Cosmos DB instance has sufficient RU/s provisioned to avoid throttling
- Use release builds for meaningful performance comparisons
- Consider running multiple iterations and averaging results for more stable measurements

### Building for Different Architectures

The `update-azurecosmos` script builds for the host architecture by default. For cross-compilation:

```bash
# Build for specific target (requires appropriate Rust target installed)
cd azure-sdk-for-rust
cargo build --release --package azure_data_cosmos_native --target x86_64-unknown-linux-gnu
```

Then manually copy the libraries from the target-specific directory.