package index

import (
	"fmt"
	"math"
	"sync"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
	"github.com/tidwall/btree"
)

// IndexEntry represents a pointer from an index to an actual log record.
type IndexEntry struct {
	Key       string // extracted field (e.g., log level, timestamp, user_id, status_code, etc.)
	FileName  string // segment file containing the log
	Offset    int64  // byte offset inside the segment
	Timestamp int64  // for range queries
}

// LessFunc defines ordering for IndexEntry in a BTree.
type LessFunc func(a, b IndexEntry) bool

// Index represents a single user-defined index
type Index struct {
	Name      string
	Extractor func(*types.LogEntry) string // extracts key from a log entry
	Tree      *btree.BTreeG[IndexEntry]
}

// IndexManager manages multiple user-defined indexes.
type IndexManager struct {
	indexes map[string]*Index
	mutex   sync.RWMutex
}

// NewIndexManager creates a new empty index manager with default timestamp index.
func NewIndexManager() *IndexManager {
	indexManager := &IndexManager{
		indexes: make(map[string]*Index),
	}

	// Default timestamp index
	indexManager.CreateIndex("timestamp", func(logEntry *types.LogEntry) string {
		// pad timestamp so string comparison is numeric
		return fmt.Sprintf("%d", logEntry.Timestamp)
	})

	return indexManager
}

// CreateIndex defines a new index with a name and extractor function
func (indexManager *IndexManager) CreateIndex(name string, extractor func(*types.LogEntry) string) {
	indexManager.mutex.Lock()
	defer indexManager.mutex.Unlock()

	// comparator: sort by Key first, then Timestamp for uniqueness
	comparator := func(a, b IndexEntry) bool {
		if a.Key == b.Key {
			return a.Timestamp < b.Timestamp
		}
		return a.Key < b.Key
	}

	indexManager.indexes[name] = &Index{
		Name:      name,
		Extractor: extractor,
		Tree:      btree.NewBTreeG(comparator),
	}
}

// DropIndex removes an index.
func (indexManager *IndexManager) DropIndex(name string) error {
	indexManager.mutex.Lock()
	defer indexManager.mutex.Unlock()

	if _, exists := indexManager.indexes[name]; !exists {
		return fmt.Errorf("index %s not found", name)
	}

	delete(indexManager.indexes, name)
	return nil
}

// Insert inserts a log into all indexes
func (indexManager *IndexManager) Insert(entry *types.LogEntry, fileName string, offset int64) {
	indexManager.mutex.RLock()
	defer indexManager.mutex.RUnlock()

	for _, idx := range indexManager.indexes {
		key := idx.Extractor(entry)

		if key == "" {
			continue
		}

		idx.Tree.Set(IndexEntry{
			Key:       key,
			FileName:  fileName,
			Offset:    offset,
			Timestamp: entry.Timestamp,
		})
	}
}

// Search looks up entries by index name and key
func (indexManager *IndexManager) Search(indexName, key string) []IndexEntry {
	indexManager.mutex.RLock()
	defer indexManager.mutex.RUnlock()

	idx, ok := indexManager.indexes[indexName]
	if !ok {
		return nil
	}

	results := []IndexEntry{}

	// pivot search starts at (key, timestamp=min value)
	pivot := IndexEntry{Key: key, Timestamp: math.MinInt64}

	idx.Tree.Ascend(pivot, func(item IndexEntry) bool {
		if item.Key != key {
			return false // stop once keys don't match
		}
		results = append(results, item)
		return true
	})

	return results
}

// RangeSearch performs a range query based on timestamp
func (indexManager *IndexManager) RangeSearch(indexName string, start, end int64) []IndexEntry {
	indexManager.mutex.RLock()
	defer indexManager.mutex.RUnlock()

	idx, ok := indexManager.indexes[indexName]
	if !ok {
		return nil
	}

	results := []IndexEntry{}
	idx.Tree.Ascend(IndexEntry{}, func(item IndexEntry) bool {
		if item.Timestamp >= start && item.Timestamp <= end {
			results = append(results, item)
		}
		if item.Timestamp > end {
			return false
		}
		return true
	})

	return results
}

func (indexManager *IndexManager) HasIndex(field string) bool {
	indexManager.mutex.RLock()
	defer indexManager.mutex.RUnlock()
	_, ok := indexManager.indexes[field]
	return ok
}

func (im *IndexManager) Lookup(indexName string, start, end int64, key string) []int64 {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	idx, ok := im.indexes[indexName]
	if !ok {
		return nil
	}

	results := []int64{}
	idx.Tree.Ascend(IndexEntry{}, func(item IndexEntry) bool {
		if item.Timestamp > end {
			return false
		}
		if item.Timestamp >= start && (key == "" || item.Key == key) {
			results = append(results, item.Offset)
		}
		return true
	})

	return results
}
