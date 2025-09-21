### two way setup - share same context

```
              ┌─────────────────────────────┐
              │      TimberLog Backend      │
              │-----------------------------│
              │ IngestManager               │
              │ SegmentManager              │
              │ WALManager                  │
              │ Manifest                    │
              │ IndexManager                │
              └─────────────┬───────────────┘
                            │
          ┌─────────────────┴─────────────────┐
          │                                   │
          ▼                                   ▼
 ┌───────────────────┐                 ┌───────────────────┐
 │ Write HTTP Server │                 │ Query HTTP Server │
 │ Port: 8080        │                 │ Port: 8081        │
 │------------------ │                 │------------------ │
 │ POST /write       │                 │ POST /query       │
 │ - Receives logs   │                 │ - Receives query  │
 │ - Calls IngestMgr │                 │ - Calls QueryEng. │
 │ - Background flush│                 │                   │
 └───────────────────┘                 └───────────────────┘

```