- Memory Buffer → WAL: IngestManager flushes buffered logs to WAL.
- WAL Rotation: When WAL reaches a certain size, we rotate:
    - Close current WAL
    - Open a new WAL file for new logs
- Old WAL → Segment: Old WAL logs are persisted into segments, and manifest is updated.
- Recovery: If crash occurs, TimberLog can read:
    - Current WAL (new WAL) for unflushed logs
    - Segment files for already persisted logs


### Write ahead log manager flow
```
       ┌─────────────────────────────┐
       │       Memory Buffer         │
       │ (Ingested logs in memory)   │
       └─────────────┬───────────────┘
                     │ Flush triggered
                     ▼
       ┌─────────────────────────────┐
       │          WAL (wal.wal)      │
       │ - Append-only log entries   │
       │ - Current file: wal_001     │
       └─────────────┬───────────────┘
                     │ WAL reaches max size
                     ▼
       ┌─────────────────────────────┐
       │       WAL Rotation          │
       │ - Close wal_001             │
       │ - Create wal_002            │
       │ - Reset lastOffset          │
       └─────────────┬───────────────┘
                     │
                     ▼
       ┌─────────────────────────────┐
       │      Segment Manager        │
       │ - Read old WAL (wal_001)    │
       │ - Write logs to segment.log │
       │ - Update manifest.json      │
       └─────────────┬───────────────┘
                     │
                     ▼
       ┌─────────────────────────────┐
       │ Manifest / Metadata         │
       │ - Track segment files       │
       │ - Track min/max timestamps  │
       └─────────────────────────────┘

```

### persisted segment log on the disk flow

```
       ┌─────────────────────────────┐
       │       Memory Buffer        │
       │ (Logs collected in memory) │
       └─────────────┬───────────────┘
                     │ Flush triggered
                     ▼
       ┌─────────────────────────────┐
       │           WAL              │
       │  (wal.wal + wal.meta)      │
       │ - Append-only              │
       └─────────────┬───────────────┘
                     │ Flush/rotation triggers
                     ▼
       ┌─────────────────────────────┐
       │       Current Segment      │
       │  segment_001.log           │
       │ - Writing logs here        │
       └─────────────┬───────────────┘
                     │ Rotation (maxSize reached)
                     ▼
       ┌─────────────────────────────┐
       │       New Segment          │
       │  segment_002.log           │
       │ - Writing new logs         │
       └─────────────┬───────────────┘
                     │
                     ▼
       ┌─────────────────────────────┐
       │      Old Segment Files      │
       │  segment_001.log (read-only)│
       │  segment_000.log (read-only)│
       │ - Persisted logs            │
       │ - Metadata in manifest.json │
       └─────────────┬───────────────┘
                     │
                     ▼
       ┌─────────────────────────────┐
       │         Manifest           │
       │ - Tracks all segment files │
       │ - Size & timestamp range   │
       └─────────────┬───────────────┘
                     │
                     ▼
       ┌─────────────────────────────┐
       │       Query Engine          │
       │ - Reads old & current       │
       │   segments based on indexes │
       └─────────────────────────────┘

```