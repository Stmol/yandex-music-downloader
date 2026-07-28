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

func onePixelJPEG() []byte {
	return []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
		0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
		0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
		0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
		0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x14, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x08, 0xff, 0xc4, 0x00, 0x14, 0x10, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00,
		0x7f, 0xff, 0xd9,
	}
}
