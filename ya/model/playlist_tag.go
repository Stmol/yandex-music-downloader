package model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type PlaylistTag struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Name  string `json:"name"`
}

func (tag *PlaylistTag) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*tag = PlaylistTag{}
		return nil
	}

	if len(data) == 0 {
		return fmt.Errorf("empty playlist tag")
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}

		*tag = PlaylistTag{
			Value: value,
			Name:  value,
		}
		return nil
	}

	type rawPlaylistTag struct {
		ID    string `json:"id"`
		Value string `json:"value"`
		Name  string `json:"name"`
	}

	var raw rawPlaylistTag
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*tag = PlaylistTag{
		ID:    raw.ID,
		Value: raw.Value,
		Name:  raw.Name,
	}

	if tag.Value == "" {
		tag.Value = tag.Name
	}

	if tag.Name == "" {
		tag.Name = tag.Value
	}

	return nil
}
