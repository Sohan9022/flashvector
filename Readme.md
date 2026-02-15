
📌 FlashVector

**FlashVector** is a **production-style embedded key-value storage engine** written in **Go**, focused on **durability, concurrency, observability, and safe lifecycle management**.

The project is designed to demonstrate **real backend and systems engineering concepts** used in databases, rather than UI or framework-centric development.


🚀 Features

* **Write-Ahead Logging (WAL)** for crash-safe durability
* **Snapshot-based recovery** for fast startup and WAL truncation
* **Concurrent reads and writes** using RWMutex
* **Lock-free atomic metrics** for observability
* **Graceful shutdown** using context cancellation and OS signals
* **Config-driven startup** (same binary, different roles)

🧱 Architecture Overview

```
Client
  |
  v
Store (RWMutex)
  ├── WAL (disk)
  ├── In-memory map
  ├── Index
  └── Metrics (atomic counters)
        |
        v
Graceful Shutdown (context + OS signals)
```

📂 Project Structure

```
flashvector/
├── cluster/        # Node & replication hooks
├── config/         # Config-driven startup
├── metrics/        # Lock-free atomic metrics
├── storage/        # Store, WAL, snapshots
├── internal/
│   └── shutdown/   # Graceful shutdown handling
├── main.go
├── go.mod
└── README.md
```

⚙️ Configuration

FlashVector uses a **JSON config file** with optional **environment variable overrides**, enabling flexible deployment without code changes.

Example `config.json`

```json
{
  "NodeID": "node-1",
  "DataDir": "./data/node1",
  "Role": "leader",
  "ListenAddr": ":8080",
  "Peers": [":8081", ":8082"],
  "EnableMetrics": true,
  "SnapshotIntervalSeconds": 60
}
```

▶️ Running the Project

```bash
go run main.go
```

Override config using environment variables:

```bash
export NODE_ID=node-2
export ROLE=follower
go run main.go
```

📊 Metrics & Observability

FlashVector maintains **in-process metrics** using **lock-free atomic counters**, including:

* write count
* read count
* delete count
* replication failure count

Metrics collection is **non-blocking** and does **not affect request latency or correctness**.

🛑 Graceful Shutdown

The system listens for `SIGINT` and `SIGTERM` and performs a controlled shutdown:

1. Stop accepting new operations
2. Finish in-flight requests
3. Flush WAL data to disk
4. Stop background goroutines
5. Exit cleanly

This prevents data loss and partial writes.


🎯 Design Principles

* Correctness before optimization
* Explicit lifecycle management
* Minimal shared state
* Clear separation of concerns
* Production-grade concurrency patterns

🛣️ Planned Improvements

* Add Go benchmarks and pprof profiling for core operations
* Introduce network/RPC layer for remote access
* Implement distributed coordination mechanisms
* Extend query and indexing capabilities


🧠 Why This Project Matters

FlashVector demonstrates **real-world backend engineering skills**, including:

* crash safety and recovery
* concurrent system design
* observability without performance impact
* clean shutdown semantics
* configuration-driven deployment

🚀 Performance Benchmarks

Benchmarks were executed on:

- CPU: Intel i5-1155G7 (11th Gen)
- OS: Windows (amd64)
- Vector Dimension: 384 (float32, ~1.5 KB per vector)

| Operation | Avg Latency | Throughput (approx.) | Allocations | Notes |
|------------|-------------|----------------------|-------------|-------|
| **Read (Get)** | **21 ns/op** | ~47,000,000 ops/sec | 0 B/op, 0 allocs | Pure in-memory lookup |
| **Write (Set)** | **0.42 ms/op** | ~2,300 ops/sec | ~559 KB/op, 6 allocs | WAL logging + vector indexing |
| **Delete** | **0.25 ms/op** | ~4,000 ops/sec | ~244 KB/op, 2 allocs | Safe removal from index |

Benchmark Command

```bash
go test ./storage -bench=. -benchmem


📜 License

MIT License


> This project focuses on systems design and correctness. Performance benchmarking and networking layers are planned extensions.
