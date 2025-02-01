package model

type Cover struct {
	Custom  bool   `json:"custom"`
	Dir     string `json:"dir"`
	Type    string `json:"type"`
	URI     string `json:"uri"`
	Version string `json:"version"`
}
