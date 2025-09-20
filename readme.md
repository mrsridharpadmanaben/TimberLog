# TimberLog

**TimberLog** is a lightweight, Go-based append-only log database designed for scalable log ingestion, storage, and indexing.  

It supports:

- High-throughput log ingestion via HTTP / gRPC APIs  
- Append-only memory buffer with write-ahead log (WAL) for durability  
- Disk-based segments for persistent storage  
- B+ tree indexes for efficient retrieval (timestamp, level, service)  
- Manifest metadata to track segments and partitions  
- Designed for single-node now, but scalable to multi-node in the future  

---

## 📂 Project Structure

```
TimberLog/
├── cmd/
│   └── main.go              # entry point
├── pkg/
│   ├── ingest/
│   │   ├── api.go           # HTTP/gRPC API endpoints - MAYY BE!
│   │   ├── buffer.go        # MemoryBuffer struct
│   │   └── ingest_manager.go # IngestManager struct
│   ├── storage/
│   │   ├── wal.go           # WAL struct
│   │   ├── segment.go       # SegmentManager struct
│   │   └── manifest.go      # Manifest struct
│   ├── index/
│   │   ├── bptree.go        # B+ tree skeleton - FOR NOW USED IN-MEMROY tidwall/btree
│   │   └── index_manager.go # IndexManager skeleton
│   ├── types/
│   │   └── log_entry.go     # LogEntry struct
├── tests/                   # unit/integration tests
```

---

## 🏗️ Development Roadmap

**Phase 1**:

- Memory buffer and basic ingestion  
- WAL for durability  
- Disk segments and manifest  
- B+ tree indexes skeleton  
- API ingestion (HTTP / gRPC endpoints placeholder)  

**Phase 2**:

- Query engine for efficient log retrieval  
- User-defined indexes  
- Multi-node scaling  
- Optional compression and retention policies  

---

<!-- github.com/mrsridharpadmanaben/TimberLog -->

```
       ┌───────────────────────────────┐
       │        Log Producers          │
       │ (Apps, Services, Scripts)     │
       └───────────────┬───────────────┘
                       │ Send logs (JSON / gRPC)
                       ▼
        ┌─────────────────────────────┐
        │        TimberLog API        │
        │ - HTTP / REST / gRPC        │
        │ - Validate & parse logs     │
        └───────────────┬─────────────┘
                        │
                        ▼
           ┌───────────────────────────┐
           │      Ingest Manager       │
           │ - Append logs to memory   │
           │   buffer                  │
           │ - Trigger flush on full   │
           │ - Handles batch ingestion │
           └───────────────┬───────────┘
                           │
                           ▼
                 ┌───────────────────────┐
                 │        WAL            │
                 │ - wal.wal             │
                 │ - wal.meta            │
                 │ - ensures durability  │
                 └───────────┬───────────┘
                             │ Flush triggered
                             ▼
            ┌─────────────────────────────────┐
            │     Disk Partition / Segment    │
            │ ┌────────────────────────────┐  │
            │ │ segment.log                │  │
            │ │ - append-only log entries  │  │
            │ │ - messages, stack traces   │  │
            │ │ - dynamic fields stored    │  │
            │ └─────────────┬──────────────┘  |
            │               │                 |
            │               ▼                 |
            │ ┌─────────────────────────────┐ │
            │ │ B+ Tree Indexes             │ │
            │ │ - ts.bptree (timestamp)     │ │
            │ │ - level.bptree (log level)  │ │
            │ │ - service.bptree            │ │
            │ │ - optional user-defined idx │ │
            │ └─────────────┬───────────────┘ │
            │               ▼                 |
            │ ┌────────────────────────────┐  │
            │ │ manifest.json              │  │
            │ │ - segment metadata         │  │
            │ │ - partition info           │  │
            │ └────────────────────────────┘  │
            └─────────────────────────────────┘
                             │
                             ▼
                     ┌─────────────────┐
                     │ Query Engine    │ <- Phase 2
                     │ - Load indexes  │
                     │ - Intersect     │
                     │ - Read segment  │
                     │ - Apply filters │
                     └─────────────────┘
                             │
                             ▼
                     ┌───────────────┐
                     │ Query Result  │
                     │ (User / UI)   │
                     └───────────────┘

```