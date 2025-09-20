package ingest

import (
	"sync"
	"time"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/index"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type IngestManager struct {
	buffer         *MemoryBuffer
	walManager     *storage.WALManager
	segmentManager *storage.SegmentManager
	manifest       *storage.Manifest
	indexManager   *index.IndexManager
	flushInterval  time.Duration
	stopChannel    chan struct{}
	mutex          sync.Mutex
}

// NewIngestManager initializes the IngestManager
func NewIngestManager(
	buffer *MemoryBuffer,
	walManager *storage.WALManager,
	segmentManager *storage.SegmentManager,
	manifest *storage.Manifest,
	indexManager *index.IndexManager,
	flushInterval time.Duration,
) *IngestManager {
	return &IngestManager{
		buffer:         buffer,
		walManager:     walManager,
		segmentManager: segmentManager,
		manifest:       manifest,
		indexManager:   indexManager,
		flushInterval:  flushInterval,
		stopChannel:    make(chan struct{}),
	}
}

// AppendLog appends a log entry to memory buffer and WAL
func (ingestManager *IngestManager) AppendLog(entry *types.LogEntry) error {
	ingestManager.mutex.Lock()
	defer ingestManager.mutex.Unlock()

	// 1. Persist immediately to WAL
	if err := ingestManager.walManager.Append(entry); err != nil {
		return err
	}

	// 2. Append to memory buffer
	ingestManager.buffer.Append(entry)

	return nil
}

// Flush writes all buffered logs to the segment and updates manifest
func (ingestManager *IngestManager) Flush() error {

	ingestManager.mutex.Lock()
	defer ingestManager.mutex.Unlock()

	// 1. Get logs from memory buffer
	logs := ingestManager.buffer.Flush()
	if len(logs) == 0 {
		return nil
	}

	// 2. Write logs to SegmentManager
	for _, entry := range logs {
		offset, err := ingestManager.segmentManager.Append(entry)
		if err != nil {
			return err
		}

		ingestManager.indexManager.Insert(entry, ingestManager.segmentManager.CurrFileName(), offset)
	}

	if ingestManager.segmentManager.IsSegmentRotated() {
		meta := storage.SegmentMeta{
			FileName:     ingestManager.segmentManager.LastRotatedFileName(),
			Size:         ingestManager.segmentManager.LastRotatedFileSize(),
			MinTimestamp: ingestManager.segmentManager.LastRotatedMinTimestamp(),
			MaxTimestamp: ingestManager.segmentManager.LastRotatedMaxTimestamp(),
		}
		if err := ingestManager.manifest.AddSegment(meta); err != nil {
			return err
		}
		ingestManager.segmentManager.ResetRotationInfo()
	}

	return nil
}

// StartBackgroundFlush starts periodic flushes in a separate goroutine
func (ingestManager *IngestManager) StartBackgroundFlush() {
	go func() {
		ticker := time.NewTicker(ingestManager.flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ingestManager.Flush()
			case <-ingestManager.stopChannel:
				return
			}
		}
	}()
}

// StopBackgroundFlush stops the periodic flush goroutine
func (ingestManager *IngestManager) StopBackgroundFlush() {
	close(ingestManager.stopChannel)
}
