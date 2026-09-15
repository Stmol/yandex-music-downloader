package utils

import (
	"fmt"
	"os"
	"strings"
)

const (
	DefaultOutputDir = "./downloads"
	EnvDownloadDir   = "YAMDL_DOWNLOAD_DIR"
)

// ResolveOutputDir returns the download destination directory configured via the
// YAMDL_DOWNLOAD_DIR environment variable, falling back to DefaultOutputDir ("./downloads").
func ResolveOutputDir() string {
	if dir := strings.TrimSpace(os.Getenv(EnvDownloadDir)); dir != "" {
		return dir
	}
	return DefaultOutputDir
}

// EnsureOutputDir creates the directory if it does not exist and verifies it is accessible and a directory.
func EnsureOutputDir(path string) error {
	if err := CreateDirIfNotExists(path); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to inspect output directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path is not a directory: %s", path)
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
