package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type SegmentMeta struct {
	FileName     string `json:"file_name"`
	Size         int64  `json:"size"`
	MinTimestamp int64  `json:"min_timestamp"`
	MaxTimestamp int64  `json:"max_timestamp"`
}

type Manifest struct {
	Segments []SegmentMeta `json:"segments"`
	path     string
	mutex    sync.Mutex
}

// NewManifest loads or creates manifest file
func NewManifest(path string) (*Manifest, error) {

	// If path is a directory, append manifest.json
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		path = filepath.Join(path, "manifest.json")
	}

	manifest := &Manifest{
		Segments: []SegmentMeta{},
		path:     path,
	}

	// Ensure manifest directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Try to read existing manifest
	manifestFile, err := os.Open(path)
	if err != nil {
		// If file does not exist, create empty manifest
		if os.IsNotExist(err) {
			manifest.Segments = []SegmentMeta{}
			return manifest, nil
		}

		return nil, err
	}
	defer manifestFile.Close()

	decoder := json.NewDecoder(manifestFile)
	if err := decoder.Decode(&manifest.Segments); err != nil {
		// empty manifest is okay
		manifest.Segments = []SegmentMeta{}
	}

	return manifest, nil
}

// AddSegment adds a new segment metadata and saves manifest
func (manifest *Manifest) AddSegment(meta SegmentMeta) error {
	manifest.mutex.Lock()
	defer manifest.mutex.Unlock()

	manifest.Segments = append(manifest.Segments, meta)
	return manifest.save()
}

// save writes the manifest to disk atomically
func (manifest *Manifest) save() error {
	tempPath := manifest.path + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(manifest.Segments); err != nil {
		file.Close()
		return err
	}

	file.Close()

	// Rename temp → manifest (atomic on most OSes)
	return os.Rename(tempPath, manifest.path)
}

// GetSegments returns a copy of all segment metadata
func (manifest *Manifest) GetSegments() []SegmentMeta {
	manifest.mutex.Lock()
	defer manifest.mutex.Unlock()

	segments := make([]SegmentMeta, len(manifest.Segments))
	copy(segments, manifest.Segments)
	return segments
}
