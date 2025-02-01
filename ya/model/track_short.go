package model

import (
	"encoding/json"
	"time"
)

type TrackShort struct {
	ID                   json.Number `json:"id"`
	OriginalIndex        int         `json:"originalIndex"`
	Timestamp            time.Time   `json:"timestamp"`
	Track                Track       `json:"track"`
	Recent               bool        `json:"recent"`
	OriginalShuffleIndex int         `json:"originalShuffleIndex"`
}
