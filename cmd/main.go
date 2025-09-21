package main

import (
	"log"
	"time"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/api"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/index"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/ingest"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/query"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
)

func main() {
	mountDirectory := "./timberlog_data"

	segmentManager, _ := storage.NewSegmentManager(mountDirectory, 1024*10)
	manifest, _ := storage.NewManifest(mountDirectory)
	walManager, _ := storage.NewWALManager(mountDirectory, mountDirectory+"/wal.meta")

	buffer := &ingest.MemoryBuffer{}
	indexManager := index.NewIndexManager()
	ingestManager := ingest.NewIngestManager(buffer, walManager, segmentManager, manifest, indexManager, 1*time.Second)

	if err := ingestManager.RecoverFromWAL(); err != nil {
		log.Fatalf("[RECOVERY FAILED] %v", err)
	}

	ingestManager.StartBackgroundFlush()

	queryEngine := query.NewQueryEngine(indexManager, manifest, segmentManager)

	// Start servers
	go func() {
		ws := api.NewWriteServer(ingestManager)
		ws.Start(":8080")
	}()

	go func() {
		qs := api.NewQueryServer(queryEngine)
		qs.Start(":8081")
	}()

	select {} // block main
}
