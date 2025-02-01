package model

type MetaData struct {
	RealID        string `json:"realId"`
	State         string `json:"state"`
	StorageDir    string `json:"storageDir"`
	Title         string `json:"title"`
	TrackSource   string `json:"trackSource"`
	UGCAtristName string `json:"ugcArtistName,omitempty"`
	UserInfo      User   `json:"userInfo"`
	Volume        int    `json:"volume"`
	Year          int    `json:"year"`
}
