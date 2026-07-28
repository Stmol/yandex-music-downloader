package ya

import (
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tommyo123/mtag"
)

func TestM4ATagInputForTrack(t *testing.T) {
	cover := onePixelPNG()

	tests := []struct {
		name      string
		track     model.Track
		album     model.Album
		coverMIME string
		cover     []byte
		want      m4aTagInput
	}{
		{
			name: "full album",
			track: model.Track{
				ID:      model.FlexibleID("123"),
				Title:   "Song",
				Version: "Live",
				Artists: []model.Artist{{Name: "Artist A"}, {Name: "Artist B"}},
				Albums: []model.Album{{
					ID:    model.FlexibleID("456"),
					Title: "Album",
					Year:  2025,
				}},
			},
			album: model.Album{
				ID:         model.FlexibleID("456"),
				Title:      "Album",
				Genre:      "indie",
				Year:       2025,
				TrackCount: 11,
				TrackPosition: model.TrackPosition{
					Volume: 2,
					Index:  3,
				},
				Volumes: [][]model.Track{{}, {}, {}, {}},
			},
			coverMIME: "image/png",
			cover:     cover,
			want: m4aTagInput{
				Title:       "Song Live",
				Artist:      "Artist A, Artist B",
				Album:       "Album",
				AlbumArtist: "Artist A, Artist B",
				Genre:       "indie",
				Year:        2025,
				Track:       3,
				TrackTotal:  11,
				Disc:        2,
				DiscTotal:   4,
				SourceURL:   "https://music.yandex.ru/album/456/track/123",
				CoverMIME:   "image/png",
				CoverData:   cover,
			},
		},
		{
			name: "compilation missing album",
			track: model.Track{
				ID:      model.FlexibleID("99"),
				Title:   "Standalone",
				Artists: []model.Artist{{Name: "Solo"}},
			},
			want: m4aTagInput{
				Title:     "Standalone",
				Artist:    "Solo",
				SourceURL: "https://music.yandex.ru/track/99",
			},
		},
		{
			name: "missing year",
			track: model.Track{
				ID:      model.FlexibleID("7"),
				Title:   "No Year",
				Artists: []model.Artist{{Name: "Artist"}},
				Albums: []model.Album{{
					ID:    model.FlexibleID("8"),
					Title: "Album",
				}},
			},
			album: model.Album{
				ID:    model.FlexibleID("8"),
				Title: "Album",
			},
			want: m4aTagInput{
				Title:       "No Year",
				Artist:      "Artist",
				Album:       "Album",
				AlbumArtist: "Artist",
				SourceURL:   "https://music.yandex.ru/album/8/track/7",
			},
		},
		{
			name: "missing track position",
			track: model.Track{
				ID:      model.FlexibleID("5"),
				Title:   "No Position",
				Artists: []model.Artist{{Name: "Artist"}},
				Albums: []model.Album{{
					ID:    model.FlexibleID("6"),
					Title: "Album",
					Year:  2020,
				}},
			},
			album: model.Album{
				ID:    model.FlexibleID("6"),
				Title: "Album",
				Year:  2020,
			},
			want: m4aTagInput{
				Title:       "No Position",
				Artist:      "Artist",
				Album:       "Album",
				AlbumArtist: "Artist",
				Year:        2020,
				SourceURL:   "https://music.yandex.ru/album/6/track/5",
			},
		},
		{
			name: "empty cover",
			track: model.Track{
				ID:      model.FlexibleID("1"),
				Title:   "Song",
				Artists: []model.Artist{{Name: "Artist"}},
			},
			coverMIME: "image/png",
			cover:     nil,
			want: m4aTagInput{
				Title:     "Song",
				Artist:    "Artist",
				SourceURL: "https://music.yandex.ru/track/1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m4aTagInputForTrack(tt.track, tt.album, tt.coverMIME, tt.cover)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteM4ATagsWritesFieldsAndCover(t *testing.T) {
	path := copyFixture(t, "taggable-stco.m4a")
	cover := onePixelPNG()
	tags := sampleM4ATagInput(cover)

	require.NoError(t, writeM4ATags(path, tags))

	file := openTaggedM4A(t, path)
	assert.Equal(t, mtag.ContainerMP4, file.Container())
	assert.Equal(t, tags.Title, file.Title())
	assert.Equal(t, tags.Artist, file.Artist())
	assert.Equal(t, tags.Album, file.Album())
	assert.Equal(t, tags.AlbumArtist, file.AlbumArtist())
	assert.Equal(t, tags.Genre, file.Genre())
	assert.Equal(t, tags.Year, file.Year())
	assert.Equal(t, tags.Track, file.Track())
	assert.Equal(t, tags.TrackTotal, file.TrackTotal())
	assert.Equal(t, tags.Disc, file.Disc())
	assert.Equal(t, tags.DiscTotal, file.DiscTotal())
	assert.Equal(t, tags.SourceURL, file.CustomValue(m4aSourceURLField))

	images := file.Images()
	require.Len(t, images, 1)
	assert.Equal(t, tags.CoverMIME, images[0].MIME)
	assert.Equal(t, tags.CoverData, images[0].Data)
}

func TestWriteM4ATagsPreservesCO64(t *testing.T) {
	path := copyFixture(t, "taggable-co64.m4a")
	require.True(t, hasMP4Atom(t, path, "co64"))

	tags := sampleM4ATagInput(nil)
	require.NoError(t, writeM4ATags(path, tags))
	require.True(t, hasMP4Atom(t, path, "co64"))

	file := openTaggedM4A(t, path)
	assert.Equal(t, tags.Title, file.Title())
	assert.Equal(t, tags.Artist, file.Artist())
	assert.Equal(t, tags.SourceURL, file.CustomValue(m4aSourceURLField))

	audio := file.AudioProperties()
	assert.NotEmpty(t, audio.Codec)
	assert.Positive(t, audio.SampleRate)
	assert.Positive(t, audio.Channels)
}

func TestWriteM4ATagsRejectsMalformedInputWithoutMutation(t *testing.T) {
	path := copyFixture(t, "malformed-truncated-track.m4a")
	before := sha256File(t, path)

	err := writeM4ATags(path, sampleM4ATagInput(nil))

	require.Error(t, err)
	assert.Equal(t, before, sha256File(t, path))
}

func TestWriteM4ATagsRejectsOversizeByConfiguredLimit(t *testing.T) {
	path := copyFixture(t, "taggable-stco.m4a")
	before := sha256File(t, path)

	originalLimit := m4aTaggingFileSizeLimit
	m4aTaggingFileSizeLimit = 8
	t.Cleanup(func() {
		m4aTaggingFileSizeLimit = originalLimit
	})

	err := writeM4ATags(path, sampleM4ATagInput(nil))

	require.Error(t, err)
	assert.ErrorIs(t, err, mtag.ErrFileTooLarge)
	assert.Equal(t, before, sha256File(t, path))
}

func sampleM4ATagInput(cover []byte) m4aTagInput {
	return m4aTagInput{
		Title:       "Song Live",
		Artist:      "Artist A, Artist B",
		Album:       "Album",
		AlbumArtist: "Artist A, Artist B",
		Genre:       "indie",
		Year:        2025,
		Track:       3,
		TrackTotal:  11,
		Disc:        2,
		DiscTotal:   4,
		SourceURL:   "https://music.yandex.ru/album/456/track/123",
		CoverMIME:   "image/png",
		CoverData:   cover,
	}
}

func openTaggedM4A(t *testing.T, path string) *mtag.File {
	t.Helper()

	file, err := mtag.Open(path, mtag.WithReadOnly())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
	})
	return file
}

func hasMP4Atom(t *testing.T, path string, target string) bool {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	_, ok, err := findMP4Atom(data, "", target)
	require.NoError(t, err)
	return ok
}

type mp4Atom struct {
	start      int
	end        int
	size       int
	headerSize int
	typ        string
}

func findMP4Atom(data []byte, parentType string, targetType string) (mp4Atom, bool, error) {
	return findMP4AtomFrom(data, 0, parentType, targetType)
}

func findMP4AtomFrom(data []byte, baseOffset int, parentType string, targetType string) (mp4Atom, bool, error) {
	for offset := 0; offset < len(data); {
		current, err := parseMP4Atom(data, offset)
		if err != nil {
			return mp4Atom{}, false, err
		}

		current.start += baseOffset
		current.end += baseOffset
		if current.typ == targetType {
			return current, true, nil
		}

		if isMP4ContainerAtom(parentType, current.typ) {
			payload := data[offset+current.headerSize : offset+current.size]
			prefixLen := 0
			if current.typ == "meta" {
				if len(payload) < 4 {
					return mp4Atom{}, false, errors.New("meta atom payload too short")
				}
				prefixLen = 4
			}
			found, ok, err := findMP4AtomFrom(payload[prefixLen:], current.start+current.headerSize+prefixLen, current.typ, targetType)
			if err != nil {
				return mp4Atom{}, false, err
			}
			if ok {
				return found, true, nil
			}
		}

		offset += current.size
	}

	return mp4Atom{}, false, nil
}

func parseMP4Atom(data []byte, offset int) (mp4Atom, error) {
	if len(data[offset:]) < 8 {
		return mp4Atom{}, errors.New("truncated atom header")
	}

	size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	typ := string(data[offset+4 : offset+8])
	headerSize := 8

	switch size {
	case 0:
		size = len(data) - offset
	case 1:
		if len(data[offset:]) < 16 {
			return mp4Atom{}, errors.New("truncated 64-bit atom header")
		}
		size64 := binary.BigEndian.Uint64(data[offset+8 : offset+16])
		if size64 > uint64(len(data)-offset) {
			return mp4Atom{}, errors.New("64-bit atom exceeds file bounds")
		}
		size = int(size64)
		headerSize = 16
	}

	if size < headerSize {
		return mp4Atom{}, errors.New("invalid atom size")
	}
	if offset+size > len(data) {
		return mp4Atom{}, errors.New("atom exceeds file bounds")
	}

	return mp4Atom{
		start:      offset,
		end:        offset + size,
		size:       size,
		headerSize: headerSize,
		typ:        typ,
	}, nil
}

func isMP4ContainerAtom(parentType string, typ string) bool {
	if parentType == "ilst" {
		return true
	}

	switch typ {
	case "moov", "trak", "mdia", "minf", "stbl", "udta", "meta", "ilst":
		return true
	default:
		return false
	}
}
