package query_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/index"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/ingest"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/query"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

func TestQueryEngineFlow(t *testing.T) {
	tmpDir := "./tmp_test_data"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// ---------------------------
	// 1. Create storage components
	// ---------------------------
	walManager, _ := storage.NewWALManager(
		filepath.Join(tmpDir, "wal.wal"),
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
		if i == 3 {
			entry.Level = "Error"
			entry.Message = "failed login"
		}
		ingestManager.AppendLog(entry)
	}

	// ---------------------------
	// 6. Trigger flush manually
	// ---------------------------
	if err := ingestManager.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// --- Create query engine ---
	qe := query.NewQueryEngine(indexManager, manifest, segmentManager)

	// --- Run query: Error logs only ---
	q := &query.Query{
		StartTime: now,
		EndTime:   now + 10000,
		Filters: []query.FilterExpression{
			{Field: "Level", Value: string(types.Error)},
		},
		SortAsc: true,
	}

	results, err := qe.Execute(q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(results))
	}

	if results[0].Message != "failed login" {
		t.Fatalf("Unexpected log message: %s", results[0].Message)
	}
}

func TestMultipleQueries(t *testing.T) {
	tmpDir := "./tmp_test_data"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// ---------------------------
	// 1. Create storage components
	// ---------------------------
	walManager, _ := storage.NewWALManager(
		filepath.Join(tmpDir, "wal.wal"),
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
	// 3. Create ingest manager
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
	// 4. Append logs
	// ---------------------------
	now := time.Now().UnixMilli()
	logs := []*types.LogEntry{
		{Timestamp: now, Level: "INFO", Message: "info 0", Properties: map[string]interface{}{"module": "auth"}},
		{Timestamp: now + 1000, Level: "INFO", Message: "info 1", Properties: map[string]interface{}{"module": "auth"}},
		{Timestamp: now + 2000, Level: "DEBUG", Message: "debug log", Properties: map[string]interface{}{"module": "billing"}},
		{Timestamp: now + 3000, Level: "ERROR", Message: "failed login", Properties: map[string]interface{}{"module": "auth"}},
		{Timestamp: now + 4000, Level: "ERROR", Message: "payment failed", Properties: map[string]interface{}{"module": "billing"}},
	}

	for _, entry := range logs {
		ingestManager.AppendLog(entry)
	}

	// ---------------------------
	// 5. Trigger flush manually
	// ---------------------------
	if err := ingestManager.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// --- Create query engine ---
	qe := query.NewQueryEngine(indexManager, manifest, segmentManager)

	// --- Run different query scenarios ---
	t.Run("QueryErrorLogs", func(t *testing.T) {
		q := &query.Query{
			StartTime: now,
			EndTime:   now + 5000,
			Filters: []query.FilterExpression{
				{Field: "Level", Value: string(types.Error)},
			},
			Limit:   50,
			SortAsc: true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Fatalf("Expected 2 error logs, got %d", len(results))
		}
	})

	t.Run("QueryInfoLogs", func(t *testing.T) {
		q := &query.Query{
			StartTime: now,
			EndTime:   now + 5000,
			Filters: []query.FilterExpression{
				{Field: "Level", Value: string(types.Info)},
			}, SortAsc: true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Fatalf("Expected 2 info logs, got %d", len(results))
		}
	})

	t.Run("QueryModuleBilling", func(t *testing.T) {
		q := &query.Query{
			StartTime: now,
			EndTime:   now + 5000,
			Filters: []query.FilterExpression{
				{Field: "module", Value: "billing"},
			},
			SortAsc: true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Fatalf("Expected 2 billing logs, got %d", len(results))
		}
	})

	t.Run("QueryAllLogsWithLimit", func(t *testing.T) {
		q := &query.Query{
			StartTime: now,
			EndTime:   now + 5000,
			Filters:   []query.FilterExpression{},
			Limit:     3,
			SortAsc:   true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 3 {
			t.Fatalf("Expected 3 logs due to limit, got %d", len(results))
		}
	})

	t.Run("QueryVagueWithoutTime", func(t *testing.T) {
		q := &query.Query{
			StartTime: 0,
			EndTime:   0,
			Filters: []query.FilterExpression{
				{Field: "Level", Value: string(types.Error)},
			},
			SortAsc: true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Fatalf("Expected 2 error logs, got %d", len(results))
		}
	})

	t.Run("Multiple AND filters", func(t *testing.T) {
		now := time.Now().UnixMilli()

		q := &query.Query{
			StartTime: now,
			EndTime:   now + 10000,
			Filters: []query.FilterExpression{
				{Field: "Level", Value: string(types.Error)},
				{Field: "Service", Value: "AuthService"},
			},
			Limit:   100,
			SortAsc: true,
		}

		results, err := qe.Execute(q)

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		t.Logf("Results count: %d", len(results))
	})

	t.Run("MultipleOrFilters", func(t *testing.T) {
		now := time.Now().UnixMilli()

		q := &query.Query{
			StartTime: now,
			EndTime:   now + 10000,
			Filters: []query.FilterExpression{
				{Field: "Level", Value: string(types.Error)},
				{Field: "Level", Value: string(types.Info), Operator: "OR"},
			},
			Limit:   100,
			SortAsc: true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		t.Logf("Results count: %d", len(results))
	})

	t.Run("MixedFilters", func(t *testing.T) {
		now := time.Now().UnixMilli()

		q := &query.Query{
			StartTime: now,
			EndTime:   now + 10000,
			Filters: []query.FilterExpression{
				{Field: "Level", Value: string(types.Error)},
				{Field: "Service", Value: "AuthService", Operator: "AND"},
				{Field: "Host", Value: "host1", Operator: "OR"},
			},
			Limit:   100,
			SortAsc: true,
		}

		results, err := qe.Execute(q)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		t.Logf("Results count: %d", len(results))
	})

}
