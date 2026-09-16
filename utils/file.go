package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultOutputDir = "./downloads"
	EnvDownloadDir   = "YAMDL_DOWNLOAD_DIR"
)

// ErrOutputPathNotDirectory indicates that the output path is not a directory.
var ErrOutputPathNotDirectory = errors.New("output path is not a directory")

// ResolveOutputDir returns the download destination directory configured via the
// YAMDL_DOWNLOAD_DIR environment variable, falling back to DefaultOutputDir ("./downloads").
func ResolveOutputDir() string {
	if dir := strings.TrimSpace(os.Getenv(EnvDownloadDir)); dir != "" {
		return dir
	}
	return DefaultOutputDir
}

func EnsureOutputDir(path string) error {
	if err := CreateDirIfNotExists(path); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to inspect output directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrOutputPathNotDirectory, path)
	}

	// Verify write permission by creating and immediately removing a temporary test file.
	testFile, err := os.CreateTemp(path, ".write_test_*")
	if err != nil {
		return fmt.Errorf("output directory is not writable: %w", err)
	}
	closeErr := testFile.Close()
	removeErr := os.Remove(testFile.Name())
	if err := errors.Join(closeErr, removeErr); err != nil {
		return fmt.Errorf("failed to finalize output directory probe: %w", err)
	}

	return nil
}

func CreateDirIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

func SanitizeFilename(filename string) string {
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}
	return filename
}

func FileExists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
