package ya

import (
	"os"
	"path/filepath"
	"testing"
	"ya-music/ya/model"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	flac "github.com/go-flac/go-flac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFLACTagsWritesVorbisCommentsAndCover(t *testing.T) {
	dir := t.TempDir()
	flacPath := filepath.Join(dir, "track.flac")
	coverPath := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(flacPath, minimalFLACBytes(), 0644))
	require.NoError(t, os.WriteFile(coverPath, onePixelPNG(), 0644))

	track := model.Track{
		ID:      model.FlexibleID("123"),
		Title:   "Song",
		Version: "Live",
		Artists: []model.Artist{
			{Name: "Artist A"},
			{Name: "Artist B"},
		},
		Albums: []model.Album{{
			ID:          model.FlexibleID("456"),
			Title:       "Album",
			Genre:       "indie",
			ReleaseDate: "2025-01-02",
			TrackPosition: model.TrackPosition{
				Volume: 2,
				Index:  3,
			},
		}},
	}

	require.NoError(t, writeFLACTags(flacPath, track, coverPath))

	file, err := flac.ParseFile(flacPath)
	require.NoError(t, err)

	var comments *flacvorbis.MetaDataBlockVorbisComment
	var picture *flacpicture.MetadataBlockPicture
	for _, block := range file.Meta {
		switch block.Type {
		case flac.VorbisComment:
			comments, err = flacvorbis.ParseFromMetaDataBlock(*block)
			require.NoError(t, err)
		case flac.Picture:
			picture, err = flacpicture.ParseFromMetaDataBlock(*block)
			require.NoError(t, err)
		}
	}

	require.NotNil(t, comments)
	assertFLACComment(t, comments, "TITLE", "Song Live")
	assertFLACComment(t, comments, "ARTIST", "Artist A, Artist B")
	assertFLACComment(t, comments, "ALBUM", "Album")
	assertFLACComment(t, comments, "ALBUMARTIST", "Artist A, Artist B")
	assertFLACComment(t, comments, "GENRE", "indie")
	assertFLACComment(t, comments, "DATE", "2025")
	assertFLACComment(t, comments, "TRACKNUMBER", "3")
	assertFLACComment(t, comments, "DISCNUMBER", "2")
	assertFLACComment(t, comments, "YANDEX_TRACK_ID", "123")
	assertFLACComment(t, comments, "COMMENT", "https://music.yandex.ru/album/456/track/123")

	require.NotNil(t, picture)
	assert.Equal(t, flacpicture.PictureTypeFrontCover, picture.PictureType)
	assert.Equal(t, "image/png", picture.MIME)
}

func TestWriteFLACTagsIgnoresInvalidCover(t *testing.T) {
	dir := t.TempDir()
	flacPath := filepath.Join(dir, "track.flac")
	coverPath := filepath.Join(dir, "cover.bin")
	require.NoError(t, os.WriteFile(flacPath, minimalFLACBytes(), 0644))
	require.NoError(t, os.WriteFile(coverPath, []byte("not an image"), 0644))

	err := writeFLACTags(flacPath, model.Track{ID: model.FlexibleID("123"), Title: "Song"}, coverPath)

	require.NoError(t, err)
	file, err := flac.ParseFile(flacPath)
	require.NoError(t, err)
	for _, block := range file.Meta {
		assert.NotEqual(t, flac.Picture, block.Type)
	}
}

func assertFLACComment(t *testing.T, comments *flacvorbis.MetaDataBlockVorbisComment, key string, want string) {
	t.Helper()

	values, err := comments.Get(key)
	require.NoError(t, err)
	require.NotEmpty(t, values)
	assert.Equal(t, want, values[0])
}

func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
}
