package lossless

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"ya-music/utils"
)

const (
	defaultSignKey = "p93jhgh689SBReK6ghtw62"
	defaultBaseURL = "https://api.music.yandex.net"
)

var (
	ErrNoFLACDownloadInfo = errors.New("no flac download info available")
	ErrNoDownloadURLs     = errors.New("no lossless download urls available")
)

type HTTPClient interface {
	GetWithContext(reqCtx utils.RequestLogContext, url string) ([]byte, error)
	DownloadBytesWithContext(reqCtx utils.RequestLogContext, url string) ([]byte, error)
}

type Downloader struct {
	httpClient HTTPClient
	baseURL    string
	now        func() time.Time
}

type DownloadInfo struct {
	Quality string
	Codec   string
	URLs    []string
	Key     string
	Bitrate int
}

type DownloadResult struct {
	Info DownloadInfo
	Data []byte
}

type fileInfoResponse struct {
	DownloadInfo      *downloadInfoPayload `json:"download_info,omitempty"`
	DownloadInfoCamel *downloadInfoPayload `json:"downloadInfo,omitempty"`
	Result            *struct {
		DownloadInfo      *downloadInfoPayload `json:"download_info,omitempty"`
		DownloadInfoCamel *downloadInfoPayload `json:"downloadInfo,omitempty"`
	} `json:"result,omitempty"`
}

type downloadInfoPayload struct {
	Quality string   `json:"quality"`
	Codec   string   `json:"codec"`
	URLs    []string `json:"urls"`
	Key     string   `json:"key"`
	Bitrate int      `json:"bitrate"`
}

func NewDownloader(httpClient HTTPClient) *Downloader {
	return &Downloader{
		httpClient: httpClient,
		baseURL:    defaultBaseURL,
		now:        time.Now,
	}
}

func (d *Downloader) Download(reqCtx utils.RequestLogContext, trackID string) (DownloadResult, error) {
	info, err := d.GetDownloadInfo(reqCtx, trackID)
	if err != nil {
		return DownloadResult{}, err
	}

	data, err := d.DownloadAudio(reqCtx, info)
	if err != nil {
		return DownloadResult{}, err
	}

	return DownloadResult{
		Info: info,
		Data: data,
	}, nil
}

func (d *Downloader) GetDownloadInfo(reqCtx utils.RequestLogContext, trackID string) (DownloadInfo, error) {
	if d == nil || d.httpClient == nil {
		return DownloadInfo{}, fmt.Errorf("lossless downloader is not configured")
	}

	endpoint := BuildFileInfoURL(d.baseURL, trackID, d.now().Unix())
	body, err := d.httpClient.GetWithContext(reqCtx, endpoint)
	if err != nil {
		return DownloadInfo{}, err
	}

	info, err := ParseDownloadInfo(body)
	if err != nil {
		return DownloadInfo{}, err
	}
	if !strings.EqualFold(info.Codec, "flac") {
		return DownloadInfo{}, fmt.Errorf("%w: codec %q", ErrNoFLACDownloadInfo, info.Codec)
	}
	if len(info.URLs) == 0 {
		return DownloadInfo{}, ErrNoDownloadURLs
	}

	return info, nil
}

func (d *Downloader) DownloadAudio(reqCtx utils.RequestLogContext, info DownloadInfo) ([]byte, error) {
	if d == nil || d.httpClient == nil {
		return nil, fmt.Errorf("lossless downloader is not configured")
	}
	if len(info.URLs) == 0 {
		return nil, ErrNoDownloadURLs
	}

	var errs []error
	for _, rawURL := range info.URLs {
		data, err := d.httpClient.DownloadBytesWithContext(reqCtx, rawURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", utils.SanitizeURL(rawURL), err))
			continue
		}

		if strings.TrimSpace(info.Key) != "" {
			data, err = DecryptData(data, info.Key)
			if err != nil {
				return nil, err
			}
		}

		return data, nil
	}

	return nil, errors.Join(errs...)
}

func BuildFileInfoURL(baseURL, trackID string, timestamp int64) string {
	values := url.Values{}
	values.Set("ts", fmt.Sprintf("%d", timestamp))
	values.Set("trackId", trackID)
	values.Set("quality", "lossless")
	values.Set("codecs", strings.Join(supportedCodecs(), ","))
	values.Set("transports", "encraw")
	values.Set("sign", SignRequest(timestamp, trackID))

	return strings.TrimRight(baseURL, "/") + "/get-file-info?" + values.Encode()
}

func SignRequest(timestamp int64, trackID string) string {
	signData := fmt.Sprintf(
		"%d%slossless%sencraw",
		timestamp,
		trackID,
		strings.Join(supportedCodecs(), ""),
	)
	mac := hmac.New(sha256.New, []byte(defaultSignKey))
	mac.Write([]byte(signData))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return strings.TrimRight(sign, "=")
}

func ParseDownloadInfo(body []byte) (DownloadInfo, error) {
	var response fileInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return DownloadInfo{}, fmt.Errorf("failed to parse lossless download info: %w", err)
	}

	payload := response.DownloadInfo
	if payload == nil {
		payload = response.DownloadInfoCamel
	}
	if payload == nil && response.Result != nil {
		payload = response.Result.DownloadInfo
		if payload == nil {
			payload = response.Result.DownloadInfoCamel
		}
	}
	if payload == nil {
		return DownloadInfo{}, ErrNoFLACDownloadInfo
	}

	return DownloadInfo{
		Quality: payload.Quality,
		Codec:   payload.Codec,
		URLs:    payload.URLs,
		Key:     payload.Key,
		Bitrate: payload.Bitrate,
	}, nil
}

func DecryptData(data []byte, key string) ([]byte, error) {
	decodedKey, err := hex.DecodeString(strings.TrimSpace(key))
	if err != nil {
		return nil, fmt.Errorf("failed to decode lossless decryption key: %w", err)
	}

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize lossless decryptor: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	stream := cipher.NewCTR(block, iv)
	decrypted := make([]byte, len(data))
	stream.XORKeyStream(decrypted, data)
	return decrypted, nil
}

func supportedCodecs() []string {
	return []string{
		"flac",
		"flac-mp4",
		"mp3",
		"aac",
		"he-aac",
		"aac-mp4",
		"he-aac-mp4",
	}
}
