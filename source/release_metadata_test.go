package source

import (
	"testing"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A track that appears on several releases can list another release first in
// its albums. Tags are written from the first album, so the release being
// downloaded has to come first.

func TestResolveAlbumPutsDownloadedReleaseFirst(t *testing.T) {
	client := &fakeSourceClient{
		album: &model.Album{
			ID:       model.FlexibleID("10"),
			Title:    "Compilation",
			Year:     2001,
			CoverURI: "compilation-cover/%%",
			Volumes: [][]model.Track{{
				{
					ID:       model.FlexibleID("1"),
					Title:    "From Single",
					CoverURI: "track-cover/%%",
					Albums: []model.Album{{
						ID:            model.FlexibleID("99"),
						Title:         "Original Single",
						Year:          1999,
						CoverURI:      "single-cover/%%",
						TrackPosition: model.TrackPosition{Volume: 1, Index: 1},
					}},
				},
				{
					ID:    model.FlexibleID("2"),
					Title: "Second",
					Albums: []model.Album{{
						ID:            model.FlexibleID("10"),
						Title:         "Compilation",
						CoverURI:      "own-cover/%%",
						TrackPosition: model.TrackPosition{Volume: 1, Index: 2},
					}},
				},
			}},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/album/10")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 2)

	first := result.Tracks[0].Albums
	require.Len(t, first, 2)
	assert.Equal(t, "10", first[0].ID.String())
	assert.Equal(t, "Compilation", first[0].Title)
	assert.Equal(t, 2001, first[0].Year)
	assert.Equal(t, "compilation-cover/%%", first[0].CoverURI)
	assert.Equal(t, "compilation-cover/%%", result.Tracks[0].CoverURI,
		"the embedded cover follows the downloaded release, not the track's other release")
	assert.Equal(t, model.TrackPosition{Volume: 1, Index: 1}, first[0].TrackPosition)
	assert.Equal(t, "99", first[1].ID.String(), "the other release stays in the list")

	second := result.Tracks[1].Albums
	require.Len(t, second, 1)
	assert.Equal(t, "10", second[0].ID.String())
	assert.Equal(t, "own-cover/%%", second[0].CoverURI, "the track's own data for this release is kept")
	assert.Equal(t, 2001, second[0].Year, "missing data is filled from the release")
	assert.Equal(t, model.TrackPosition{Volume: 1, Index: 2}, second[0].TrackPosition)
}

func TestResolveArtistTagsSharedTrackWithEachRelease(t *testing.T) {
	shared := func() model.Track {
		return model.Track{
			ID:    model.FlexibleID("5"),
			Title: "Hit",
			Albums: []model.Album{{
				ID:            model.FlexibleID("1"),
				Title:         "Single",
				Year:          1975,
				TrackPosition: model.TrackPosition{Volume: 1, Index: 1},
			}},
		}
	}
	client := &fakeSourceClient{
		directAlbums: []model.Album{
			{ID: model.FlexibleID("1"), Title: "Single", Year: 1975},
			{ID: model.FlexibleID("2"), Title: "Album", Year: 1976},
		},
		albumsByID: map[string]*model.Album{
			"1": {ID: model.FlexibleID("1"), Title: "Single", Year: 1975, Volumes: [][]model.Track{{shared()}}},
			"2": {
				ID: model.FlexibleID("2"), Title: "Album", Year: 1976,
				Volumes: [][]model.Track{
					{{ID: model.FlexibleID("6"), Title: "Intro"}},
					{shared()},
				},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/artist/170660")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 3)

	fromSingle, fromAlbum := result.Tracks[0], result.Tracks[2]
	require.Equal(t, "5", fromSingle.ID.String())
	require.Equal(t, "5", fromAlbum.ID.String())

	assert.Equal(t, "1", fromSingle.Albums[0].ID.String())
	assert.Equal(t, "Single", fromSingle.Albums[0].Title)
	assert.Equal(t, model.TrackPosition{Volume: 1, Index: 1}, fromSingle.Albums[0].TrackPosition)

	assert.Equal(t, "2", fromAlbum.Albums[0].ID.String())
	assert.Equal(t, "Album", fromAlbum.Albums[0].Title)
	assert.Equal(t, 1976, fromAlbum.Albums[0].Year)
	assert.Equal(t, model.TrackPosition{Volume: 2, Index: 1}, fromAlbum.Albums[0].TrackPosition)
}

func TestResolveAlbumWithoutIDKeepsTrackAlbums(t *testing.T) {
	own := []model.Album{{ID: model.FlexibleID("7"), Title: "Own"}}
	client := &fakeSourceClient{
		album: &model.Album{Volumes: [][]model.Track{{{ID: model.FlexibleID("1"), Albums: own}}}},
	}

	result, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, own, result.Tracks[0].Albums)
}

func TestResolveAlbumKeepsTrackCoverWhenReleaseHasNone(t *testing.T) {
	client := &fakeSourceClient{
		album: &model.Album{
			ID:    model.FlexibleID("10"),
			Title: "Compilation",
			Volumes: [][]model.Track{{
				{ID: model.FlexibleID("1"), CoverURI: "track-cover/%%"},
			}},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/album/10")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "track-cover/%%", result.Tracks[0].CoverURI)
	require.Len(t, result.Tracks[0].Albums, 1)
	assert.Equal(t, model.TrackPosition{Volume: 1, Index: 1}, result.Tracks[0].Albums[0].TrackPosition)
}

func TestResolveAlbumCompletesPartialTrackPosition(t *testing.T) {
	client := &fakeSourceClient{
		album: &model.Album{
			ID:    model.FlexibleID("10"),
			Title: "Multi-disc",
			Volumes: [][]model.Track{
				{{ID: model.FlexibleID("1")}},
				{
					{ID: model.FlexibleID("2")},
					{ID: model.FlexibleID("3")},
					{
						ID: model.FlexibleID("4"),
						Albums: []model.Album{{
							ID:            model.FlexibleID("10"),
							TrackPosition: model.TrackPosition{Index: 5},
						}},
					},
				},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/album/10")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 4)
	assert.Equal(t, model.TrackPosition{Volume: 2, Index: 5},
		result.Tracks[3].Albums[0].TrackPosition,
		"the missing disc number is filled, the track's own number is kept")
}
