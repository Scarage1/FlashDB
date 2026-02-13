package snapshot

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// KVEntry represents a single key-value pair with optional TTL.
type KVEntry struct {
	Key      string
	Value    string
	ExpireAt int64
	Type     string
}

// HashEntry is a full hash map snapshot.
type HashEntry struct {
	Key    string
	Fields map[string]string
}

// Snapshot is the full serialisable state captured at a moment in time.
type Snapshot struct {
	ID        string
	CreatedAt time.Time
	Strings   []KVEntry
	Hashes    []HashEntry
}

// Meta describes a snapshot without loading the full data.
type Meta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	FilePath  string    `json:"file_path"`
}

// Manager handles snapshot CRUD backed by a directory on disk.
type Manager struct {
	dir string
}

// NewManager creates a Manager that stores snapshots in dir.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("snapshot: mkdir %s: %w", dir, err)
	}
	return &Manager{dir: dir}, nil
}

// Create serialises snap to disk and returns its metadata.
func (m *Manager) Create(snap *Snapshot) (Meta, error) {
	if snap.ID == "" {
		snap.ID = fmt.Sprintf("snap-%d", time.Now().UnixMilli())
	}
	snap.CreatedAt = time.Now()

	filename := snap.ID + ".snap"
	path := filepath.Join(m.dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return Meta{}, fmt.Errorf("snapshot: create file: %w", err)
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	if err := enc.Encode(snap); err != nil {
		return Meta{}, fmt.Errorf("snapshot: encode: %w", err)
	}

	info, _ := f.Stat()
	return Meta{
		ID:        snap.ID,
		CreatedAt: snap.CreatedAt,
		SizeBytes: info.Size(),
		FilePath:  path,
	}, nil
}

// List returns metadata for all snapshots, sorted newest first.
func (m *Manager) List() ([]Meta, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("snapshot: list dir: %w", err)
	}

	var metas []Meta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".snap") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".snap")
		metas = append(metas, Meta{
			ID:        id,
			CreatedAt: info.ModTime(),
			SizeBytes: info.Size(),
			FilePath:  filepath.Join(m.dir, e.Name()),
		})
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas, nil
}

// Load reads and decodes a snapshot from disk by ID.
func (m *Manager) Load(id string) (*Snapshot, error) {
	path := filepath.Join(m.dir, id+".snap")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot: open %s: %w", id, err)
	}
	defer f.Close()

	var snap Snapshot
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&snap); err != nil {
		return nil, fmt.Errorf("snapshot: decode %s: %w", id, err)
	}
	return &snap, nil
}

// Delete removes a snapshot file by ID.
func (m *Manager) Delete(id string) error {
	path := filepath.Join(m.dir, id+".snap")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("snapshot: delete %s: %w", id, err)
	}
	return nil
}
