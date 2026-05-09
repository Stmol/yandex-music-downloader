package ya

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"ya-music/utils"
	"ya-music/ya/lossless"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	filename := buildTrackFilename(track, outputDir)
	require.NoError(t, os.WriteFile(filename, []byte("existing"), 0644))

	gotFilename, err := NewClient(nil).DownloadTrackWithOptions(track, outputDir, DownloadOptions{})

	assert.Equal(t, filename, gotFilename)
	assert.ErrorIs(t, err, ErrTrackAlreadyExists)
}

func TestDownloadTrackReturnsAlreadyExistsSentinelForFLACFormat(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("1"),
		Title:     "Existing",
		Available: true,
	}
	filename := buildTrackFilenameWithExtension(track, outputDir, ".flac")
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

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	assert.Equal(t, buildTrackFilenameWithExtension(track, outputDir, ".flac"), filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Contains(t, string(data), "TITLE=Song")
	assert.Contains(t, string(data), "ARTIST=Artist")
	assert.Contains(t, string(data), "ALBUM=Album")
	assert.Contains(t, string(data), "YANDEX_TRACK_ID=10")
}

func TestDownloadTrackWithOptionsWritesFLACMP4AsM4A(t *testing.T) {
	outputDir := t.TempDir()
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
		data: []byte("mp4 container"),
	}
	client.mp3Downloader = func(_ model.Track, _ string, _ DownloadOptions) (string, error) {
		t.Fatal("mp3 fallback should not be called")
		return "", nil
	}

	filename, err := client.DownloadTrackWithOptions(track, outputDir, DownloadOptions{AudioFormat: AudioFormatFLAC})

	require.NoError(t, err)
	assert.Equal(t, buildTrackFilenameWithExtension(track, outputDir, ".m4a"), filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, "mp4 container", string(data))
}

func TestDownloadTrackWithOptionsDoesNotDownloadLosslessWhenTargetExists(t *testing.T) {
	outputDir := t.TempDir()
	track := model.Track{
		ID:        model.FlexibleID("12"),
		Title:     "Existing M4A",
		Available: true,
	}
	filename := buildTrackFilenameWithExtension(track, outputDir, ".m4a")
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

	filename := buildTrackFilename(track, outputDir)

	assert.Equal(t, filepath.Join(outputDir, "Artist_ One, Artist Two - Track_Name Live.mp3"), filename)
}

func TestTrackFilenameBaseFallsBackWhenArtistIsMissing(t *testing.T) {
	track := model.Track{
		ID:    model.FlexibleID("123"),
		Title: "Track Name",
	}

	assert.Equal(t, "Track Name", trackFilenameBase(track))
}

func minimalFLACBytes() []byte {
	data := []byte("fLaC")
	data = append(data, 0x80, 0x00, 0x00, 0x22)
	data = append(data, make([]byte, 34)...)
	data = append(data, 0xff, 0xf8, 0x00, 0x00)
	return data
}
