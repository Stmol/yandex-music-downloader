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

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis"
	flac "github.com/go-flac/go-flac"
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

func TestPublishAudioArtifactFailsWhenCreateTempFails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	destination := filepath.Join(dir, "track.mp3")
	track := model.Track{ID: model.FlexibleID("1"), Title: "Song"}
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
	assert.Contains(t, err.Error(), "error creating artifact temp file")
	assert.Empty(t, result.Filename)
	assert.Empty(t, tagger.paths)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))

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

func testArtifactTrack() model.Track {
	return model.Track{
		ID:      model.FlexibleID("123"),
		Title:   "Song",
		Version: "Live",
		Artists: []model.Artist{{Name: "Artist A"}, {Name: "Artist B"}},
		Albums: []model.Album{{
			ID:    model.FlexibleID("456"),
			Title: "Album",
			Genre: "indie",
			Year:  2025,
			TrackPosition: model.TrackPosition{
				Index: 3,
			},
		}},
	}
}

func TestAudioArtifactMp3TaggerWritesID3Tags(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "track.mp3")
	coverPath := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(mp3Path, []byte("audio payload"), 0644))
	require.NoError(t, os.WriteFile(coverPath, tinyPNG, 0644))

	track := testArtifactTrack()
	tagger := mp3ArtifactTagger{}
	require.NoError(t, tagger.Write(mp3Path, artifactMetadata{Track: track, CoverPath: coverPath}))

	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	require.NoError(t, err)
	defer tag.Close()

	assert.Equal(t, "Song Live", tag.Title())
	assert.Equal(t, "Artist A, Artist B", tag.Artist())
	assert.Equal(t, "Album", tag.Album())
	assert.Len(t, tag.GetFrames(tag.CommonID("Attached picture")), 1)
}

func TestAudioArtifactMp3TaggerReturnsWriteID3TagsError(t *testing.T) {
	tagger := mp3ArtifactTagger{}
	missing := filepath.Join(t.TempDir(), "missing.mp3")

	err := tagger.Write(missing, artifactMetadata{Track: model.Track{Title: "Song"}})
	want := writeID3Tags(missing, model.Track{Title: "Song"}, "")
	require.Error(t, err)
	assert.Equal(t, want.Error(), err.Error())
}

func TestAudioArtifactFlacTaggerWritesFLACTags(t *testing.T) {
	dir := t.TempDir()
	flacPath := filepath.Join(dir, "track.flac")
	coverPath := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(flacPath, minimalFLACBytes(), 0644))
	require.NoError(t, os.WriteFile(coverPath, onePixelPNG(), 0644))

	track := testArtifactTrack()
	tagger := flacArtifactTagger{}
	require.NoError(t, tagger.Write(flacPath, artifactMetadata{Track: track, CoverPath: coverPath}))

	file, err := flac.ParseFile(flacPath)
	require.NoError(t, err)

	var comments *flacvorbis.MetaDataBlockVorbisComment
	var hasPicture bool
	for _, block := range file.Meta {
		switch block.Type {
		case flac.VorbisComment:
			comments, err = flacvorbis.ParseFromMetaDataBlock(*block)
			require.NoError(t, err)
		case flac.Picture:
			hasPicture = true
		}
	}

	require.NotNil(t, comments)
	assertFLACComment(t, comments, "TITLE", "Song Live")
	assertFLACComment(t, comments, "ALBUM", "Album")
	assert.True(t, hasPicture)
}

func TestAudioArtifactFlacTaggerReturnsWriteFLACTagsError(t *testing.T) {
	tagger := flacArtifactTagger{}
	missing := filepath.Join(t.TempDir(), "missing.flac")

	err := tagger.Write(missing, artifactMetadata{Track: model.Track{Title: "Song"}})
	want := writeFLACTags(missing, model.Track{Title: "Song"}, "")
	require.Error(t, err)
	assert.Equal(t, want.Error(), err.Error())
}

func TestAudioArtifactM4aTaggerDelegatesToClientM4ATagger(t *testing.T) {
	client := NewClient(utils.NewHttpClient())
	recorder := &recordingM4ATagger{}
	client.m4aTagger = recorder

	track := testArtifactTrack()
	album := *firstAlbum(track)
	coverPath := filepath.Join(t.TempDir(), "cover.png")
	require.NoError(t, os.WriteFile(coverPath, onePixelPNG(), 0644))

	tagger := m4aArtifactTagger{client: client}
	path := "/tmp/track.m4a"
	require.NoError(t, tagger.Write(path, artifactMetadata{Track: track, CoverPath: coverPath}))

	require.Equal(t, 1, recorder.calls)
	assert.Equal(t, []string{path}, recorder.paths)
	assert.Equal(t, m4aTagInputForTrack(track, album, "image/png", onePixelPNG()), recorder.lastTags)
}

func TestAudioArtifactM4aTaggerCoverHandling(t *testing.T) {
	track := testArtifactTrack()
	album := *firstAlbum(track)
	client := NewClient(utils.NewHttpClient())
	path := "/tmp/track.m4a"

	t.Run("png cover", func(t *testing.T) {
		recorder := &recordingM4ATagger{}
		client.m4aTagger = recorder
		coverPath := filepath.Join(t.TempDir(), "cover.png")
		require.NoError(t, os.WriteFile(coverPath, onePixelPNG(), 0644))

		tagger := m4aArtifactTagger{client: client}
		require.NoError(t, tagger.Write(path, artifactMetadata{Track: track, CoverPath: coverPath}))

		assert.Equal(t, "image/png", recorder.lastTags.CoverMIME)
		assert.Equal(t, onePixelPNG(), recorder.lastTags.CoverData)
	})

	t.Run("jpeg cover", func(t *testing.T) {
		recorder := &recordingM4ATagger{}
		client.m4aTagger = recorder
		coverPath := filepath.Join(t.TempDir(), "cover.jpg")
		require.NoError(t, os.WriteFile(coverPath, onePixelJPEG(), 0644))

		tagger := m4aArtifactTagger{client: client}
		require.NoError(t, tagger.Write(path, artifactMetadata{Track: track, CoverPath: coverPath}))

		assert.Equal(t, "image/jpeg", recorder.lastTags.CoverMIME)
		assert.Equal(t, onePixelJPEG(), recorder.lastTags.CoverData)
	})

	t.Run("missing cover", func(t *testing.T) {
		recorder := &recordingM4ATagger{}
		client.m4aTagger = recorder

		tagger := m4aArtifactTagger{client: client}
		require.NoError(t, tagger.Write(path, artifactMetadata{Track: track, CoverPath: ""}))

		want := m4aTagInputForTrack(track, album, "", nil)
		assert.Equal(t, want, recorder.lastTags)
		assert.Empty(t, recorder.lastTags.CoverMIME)
		assert.Empty(t, recorder.lastTags.CoverData)
	})
}

func TestAudioArtifactMp3Spec(t *testing.T) {
	spec := mp3ArtifactSpec()

	assert.Equal(t, "mp3", spec.Format)
	assert.Equal(t, "download_file", spec.DownloadStage)
	assert.Equal(t, "id3_tags", spec.MetadataStage)
	assert.Equal(t, "id3_tags", spec.CompletionStage)
	assert.Equal(t, metadataRequired, spec.FailurePolicy)
	assert.Equal(t, "ID3 metadata written", spec.MetadataSuccessMsg)
	assert.Empty(t, spec.MetadataSkipMsg)
	require.NotNil(t, spec.Tagger)
	_, ok := spec.Tagger.(mp3ArtifactTagger)
	assert.True(t, ok)
}

func TestAudioArtifactFlacSpec(t *testing.T) {
	spec := flacArtifactSpec()

	assert.Equal(t, "flac", spec.Format)
	assert.Equal(t, "download_file", spec.DownloadStage)
	assert.Equal(t, "flac_tags", spec.MetadataStage)
	assert.Equal(t, "lossless_complete", spec.CompletionStage)
	assert.Equal(t, metadataRequired, spec.FailurePolicy)
	assert.Equal(t, "FLAC metadata written", spec.MetadataSuccessMsg)
	assert.Empty(t, spec.MetadataSkipMsg)
	require.NotNil(t, spec.Tagger)
	_, ok := spec.Tagger.(flacArtifactTagger)
	assert.True(t, ok)
}

func TestAudioArtifactM4aSpec(t *testing.T) {
	client := NewClient(utils.NewHttpClient())
	spec := client.m4aArtifactSpec()

	assert.Equal(t, "m4a", spec.Format)
	assert.Equal(t, "download_file", spec.DownloadStage)
	assert.Equal(t, "m4a_tags", spec.MetadataStage)
	assert.Equal(t, "lossless_complete", spec.CompletionStage)
	assert.Equal(t, metadataBestEffort, spec.FailurePolicy)
	assert.Equal(t, "M4A metadata written", spec.MetadataSuccessMsg)
	assert.Equal(t, "M4A metadata skipped; keeping audio", spec.MetadataSkipMsg)
	require.NotNil(t, spec.Tagger)
	taggers, ok := spec.Tagger.(m4aArtifactTagger)
	require.True(t, ok)
	assert.Same(t, client, taggers.client)
}
