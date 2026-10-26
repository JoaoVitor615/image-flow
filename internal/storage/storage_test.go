package storage_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/storage"
)

func TestDiskStorage(t *testing.T) {
	tempDir := t.TempDir()

	strg, err := storage.NewDiskStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create disk storage: %v", err)
	}

	t.Run("should create originals and variants directories", func(t *testing.T) {
		for _, subDir := range []string{"originals", "variants"} {
			path := filepath.Join(tempDir, subDir)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("expected directory %s to exist: %v", path, err)
			}
			if !info.IsDir() {
				t.Fatalf("expected %s to be a directory", path)
			}
		}
	})

	t.Run("should save and open original file correctly", func(t *testing.T) {
		content := []byte("fake image data")
		savedPath, err := strg.SaveOriginal("test-image.jpg", bytes.NewReader(content))
		if err != nil {
			t.Fatalf("failed to save original: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "originals", "test-image.jpg")
		if savedPath != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, savedPath)
		}

		rc, err := strg.Open(savedPath)
		if err != nil {
			t.Fatalf("failed to open saved file: %v", err)
		}
		defer rc.Close()

		readBytes, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read file content: %v", err)
		}

		if !bytes.Equal(readBytes, content) {
			t.Fatalf("expected content '%s', got '%s'", content, readBytes)
		}
	})

	t.Run("should save variant file correctly", func(t *testing.T) {
		content := []byte("variant image data")
		savedPath, err := strg.SaveVariant("thumb.webp", bytes.NewReader(content))
		if err != nil {
			t.Fatalf("failed to save variant: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "variants", "thumb.webp")
		if savedPath != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, savedPath)
		}
	})

	t.Run("should prevent directory traversal and sanitize filename", func(t *testing.T) {
		maliciousName := "../../malicious.png"
		savedPath, err := strg.SaveOriginal(maliciousName, bytes.NewReader([]byte("payload")))
		if err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "originals", "malicious.png")
		if savedPath != expectedPath {
			t.Fatalf("expected safe path %s, got %s", expectedPath, savedPath)
		}
	})

	t.Run("should return error when opening non-existent file", func(t *testing.T) {
		_, err := strg.Open(filepath.Join(tempDir, "non_existent.jpg"))
		if err == nil {
			t.Fatal("expected error opening non-existent file, got nil")
		}
	})
}
