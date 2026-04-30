package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackDecodesTypedAlbums(t *testing.T) {
	var track Track

	err := json.Unmarshal([]byte(`{
		"id": 123,
		"title": "Track",
		"albums": [{
			"id": 456,
			"title": "Album",
			"coverUri": "avatars.yandex.net/get-music-content/cover/%%",
			"genre": "rock",
			"year": 2024,
			"releaseDate": "2024-01-02",
			"trackPosition": {"volume": 1, "index": 7}
		}]
	}`), &track)

	require.NoError(t, err)
	require.Len(t, track.Albums, 1)
	assert.Equal(t, "456", track.Albums[0].ID.String())
	assert.Equal(t, "Album", track.Albums[0].Title)
	assert.Equal(t, "avatars.yandex.net/get-music-content/cover/%%", track.Albums[0].CoverURI)
	assert.Equal(t, "rock", track.Albums[0].Genre)
	assert.Equal(t, 2024, track.Albums[0].Year)
	assert.Equal(t, "2024-01-02", track.Albums[0].ReleaseDate)
	assert.Equal(t, 1, track.Albums[0].TrackPosition.Volume)
	assert.Equal(t, 7, track.Albums[0].TrackPosition.Index)
}

func TestAlbumDecodesVolumes(t *testing.T) {
	var response AlbumResponse

	err := json.Unmarshal([]byte(`{
		"result": {
			"id": 5942930,
			"title": "The Greatest Video Game Music 2",
			"trackCount": 2,
			"available": true,
			"volumes": [[
				{
					"id": "44338323",
					"title": "Assassin's Creed - Revelations: Main Theme",
					"available": true,
					"albums": [{
						"id": 5942930,
						"title": "The Greatest Video Game Music 2",
						"trackPosition": {"volume": 1, "index": 1}
					}]
				},
				{
					"id": "44338329",
					"title": "Elder Scrolls - Skyrim: Far Horizons",
					"available": true,
					"albums": [{
						"id": 5942930,
						"title": "The Greatest Video Game Music 2",
						"trackPosition": {"volume": 1, "index": 2}
					}]
				}
			]]
		}
	}`), &response)

	require.NoError(t, err)
	assert.Equal(t, "5942930", response.Result.ID.String())
	assert.Equal(t, "The Greatest Video Game Music 2", response.Result.Title)
	assert.Equal(t, 2, response.Result.TrackCount)
	require.Len(t, response.Result.Volumes, 1)
	require.Len(t, response.Result.Volumes[0], 2)
	assert.Equal(t, "44338323", response.Result.Volumes[0][0].ID.String())
	assert.Equal(t, 1, response.Result.Volumes[0][0].Albums[0].TrackPosition.Index)
}
