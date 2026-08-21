package ya

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"ya-music/utils"
	"ya-music/ya/lossless"
	"ya-music/ya/model"

	"github.com/bogem/id3v2/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const malformedTruncatedTrackFixtureSHA256 = "66c48eafd91c768fa511b2a551ad0e8a32307f1719da010c746d1436cd15f975"

type fakeLosslessDownloader struct {
	info          lossless.DownloadInfo
	data          []byte
	infoErr       error
	downloadErr   error
	infoCalls     int
	downloadCalls int
	userID        int
}

func (f *fakeLosslessDownloader) GetDownloadInfo(_ utils.RequestLogContext, _ string, userUID int) (lossless.DownloadInfo, error) {
	f.infoCalls++
	f.userID = userUID
	if f.infoErr != nil {
		return lossless.DownloadInfo{}, f.infoErr
	}
	return f.info, nil
}

func (f *fakeLosslessDownloader) DownloadAudio(_ utils.RequestLogContext, info lossless.DownloadInfo) ([]byte, error) {
	f.downloadCalls++
	f.info = info
	if f.downloadErr != nil {
		return nil, f.downloadErr
	}
	return f.data, nil
}

func TestDownloadTrackReturnsAlreadyExistsSentinel(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("1"),
		Title:     "Existing",
		Available: true,
	}
	filename := buildTrackFilename(track, outputDir, "")
	require.NoError(t, os.WriteFile(filename, []byte("existing"), 0644))

	var httpCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls++
		t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	httpClient := utils.NewHttpClient()
	httpClient.SetTransport(&hostRewriteTransport{targetHost: server.Listener.Addr().String()})

	gotFilename, err := NewClient(httpClient).DownloadTrackWithOptions(track, outputDir, DownloadOptions{})

	assert.Equal(t, filename, gotFilename)
	assert.ErrorIs(t, err, ErrTrackAlreadyExists)
	assert.Zero(t, httpCalls)
}

func TestDownloadTrackReturnsAlreadyExistsSentinelForFLACFormat(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("1"),
		Title:     "Existing",
		Available: true,
	}
	filename := buildTrackFilenameWithExtension(track, outputDir, ".flac", "")
	require.NoError(t, os.WriteFile(filename, []byte("existing"), 0644))

	client := NewClient(nil)
	client.userUID = 1
	fakeLossless := &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac", Bitrate: 1411},
	}
	client.losslessDownloader = fakeLossless
	gotFilename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	assert.Equal(t, filename, gotFilename)
	assert.ErrorIs(t, err, ErrTrackAlreadyExists)
	assert.Equal(t, 1, fakeLossless.infoCalls)
	assert.Zero(t, fakeLossless.downloadCalls)
}

func TestDownloadTrackWithOptionsUsesMP3ByDefault(t *testing.T) {
	client := NewClient(nil)
	track := model.Track{ID: model.FlexibleID("1"), Title: "Song"}
	client.mp3Downloader = func(gotTrack model.Track, outputDir string, options DownloadOptions) (string, error) {
		assert.Equal(t, track, gotTrack)
		assert.Equal(t, "/tmp/out", outputDir)
		assert.Equal(t, AudioFormatMP3, options.FormatOrDefault())
		return "song.mp3", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, "/tmp/out", DownloadOptions{})

	require.NoError(t, err)
	assert.Equal(t, "song.mp3", filename)
}

func TestDownloadTrackWithOptionsFallsBackToMP3WhenFLACFails(t *testing.T) {
	client := NewClient(nil)
	client.losslessDownloader = &fakeLosslessDownloader{infoErr: errors.New("no flac")}
	client.userUID = 1
	client.mp3Downloader = func(_ model.Track, _ string, options DownloadOptions) (string, error) {
		assert.Equal(t, AudioFormatFLAC, options.FormatOrDefault())
		return "fallback.mp3", nil
	}

	filename, err := client.DownloadTrackWithOptions(
		model.Track{ID: model.FlexibleID("1"), Title: "Song"},
		t.TempDir(),
		DownloadOptions{AudioFormat: AudioFormatFLAC},
	)

	require.NoError(t, err)
	assert.Equal(t, "fallback.mp3", filename)
}

func TestDownloadTrackWithOptionsWritesFLACWhenLosslessSucceeds(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("10"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
		Albums: []model.Album{{
			ID:    model.FlexibleID("20"),
			Title: "Album",
			Year:  2026,
		}},
	}
	client := NewClient(nil)
	client.userUID = 99
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac", Bitrate: 1411},
		data: minimalFLACBytes(),
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{
		AudioFormat:    AudioFormatFLAC,
		FilenameSuffix: "[10]",
	})

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(outputDir, "Artist - Song [10].flac"), filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Contains(t, string(data), "TITLE=Song")
	assert.Contains(t, string(data), "ARTIST=Artist")
	assert.Contains(t, string(data), "ALBUM=Album")
	assert.Contains(t, string(data), "YANDEX_TRACK_ID=10")
}

func TestDownloadTrackWithOptionsWritesFLACMP4AsM4A(t *testing.T) {
	outputDir := t.TempDir()
	fixture := copyFixture(t, "taggable-stco.m4a")
	fixtureData, err := os.ReadFile(fixture)
	require.NoError(t, err)
	fixtureFTYP := readMP4FileTypeBox(t, fixture)

	track := model.Track{
		ID:        model.FlexibleID("11"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
	}
	client := NewClient(nil)
	client.userUID = 77
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac-mp4", Bitrate: 0},
		data: fixtureData,
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	assert.Equal(t, buildTrackFilenameWithExtension(track, outputDir, ".m4a", ""), filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, fixtureFTYP, readMP4FileTypeBox(t, filename))
	assert.NotEmpty(t, data)
}

func TestDownloadTrackWithOptionsWritesFLACMP4AsM4AWithMetadata(t *testing.T) {
	outputDir := t.TempDir()
	fixtureData, err := os.ReadFile(copyFixture(t, "taggable-stco.m4a"))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(onePixelPNG())
	}))
	t.Cleanup(server.Close)

	track := model.Track{
		ID:        model.FlexibleID("11"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
		CoverURI:  server.URL + "/cover/%%",
		Albums: []model.Album{{
			ID:    model.FlexibleID("22"),
			Title: "Album",
			Year:  2024,
		}},
	}

	client := NewClient(utils.NewHttpClient())
	client.userUID = 77
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac-mp4", Bitrate: 0},
		data: fixtureData,
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	file := openTaggedM4A(t, filename)
	assert.Equal(t, "Song", file.Title())
	images := file.Images()
	require.Len(t, images, 1)
	assert.Equal(t, "image/png", images[0].MIME)
	assert.Equal(t, onePixelPNG(), images[0].Data)
}

type recordingM4ATagger struct {
	err      error
	paths    []string
	calls    int
	lastTags m4aTagInput
}

func (r *recordingM4ATagger) Write(path string, tags m4aTagInput) error {
	r.calls++
	r.paths = append(r.paths, path)
	r.lastTags = tags
	return r.err
}

func TestDownloadTrackWithOptionsKeepsM4AWhenTaggingFails(t *testing.T) {
	outputDir := t.TempDir()
	fixtureData, err := os.ReadFile(copyFixture(t, "taggable-stco.m4a"))
	require.NoError(t, err)

	var logs bytes.Buffer
	httpClient := utils.NewHttpClientWithLogger(utils.NewDownloadLoggerForWriter(&logs))
	client := NewClient(httpClient)
	client.userUID = 77
	tagger := &recordingM4ATagger{err: errors.New("tag boom")}
	client.m4aTagger = tagger
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac-mp4", Bitrate: 0},
		data: fixtureData,
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	track := model.Track{
		ID:        model.FlexibleID("11"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
	}
	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	assert.Equal(t, buildTrackFilenameWithExtension(track, outputDir, ".m4a", ""), filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, fixtureData, data)
	assert.Equal(t, 1, tagger.calls)
	assert.Contains(t, logs.String(), "M4A metadata skipped; keeping audio")
	assert.Contains(t, logs.String(), "tag boom")
	assertNoArtifactTempFiles(t, outputDir)
}

func TestDownloadTrackWithOptionsKeepsM4AWhenCoverFails(t *testing.T) {
	outputDir := t.TempDir()
	fixtureData, err := os.ReadFile(copyFixture(t, "taggable-stco.m4a"))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	track := model.Track{
		ID:        model.FlexibleID("11"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
		CoverURI:  server.URL + "/missing/%%",
		Albums: []model.Album{{
			ID:    model.FlexibleID("22"),
			Title: "Album",
		}},
	}
	client := NewClient(utils.NewHttpClient())
	client.userUID = 77
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac-mp4", Bitrate: 0},
		data: fixtureData,
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	file := openTaggedM4A(t, filename)
	assert.Equal(t, "Song", file.Title())
	assert.Equal(t, "Artist", file.Artist())
	assert.Equal(t, "Album", file.Album())
	assert.Empty(t, file.Images())
}

func TestDownloadTrackWithOptionsDoesNotTouchExistingFinalM4AOnTagFailure(t *testing.T) {
	outputDir := t.TempDir()
	fixtureData, err := os.ReadFile(copyFixture(t, "taggable-stco.m4a"))
	require.NoError(t, err)

	unrelated := filepath.Join(outputDir, "other-final.m4a")
	require.NoError(t, os.WriteFile(unrelated, []byte("do-not-touch"), 0644))
	unrelatedBefore := sha256File(t, unrelated)

	tagger := &recordingM4ATagger{err: errors.New("tag boom")}
	client := NewClient(nil)
	client.userUID = 77
	client.m4aTagger = tagger
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac-mp4", Bitrate: 0},
		data: fixtureData,
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	track := model.Track{
		ID:        model.FlexibleID("11"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
	}
	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	require.Len(t, tagger.paths, 1)
	assert.NotEqual(t, filename, tagger.paths[0])
	assert.Contains(t, tagger.paths[0], ".artifact-")
	assert.Equal(t, unrelatedBefore, sha256File(t, unrelated))
	assert.FileExists(t, filename)
}

func TestMalformedM4AFixtureCopyPreservesChecksum(t *testing.T) {
	sourcePath := filepath.Join("testdata", "m4a", "malformed-truncated-track.m4a")
	sourceHashBefore := sha256File(t, sourcePath)
	assert.Equal(t, malformedTruncatedTrackFixtureSHA256, sourceHashBefore)

	copyPath := copyFixture(t, "malformed-truncated-track.m4a")
	copyHashBefore := sha256File(t, copyPath)
	assert.Equal(t, sourceHashBefore, copyHashBefore)

	data, err := os.ReadFile(copyPath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	assert.Equal(t, copyHashBefore, sha256File(t, copyPath))
	assert.Equal(t, sourceHashBefore, sha256File(t, sourcePath))
}

func TestDownloadTrackWithOptionsRejectsInvalidFLACMagic(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("13"),
		Title:     "Song",
		Available: true,
	}
	client := NewClient(nil)
	client.userUID = 1
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac", Bitrate: 1411},
		data: []byte("not-flac-data"),
	}

	destination := buildTrackFilenameWithExtension(track, outputDir, ".flac", "")
	filename, err := client.downloadTrackLossless(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lossless response is not a flac stream")
	assert.Empty(t, filename)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
	assertNoArtifactTempFiles(t, outputDir)
}

func TestDownloadTrackWithOptionsRejectsUnknownLosslessCodec(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("14"),
		Title:     "Song",
		Available: true,
	}
	client := NewClient(nil)
	client.userUID = 1
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "alac", Bitrate: 1411},
		data: []byte("audio"),
	}

	destination := buildTrackFilenameWithExtension(track, outputDir, ".flac", "")
	filename, err := client.downloadTrackLossless(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	assert.Error(t, err)
	assert.ErrorIs(t, err, lossless.ErrNoFLACDownloadInfo)
	assert.Empty(t, filename)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
	assertNoArtifactTempFiles(t, outputDir)
}

func TestDownloadTrackWithOptionsReturnsFLACTagErrorWithoutDestination(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("15"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
	}
	client := NewClient(nil)
	client.userUID = 1
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac", Bitrate: 1411},
		data: []byte("fLaCinvalid"),
	}

	destination := buildTrackFilenameWithExtension(track, outputDir, ".flac", "")
	filename, err := client.downloadTrackLossless(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write flac tags")
	assert.Equal(t, destination, filename)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
	assertNoArtifactTempFiles(t, outputDir)
}

func TestDownloadTrackWithOptionsFallsBackToMP3WhenFLACTaggingFails(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("16"),
		Title:     "Song",
		Available: true,
	}
	client := NewClient(nil)
	client.userUID = 1
	client.losslessDownloader = &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac", Bitrate: 1411},
		data: []byte("fLaCinvalid"),
	}
	client.mp3Downloader = func(_ model.Track, _ string, options DownloadOptions) (string, error) {
		assert.Equal(t, AudioFormatFLAC, options.FormatOrDefault())
		return "fallback.mp3", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	assert.Equal(t, "fallback.mp3", filename)
}

func TestDownloadTrackWithOptionsDoesNotDownloadLosslessWhenTargetExists(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("12"),
		Title:     "Existing M4A",
		Available: true,
	}
	filename := buildTrackFilenameWithExtension(track, outputDir, ".m4a", "")
	require.NoError(t, os.WriteFile(filename, []byte("existing"), 0644))

	client := NewClient(nil)
	client.userUID = 15
	fakeLossless := &fakeLosslessDownloader{
		info: lossless.DownloadInfo{Quality: "lossless", Codec: "flac-mp4", Bitrate: 0},
		data: []byte("should not be downloaded"),
	}
	client.losslessDownloader = fakeLossless

	gotFilename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	assert.Equal(t, filename, gotFilename)
	assert.ErrorIs(t, err, ErrTrackAlreadyExists)
	assert.Equal(t, 1, fakeLossless.infoCalls)
	assert.Zero(t, fakeLossless.downloadCalls)
}

type hostRewriteTransport struct {
	targetHost string
	base       http.RoundTripper
}

func (t *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	cloned.URL.Host = t.targetHost
	cloned.Host = t.targetHost

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

type mp3TestServer struct {
	server   *httptest.Server
	requests []string
}

type mp3TestServerConfig struct {
	trackID        string
	mp3Payload     []byte
	downloadStatus int
}

func newMP3TestServer(t *testing.T, cfg mp3TestServerConfig) *mp3TestServer {
	t.Helper()

	if cfg.trackID == "" {
		cfg.trackID = "42"
	}
	if cfg.mp3Payload == nil {
		cfg.mp3Payload = []byte("audio payload")
	}
	if cfg.downloadStatus == 0 {
		cfg.downloadStatus = http.StatusOK
	}

	ts := &mp3TestServer{}
	infoURL := "https://api.music.yandex.net/download-info-xml"
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.requests = append(ts.requests, r.Method+" "+r.URL.Path)

		switch {
		case r.URL.Path == "/tracks/"+cfg.trackID+"/download-info":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(model.DownloadInfoResponse{
				Result: []model.DownloadInfo{{
					BitrateInKbps:   320,
					Codec:           "mp3",
					DownloadInfoURL: infoURL,
				}},
			})
		case r.URL.Path == "/download-info-xml":
			w.Header().Set("Content-Type", "application/xml")
			_ = xml.NewEncoder(w).Encode(model.TrackDownloadInfo{
				Host: "strm.test",
				Path: "/mp3/track",
				Ts:   "123",
				S:    "secret",
			})
		case strings.HasPrefix(r.URL.Path, "/get-mp3/"):
			if cfg.downloadStatus != http.StatusOK {
				w.WriteHeader(cfg.downloadStatus)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cfg.mp3Payload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.server.Close)
	return ts
}

func newMP3TestClient(t *testing.T, ts *mp3TestServer) *Client {
	t.Helper()

	httpClient := utils.NewHttpClient()
	httpClient.SetTransport(&hostRewriteTransport{targetHost: ts.server.Listener.Addr().String()})
	return NewClient(httpClient)
}

func TestDownloadTrackMP3PublishesThroughArtifactPipeline(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("42"),
		Title:     "Song",
		Available: true,
		Artists:   []model.Artist{{Name: "Artist"}},
	}

	ts := newMP3TestServer(t, mp3TestServerConfig{trackID: "42"})
	client := newMP3TestClient(t, ts)

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{FilenameSuffix: "[42]"})

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(outputDir, "Artist - Song [42].mp3"), filename)
	assert.Equal(t, filename+".cover", buildCoverFilename(filename))

	tag, err := id3v2.Open(filename, id3v2.Options{Parse: true})
	require.NoError(t, err)
	defer tag.Close()
	assert.Equal(t, "Song", tag.Title())
	assert.Equal(t, "Artist", tag.Artist())

	assert.Contains(t, ts.requests, "GET /tracks/42/download-info")
	assert.Contains(t, ts.requests, "GET /download-info-xml")
	assert.True(t, strings.Contains(ts.requests[len(ts.requests)-1], "GET /get-mp3/"))
	assertNoArtifactTempFiles(t, outputDir)
}

func TestDownloadTrackMP3ReturnsDownloadErrorWithoutPartialDestination(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("42"),
		Title:     "Song",
		Available: true,
	}

	ts := newMP3TestServer(t, mp3TestServerConfig{
		trackID:        "42",
		downloadStatus: http.StatusServiceUnavailable,
	})
	client := newMP3TestClient(t, ts)
	destination := buildTrackFilename(track, outputDir, "")

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download file")
	assert.Empty(t, filename)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
	assertNoArtifactTempFiles(t, outputDir)
}

func TestDownloadTrackMP3ReturnsTaggerErrorWithoutPartialDestination(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("42"),
		Title:     "Song",
		Available: true,
	}

	ts := newMP3TestServer(t, mp3TestServerConfig{trackID: "42"})
	client := newMP3TestClient(t, ts)
	destination := buildTrackFilename(track, outputDir, "")

	tagger := &recordingArtifactTagger{err: errors.New("tag failure")}
	client.mp3Tagger = tagger

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write mp3 tags")
	assert.Equal(t, destination, filename)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
	assertNoArtifactTempFiles(t, outputDir)
}

func TestBuildTrackFilenameUsesCanonicalArtistTrackPattern(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:      model.FlexibleID("123"),
		Title:   "Track/Name",
		Version: "Live",
		Artists: []model.Artist{
			{Name: "Artist: One"},
			{Name: "Artist Two"},
		},
	}

	filename := buildTrackFilename(track, outputDir, "")

	assert.Equal(t, filepath.Join(outputDir, "Artist_ One, Artist Two - Track_Name Live.mp3"), filename)
}

func TestTrackFilenameKeyAndSuffixUseSanitizedBase(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:      model.FlexibleID("123"),
		Title:   "Track/Name",
		Version: "Live",
		Artists: []model.Artist{{Name: "Artist: One"}},
	}

	assert.Equal(t, "Artist_ One - Track_Name Live", TrackFilenameKey(track))
	assert.Equal(t, "456", TrackFilenameKey(model.Track{ID: model.FlexibleID("456")}))
	filename := buildTrackFilename(track, outputDir, "[123]")
	assert.Equal(t, filepath.Join(outputDir, "Artist_ One - Track_Name Live [123].mp3"), filename)
	assert.Equal(t, filepath.Join(outputDir, "Artist_ One - Track_Name Live [123].mp3.cover"), buildCoverFilename(filename))
}

func TestTrackFilenameBaseFallsBackWhenArtistIsMissing(t *testing.T) {
	track := model.Track{
		ID:    model.FlexibleID("123"),
		Title: "Track Name",
	}

	assert.Equal(t, "Track Name", trackFilenameBase(track))
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()

	sourcePath := filepath.Join("testdata", "m4a", name)
	data, err := os.ReadFile(sourcePath)
	require.NoError(t, err)

	destinationPath := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(destinationPath, data, 0644))
	return destinationPath
}

func readMP4FileTypeBox(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 8)

	size := int(binary.BigEndian.Uint32(data[:4]))
	require.GreaterOrEqual(t, size, 8)
	require.LessOrEqual(t, size, len(data))
	require.Equal(t, "ftyp", string(data[4:8]))

	return append([]byte(nil), data[:size]...)
}

func sha256File(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func minimalFLACBytes() []byte {
	data := []byte("fLaC")
	data = append(data, 0x80, 0x00, 0x00, 0x22)
	data = append(data, make([]byte, 34)...)
	data = append(data, 0xff, 0xf8, 0x00, 0x00)
	return data
}
