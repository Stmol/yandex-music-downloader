package model

import "encoding/json"

type Artist struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}
