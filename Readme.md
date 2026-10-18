# ⚡ FlashVector

**FlashVector** is a high-performance, embedded vector database and storage engine written in **Go**, engineered for **sub-microsecond in-memory retrieval, 8-bit scalar quantization (SQ8), durability via Write-Ahead Logging (WAL), and hybrid search capabilities**.

Designed with low-level systems principles, FlashVector eliminates runtime memory allocations on hot paths, guarantees crash-safety with binary WAL checkpoint recovery, and provides an embedded HTTP REST API alongside experimental gRPC replication primitives.

---

## 🚀 Key Features

* **8-bit Scalar Quantization (SQ8):** Compresses high-dimensional float32 vector embeddings into 8-bit integers, reducing memory overhead by **75%** (from 4 bytes to 1 byte per dimension) while retaining high recall ($\ge 70\%$).
* **Inverted File (IVF) Indexing:** Partitions the vector space into Voronoi cells to prune candidate searches, delivering a **~4.9x retrieval speedup** over exhaustive linear scans.
* **Crash-Safe Write-Ahead Logging (WAL):** Append-only log with little-endian length-prefixed binary serialization and automated truncation of corrupted record tails during recovery.
* **Adaptive Hybrid Search & RRF:** Fuses lexical keyword matching with dense semantic vector similarity using concurrent **Reciprocal Rank Fusion (RRF)**.
* **Sub-Microsecond Query Planner:** Fast rule-based intent analyzer that inspects incoming queries and routes execution strategies in under **250 nanoseconds**.
* **Zero-Allocation Hot-Path Reads:** Pure in-memory key-value lookups executing in **~30 ns/op with 0 heap allocations**.
* **Lock-Free Observability:** Thread-safe telemetry using atomic counters (`sync/atomic`) for non-blocking latency, throughput, and error metrics.
* **Graceful Lifecycle Management:** Controlled OS signal interception (`SIGINT`, `SIGTERM`) to flush in-flight queries and sync pending disk I/O cleanly.
* **Replication Primitives (Experimental):** Protobuf and gRPC service definitions (`cluster/rpc`) for streaming WAL mutations to standby replicas.

---

## 🧱 Architecture Overview

```
                      +---------------------------------------+
                      |         Client (HTTP REST API)        |
                      |          POST /insert, /search        |
                      +---------------------------------------+
                                          |
                                          v
                      +---------------------------------------+
                      |         Query Intent Planner          |
                      |   (Intent Detection & Plan Routing)   |
                      +---------------------------------------+
                                          |
                     +--------------------+--------------------+
                     |                                         |
                     v                                         v
        +-------------------------+               +-------------------------+
        |   Lexical Keyword Map   |               |     IVF Vector Index    |
        |    (Inverted Index)     |               |  (SQ8 Quantized Vectors)|
        +-------------------------+               +-------------------------+
                     \                                         /
                      \                                       /
                       +-------------------------------------+
                       |   Reciprocal Rank Fusion (RRF)      |
                       +-------------------------------------+
                                          |
                                          v
+---------------------------------------------------------------------------------+
|                               Storage Engine                                    |
|  +---------------------------+   +-------------------+   +-------------------+  |
|  |  In-Memory KV (RWMutex)   |   | Append-Only WAL   |   | Atomic Metrics    |  |
|  |   (~30 ns Zero-alloc Get) |   | (Binary encoding) |   | (Lock-free stats) |  |
|  +---------------------------+   +-------------------+   +-------------------+  |
+---------------------------------------------------------------------------------+
```

---

## 📂 Project Structure

```
flashvector/
├── cluster/             # Replication hooks, election prototype & node monitor
│   └── rpc/             # Protocol Buffers & gRPC replication bindings
├── config/              # Configuration structs & environment variable overrides
├── internal/            # Graceful shutdown handler with OS signal traps
├── metrics/             # Lock-free atomic performance counters
├── query/               # Query intent analyzer, AST-like planner & benchmarks
├── server/              # HTTP REST API server (/insert, /search)
├── storage/             # Core Key-Value store, WAL integration, snapshots & recovery
├── vector/              # IVF index, SQ8 scalar quantization, centroids, RRF fusion
├── wal/                 # Binary length-prefixed Write-Ahead Log engine
├── main.go              # Application entrypoint
├── go.mod               # Module dependencies
└── README.md
```

---

## ▶️ Quickstart & Usage

### 1. Run the Database
```bash
go run main.go
```
The server will boot the storage engine, replay any existing `data.wal` log, and start listening on `http://localhost:8080`.

### 2. Insert a Vector (`POST /insert`)
```bash
curl -X POST http://localhost:8080/insert \
  -H "Content-Type: application/json" \
  -d '{
    "id": "vec-1",
    "vector": [0.05, -0.12, 0.33, 0.81],
    "metadata": {"category": "systems", "author": "sohan"}
  }'
```

### 3. Query the Engine (`POST /search`)
The query planner automatically inspects the payload to decide between **Vector Only**, **Keyword Only**, or **Hybrid RRF Search**:
```bash
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{
    "text": "systems engineering",
    "vector": [0.05, -0.12, 0.33, 0.81],
    "k": 5
  }'
```

---

## 🚀 Performance Benchmarks

All benchmarks were executed on an bare-metal system under production build flags:
* **CPU:** 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz (4 Cores, 8 Threads)
* **OS:** Windows 11 (amd64)
* **Compiler:** Go 1.22+ (`go test -bench=. -benchmem`)
* **Vector Dimension:** 384 dimensions (standard embedding dimension, float32)

### 1. Core Storage & KV Engine

| Operation | Latency | Throughput (approx.) | Memory / Op | Allocations | Notes |
|:---|:---|:---|:---|:---|:---|
| **Read (`Store.Get`)** | **30.14 ns/op** | **~33,100,000 ops/sec** | **0 B/op** | **0 allocs/op** | Zero-alloc in-memory lookup |
| **Write (`Store.Set`)** | **176.7 µs/op** | **~5,650 ops/sec** | 559 KB/op | 6 allocs/op | WAL write + vector indexing |
| **Delete (`Store.Delete`)** | **159.8 µs/op** | **~6,250 ops/sec** | 319 KB/op | 3 allocs/op | Safe tombstone & index removal |

### 2. Vector Search: IVF Approximate Indexing vs. Brute-Force

Evaluated across a 10,000-vector dataset (Top-$k=10$ neighbors):

| Search Strategy | Avg Latency | Speedup | Allocations | Notes |
|:---|:---|:---|:---|:---|
| **Brute Force (Exact Flat)** | 2.91 ms/op | 1.0x (baseline) | 21 allocs/op | Full linear distance scan across all vectors |
| **IVF Index (ANN, 5 probes)** | **0.59 ms/op** | **~4.9x faster** | 1,489 allocs/op | Voronoi inverted centroid pruning |

### 3. Adaptive Hybrid Search & Fusion (Semantic + Lexical)

Evaluated across 1,000 indexed documents with 384-dimensional embeddings fusing dense vector similarity with BM25-style lexical keyword matching via concurrent Reciprocal Rank Fusion:

| Hybrid Mode | Avg Latency | Memory / Op | Allocations |
|:---|:---|:---|:---|
| **Semantic Priority ($k=20$)** | 7.48 ms/op | ~3.13 MB/op | 2,030 allocs/op |
| **Keyword Priority ($k=100$)** | 7.31 ms/op | ~3.13 MB/op | 2,031 allocs/op |

### 4. Query Intent Analysis & Routing

| Operation | Avg Latency | Throughput (approx.) | Allocations |
|:---|:---|:---|:---|
| **Intent Analysis (Long Text)** | **135.0 ns/op** | **~7,400,000 ops/sec** | 1 alloc/op |
| **Full Hybrid Query Planning** | **248.7 ns/op** | **~4,000,000 ops/sec** | 2 allocs/op |

---

## 🧪 Reproducing the Benchmarks

You can reproduce all benchmark metrics locally using Go's built-in testing harness:

```bash
# Run Core Storage Benchmarks
go test -v ./storage -bench=Benchmark -benchmem -run=^$

# Run Vector Search & IVF Indexing Benchmarks
go test -v ./vector -bench=Benchmark -benchmem -run=^$

# Run Query Intent & Planning Benchmarks
go test -v ./query -bench=Benchmark -benchmem -run=^$
```

To run the numerical recall validation test comparing IVF retrieval accuracy against brute-force ground truth:

```bash
go test -v ./vector -run=TestANNvsBruteForceRecall
```

---

## 🛡️ Reliability & Crash Recovery

* **Write-Ahead Logging (WAL):** Every write operation is serialized with an opcode, length prefixes, and raw byte values to disk and flushed via `file.Sync()` prior to in-memory mutation.
* **Corrupted Record Truncation:** On process reboot, `WAL.Replay()` iterates through recorded offsets. If an unexpected power kill left a half-written record tail, the engine detects the invalid boundary and truncates the file back to the last verifiable checkpoint.
* **Non-Blocking Telemetry:** Performance counters rely exclusively on atomic instructions (`sync/atomic`), ensuring telemetry collection introduces no lock contention to concurrent operations.

---

## 🛣️ Architectural Roadmap & Trade-offs

* **Consensus & Clustering:** The `cluster/` package currently implements an experimental primary-follower replication prototype. Transitioning to a distributed consensus protocol (e.g. Raft) with distributed log catch-up is the primary architectural next step.
* **SIMD Acceleration:** Core vector operations (`Dot`, `CosineSimilarity`) currently run in scalar Go loops. Rewriting math kernels with AVX2 / AVX-512 SIMD intrinsics or GPU CUDA kernels is planned to further reduce vector search latency.
* **Centroid Persistence:** Centroids are currently initialized in memory; persisting trained k-means cluster centroids across restarts is planned for index stability.

---

## 📜 License

MIT License. Designed and built as an exploration of low-latency systems programming, numerical quantization, and crash-safe storage engines.
