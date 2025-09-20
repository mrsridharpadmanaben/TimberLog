// write ahead logging

package storage

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type WALManager struct {
	walFile    *os.File
	walPath    string
	metaFile   *os.File
	metaPath   string
	lastOffset int64
	mutex      sync.Mutex
}

// opens and create WAL files
func NewWALManager(walPath, metaPath string) (*WALManager, error) {
	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return nil, err
	}

	metaFile, err := os.OpenFile(metaPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		walFile.Close()
		return nil, err
	}

	walManager := &WALManager{
		walFile:  walFile,
		walPath:  walPath,
		metaFile: metaFile,
		metaPath: metaPath,
	}

	walManager.loadMeta()

	return walManager, nil
}

// Appends a log entry to WAL
func (walManager *WALManager) Append(entry *types.LogEntry) error {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	n, err := walManager.walFile.Write(data)
	if err != nil {
		return err
	}

	walManager.lastOffset += int64(n)

	return walManager.writeMetaInPlace(entry.Timestamp)
}

// Replay reads all entries from WAL (for recovery)
// func (walManager *WALManager) Replay() ([]*types.LogEntry, error) {
// 	walManager.mutex.Lock()
// 	defer walManager.mutex.Unlock()

// 	stat, err := walManager.walFile.Stat()
// 	if err != nil {
// 		return nil, err
// 	}

// 	data := make([]byte, stat.Size())
// 	if _, err := walManager.walFile.ReadAt(data, 0); err != nil {
// 		return nil, err
// 	}

// 	var logs []*types.LogEntry
// 	lines := splitLines(data)
// 	for _, line := range lines {
// 		var entry types.LogEntry
// 		if err := json.Unmarshal(line, &entry); err == nil {
// 			logs = append(logs, &entry)
// 		}
// 	}

// 	return logs, nil
// }

func (walManager *WALManager) Replay() ([]*types.LogEntry, error) {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	// Open WAL file in read-only mode for replay
	file, err := os.Open(walManager.walPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var logs []*types.LogEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry types.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			logs = append(logs, &entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

// splitLines splits WAL content into individual lines
// func splitLines(data []byte) [][]byte {
// 	var lines [][]byte
// 	start := 0
// 	for i, b := range data {
// 		if b == '\n' {
// 			lines = append(lines, data[start:i])
// 			start = i + 1
// 		}
// 	}
// 	if start < len(data) {
// 		lines = append(lines, data[start:])
// 	}
// 	return lines
// }

// writeMetaInPlace safely writes meta using Truncate + Seek + Sync
func (walManager *WALManager) writeMetaInPlace(lastTimestamp int64) error {

	meta := map[string]interface{}{
		"last_offset":   walManager.lastOffset,
		"last_entry_ts": lastTimestamp,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	// Truncate file to zero, seek to start
	if err := walManager.metaFile.Truncate(0); err != nil {
		return err
	}
	if _, err := walManager.metaFile.Seek(0, 0); err != nil {
		return err
	}

	if _, err := walManager.metaFile.Write(data); err != nil {
		return err
	}

	// Ensure data is flushed to disk
	return walManager.metaFile.Sync()
}

// loadMeta reads meta file
func (walManager *WALManager) loadMeta() error {
	stat, err := walManager.metaFile.Stat()
	if err != nil {
		return err
	}

	if stat.Size() == 0 {
		return nil // empty meta ok
	}

	data := make([]byte, stat.Size())
	if _, err := walManager.metaFile.ReadAt(data, 0); err != nil {
		return err
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	if offset, ok := meta["last_offset"].(float64); ok {
		walManager.lastOffset = int64(offset)
	}

	return nil
}

// Rotate closes current WAL and meta, opens new ones
func (walManager *WALManager) Rotate(newWalPath, newMetaPath string) error {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	if walManager.walFile != nil {
		walManager.walFile.Close()
	}
	if walManager.metaFile != nil {
		walManager.metaFile.Close()
	}

	walFile, err := os.OpenFile(newWalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	metaFile, err := os.OpenFile(newMetaPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		walFile.Close()
		return err
	}

	walManager.walFile = walFile
	walManager.metaFile = metaFile
	walManager.walPath = newWalPath
	walManager.metaPath = newMetaPath
	walManager.lastOffset = 0

	return nil
}

// Close closes WAL and meta files
func (walManager *WALManager) Close() error {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()
	if walManager.walFile != nil {
		walManager.walFile.Close()
	}
	if walManager.metaFile != nil {
		walManager.metaFile.Close()
	}
	return nil
}
