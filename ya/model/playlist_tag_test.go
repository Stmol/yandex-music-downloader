package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaylistTagUnmarshalString(t *testing.T) {
	var tag PlaylistTag

	err := json.Unmarshal([]byte(`"lossless"`), &tag)

	require.NoError(t, err)
	assert.Equal(t, "lossless", tag.Value)
	assert.Equal(t, "lossless", tag.Name)
	assert.Empty(t, tag.ID)
}

func TestPlaylistResponseUnmarshalWithObjectTagsInLastOwnerPlaylists(t *testing.T) {
	var response PlaylistResponse

	err := json.Unmarshal([]byte(`{
		"result": {
			"playlistUuid": "523738f1-97ed-fa0c-b870-0a3dddf06e58",
			"title": "Прислушайтесь к Lossless",
			"tags": ["lossless"],
			"lastOwnerPlaylists": [
				{
					"playlistUuid": "nested-playlist",
					"title": "Nested",
					"tags": [
						{
							"id": "featured",
							"value": "lossless",
							"name": "Lossless"
						}
					]
				}
			]
		}
	}`), &response)

	require.NoError(t, err)
	require.Len(t, response.Result.Tags, 1)
	assert.Equal(t, "lossless", response.Result.Tags[0].Value)
	require.Len(t, response.Result.LastOwnerPlaylists, 1)
	require.Len(t, response.Result.LastOwnerPlaylists[0].Tags, 1)
	assert.Equal(t, "featured", response.Result.LastOwnerPlaylists[0].Tags[0].ID)
	assert.Equal(t, "lossless", response.Result.LastOwnerPlaylists[0].Tags[0].Value)
	assert.Equal(t, "Lossless", response.Result.LastOwnerPlaylists[0].Tags[0].Name)
}
