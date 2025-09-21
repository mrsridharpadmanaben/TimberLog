// write ahead logging

package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

const walFilePattern = "wal_%08d.wal"

type WalMeta struct {
	LastFlushedSeq int64 `json:"last_flushed_seq"`
	CurrentSeq     int64 `json:"current_seq"`
	LastOffset     int64 `json:"last_offset"`
}

type WALManager struct {
	dir        string
	walFile    *os.File
	walPath    string
	metaFile   *os.File
	metaPath   string
	lastOffset int64
	mutex      sync.Mutex
	Meta       WalMeta
}

// opens and create WAL files
func NewWALManager(walPath, metaPath string) (*WALManager, error) {
	dir := walPath
	// if walPath is a file (ends with .wal or contains a file name), use dir of it
	if !strings.HasSuffix(walPath, string(os.PathSeparator)) && filepath.Ext(walPath) != "" {
		dir = filepath.Dir(walPath)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	walManager := &WALManager{
		dir:      dir,
		metaPath: metaPath,
	}

	// ensure current seq >= 1
	if walManager.Meta.CurrentSeq <= 0 {
		walManager.Meta.CurrentSeq = 1
	}

	walPath = filepath.Join(walManager.dir, fmt.Sprintf(walFilePattern, walManager.Meta.CurrentSeq))
	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	metaFile, err := os.OpenFile(metaPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		walFile.Close()
		return nil, err
	}

	walManager.walPath = walPath
	walManager.walFile = walFile
	walManager.metaFile = metaFile

	// load existing meta (if any)
	if err := walManager.loadWalMeta(); err != nil {
		return nil, err
	}

	walManager.lastOffset = walManager.Meta.LastOffset

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
	walManager.Meta.LastOffset = walManager.lastOffset

	if err := walManager.walFile.Sync(); err != nil {
		return err
	}

	// persist meta (atomic)
	return walManager.saveWalMeta(&walManager.Meta)
}

// ReplaySingleFile reads a single WAL file and returns entries (helper)
func (walManager *WALManager) ReplaySingleFile(path string) ([]*types.LogEntry, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	var entries []*types.LogEntry
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		var e types.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			// skip malformed line
			continue
		}
		entries = append(entries, &e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ReplayAllUnflushed reads WAL files in ascending seq order for seq > LastFlushedSeq
// and returns entries. It does NOT modify meta or delete files.
func (walManager *WALManager) ReplayAllUnflushed() ([]*types.LogEntry, error) {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	// load meta to ensure we have latest
	if err := walManager.loadWalMeta(); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(walManager.dir)
	if err != nil {
		return nil, err
	}

	type walFileInfo struct {
		path string
		seq  int64
	}

	var toReplay []walFileInfo
	for _, walFile := range files {
		if walFile.IsDir() {
			continue
		}
		name := walFile.Name()
		if !strings.HasPrefix(name, "wal_") || !strings.HasSuffix(name, ".wal") {
			continue
		}
		seq, err := walSeqFromName(name)
		if err != nil {
			continue
		}
		if seq > walManager.Meta.LastFlushedSeq {
			toReplay = append(toReplay, walFileInfo{
				path: filepath.Join(walManager.dir, name),
				seq:  seq,
			})
		}
	}

	sort.Slice(toReplay, func(i, j int) bool { return toReplay[i].seq < toReplay[j].seq })

	var entries []*types.LogEntry
	for _, wf := range toReplay {
		fh, err := os.Open(wf.path)
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(fh)
		for scanner.Scan() {
			var e types.LogEntry
			if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
				// skip malformed line (likely partial on crash)
				continue
			}
			entries = append(entries, &e)
		}
		if err := scanner.Err(); err != nil {
			fh.Close()
			return nil, err
		}
		fh.Close()
	}

	return entries, nil
}

// MarkFlushed updates LastFlushedSeq in meta and deletes wal files up to that seq (inclusive).
// Call this after you deterministically persisted WALs up to seq `seq`.
func (walManager *WALManager) MarkFlushed(seq int64) error {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	// load meta (defensive)
	if err := walManager.loadWalMeta(); err != nil {
		return err
	}

	if seq <= walManager.Meta.LastFlushedSeq {
		return nil
	}

	walManager.Meta.LastFlushedSeq = seq
	if err := walManager.saveWalMeta(&walManager.Meta); err != nil {
		return err
	}

	// delete files
	files, err := os.ReadDir(walManager.dir)
	if err != nil {
		return err
	}
	for _, walFile := range files {
		if walFile.IsDir() {
			continue
		}
		name := walFile.Name()
		if !strings.HasPrefix(name, "wal_") || !strings.HasSuffix(name, ".wal") {
			continue
		}
		seqCandidate, err := walSeqFromName(name)
		if err != nil {
			continue
		}
		if seqCandidate <= seq {
			_ = os.Remove(filepath.Join(walManager.dir, name)) // ignore errors
		}
	}

	return nil
}

func walSeqFromName(name string) (int64, error) {
	// name expected wal_00000001.wal
	base := filepath.Base(name)

	if !strings.HasPrefix(base, "wal_") || !strings.HasSuffix(base, ".wal") {
		return 0, fmt.Errorf("invalid wal name: %s", name)
	}

	seqPart := strings.TrimSuffix(strings.TrimPrefix(base, "wal_"), ".wal")

	return strconv.ParseInt(seqPart, 10, 64)
}

// loadMeta reads meta file
// loadWalMeta reads meta file if exists; if not, sets defaults.
func (walManager *WALManager) loadWalMeta() error {
	// If metaPath is empty, set default meta and return
	if walManager.metaPath == "" {
		walManager.Meta = WalMeta{LastFlushedSeq: 0, CurrentSeq: 1, LastOffset: 0}
		return nil
	}

	walMetaFile, err := os.ReadFile(walManager.metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			walManager.Meta = WalMeta{LastFlushedSeq: 0, CurrentSeq: 1, LastOffset: 0}
			return nil
		}
		return err
	}

	var walMeta WalMeta
	if len(walMetaFile) == 0 {
		walManager.Meta = WalMeta{LastFlushedSeq: 0, CurrentSeq: 1, LastOffset: 0}
		return nil
	}

	if err := json.Unmarshal(walMetaFile, &walMeta); err != nil {
		return err
	}

	// Ensure defaults
	if walMeta.CurrentSeq == 0 {
		walMeta.CurrentSeq = 1
	}

	walManager.Meta = walMeta

	// reflect last offset into manager
	walManager.lastOffset = walMeta.LastOffset
	return nil
}

// saveWalMeta writes wal meta atomically (write tmp -> rename)
func (walManager *WALManager) saveWalMeta(m *WalMeta) error {
	if walManager.metaPath == "" {
		return nil
	}

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	tmp := walManager.metaPath + ".temp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}

	// Close original meta file first
	if walManager.metaFile != nil {
		walManager.metaFile.Close()
		walManager.metaFile = nil
	}

	// Atomically replace old meta with new one
	if err := os.Rename(tmp, walManager.metaPath); err != nil {
		return err
	}

	// Re-open metaFile for further use
	metaFile, err := os.OpenFile(walManager.metaPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	walManager.metaFile = metaFile

	return nil
}

// Rotate closes current WAL and meta, opens new ones
func (walManager *WALManager) Rotate() error {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	if walManager.walFile != nil {
		walManager.walFile.Close()
	}
	if walManager.metaFile != nil {
		walManager.metaFile.Close()
	}

	walManager.Meta.CurrentSeq++
	walManager.Meta.LastOffset = 0
	walManager.lastOffset = 0

	newWalPath := filepath.Join(walManager.dir, fmt.Sprintf(walFilePattern, walManager.Meta.CurrentSeq))
	walFile, err := os.OpenFile(newWalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	walManager.walFile = walFile
	walManager.walPath = newWalPath
	walManager.lastOffset = 0

	return walManager.saveWalMeta(&walManager.Meta)
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

// Truncate clears current wal file and resets lastOffset. Useful in single-file workflows.
// With multi-file rotation prefer MarkFlushed.
func (walManager *WALManager) Truncate() error {
	walManager.mutex.Lock()
	defer walManager.mutex.Unlock()

	// close current handle, truncate underlying file
	if walManager.walFile != nil {
		_ = walManager.walFile.Close()
	}

	if err := os.WriteFile(walManager.walPath, []byte{}, 0644); err != nil {
		return err
	}

	// reopen for append
	f, err := os.OpenFile(walManager.walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	walManager.walFile = f
	walManager.lastOffset = 0
	walManager.Meta.LastOffset = 0

	// truncate meta (not strictly necessary) — reset last_offset
	if err := walManager.saveWalMeta(&walManager.Meta); err != nil {
		return err
	}
	return nil
}
