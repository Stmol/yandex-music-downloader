package ya

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"ya-music/utils"
	"ya-music/ya/model"
)

// Constants for API endpoints and configuration
const (
	baseURL  = "https://api.music.yandex.net"
	signSalt = "XGRlBW9FXlekgbPrRHuSiA"
)

// Audio quality constants
const (
	Bitrate64  = 64
	Bitrate128 = 128
	Bitrate192 = 192
	Bitrate320 = 320
)

// Audio codec constants
const (
	CodecMP3 = "mp3"
	CodecAAC = "aac"
)

type YaClient interface {
	TrackInfo(id string) (*model.Track, error)
	UsersPlaylist(id string, username string) (*model.Playlist, error)
	DownloadTrack(track model.Track, outputDir string) (string, error)
	SetToken(token string)
	AccountStatus() (*model.Account, error)
}

type Client struct {
	httpClient *utils.HttpClient
	userUID    int
	username   string
}

func NewClient(httpClient *utils.HttpClient) *Client {
	return &Client{
		httpClient: httpClient,
	}
}

func (c *Client) SetToken(token string) {
	c.httpClient.SetToken(token)
}

func (c *Client) AccountStatus() (*model.Account, error) {
	endpoint := fmt.Sprintf("%s/account/status", baseURL)

	res, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get account status: %w", err)
	}

	var data model.AccountStatusResponse
	if err := parseResponse(res, &data); err != nil {
		return nil, err
	}

	if uid := data.Result.Account.Uid; uid != 0 {
		c.userUID = uid
		c.username = data.Result.Account.Login
	}

	return &data.Result.Account, nil
}

func (c *Client) TrackInfo(id string) (*model.Track, error) {
	url := fmt.Sprintf("%s/tracks/%s", baseURL, id)

	if res, err := c.httpClient.Get(url); err != nil {
		return nil, err
	} else {
		var data model.TracksResponse
		err = parseResponse(res, &data)
		if err != nil {
			return nil, err
		}

		if len(data.Result) == 0 {
			return nil, fmt.Errorf("track not found")
		}

		return &data.Result[0], nil
	}
}

func (c *Client) UsersPlaylist(id string, username string) (*model.Playlist, error) {
	url := fmt.Sprintf("%s/users/%s/playlists/%s", baseURL, username, id)
	if res, err := c.httpClient.Get(url); err != nil {
		return nil, err
	} else {
		var data model.PlaylistResponse

		err = parseResponse(res, &data)
		if err != nil {
			return nil, err
		}

		return &data.Result, nil
	}
}

func (c *Client) UsersPlaylists(ids string) ([]model.Playlist, error) {
	if c.userUID == 0 {
		return nil, fmt.Errorf("This API required user authorization")
	}

	if len(ids) < 1 {
		return nil, fmt.Errorf("You must specify playlist ids more then one")
	}

	params := url.Values{}
	params.Add("kinds", ids)

	url := fmt.Sprintf("%s/users/%d/playlists?%s", baseURL, c.userUID, params.Encode())

	res, err := c.httpClient.Post(url, nil)
	if err != nil {
		return nil, err
	}

	var data model.PlaylistsResponse
	err = parseResponse(res, &data)
	if err != nil {
		return nil, err
	}

	return data.Result, nil
}

func (c *Client) TracksDownloadInfo(trackId string) ([]model.DownloadInfo, error) {
	url := fmt.Sprintf("%s/tracks/%s/download-info", baseURL, trackId)

	res, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}

	var data model.DownloadInfoResponse
	err = parseResponse(res, &data)
	if err != nil {
		return nil, err
	}

	return data.Result, nil
}

func (c *Client) TrackDownloadLink(url string) (string, error) {
	res, err := c.httpClient.Get(url)
	if err != nil {
		return "", err
	}

	var info model.TrackDownloadInfo
	err = xml.Unmarshal(res, &info)
	if err != nil {
		return "", err
	}

	link := buildDirectLink(info)

	return link, nil
}

func (c *Client) DownloadTrack(track model.Track, outputDir string) (string, error) {
	filename := buildTrackFilename(track, outputDir)

	if exists, _ := utils.FileExists(filename); exists {
		return filename, fmt.Errorf("file already exists: %s", filename)
	}

	info, err := c.TracksDownloadInfo(track.ID.String())
	if err != nil {
		return "", fmt.Errorf("failed to get download info: %w", err)
	}

	bestBitrate := pickBestBitrate(info)
	if bestBitrate.BitrateInKbps == 0 {
		return "", fmt.Errorf("no download options available")
	}

	link, err := c.TrackDownloadLink(bestBitrate.DownloadInfoURL)
	if err != nil {
		return "", fmt.Errorf("failed to get download link: %w", err)
	}

	if err := c.httpClient.DownloadFile(link, filename); err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	return filename, nil
}

func buildTrackFilename(track model.Track, outputDir string) string {
	title := fmt.Sprintf("%s - %s", track.FullTitle(), track.ArtistsString())
	return filepath.Join(outputDir, utils.SanitizeFilename(title)+".mp3")
}

func parseResponse(responseBody []byte, response interface{}) error {
	err := json.Unmarshal(responseBody, &response)
	if err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	return nil
}

func buildDirectLink(info model.TrackDownloadInfo) string {
	pathWithoutFirstChar := info.Path[1:]
	signData := signSalt + pathWithoutFirstChar + info.S
	hash := md5.Sum([]byte(signData))
	sign := hex.EncodeToString(hash[:])

	return fmt.Sprintf("https://%s/get-mp3/%s/%s%s",
		info.Host, sign, info.Ts, info.Path)
}

func pickBestBitrate(info []model.DownloadInfo) model.DownloadInfo {
	if len(info) == 0 {
		return model.DownloadInfo{}
	}

	sort.Slice(info, func(i, j int) bool {
		return info[i].BitrateInKbps > info[j].BitrateInKbps
	})

	return info[0]
}
