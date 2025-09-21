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
- API ingestion (HTTP / gRPC endpoints)  

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

### query engine

```
         ┌───────────────────────────────┐
         │          User Query           │
         │  - Timestamp range            │
         │  - Field filters (level, ...)│
         │  - Optional: aggregation, sort│
         └───────────────┬──────────────┘
                         │
                         ▼
                ┌─────────────────┐
                │ Query Planner   │
                │ - Parse query   │
                │ - Check which   │
                │   indexes exist │
                │ - Determine     │
                │   relevant segments│
                └───────────┬─────┘
                            │
         ┌──────────────────┴───────────────────┐
         │                                      │
         ▼                                      ▼
┌─────────────────────┐                  ┌───────────────────────┐
│ IndexManager        │                  │ Manifest Manager       │
│ - Lookup indexes    │                  │ - Read segment metadata│
│ - Returns offsets   │                  │   (min/max timestamp) │
│   per segment       │                  │ - Determine which      │
│                     │                  │   segments intersect   │
│   timestamp index   │                  │   query range          │
│   user-defined idx  │                  └─────────────┬─────────┘
└─────────┬───────────┘                                │
          │                                            │
          ▼                                            ▼
   ┌─────────────────────────┐           ┌─────────────────────────┐
   │ SegmentManager / Reader │           │ SegmentManager / Reader │
   │ - Open segment file     │           │ - Open segment file     │
   │ - Seek(offset)          │           │ - Seek(offset)          │
   │ - Read log entries      │           │ - Read log entries      │
   └─────────────┬───────────┘           └─────────────┬───────────┘
                 │                                     │
                 └─────────┐      ┌──────────────────┘
                           ▼      ▼
                  ┌───────────────────────────┐
                  │ Filter & Projection Layer │
                  │ - Apply user filters      │
                  │ - Return requested fields │
                  └─────────────┬─────────────┘
                                │
                                ▼
                  ┌───────────────────────────┐
                  │ Aggregation / Sorting     │
                  │ - Optional                │
                  │ - Group by field          │
                  │ - Limit / pagination      │
                  └─────────────┬─────────────┘
                                │
                                ▼
                  ┌───────────────────────────┐
                  │ Result Set / API Response │
                  │ - JSON / CSV / streaming  │
                  └───────────────────────────┘

``` 

### endpoints

``` curl
curl -X POST http://localhost:8080/write \
     -H "Content-Type: application/json" \
     -d '{
           "Timestamp": 1690000000000,
           "Level": "ERROR",
           "Service": "auth",
           "Host": "host1",
           "Message": "Failed login",
           "StackTrace": "",
           "Properties": {"user_id": "123"}
         }'

curl -X POST http://localhost:8081/query \
     -H "Content-Type: application/json" \
     -d '{
           "StartTime": 1690000000000,
           "EndTime": 1690000100000,
           "Filters": [
               {"Field": "Level", "Value": "ERROR"},
               {"Field": "Service", "Value": "auth"}
           ],
           "Limit": 100,
           "SortAsc": true
         }'
```