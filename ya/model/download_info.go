package model

type DownloadInfo struct {
	BitrateInKbps   int    `json:"bitrateInKbps"`
	Codec           string `json:"codec"`
	Direct          bool   `json:"direct"`
	DownloadInfoURL string `json:"downloadInfoUrl"`
	Gain            bool   `json:"gain"`
	Preview         bool   `json:"preview"`
}
