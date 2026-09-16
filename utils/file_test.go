package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDirIfNotExists(t *testing.T) {
	testDir := "test_dir"
	defer os.RemoveAll(testDir)

	err := CreateDirIfNotExists(testDir)
	assert.NoError(t, err)

	_, err = os.Stat(testDir)
	assert.False(t, os.IsNotExist(err))

	err = CreateDirIfNotExists(testDir)
	assert.NoError(t, err)
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"file/with\\invalid:chars*?.txt", "file_with_invalid_chars__.txt"},
		{"file\"with<>|chars.txt", "file_with___chars.txt"},
		{"", ""},
	}

	for _, test := range tests {
		result := SanitizeFilename(test.input)
		assert.Equal(t, test.expected, result)
	}
}

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	exists, err := FileExists(tmpFile.Name())
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = FileExists("non_existing_file")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestResolveOutputDir(t *testing.T) {
	t.Run("default fallback", func(t *testing.T) {
		t.Setenv(EnvDownloadDir, "")
		assert.Equal(t, DefaultOutputDir, ResolveOutputDir())
	})

	t.Run("from environment variable", func(t *testing.T) {
		t.Setenv(EnvDownloadDir, "/custom/music/path")
		assert.Equal(t, "/custom/music/path", ResolveOutputDir())
	})

	t.Run("whitespace fallback", func(t *testing.T) {
		t.Setenv(EnvDownloadDir, "   ")
		assert.Equal(t, DefaultOutputDir, ResolveOutputDir())
	})
}

func TestEnsureOutputDir(t *testing.T) {
	t.Run("creates valid directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "downloads")
		err := EnsureOutputDir(dir)
		assert.NoError(t, err)

		info, err := os.Stat(dir)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("fails when path is a file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "not_a_dir")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		err = EnsureOutputDir(tmpFile.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("fails when directory is not writable", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping chmod write-permission test on Windows")
		}

		roDir := filepath.Join(t.TempDir(), "readonly")
		err := os.Mkdir(roDir, 0555)
		assert.NoError(t, err)
		defer os.Chmod(roDir, 0755)

		err = EnsureOutputDir(roDir)
		if err == nil {
			t.Skip("skipping: running as root or filesystem ignores 0555 permissions")
		}
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not writable")
	})
}
