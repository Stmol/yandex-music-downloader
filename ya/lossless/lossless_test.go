package lossless

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"net/url"
	"testing"
	"time"
	"ya-music/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHTTPClient struct {
	getBody      []byte
	downloadData map[string][]byte
	downloadErrs map[string]error
	downloaded   []string
}

func (f *fakeHTTPClient) GetWithContext(_ utils.RequestLogContext, _ string) ([]byte, error) {
	return f.getBody, nil
}

func (f *fakeHTTPClient) DownloadBytesWithContext(_ utils.RequestLogContext, rawURL string) ([]byte, error) {
	f.downloaded = append(f.downloaded, rawURL)
	if err := f.downloadErrs[rawURL]; err != nil {
		return nil, err
	}
	return f.downloadData[rawURL], nil
}

func TestBuildFileInfoURLSignsLosslessRequest(t *testing.T) {
	rawURL := BuildFileInfoURL("https://api.music.yandex.net", "12345", 1700000000)
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	query := parsed.Query()
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "api.music.yandex.net", parsed.Host)
	assert.Equal(t, "/get-file-info", parsed.Path)
	assert.Equal(t, "1700000000", query.Get("ts"))
	assert.Equal(t, "12345", query.Get("trackId"))
	assert.Equal(t, "lossless", query.Get("quality"))
	assert.Equal(t, "flac,flac-mp4,mp3,aac,he-aac,aac-mp4,he-aac-mp4", query.Get("codecs"))
	assert.Equal(t, "encraw", query.Get("transports"))
	assert.Equal(t, "A8rhGDs/OL1IOCIm7vYn4rj4ieadMfXvKJn2YoqVtoc", query.Get("sign"))
}

func TestGetDownloadInfoAcceptsOnlyFLACContainer(t *testing.T) {
	client := &fakeHTTPClient{
		getBody: []byte(`{"download_info":{"quality":"lossless","codec":"flac-mp4","urls":["https://cdn.test/a"],"bitrate":999}}`),
	}
	downloader := NewDownloader(client)
	downloader.now = func() time.Time { return time.Unix(1700000000, 0) }

	_, err := downloader.GetDownloadInfo(utils.RequestLogContext{}, "1")

	assert.ErrorIs(t, err, ErrNoFLACDownloadInfo)
}

func TestDownloadAudioDecryptsAESCTRPayload(t *testing.T) {
	key := "00112233445566778899aabbccddeeff"
	plain := []byte("fLaC encrypted payload")
	encrypted := encryptAESCTRForTest(t, plain, key)
	client := &fakeHTTPClient{
		downloadData: map[string][]byte{"https://cdn.test/a": encrypted},
	}
	downloader := NewDownloader(client)

	got, err := downloader.DownloadAudio(utils.RequestLogContext{}, DownloadInfo{
		URLs: []string{"https://cdn.test/a"},
		Key:  key,
	})

	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestDownloadAudioTriesNextURLAfterFailure(t *testing.T) {
	client := &fakeHTTPClient{
		downloadData: map[string][]byte{"https://cdn.test/b": []byte("fLaC ok")},
		downloadErrs: map[string]error{"https://cdn.test/a": errors.New("network")},
	}
	downloader := NewDownloader(client)

	got, err := downloader.DownloadAudio(utils.RequestLogContext{}, DownloadInfo{
		URLs: []string{"https://cdn.test/a", "https://cdn.test/b"},
	})

	require.NoError(t, err)
	assert.Equal(t, []byte("fLaC ok"), got)
	assert.Equal(t, []string{"https://cdn.test/a", "https://cdn.test/b"}, client.downloaded)
}

func TestParseDownloadInfoSupportsResultWrapper(t *testing.T) {
	info, err := ParseDownloadInfo([]byte(`{"result":{"download_info":{"quality":"lossless","codec":"flac","urls":["u"],"key":"k","bitrate":1411}}}`))

	require.NoError(t, err)
	assert.Equal(t, "lossless", info.Quality)
	assert.Equal(t, "flac", info.Codec)
	assert.Equal(t, []string{"u"}, info.URLs)
	assert.Equal(t, "k", info.Key)
	assert.Equal(t, 1411, info.Bitrate)
}

func TestParseDownloadInfoSupportsCamelCaseResultWrapper(t *testing.T) {
	info, err := ParseDownloadInfo([]byte(`{"result":{"downloadInfo":{"quality":"lossless","codec":"flac","urls":["u"],"key":"k","bitrate":1411}}}`))

	require.NoError(t, err)
	assert.Equal(t, "lossless", info.Quality)
	assert.Equal(t, "flac", info.Codec)
	assert.Equal(t, []string{"u"}, info.URLs)
	assert.Equal(t, "k", info.Key)
	assert.Equal(t, 1411, info.Bitrate)
}

func encryptAESCTRForTest(t *testing.T, data []byte, key string) []byte {
	t.Helper()

	block, err := aes.NewCipher(mustHexKeyForTest(t, key))
	require.NoError(t, err)
	stream := cipher.NewCTR(block, make([]byte, aes.BlockSize))
	encrypted := make([]byte, len(data))
	stream.XORKeyStream(encrypted, data)
	return encrypted
}

func mustHexKeyForTest(t *testing.T, key string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(key)
	require.NoError(t, err)
	return decoded
}
