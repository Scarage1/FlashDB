package snapshot

import (
	"os"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "flashdb-snap-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestCreateAndLoad(t *testing.T) {
	mgr, err := NewManager(tempDir(t))
	if err != nil {
		t.Fatal(err)
	}

	snap := &Snapshot{
		ID: "test-1",
		Strings: []KVEntry{
			{Key: "k1", Value: "v1", Type: "string"},
			{Key: "k2", Value: "v2", Type: "string", ExpireAt: 9999999},
		},
		Hashes: []HashEntry{
			{Key: "h1", Fields: map[string]string{"f1": "a", "f2": "b"}},
		},
	}

	meta, err := mgr.Create(snap)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID != "test-1" {
		t.Fatalf("expected id test-1, got %s", meta.ID)
	}
	if meta.SizeBytes == 0 {
		t.Fatal("expected non-zero size")
	}

	loaded, err := mgr.Load("test-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Strings) != 2 {
		t.Fatalf("expected 2 strings, got %d", len(loaded.Strings))
	}
	if loaded.Strings[0].Key != "k1" || loaded.Strings[0].Value != "v1" {
		t.Fatalf("unexpected string entry: %+v", loaded.Strings[0])
	}
	if len(loaded.Hashes) != 1 {
		t.Fatalf("expected 1 hash, got %d", len(loaded.Hashes))
	}
	if loaded.Hashes[0].Fields["f1"] != "a" {
		t.Fatal("hash field mismatch")
	}
}

func TestList(t *testing.T) {
	mgr, err := NewManager(tempDir(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"a", "b", "c"} {
		_, err := mgr.Create(&Snapshot{ID: id})
		if err != nil {
			t.Fatal(err)
		}
	}

	metas, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(metas))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	mgr, err := NewManager(tempDir(t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Create(&Snapshot{ID: "del-me"})
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Delete("del-me"); err != nil {
		t.Fatal(err)
	}

	metas, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 0 {
		t.Fatal("expected empty list after delete")
	}
}

func TestLoad_NotFound(t *testing.T) {
	mgr, err := NewManager(tempDir(t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}
