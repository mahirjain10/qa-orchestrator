package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestStore(t *testing.T) (*ArtifactStore, string) {
	tmpDir := t.TempDir()
	store, err := NewArtifactStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create artifact store: %v", err)
	}
	return store, tmpDir
}

func TestNewArtifactStore(t *testing.T) {
	store, _ := setupTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSave(t *testing.T) {
	store, _ := setupTestStore(t)

	artifact, err := store.Save("run_1", "flow_1", ArtifactScreenshot, "test.png", []byte("fake image data"), nil)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected non-nil artifact")
	}
	if artifact.ArtifactID == "" {
		t.Fatal("expected artifact ID to be set")
	}
	if _, err := os.Stat(artifact.Path); err != nil {
		t.Fatalf("artifact file not found: %v", err)
	}
}

func TestSaveFromFile(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	sourcePath := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourcePath, []byte("test content"), 0644)

	artifact, err := store.SaveFromFile("run_1", "flow_1", ArtifactLog, sourcePath, nil)
	if err != nil {
		t.Fatalf("save from file failed: %v", err)
	}
	if artifact.Size != 12 {
		t.Fatalf("expected size 12, got %d", artifact.Size)
	}
}

func TestGetByRunID(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Save("run_1", "flow_1", ArtifactScreenshot, "shot1.png", []byte("data1"), nil)
	store.Save("run_1", "flow_2", ArtifactScreenshot, "shot2.png", []byte("data2"), nil)
	store.Save("run_2", "flow_3", ArtifactScreenshot, "shot3.png", []byte("data3"), nil)

	artifacts, err := store.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get by run ID failed: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts for run_1, got %d", len(artifacts))
	}
}

func TestGetByFlowID(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Save("run_1", "flow_1", ArtifactScreenshot, "shot1.png", []byte("data1"), nil)
	store.Save("run_1", "flow_1", ArtifactLog, "log1.log", []byte("log data"), nil)
	store.Save("run_1", "flow_2", ArtifactScreenshot, "shot2.png", []byte("data2"), nil)

	artifacts, err := store.GetByFlowID("run_1", "flow_1")
	if err != nil {
		t.Fatalf("get by flow ID failed: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts for flow_1, got %d", len(artifacts))
	}
}

func TestListByType(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Save("run_1", "flow_1", ArtifactScreenshot, "shot1.png", []byte("data1"), nil)
	store.Save("run_1", "flow_2", ArtifactScreenshot, "shot2.png", []byte("data2"), nil)
	store.Save("run_1", "flow_3", ArtifactLog, "log1.log", []byte("log"), nil)

	screenshots, err := store.ListByType("run_1", ArtifactScreenshot)
	if err != nil {
		t.Fatalf("list by type failed: %v", err)
	}
	if len(screenshots) != 2 {
		t.Fatalf("expected 2 screenshots, got %d", len(screenshots))
	}
}

func TestDelete(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Save("run_1", "flow_1", ArtifactScreenshot, "shot1.png", []byte("data1"), nil)

	err := store.Delete("run_1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	artifacts, _ := store.GetByRunID("run_1")
	if len(artifacts) != 0 {
		t.Fatalf("expected 0 artifacts after delete, got %d", len(artifacts))
	}
}

func TestGetRecentPaths(t *testing.T) {
	store, _ := setupTestStore(t)

	for i := 0; i < 5; i++ {
		store.Save("run_1", "flow_1", ArtifactScreenshot, "shot.png", []byte("data"), nil)
	}

	paths := store.GetRecentPaths("run_1", 3)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
}

func TestSaveArtifactHelper(t *testing.T) {
	store, _ := setupTestStore(t)

	path, err := SaveArtifact(store, "run_1", "flow_1", ArtifactScreenshot, "test.png", []byte("test"), nil)
	if err != nil {
		t.Fatalf("save artifact helper failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	path2, err := SaveArtifact(store, "run_1", "flow_1", ArtifactLog, "test.log", "string content", nil)
	if err != nil {
		t.Fatalf("save artifact helper with string failed: %v", err)
	}
	if path2 == "" {
		t.Fatal("expected non-empty path for string content")
	}
}

func TestGetArtifactPaths(t *testing.T) {
	store, _ := setupTestStore(t)

	for i := 0; i < 5; i++ {
		store.Save("run_1", "flow_1", ArtifactScreenshot, "shot.png", []byte("data"), nil)
	}

	paths := GetArtifactPaths(store, "run_1", ArtifactScreenshot, 3)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
}

func TestGetArtifactPathsWithNilStore(t *testing.T) {
	paths := GetArtifactPaths(nil, "run_1", ArtifactScreenshot, 3)
	if paths != nil {
		t.Fatal("expected nil for nil store")
	}
}