package model

import "time"

type TrackShort struct {
	ID                   FlexibleID `json:"id"`
	OriginalIndex        int        `json:"originalIndex"`
	Timestamp            time.Time  `json:"timestamp"`
	Track                Track      `json:"track"`
	Recent               bool       `json:"recent"`
	OriginalShuffleIndex int        `json:"originalShuffleIndex"`
}
