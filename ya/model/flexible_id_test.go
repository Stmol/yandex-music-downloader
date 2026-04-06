package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlexibleIDUnmarshalNumber(t *testing.T) {
	var id FlexibleID

	err := json.Unmarshal([]byte(`12345`), &id)

	assert.NoError(t, err)
	assert.Equal(t, "12345", id.String())
}

func TestFlexibleIDUnmarshalString(t *testing.T) {
	var id FlexibleID

	err := json.Unmarshal([]byte(`"4ea38ec0-d54f-47b3-adea-0b1f53e9bc5d"`), &id)

	assert.NoError(t, err)
	assert.Equal(t, "4ea38ec0-d54f-47b3-adea-0b1f53e9bc5d", id.String())
}

func TestFlexibleIDUnmarshalTrackShortUUID(t *testing.T) {
	var trackShort TrackShort

	err := json.Unmarshal([]byte(`{"id":"4ea38ec0-d54f-47b3-adea-0b1f53e9bc5d","track":{"id":123}}`), &trackShort)

	assert.NoError(t, err)
	assert.Equal(t, "4ea38ec0-d54f-47b3-adea-0b1f53e9bc5d", trackShort.ID.String())
	assert.Equal(t, "123", trackShort.Track.ID.String())
}
