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
├── cmd/                  # main.go entry point
├── pkg/
│   ├── ingest/           # ingestion manager, buffer, API
│   ├── storage/          # WAL, segments, manifest
│   ├── index/            # B+ tree and index manager
│   ├── types/            # LogEntry struct
│   └── utils/            # helper utilities (file, compression)
├── tests/                # unit and integration tests
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
