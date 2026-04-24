package ya

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
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

var ErrTrackAlreadyExists = errors.New("track file already exists")

type YaClient interface {
	TrackInfo(id string) (*model.Track, error)
	AlbumWithTracks(id string) (*model.Album, error)
	UsersPlaylist(id string, username string) (*model.Playlist, error)
	PlaylistByUUID(id string) (*model.Playlist, error)
	DownloadTrack(track model.Track, outputDir string) (string, error)
	DownloadTrackWithOptions(track model.Track, outputDir string, options DownloadOptions) (string, error)
	SetToken(token string)
	AccountStatus() (*model.Account, error)
}

type Client struct {
	httpClient *utils.HttpClient
	logger     *utils.DownloadLogger
	userUID    int
	username   string
}

func NewClient(httpClient *utils.HttpClient) *Client {
	if httpClient == nil {
		httpClient = utils.NewHttpClient()
	}

	return &Client{
		httpClient: httpClient,
		logger:     httpClient.Logger(),
	}
}

func (c *Client) SetToken(token string) {
	c.httpClient.SetToken(token)
}

func (c *Client) Logger() *utils.DownloadLogger {
	if c == nil || c.logger == nil {
		return utils.NewDiscardDownloadLogger()
	}

	return c.logger
}

func (c *Client) Cancel() {
	if c == nil || c.httpClient == nil {
		return
	}

	c.httpClient.Cancel()
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

func (c *Client) AlbumWithTracks(id string) (*model.Album, error) {
	url := fmt.Sprintf("%s/albums/%s/with-tracks", baseURL, id)

	if res, err := c.httpClient.Get(url); err != nil {
		return nil, err
	} else {
		var data model.AlbumResponse
		err = parseResponse(res, &data)
		if err != nil {
			return nil, err
		}

		if data.Result.ID.String() == "" && len(data.Result.Volumes) == 0 {
			return nil, fmt.Errorf("album not found")
		}

		return &data.Result, nil
	}
}

func (c *Client) UsersPlaylist(id string, username string) (*model.Playlist, error) {
	return c.fetchPlaylist(fmt.Sprintf("%s/users/%s/playlists/%s", baseURL, username, id))
}

func (c *Client) PlaylistByUUID(id string) (*model.Playlist, error) {
	return c.fetchPlaylist(fmt.Sprintf("%s/playlist/%s", baseURL, id))
}

func (c *Client) fetchPlaylist(url string) (*model.Playlist, error) {
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
	return c.tracksDownloadInfo(utils.RequestLogContext{}, trackId)
}

func (c *Client) tracksDownloadInfo(reqCtx utils.RequestLogContext, trackId string) ([]model.DownloadInfo, error) {
	url := fmt.Sprintf("%s/tracks/%s/download-info", baseURL, trackId)

	res, err := c.httpClient.GetWithContext(reqCtx, url)
	if err != nil {
		return nil, err
	}

	var data model.DownloadInfoResponse
	err = parseResponse(res, &data)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "download info parse failed",
			"error", err,
		)
		return nil, err
	}

	c.logRequest(slog.LevelInfo, reqCtx, "download info parsed",
		"options_count", len(data.Result),
	)

	return data.Result, nil
}

func (c *Client) TrackDownloadLink(url string) (string, error) {
	return c.trackDownloadLink(utils.RequestLogContext{}, url)
}

func (c *Client) trackDownloadLink(reqCtx utils.RequestLogContext, url string) (string, error) {
	res, err := c.httpClient.GetWithContext(reqCtx, url)
	if err != nil {
		return "", err
	}

	var info model.TrackDownloadInfo
	err = xml.Unmarshal(res, &info)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "download link parse failed",
			"error", err,
		)
		return "", err
	}

	link := buildDirectLink(info)
	c.logRequest(slog.LevelInfo, reqCtx, "download link resolved",
		"download_url", utils.SanitizeURL(link),
	)

	return link, nil
}

func (c *Client) DownloadTrack(track model.Track, outputDir string) (string, error) {
	return c.DownloadTrackWithOptions(track, outputDir, DownloadOptions{})
}

func (c *Client) DownloadTrackWithOptions(track model.Track, outputDir string, options DownloadOptions) (string, error) {
	trackCtx := utils.NewTrackLogContext(track)
	filename := buildTrackFilename(track, outputDir)
	c.logTrack(slog.LevelInfo, trackCtx, "download started",
		"stage", "start",
		"filename", filename,
		"skip_cover", options.SkipCover,
	)

	exists, err := utils.FileExists(filename)
	if err != nil {
		c.logTrackFailure(trackCtx, "precheck", err,
			"filename", filename,
		)
		return "", fmt.Errorf("failed to inspect destination file: %w", err)
	}

	if exists {
		c.logTrack(slog.LevelInfo, trackCtx, "skipped",
			"stage", "precheck",
			"reason", "already_exists",
			"filename", filename,
		)
		return filename, fmt.Errorf("%w: %s", ErrTrackAlreadyExists, filename)
	}

	info, err := c.tracksDownloadInfo(c.requestContext(trackCtx, "download_info", "fetch_download_info"), track.ID.String())
	if err != nil {
		c.logTrackFailure(trackCtx, "download_info", err)
		return "", fmt.Errorf("failed to get download info: %w", err)
	}

	bestBitrate := pickBestBitrate(info)
	if bestBitrate.BitrateInKbps == 0 {
		c.logTrack(slog.LevelError, trackCtx, "failed",
			"stage", "select_bitrate",
			"reason", "no_download_options",
		)
		return "", fmt.Errorf("no download options available")
	}

	c.logTrack(slog.LevelInfo, trackCtx, "download option selected",
		"stage", "select_bitrate",
		"bitrate_kbps", bestBitrate.BitrateInKbps,
		"codec", bestBitrate.Codec,
		"download_info_url", utils.SanitizeURL(bestBitrate.DownloadInfoURL),
	)

	link, err := c.trackDownloadLink(c.requestContext(trackCtx, "download_link", "fetch_direct_link"), bestBitrate.DownloadInfoURL)
	if err != nil {
		c.logTrackFailure(trackCtx, "download_link", err)
		return "", fmt.Errorf("failed to get download link: %w", err)
	}

	coverCh := c.startCoverDownload(track, filename, options)
	if err := c.httpClient.DownloadFileWithContext(c.requestContext(trackCtx, "download_file", "download_mp3"), link, filename); err != nil {
		cover := c.waitCoverDownload(trackCtx, coverCh)
		if cover.filename != "" {
			c.removeCoverFile(trackCtx, cover.filename)
		}
		c.logTrackFailure(trackCtx, "download_file", err,
			"filename", filename,
		)
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	cover := c.waitCoverDownload(trackCtx, coverCh)
	if cover.filename != "" {
		defer c.removeCoverFile(trackCtx, cover.filename)
	}

	if err := writeID3Tags(filename, track, cover.filename); err != nil {
		c.logTrackFailure(trackCtx, "id3_tags", err,
			"filename", filename,
			"cover_filename", cover.filename,
		)
		return filename, fmt.Errorf("failed to write id3 tags: %w", err)
	}

	c.logTrack(slog.LevelInfo, trackCtx, "success",
		"stage", "id3_tags",
		"filename", filename,
		"cover_filename", cover.filename,
	)

	return filename, nil
}

func (c *Client) waitCoverDownload(trackCtx utils.TrackLogContext, coverCh <-chan coverDownloadResult) coverDownloadResult {
	cover := <-coverCh
	if cover.err != nil {
		c.logTrack(slog.LevelWarn, trackCtx, "cover download ignored",
			"stage", "download_cover",
			"error", cover.err,
		)
	}

	return cover
}

func buildTrackFilename(track model.Track, outputDir string) string {
	return filepath.Join(outputDir, utils.SanitizeFilename(trackFilenameBase(track))+".mp3")
}

func trackFilenameBase(track model.Track) string {
	artist := strings.TrimSpace(track.ArtistsString())
	title := strings.TrimSpace(track.FullTitle())

	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case title != "":
		return title
	case artist != "":
		return artist
	default:
		return strings.TrimSpace(track.ID.String())
	}
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

func (c *Client) requestContext(trackCtx utils.TrackLogContext, stage, operation string) utils.RequestLogContext {
	return utils.RequestLogContext{
		Track:     trackCtx,
		Stage:     stage,
		Operation: operation,
	}
}

func (c *Client) logRequest(level slog.Level, reqCtx utils.RequestLogContext, msg string, args ...any) {
	c.Logger().LogRequest(level, reqCtx, msg, args...)
}

func (c *Client) logTrack(level slog.Level, trackCtx utils.TrackLogContext, msg string, args ...any) {
	c.Logger().LogTrack(level, trackCtx, msg, args...)
}

func (c *Client) logTrackFailure(trackCtx utils.TrackLogContext, stage string, err error, args ...any) {
	attrs := []any{
		"stage", stage,
		"error", err,
	}
	attrs = append(attrs, args...)
	c.logTrack(slog.LevelError, trackCtx, "failed", attrs...)
}
