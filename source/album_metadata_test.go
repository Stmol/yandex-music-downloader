package source

import (
	"testing"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeTrackAlbumFillsMissingReleaseMetadata(t *testing.T) {
	parent := model.Album{
		ID: "1", CoverURI: "example.org/cover/%%", Genre: "rock", ReleaseDate: "2024-01-02",
	}
	for _, id := range []model.FlexibleID{"1", ""} {
		t.Run(string(id), func(t *testing.T) {
			albums := mergeTrackAlbum([]model.Album{{ID: id, Title: "Album",
				TrackPosition: model.TrackPosition{Volume: 2, Index: 3}}}, parent)
			require.Len(t, albums, 1)
			assert.Equal(t, parent.CoverURI, albums[0].CoverURI)
			assert.Equal(t, parent.Genre, albums[0].Genre)
			assert.Equal(t, parent.ReleaseDate, albums[0].ReleaseDate)
			assert.Equal(t, model.TrackPosition{Volume: 2, Index: 3}, albums[0].TrackPosition)
		})
	}
}

func TestMergeTrackAlbumPreservesReleaseMetadata(t *testing.T) {
	parent := model.Album{ID: "1", CoverURI: "parent-cover", Genre: "rock", ReleaseDate: "2024-01-02"}
	current := model.Album{ID: "1", Title: "Album", Year: 2023,
		CoverURI: "track-cover", Genre: "pop", ReleaseDate: "2023-02-03"}
	assert.Equal(t, current, mergeTrackAlbum([]model.Album{current}, parent)[0])

	other := model.Album{ID: "2", Title: "Other release", Year: 2023}
	assert.Equal(t, other, mergeTrackAlbum([]model.Album{other}, parent)[0])
}
