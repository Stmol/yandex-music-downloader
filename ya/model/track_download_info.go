package model

type TrackDownloadInfo struct {
	Host   string `xml:"host"`
	Path   string `xml:"path"`
	Ts     string `xml:"ts"`
	Region int    `xml:"region"`
	S      string `xml:"s"`
}
