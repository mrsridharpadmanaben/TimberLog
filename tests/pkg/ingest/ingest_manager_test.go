package ingest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/index"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/ingest"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

func TestIngestFlow(t *testing.T) {

	tmpDir := "./tmp_test_data"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// ---------------------------
	// 1. Create storage components
	// ---------------------------
	walManager, _ := storage.NewWALManager(
		filepath.Join(tmpDir),
		filepath.Join(tmpDir, "wal.meta"),
	)

	segmentManager, _ := storage.NewSegmentManager(tmpDir, 1024*10) // 10 KB per segment

	manifest, _ := storage.NewManifest(filepath.Join(tmpDir, "manifest.json"))

	// ---------------------------
	// 2. Create memory buffer
	// ---------------------------
	buffer := &ingest.MemoryBuffer{}

	indexManager := index.NewIndexManager()

	// ---------------------------
	// 4. Create ingest manager
	// ---------------------------
	ingestManager := ingest.NewIngestManager(
		buffer,
		walManager,
		segmentManager,
		manifest,
		indexManager,
		1*time.Second, // flush interval
	)

	// ---------------------------
	// 5. Append some logs
	// ---------------------------
	now := time.Now().UnixMilli()
	for i := range 5 {
		entry := &types.LogEntry{
			Timestamp:  now + int64(i*1000),
			Level:      "INFO",
			Message:    fmt.Sprintf("Test log %d", i),
			Properties: map[string]interface{}{"service": "test"},
		}
		ingestManager.AppendLog(entry)
	}

	// ---------------------------
	// 6. Trigger flush manually
	// ---------------------------
	if err := ingestManager.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// ---------------------------
	// 7. Query index manager by timestamp
	// ---------------------------
	tsKey := fmt.Sprintf("%d", now)
	results := indexManager.Search("timestamp", tsKey)

	if len(results) == 0 {
		t.Fatalf("No results found in index for timestamp %s", tsKey)
	}

	t.Logf("Found %d log(s) for timestamp %s: %+v", len(results), tsKey, results)
}

func TestRecoveryFlow(t *testing.T) {
	tmpDir := "./tmp_recovery_test"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// ---------------------------
	// Step 1: Setup initial system
	// ---------------------------
	walPath := filepath.Join(tmpDir)
	metaPath := filepath.Join(tmpDir, "wal.meta")

	walManager, err := storage.NewWALManager(walPath, metaPath)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}

	segmentManager, _ := storage.NewSegmentManager(tmpDir, 1024*10)
	manifest, _ := storage.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	buffer := &ingest.MemoryBuffer{}

	indexManager := index.NewIndexManager()

	ingestManager := ingest.NewIngestManager(
		buffer, walManager, segmentManager, manifest, indexManager, 1*time.Second,
	)

	// ---------------------------
	// Step 2: Append logs
	// ---------------------------
	now := time.Now().UnixMilli()
	totalLogs := 10
	for i := range totalLogs {
		entry := &types.LogEntry{
			Timestamp:  now + int64(i*1000),
			Level:      "ERROR",
			Message:    fmt.Sprintf("Log entry %d", i),
			Properties: map[string]interface{}{"service": "recovery_test"},
		}
		if err := ingestManager.AppendLog(entry); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}

		// Flush halfway
		if i == 4 {
			if err := ingestManager.Flush(); err != nil {
				t.Fatalf("Flush failed: %v", err)
			}
		}
	}

	// ---------------------------
	// Step 3: Simulate process restart
	// ---------------------------
	ingestManager.StopBackgroundFlush()
	walManager.Close()

	// ---------------------------
	// Step 4: Re-create managers to simulate recovery
	// ---------------------------
	recoveredWal, err := storage.NewWALManager(walPath, metaPath)
	if err != nil {
		t.Fatalf("Failed to re-create WAL manager: %v", err)
	}

	recoveredSegment, _ := storage.NewSegmentManager(tmpDir, 1024*10)
	recoveredManifest, _ := storage.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	recoveredBuffer := &ingest.MemoryBuffer{}

	recoveredIndexManager := index.NewIndexManager()
	recoveredIndexManager.CreateIndex("timestamp", func(entry *types.LogEntry) string {
		return fmt.Sprintf("%d", entry.Timestamp)
	})

	recoveredIngest := ingest.NewIngestManager(
		recoveredBuffer,
		recoveredWal,
		recoveredSegment,
		recoveredManifest,
		recoveredIndexManager,
		1*time.Second,
	)

	// ---------------------------
	// Step 5: Replay WAL
	// ---------------------------
	entries, err := recoveredWal.ReplayAllUnflushed()
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	for _, entry := range entries {
		if err := recoveredIngest.AppendLog(entry); err != nil {
			t.Fatalf("AppendLog during replay failed: %v", err)
		}
	}

	if err := recoveredIngest.Flush(); err != nil {
		t.Fatalf("Flush after replay failed: %v", err)
	}

	// ---------------------------
	// Step 6: Verify index recovery
	// ---------------------------
	for i := 0; i < totalLogs; i++ {
		tsKey := fmt.Sprintf("%d", now+int64(i*1000))
		results := recoveredIndexManager.Search("timestamp", tsKey)
		if len(results) == 0 {
			t.Errorf("Timestamp %s not found in recovered index", tsKey)
		} else {
			t.Logf("Recovered log for timestamp %s: %+v", tsKey, results[0])
		}
	}
}
