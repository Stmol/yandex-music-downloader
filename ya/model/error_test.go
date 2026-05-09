package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorResponseSupportsTopLevelError(t *testing.T) {
	var response ErrorResponse

	err := json.Unmarshal([]byte(`{"error":{"name":"bad-request","message":"broken"}}`), &response)

	require.NoError(t, err)
	assert.True(t, response.IsError())
	assert.Equal(t, "broken", response.Error())
}

func TestErrorResponseSupportsResultWrappedError(t *testing.T) {
	var response ErrorResponse

	err := json.Unmarshal([]byte(`{"result":{"name":"track-download-info-error","message":"not-allowed"}}`), &response)

	require.NoError(t, err)
	assert.True(t, response.IsError())
	assert.Equal(t, "not-allowed", response.Error())
}
