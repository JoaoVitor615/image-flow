package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage defines the contract for persisting and retrieving file artifacts
type Storage interface {
	SaveOriginal(filename string, r io.Reader) (string, error)
	SaveVariant(filename string, r io.Reader) (string, error)
	Open(path string) (io.ReadCloser, error)
}

// DiskStorage implements Storage using the local filesystem
type DiskStorage struct {
	basePath     string
	originalsDir string
	variantsDir  string
}

// NewDiskStorage creates a new DiskStorage instance and ensures directories exist
func NewDiskStorage(basePath string) (*DiskStorage, error) {
	originalsDir := filepath.Join(basePath, "originals")
	variantsDir := filepath.Join(basePath, "variants")

	for _, dir := range []string{originalsDir, variantsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &DiskStorage{
		basePath:     basePath,
		originalsDir: originalsDir,
		variantsDir:  variantsDir,
	}, nil
}

// SaveOriginal writes an original source image file to disk and returns its saved path
func (s *DiskStorage) SaveOriginal(filename string, r io.Reader) (string, error) {
	safeFilename := filepath.Base(filename)
	destPath := filepath.Join(s.originalsDir, safeFilename)

	if err := s.writeFile(destPath, r); err != nil {
		return "", fmt.Errorf("failed to save original file %s: %w", safeFilename, err)
	}

	return destPath, nil
}

// SaveVariant writes a generated derivative image variant to disk and returns its saved path
func (s *DiskStorage) SaveVariant(filename string, r io.Reader) (string, error) {
	safeFilename := filepath.Base(filename)
	destPath := filepath.Join(s.variantsDir, safeFilename)

	if err := s.writeFile(destPath, r); err != nil {
		return "", fmt.Errorf("failed to save variant file %s: %w", safeFilename, err)
	}

	return destPath, nil
}

// Open opens a stored file for reading
func (s *DiskStorage) Open(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file at %s: %w", path, err)
	}
	return file, nil
}

// writeFile streams contents from reader into destination file
func (s *DiskStorage) writeFile(destPath string, r io.Reader) error {
	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("failed to copy content to file: %w", err)
	}

	return nil
}
