package model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type FlexibleID string

func (id FlexibleID) String() string {
	return string(id)
}

func (id *FlexibleID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}

	if len(data) == 0 {
		return fmt.Errorf("empty id")
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*id = FlexibleID(value)
		return nil
	}

	var value json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}

	*id = FlexibleID(value.String())
	return nil
}
