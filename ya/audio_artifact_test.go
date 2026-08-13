package ya

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"ya-music/utils"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAudioPayload = "audio-data"

type recordingArtifactTagger struct {
	paths   []string
	err     error
	onWrite func(path string, metadata artifactMetadata)
}

func (t *recordingArtifactTagger) Write(path string, metadata artifactMetadata) error {
	t.paths = append(t.paths, path)
	if t.onWrite != nil {
		t.onWrite(path, metadata)
	}
	return t.err
}

func assertNoArtifactTempFiles(t *testing.T, dir string) {
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".artifact-", "leftover temp file: %s", entry.Name())
	}
}

func newCoverTestServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cover-bytes"))
	}))
	t.Cleanup(server.Close)
	return server
}

func testArtifactSpec(tagger artifactTagger, policy metadataFailurePolicy) artifactSpec {
	return artifactSpec{
		Format:             "mp3",
		DownloadStage:      "download_file",
		MetadataStage:      "id3_tags",
		CompletionStage:    "mp3_complete",
		Tagger:             tagger,
		FailurePolicy:      policy,
		MetadataSuccessMsg: "ID3 metadata written",
		MetadataSkipMsg:    "ID3 metadata skipped; keeping audio",
	}
}

func writeAudioBytes(payload string) artifactWriteFunc {
	return func(path string) error {
		return os.WriteFile(path, []byte(payload), 0644)
	}
}

func TestPublishAudioArtifactPublishesAfterRequiredTags(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")
	coverServer := newCoverTestServer(t)

	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "Song",
		CoverURI: coverServer.URL + "/%%",
	}

	var writerPath string
	writeAudio := func(path string) error {
		writerPath = path
		return writeAudioBytes(testAudioPayload)(path)
	}

	tagger := &recordingArtifactTagger{}
	client := NewClient(utils.NewHttpClient())

	result, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{},
		testArtifactSpec(tagger, metadataRequired),
		writeAudio,
	)

	require.NoError(t, err)
	assert.Equal(t, destination, result.Filename)
	assert.Equal(t, buildCoverFilename(destination), result.CoverFilename)

	assert.Contains(t, writerPath, ".artifact-")
	assert.NotEqual(t, destination, writerPath)
	require.Len(t, tagger.paths, 1)
	assert.Equal(t, writerPath, tagger.paths[0])

	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, testAudioPayload, string(data))

	_, err = os.Stat(buildCoverFilename(destination))
	assert.True(t, os.IsNotExist(err))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assertNoArtifactTempFiles(t, dir)
}

func TestPublishAudioArtifactRemovesTempWhenRequiredTagsFail(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")
	coverServer := newCoverTestServer(t)

	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "Song",
		CoverURI: coverServer.URL + "/%%",
	}

	tagger := &recordingArtifactTagger{err: errors.New("tag failure")}
	client := NewClient(utils.NewHttpClient())

	result, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{},
		testArtifactSpec(tagger, metadataRequired),
		writeAudioBytes(testAudioPayload),
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write mp3 tags")
	assert.Equal(t, destination, result.Filename)

	_, err = os.Stat(destination)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(buildCoverFilename(destination))
	assert.True(t, os.IsNotExist(err))

	assertNoArtifactTempFiles(t, dir)
}

func TestPublishAudioArtifactPublishesRawAudioWhenBestEffortTagsFail(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")

	track := model.Track{
		ID:    model.FlexibleID("1"),
		Title: "Song",
	}

	tagger := &recordingArtifactTagger{err: errors.New("tag failure")}
	client := NewClient(utils.NewHttpClient())

	result, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{SkipCover: true},
		testArtifactSpec(tagger, metadataBestEffort),
		writeAudioBytes(testAudioPayload),
	)

	require.NoError(t, err)
	assert.Equal(t, destination, result.Filename)

	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, testAudioPayload, string(data))

	assertNoArtifactTempFiles(t, dir)
}

func TestPublishAudioArtifactCleansCoverWhenAudioWriteFails(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")
	coverServer := newCoverTestServer(t)

	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "Song",
		CoverURI: coverServer.URL + "/%%",
	}

	writeErr := errors.New("write failed")
	writeAudio := func(path string) error {
		return writeErr
	}

	tagger := &recordingArtifactTagger{}
	client := NewClient(utils.NewHttpClient())

	result, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{},
		testArtifactSpec(tagger, metadataRequired),
		writeAudio,
	)

	assert.ErrorIs(t, err, writeErr)
	assert.Empty(t, result.Filename)
	assert.Empty(t, tagger.paths)

	_, err = os.Stat(destination)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(buildCoverFilename(destination))
	assert.True(t, os.IsNotExist(err))

	assertNoArtifactTempFiles(t, dir)
}

func TestPublishAudioArtifactTagsTempFileBeforeRename(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")

	track := model.Track{
		ID:    model.FlexibleID("1"),
		Title: "Song",
	}

	tagger := &recordingArtifactTagger{
		onWrite: func(path string, metadata artifactMetadata) {
			_, statErr := os.Stat(destination)
			assert.True(t, os.IsNotExist(statErr), "destination must not exist before rename")
			assert.Contains(t, path, ".artifact-")
		},
	}
	client := NewClient(utils.NewHttpClient())

	_, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{SkipCover: true},
		testArtifactSpec(tagger, metadataRequired),
		writeAudioBytes(testAudioPayload),
	)
	require.NoError(t, err)

	assertNoArtifactTempFiles(t, dir)
}

func TestPublishAudioArtifactRejectsInvalidDestination(t *testing.T) {
	client := NewClient(utils.NewHttpClient())
	track := model.Track{ID: model.FlexibleID("1"), Title: "Song"}
	tagger := &recordingArtifactTagger{}

	t.Run("empty destination", func(t *testing.T) {
		result, err := client.publishAudioArtifact(
			track,
			"",
			DownloadOptions{SkipCover: true},
			testArtifactSpec(tagger, metadataRequired),
			writeAudioBytes(testAudioPayload),
		)
		assert.Error(t, err)
		assert.Empty(t, result.Filename)
	})

	t.Run("missing directory", func(t *testing.T) {
		dir := t.TempDir()
		destination := filepath.Join(dir, "missing", "track.mp3")
		result, err := client.publishAudioArtifact(
			track,
			destination,
			DownloadOptions{SkipCover: true},
			testArtifactSpec(tagger, metadataRequired),
			writeAudioBytes(testAudioPayload),
		)
		assert.Error(t, err)
		assert.Empty(t, result.Filename)
		assertNoArtifactTempFiles(t, dir)
	})
}

func TestPublishAudioArtifactRejectsNilDependencies(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")
	client := NewClient(utils.NewHttpClient())
	track := model.Track{ID: model.FlexibleID("1"), Title: "Song"}

	t.Run("nil tagger", func(t *testing.T) {
		spec := testArtifactSpec(nil, metadataRequired)
		result, err := client.publishAudioArtifact(
			track,
			destination,
			DownloadOptions{SkipCover: true},
			spec,
			writeAudioBytes(testAudioPayload),
		)
		assert.Error(t, err)
		assert.Empty(t, result.Filename)
		assertNoArtifactTempFiles(t, dir)
	})

	t.Run("nil writer", func(t *testing.T) {
		tagger := &recordingArtifactTagger{}
		result, err := client.publishAudioArtifact(
			track,
			destination,
			DownloadOptions{SkipCover: true},
			testArtifactSpec(tagger, metadataRequired),
			nil,
		)
		assert.Error(t, err)
		assert.Empty(t, result.Filename)
		assert.Empty(t, tagger.paths)
		assertNoArtifactTempFiles(t, dir)
	})
}

func TestPublishAudioArtifactCleansTempOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")
	require.NoError(t, os.Mkdir(destination, 0755))

	track := model.Track{
		ID:    model.FlexibleID("1"),
		Title: "Song",
	}
	tagger := &recordingArtifactTagger{}
	client := NewClient(utils.NewHttpClient())

	result, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{SkipCover: true},
		testArtifactSpec(tagger, metadataRequired),
		writeAudioBytes(testAudioPayload),
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to publish mp3 file")
	assert.Empty(t, result.Filename)
	assertNoArtifactTempFiles(t, dir)
}

func TestPublishAudioArtifactContinuesWhenCoverDownloadFails(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.mp3")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "Song",
		CoverURI: server.URL + "/%%",
	}
	tagger := &recordingArtifactTagger{}
	client := NewClient(utils.NewHttpClient())

	result, err := client.publishAudioArtifact(
		track,
		destination,
		DownloadOptions{},
		testArtifactSpec(tagger, metadataRequired),
		writeAudioBytes(testAudioPayload),
	)

	require.NoError(t, err)
	assert.Equal(t, destination, result.Filename)
	assert.Empty(t, result.CoverFilename)

	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, testAudioPayload, string(data))

	_, err = os.Stat(buildCoverFilename(destination))
	assert.True(t, os.IsNotExist(err))

	assertNoArtifactTempFiles(t, dir)
}
