package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type SegmentManager struct {
	dir                 string // directory to store segment files
	maxSize             int64  // max size per segment in bytes
	currSize            int64
	currName            string
	currFile            *os.File
	lastTimestamp       int64 // last segment timestamp
	counter             int
	rotated             bool
	rotatedMeta         SegmentMeta
	minTimestampSegment int64
	maxTimestampSegment int64
	mutex               sync.Mutex
}

// NewSegmentManager initializes segment manager
func NewSegmentManager(dir string, maxSize int64) (*SegmentManager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	segmentManager := &SegmentManager{
		dir:           dir,
		maxSize:       maxSize,
		lastTimestamp: 0,
		counter:       0,
	}

	if err := segmentManager.newSegment(); err != nil {
		return nil, err
	}
	return segmentManager, nil
}

// Append log entry to current segment
func (segmentManager *SegmentManager) Append(entry *types.LogEntry) (int64, error) {
	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()

	if segmentManager.currFile == nil {
		if err := segmentManager.newSegment(); err != nil {
			return 0, err
		}
		segmentManager.minTimestampSegment = entry.Timestamp
		segmentManager.maxTimestampSegment = entry.Timestamp
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}

	data = append(data, '\n')

	offset, err := segmentManager.currFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// write log entry
	n, err := segmentManager.currFile.Write(data)
	if err != nil {
		return 0, err
	}

	segmentManager.currSize += int64(n)

	// Update min/max timestamp
	if entry.Timestamp < segmentManager.minTimestampSegment {
		segmentManager.minTimestampSegment = entry.Timestamp
	}
	if entry.Timestamp > segmentManager.maxTimestampSegment {
		segmentManager.maxTimestampSegment = entry.Timestamp
	}

	// Rotate segment if exceeds maxsize
	if segmentManager.currSize >= segmentManager.maxSize {
		if err := segmentManager.rotateSegment(); err != nil {
			return 0, err
		}
	}

	return offset, nil
}

// newSegment creates a new segment file
func (segmentManager *SegmentManager) newSegment() error {
	segmentID := segmentManager.nextSegmentID()

	fileName := fmt.Sprintf("segment_%d.log", segmentID)
	path := filepath.Join(segmentManager.dir, fileName)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	segmentManager.currFile = file
	segmentManager.currSize = 0
	segmentManager.currName = fileName
	segmentManager.minTimestampSegment = 0
	segmentManager.maxTimestampSegment = 0
	segmentManager.rotated = false
	return nil
}

// nextSegmentID generates unique segment ID using timestamp + counter
func (segmentManager *SegmentManager) nextSegmentID() int64 {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	if now == segmentManager.lastTimestamp {
		segmentManager.counter++
	} else {
		segmentManager.lastTimestamp = now
		segmentManager.counter = 0
	}
	// combine timestamp and counter: shift timestamp and add counter
	id := (segmentManager.lastTimestamp << 8) | int64(segmentManager.counter)
	return id
}

// rotateSegment closes current file and starts new segment
func (segmentManager *SegmentManager) rotateSegment() error {
	if segmentManager.currFile != nil {
		segmentManager.currFile.Close()
	}
	// Save rotated metadata
	segmentManager.rotatedMeta = SegmentMeta{
		FileName:     segmentManager.currName,
		Size:         segmentManager.currSize,
		MinTimestamp: segmentManager.minTimestampSegment,
		MaxTimestamp: segmentManager.maxTimestampSegment,
	}
	segmentManager.rotated = true

	return segmentManager.newSegment()
}

// Flush syncs current segment to disk
func (segmentManager *SegmentManager) Flush() error {
	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()
	if segmentManager.currFile != nil {
		return segmentManager.currFile.Sync()
	}
	return nil
}

// CurrFileName returns the current segment file name
func (segmentManager *SegmentManager) CurrFileName() string {
	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()
	return segmentManager.currName
}

// CurrFileSize returns the current segment file size
func (segmentManager *SegmentManager) CurrFileSize() int64 {
	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()
	return segmentManager.currSize
}

// Rotation info for ingest manager
func (segmentManager *SegmentManager) IsSegmentRotated() bool {
	return segmentManager.rotated
}

func (segmentManager *SegmentManager) LastRotatedFileName() string {
	return segmentManager.rotatedMeta.FileName
}

func (segmentManager *SegmentManager) LastRotatedFileSize() int64 {
	return segmentManager.rotatedMeta.Size
}

func (segmentManager *SegmentManager) LastRotatedMinTimestamp() int64 {
	return segmentManager.rotatedMeta.MinTimestamp
}

func (segmentManager *SegmentManager) LastRotatedMaxTimestamp() int64 {
	return segmentManager.rotatedMeta.MaxTimestamp
}

func (segmentManager *SegmentManager) ResetRotationInfo() {
	segmentManager.rotated = false
	segmentManager.rotatedMeta = SegmentMeta{}
}

// CurrFileName returns the current segment file name
func (segmentManager *SegmentManager) Dir() string {
	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()
	return segmentManager.dir
}

func (segmentManager *SegmentManager) ActiveSegmentMeta() SegmentMeta {
	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()

	return SegmentMeta{
		FileName:     segmentManager.currName,
		Size:         segmentManager.currSize,
		MinTimestamp: segmentManager.minTimestampSegment,
		MaxTimestamp: segmentManager.maxTimestampSegment,
	}
}

// ReadSegment reads logs from a segment file at given offsets.
// If offsets is empty, read the whole file.
func (segmentManager *SegmentManager) ReadSegment(fileName string, offsets []int64) ([]types.LogEntry, error) {

	segmentManager.mutex.Lock()
	defer segmentManager.mutex.Unlock()

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var results []types.LogEntry

	if len(offsets) == 0 {
		// full scan
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var entry types.LogEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
				results = append(results, entry)
			}
		}

		return results, scanner.Err()
	}

	// read only specific offsets
	for _, off := range offsets {
		if _, err := file.Seek(off, io.SeekStart); err != nil {
			return nil, err
		}

		line, err := bufio.NewReader(file).ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		var entry types.LogEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			results = append(results, entry)
		}
	}

	return results, nil
}
