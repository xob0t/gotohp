package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddAndRemoveAutoBackupFolder(t *testing.T) {
	restoreConfigGlobals(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gotohp.config")
	if err := LoadConfig(cfgPath); err != nil {
		t.Fatal(err)
	}

	folder1 := filepath.Join(dir, "folder1")
	folder2 := filepath.Join(dir, "folder2")
	_ = os.Mkdir(folder1, 0o755)
	_ = os.Mkdir(folder2, 0o755)

	cm := &ConfigManager{}
	folders, err := cm.AddAutoBackupFolder(folder1)
	if err != nil {
		t.Fatalf("AddAutoBackupFolder 1: %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}

	folders, err = cm.AddAutoBackupFolder(folder2)
	if err != nil {
		t.Fatalf("AddAutoBackupFolder 2: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}

	// Reload config from disk to test persistence
	if err := LoadConfig(cfgPath); err != nil {
		t.Fatal(err)
	}
	loaded := cm.GetAutoBackup()
	if len(loaded.Folders) != 2 {
		t.Fatalf("after reload: expected 2 folders, got %d: %+v", len(loaded.Folders), loaded.Folders)
	}
}
