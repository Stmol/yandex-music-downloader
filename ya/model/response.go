package model

type PlaylistResponse struct {
	Result Playlist `json:"result"`
}

type PlaylistsResponse struct {
	Result []Playlist `json:"result"`
}

type AccountStatusResponse struct {
	Result Status `json:"result"`
}

type DownloadInfoResponse struct {
	Result []DownloadInfo `json:"result"`
}

type TracksResponse struct {
	Result []Track `json:"result"`
}

type Status struct {
	Account Account `json:"account"`
}
