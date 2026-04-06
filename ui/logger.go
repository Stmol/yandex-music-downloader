package ui

import (
	"ya-music/utils"
	"ya-music/ya"
)

func downloadLogger(client *ya.Client) *utils.DownloadLogger {
	if client == nil || client.Logger() == nil {
		return utils.NewDiscardDownloadLogger()
	}

	return client.Logger()
}
