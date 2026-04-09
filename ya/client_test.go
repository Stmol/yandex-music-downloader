package ya

import (
	"os"
	"path/filepath"
	"testing"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadTrackReturnsAlreadyExistsSentinel(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("1"),
		Title:     "Existing",
		Available: true,
	}
	filename := buildTrackFilename(track, outputDir)
	require.NoError(t, os.WriteFile(filename, []byte("existing"), 0644))

	gotFilename, err := NewClient(nil).DownloadTrackWithOptions(track, outputDir, DownloadOptions{})

	assert.Equal(t, filename, gotFilename)
	assert.ErrorIs(t, err, ErrTrackAlreadyExists)
}

func TestBuildTrackFilenameUsesCanonicalArtistTrackPattern(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:      model.FlexibleID("123"),
		Title:   "Track/Name",
		Version: "Live",
		Artists: []model.Artist{
			{Name: "Artist: One"},
			{Name: "Artist Two"},
		},
	}

	filename := buildTrackFilename(track, outputDir)

	assert.Equal(t, filepath.Join(outputDir, "Artist_ One, Artist Two - Track_Name Live.mp3"), filename)
}

func TestTrackFilenameBaseFallsBackWhenArtistIsMissing(t *testing.T) {
	track := model.Track{
		ID:    model.FlexibleID("123"),
		Title: "Track Name",
	}

	assert.Equal(t, "Track Name", trackFilenameBase(track))
}
