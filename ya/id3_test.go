package ya

import (
	"os"
	"path/filepath"
	"testing"
	"ya-music/ya/model"

	"github.com/bogem/id3v2/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
	0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestWriteID3TagsWritesBasicMetadataAndCover(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "track.mp3")
	coverPath := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(mp3Path, []byte("audio payload"), 0644))
	require.NoError(t, os.WriteFile(coverPath, tinyPNG, 0644))

	track := model.Track{
		ID:      model.FlexibleID("123"),
		Title:   "Song",
		Version: "Acoustic",
		Artists: []model.Artist{
			{Name: "Artist A"},
			{Name: "Artist B"},
		},
		Albums: []model.Album{
			{
				Title:       "Album",
				Genre:       "indie",
				Year:        2025,
				ReleaseDate: "2024-01-02",
				TrackPosition: model.TrackPosition{
					Index: 3,
				},
			},
		},
	}

	require.NoError(t, writeID3Tags(mp3Path, track, coverPath))

	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	require.NoError(t, err)
	defer tag.Close()

	assert.Equal(t, "Song Acoustic", tag.Title())
	assert.Equal(t, "Artist A, Artist B", tag.Artist())
	assert.Equal(t, "Album", tag.Album())
	assert.Equal(t, "indie", tag.Genre())
	assert.Equal(t, "2025", tag.Year())
	assert.Equal(t, "3", tag.GetTextFrame(tag.CommonID("Track number/Position in set")).Text)

	pictures := tag.GetFrames(tag.CommonID("Attached picture"))
	require.Len(t, pictures, 1)
	picture, ok := pictures[0].(id3v2.PictureFrame)
	require.True(t, ok)
	assert.Equal(t, "image/png", picture.MimeType)
	assert.Equal(t, byte(id3v2.PTFrontCover), picture.PictureType)

	ufids := tag.GetFrames(tag.CommonID("Unique file identifier"))
	require.Len(t, ufids, 1)
	ufid, ok := ufids[0].(id3v2.UFIDFrame)
	require.True(t, ok)
	assert.Equal(t, yandexTrackOwnerIdentifier, ufid.OwnerIdentifier)
	assert.Equal(t, []byte("123"), ufid.Identifier)
}

func TestWriteID3TagsUsesReleaseDateYearFallback(t *testing.T) {
	mp3Path := filepath.Join(t.TempDir(), "track.mp3")
	require.NoError(t, os.WriteFile(mp3Path, []byte("audio payload"), 0644))

	track := model.Track{
		ID:    model.FlexibleID("123"),
		Title: "Song",
		Albums: []model.Album{
			{ReleaseDate: "2022-10-20"},
		},
		MetaData: model.MetaData{Year: 2021},
	}

	require.NoError(t, writeID3Tags(mp3Path, track, ""))

	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	require.NoError(t, err)
	defer tag.Close()

	assert.Equal(t, "2022", tag.Year())
	assert.Empty(t, tag.GetFrames(tag.CommonID("Attached picture")))
}

func TestWriteID3TagsIgnoresMissingCover(t *testing.T) {
	mp3Path := filepath.Join(t.TempDir(), "track.mp3")
	require.NoError(t, os.WriteFile(mp3Path, []byte("audio payload"), 0644))

	track := model.Track{
		ID:    model.FlexibleID("123"),
		Title: "Song",
	}

	require.NoError(t, writeID3Tags(mp3Path, track, filepath.Join(t.TempDir(), "missing.jpg")))
}

func TestWriteID3TagsReturnsErrorForMissingMP3(t *testing.T) {
	err := writeID3Tags(filepath.Join(t.TempDir(), "missing.mp3"), model.Track{Title: "Song"}, "")

	assert.Error(t, err)
}
