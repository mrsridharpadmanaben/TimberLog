package ingest

import (
	"sync"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type MemoryBuffer struct {
	buffer  []*types.LogEntry
	maxSize int
	mutex   sync.Mutex
	// b+ tree - inmemory index for recent log retrieval
}

func NewMemoryBuffer(maxSize int) *MemoryBuffer {
	return &MemoryBuffer{
		buffer:  []*types.LogEntry{},
		maxSize: maxSize,
	}
}

// Append a log entry to the buffer
func (memoryBuffer *MemoryBuffer) Append(entry *types.LogEntry) {
	memoryBuffer.mutex.Lock()
	defer memoryBuffer.mutex.Unlock()
	memoryBuffer.buffer = append(memoryBuffer.buffer, entry)
}

// Flush returns all entries and resets the buffer
func (memoryBuffer *MemoryBuffer) Flush() []*types.LogEntry {
	memoryBuffer.mutex.Lock()
	defer memoryBuffer.mutex.Unlock()

	logEntries := memoryBuffer.buffer

	memoryBuffer.buffer = []*types.LogEntry{}

	return logEntries
}

// length
func (memoryBuffer *MemoryBuffer) Length() int {
	memoryBuffer.mutex.Lock()
	defer memoryBuffer.mutex.Unlock()

	return len(memoryBuffer.buffer)
}
