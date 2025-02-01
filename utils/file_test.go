package utils

import (
	"os"
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
