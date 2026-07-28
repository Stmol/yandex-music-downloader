package ya

import (
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tommyo123/mtag"
)

func TestM4ATagInputForTrack(t *testing.T) {
	cover := onePixelPNG()

	tests := []struct {
		name      string
		track     model.Track
		album     model.Album
		coverMIME string
		cover     []byte
		want      m4aTagInput
	}{
		{
			name: "full album",
			track: model.Track{
				ID:      model.FlexibleID("123"),
				Title:   "Song",
				Version: "Live",
				Artists: []model.Artist{{Name: "Artist A"}, {Name: "Artist B"}},
				Albums: []model.Album{{
					ID:    model.FlexibleID("456"),
					Title: "Album",
					Year:  2025,
				}},
			},
			album: model.Album{
				ID:         model.FlexibleID("456"),
				Title:      "Album",
				Genre:      "indie",
				Year:       2025,
				TrackCount: 11,
				TrackPosition: model.TrackPosition{
					Volume: 2,
					Index:  3,
				},
				Volumes: [][]model.Track{{}, {}, {}, {}},
			},
			coverMIME: "image/png",
			cover:     cover,
			want: m4aTagInput{
				Title:       "Song Live",
				Artist:      "Artist A, Artist B",
				Album:       "Album",
				AlbumArtist: "Artist A, Artist B",
				Genre:       "indie",
				Year:        2025,
				Track:       3,
				TrackTotal:  11,
				Disc:        2,
				DiscTotal:   4,
				SourceURL:   "https://music.yandex.ru/album/456/track/123",
				CoverMIME:   "image/png",
				CoverData:   cover,
			},
		},
		{
			name: "compilation missing album",
			track: model.Track{
				ID:      model.FlexibleID("99"),
				Title:   "Standalone",
				Artists: []model.Artist{{Name: "Solo"}},
			},
			want: m4aTagInput{
				Title:     "Standalone",
				Artist:    "Solo",
				SourceURL: "https://music.yandex.ru/track/99",
			},
		},
		{
			name: "missing year",
			track: model.Track{
				ID:      model.FlexibleID("7"),
				Title:   "No Year",
				Artists: []model.Artist{{Name: "Artist"}},
				Albums: []model.Album{{
					ID:    model.FlexibleID("8"),
					Title: "Album",
				}},
			},
			album: model.Album{
				ID:    model.FlexibleID("8"),
				Title: "Album",
			},
			want: m4aTagInput{
				Title:       "No Year",
				Artist:      "Artist",
				Album:       "Album",
				AlbumArtist: "Artist",
				SourceURL:   "https://music.yandex.ru/album/8/track/7",
			},
		},
		{
			name: "missing track position",
			track: model.Track{
				ID:      model.FlexibleID("5"),
				Title:   "No Position",
				Artists: []model.Artist{{Name: "Artist"}},
				Albums: []model.Album{{
					ID:    model.FlexibleID("6"),
					Title: "Album",
					Year:  2020,
				}},
			},
			album: model.Album{
				ID:    model.FlexibleID("6"),
				Title: "Album",
				Year:  2020,
			},
			want: m4aTagInput{
				Title:       "No Position",
				Artist:      "Artist",
				Album:       "Album",
				AlbumArtist: "Artist",
				Year:        2020,
				SourceURL:   "https://music.yandex.ru/album/6/track/5",
			},
		},
		{
			name: "empty cover",
			track: model.Track{
				ID:      model.FlexibleID("1"),
				Title:   "Song",
				Artists: []model.Artist{{Name: "Artist"}},
			},
			coverMIME: "image/png",
			cover:     nil,
			want: m4aTagInput{
				Title:     "Song",
				Artist:    "Artist",
				SourceURL: "https://music.yandex.ru/track/1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m4aTagInputForTrack(tt.track, tt.album, tt.coverMIME, tt.cover)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteM4ATagsWritesFieldsAndCover(t *testing.T) {
	path := copyFixture(t, "taggable-stco.m4a")
	cover := onePixelPNG()
	tags := sampleM4ATagInput(cover)

	require.NoError(t, writeM4ATags(path, tags))

	file := openTaggedM4A(t, path)
	assert.Equal(t, mtag.ContainerMP4, file.Container())
	assert.Equal(t, tags.Title, file.Title())
	assert.Equal(t, tags.Artist, file.Artist())
	assert.Equal(t, tags.Album, file.Album())
	assert.Equal(t, tags.AlbumArtist, file.AlbumArtist())
	assert.Equal(t, tags.Genre, file.Genre())
	assert.Equal(t, tags.Year, file.Year())
	assert.Equal(t, tags.Track, file.Track())
	assert.Equal(t, tags.TrackTotal, file.TrackTotal())
	assert.Equal(t, tags.Disc, file.Disc())
	assert.Equal(t, tags.DiscTotal, file.DiscTotal())
	assert.Equal(t, tags.SourceURL, file.CustomValue(m4aSourceURLField))

	images := file.Images()
	require.Len(t, images, 1)
	assert.Equal(t, tags.CoverMIME, images[0].MIME)
	assert.Equal(t, tags.CoverData, images[0].Data)
}

func TestWriteM4ATagsCreatesMissingMetadataTree(t *testing.T) {
	path := copyFixture(t, "taggable-no-ilst-stco.m4a")
	beforeMdat := mdatPayload(t, path)
	tags := sampleM4ATagInput(onePixelPNG())

	require.False(t, hasMP4Atom(t, path, "ilst"))
	require.NoError(t, writeM4ATags(path, tags))

	assertM4ATags(t, path, tags)
	assertM4AMetadataTree(t, path)
	assert.Equal(t, beforeMdat, mdatPayload(t, path))
}

func TestWriteM4ATagsCreatesMissingMetadataTreeWithCO64(t *testing.T) {
	path := copyFixture(t, "taggable-no-ilst-co64.m4a")
	beforeMdat := mdatPayload(t, path)
	tags := sampleM4ATagInput(onePixelPNG())

	require.False(t, hasMP4Atom(t, path, "ilst"))
	require.True(t, hasMP4Atom(t, path, "co64"))
	require.NoError(t, writeM4ATags(path, tags))

	assertM4ATags(t, path, tags)
	assertM4AMetadataTree(t, path)
	assert.True(t, hasMP4Atom(t, path, "co64"))
	assert.Equal(t, beforeMdat, mdatPayload(t, path))

	file := openTaggedM4A(t, path)
	audio := file.AudioProperties()
	assert.NotEmpty(t, audio.Codec)
	assert.Positive(t, audio.SampleRate)
	assert.Positive(t, audio.Channels)
}

func TestWriteM4ATagsPatchesOffsetsWhenMoovPrecedesMdat(t *testing.T) {
	path := copyFixture(t, "taggable-no-ilst-stco-moov-before-mdat.m4a")
	beforeMdat := mdatPayload(t, path)
	beforeOffsets := readMP4ChunkOffsetLayout(t, path)
	tags := sampleM4ATagInput(nil)

	require.NoError(t, writeM4ATags(path, tags))
	assertM4ATags(t, path, tags)
	assert.Equal(t, beforeMdat, mdatPayload(t, path))
	assertMP4ChunkOffsetsRemainRelative(t, beforeOffsets, readMP4ChunkOffsetLayout(t, path))
}

func TestWriteM4ATagsPatchesCO64OffsetsWhenMoovPrecedesMdat(t *testing.T) {
	path := copyFixture(t, "taggable-no-ilst-co64-moov-before-mdat.m4a")
	beforeMdat := mdatPayload(t, path)
	beforeOffsets := readMP4ChunkOffsetLayout(t, path)
	tags := sampleM4ATagInput(nil)

	require.Len(t, beforeOffsets.offsets, 1)
	assert.Equal(t, "co64", beforeOffsets.offsets[0].atomType)
	require.NoError(t, writeM4ATags(path, tags))
	assertM4ATags(t, path, tags)
	assert.Equal(t, beforeMdat, mdatPayload(t, path))
	assertMP4ChunkOffsetsRemainRelative(t, beforeOffsets, readMP4ChunkOffsetLayout(t, path))
}

func TestWriteM4ATagsPreservesExistingCoverArt(t *testing.T) {
	path := copyFixture(t, "taggable-stco.m4a")
	existingCover := onePixelJPEG()
	ourCover := onePixelPNG()

	seed, err := mtag.Open(path)
	require.NoError(t, err)
	seed.AddImage(mtag.Picture{
		MIME: "image/jpeg",
		Type: mtag.PictureCoverFront,
		Data: existingCover,
	})
	require.NoError(t, seed.Save())
	require.NoError(t, seed.Close())

	require.NoError(t, writeM4ATags(path, sampleM4ATagInput(ourCover)))

	file := openTaggedM4A(t, path)
	images := file.Images()
	require.Len(t, images, 2)

	gotData := make([][]byte, 0, len(images))
	gotMIME := make([]string, 0, len(images))
	for _, img := range images {
		gotData = append(gotData, img.Data)
		gotMIME = append(gotMIME, img.MIME)
	}
	assert.Contains(t, gotData, existingCover)
	assert.Contains(t, gotData, ourCover)
	assert.Contains(t, gotMIME, "image/jpeg")
	assert.Contains(t, gotMIME, "image/png")
}

func TestWriteM4ATagsPreservesCO64(t *testing.T) {
	path := copyFixture(t, "taggable-co64.m4a")
	require.True(t, hasMP4Atom(t, path, "co64"))

	tags := sampleM4ATagInput(nil)
	require.NoError(t, writeM4ATags(path, tags))
	require.True(t, hasMP4Atom(t, path, "co64"))

	file := openTaggedM4A(t, path)
	assert.Equal(t, tags.Title, file.Title())
	assert.Equal(t, tags.Artist, file.Artist())
	assert.Equal(t, tags.SourceURL, file.CustomValue(m4aSourceURLField))

	audio := file.AudioProperties()
	assert.NotEmpty(t, audio.Codec)
	assert.Positive(t, audio.SampleRate)
	assert.Positive(t, audio.Channels)
}

func TestWriteM4ATagsRejectsMalformedInputWithoutMutation(t *testing.T) {
	path := copyFixture(t, "malformed-truncated-track.m4a")
	before := sha256File(t, path)

	err := writeM4ATags(path, sampleM4ATagInput(nil))

	require.Error(t, err)
	assert.Equal(t, before, sha256File(t, path))
}

func TestEnsureM4AMetadataTreeRejectsMalformedInputWithoutMutation(t *testing.T) {
	path := copyFixture(t, "malformed-truncated-track.m4a")
	before := sha256File(t, path)

	err := ensureM4AMetadataTree(path)

	require.Error(t, err)
	assert.Equal(t, before, sha256File(t, path))
}

func TestEnsureM4AMetadataTreeRejectsMixedMdatLayoutWithoutMutation(t *testing.T) {
	path := copyFixture(t, "taggable-no-ilst-stco.m4a")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	data = append(data, mustBuildM4AAtom(t, "mdat", []byte{0})...)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	before := sha256File(t, path)

	err = ensureM4AMetadataTree(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mdat atoms both before and after moov")
	assert.Equal(t, before, sha256File(t, path))
}

func TestBootstrapM4AMetadataTreeAddsMetaAfterExistingUDTAChildren(t *testing.T) {
	chpl := mustBuildM4AAtom(t, "chpl", []byte{1, 2, 3})
	free := mustBuildM4AAtom(t, "free", []byte{4, 5})
	udta := mustBuildM4AAtom(t, "udta", append(chpl, free...))
	trak := mustBuildM4AAtom(t, "trak", []byte{6, 7})

	got, changed, err := bootstrapM4AMetadataTree(append(trak, udta...))
	require.NoError(t, err)
	require.True(t, changed)
	assert.Equal(t, []string{"trak", "udta"}, directM4AAtomTypes(t, got))

	gotUDTA := directM4AChildBody(t, got, "udta")
	assert.Equal(t, []string{"chpl", "free", "meta"}, directM4AAtomTypes(t, gotUDTA))
	assert.Equal(t, chpl, directM4AChildAtom(t, gotUDTA, "chpl"))
	assert.Equal(t, free, directM4AChildAtom(t, gotUDTA, "free"))
}

func TestBootstrapM4AMetadataTreeAddsILSTAfterExistingMetaChildren(t *testing.T) {
	hdlr := mustBuildM4AAtom(t, "hdlr", []byte{0, 0, 0, 0, 0, 0, 0, 0, 'm', 'd', 'i', 'r', 'a', 'p', 'p', 'l', 0, 0, 0, 0, 0, 0, 0, 0, 0})
	freeInMeta := mustBuildM4AAtom(t, "free", []byte{1, 2})
	meta := mustBuildM4AAtom(t, "meta", append([]byte{0, 0, 0, 0}, append(hdlr, freeInMeta...)...))
	chpl := mustBuildM4AAtom(t, "chpl", []byte{3})
	freeAfterMeta := mustBuildM4AAtom(t, "free", []byte{4})
	udta := mustBuildM4AAtom(t, "udta", append(chpl, append(meta, freeAfterMeta...)...))

	got, changed, err := bootstrapM4AMetadataTree(udta)
	require.NoError(t, err)
	require.True(t, changed)

	gotUDTA := directM4AChildBody(t, got, "udta")
	assert.Equal(t, []string{"chpl", "meta", "free"}, directM4AAtomTypes(t, gotUDTA))
	assert.Equal(t, chpl, directM4AChildAtom(t, gotUDTA, "chpl"))
	assert.Equal(t, freeAfterMeta, directM4AChildAtom(t, gotUDTA, "free"))

	gotMeta := directM4AChildBody(t, gotUDTA, "meta")
	require.GreaterOrEqual(t, len(gotMeta), 4)
	assert.Equal(t, []string{"hdlr", "free", "ilst"}, directM4AAtomTypes(t, gotMeta[4:]))
	assert.Equal(t, hdlr, directM4AChildAtom(t, gotMeta[4:], "hdlr"))
	assert.Equal(t, freeInMeta, directM4AChildAtom(t, gotMeta[4:], "free"))
}

func TestBootstrapM4AMetadataTreeNormalizesTerminalAtomBeforeAppendingSibling(t *testing.T) {
	tests := []struct {
		name          string
		moovBody      []byte
		childBody     func(t *testing.T, moovBody []byte) []byte
		expectedTypes []string
		terminalType  string
	}{
		{
			name:          "moov",
			moovBody:      terminalM4AAtom("free", []byte{1}),
			childBody:     func(_ *testing.T, body []byte) []byte { return body },
			expectedTypes: []string{"free", "udta"},
			terminalType:  "free",
		},
		{
			name:     "udta",
			moovBody: mustBuildM4AAtom(t, "udta", terminalM4AAtom("free", []byte{2})),
			childBody: func(t *testing.T, body []byte) []byte {
				return directM4AChildBody(t, body, "udta")
			},
			expectedTypes: []string{"free", "meta"},
			terminalType:  "free",
		},
		{
			name:     "meta",
			moovBody: mustBuildM4AAtom(t, "udta", mustBuildM4AAtom(t, "meta", append([]byte{0, 0, 0, 0}, terminalM4AAtom("free", []byte{3})...))),
			childBody: func(t *testing.T, body []byte) []byte {
				udta := directM4AChildBody(t, body, "udta")
				meta := directM4AChildBody(t, udta, "meta")
				require.GreaterOrEqual(t, len(meta), 4)
				return meta[4:]
			},
			expectedTypes: []string{"free", "ilst"},
			terminalType:  "free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed, err := bootstrapM4AMetadataTree(tt.moovBody)
			require.NoError(t, err)
			require.True(t, changed)

			body := tt.childBody(t, got)
			assert.Equal(t, tt.expectedTypes, directM4AAtomTypes(t, body))
			terminal := directM4AChildAtom(t, body, tt.terminalType)
			require.Len(t, terminal, 9)
			assert.NotZero(t, binary.BigEndian.Uint32(terminal[:4]))
		})
	}
}

func TestWriteM4ATagsRejectsOversizeByConfiguredLimit(t *testing.T) {
	path := copyFixture(t, "taggable-stco.m4a")
	before := sha256File(t, path)

	originalLimit := m4aTaggingFileSizeLimit
	m4aTaggingFileSizeLimit = 8
	t.Cleanup(func() {
		m4aTaggingFileSizeLimit = originalLimit
	})

	err := writeM4ATags(path, sampleM4ATagInput(nil))

	require.Error(t, err)
	assert.ErrorIs(t, err, mtag.ErrFileTooLarge)
	assert.Equal(t, before, sha256File(t, path))
}

func sampleM4ATagInput(cover []byte) m4aTagInput {
	return m4aTagInput{
		Title:       "Song Live",
		Artist:      "Artist A, Artist B",
		Album:       "Album",
		AlbumArtist: "Artist A, Artist B",
		Genre:       "indie",
		Year:        2025,
		Track:       3,
		TrackTotal:  11,
		Disc:        2,
		DiscTotal:   4,
		SourceURL:   "https://music.yandex.ru/album/456/track/123",
		CoverMIME:   "image/png",
		CoverData:   cover,
	}
}

func openTaggedM4A(t *testing.T, path string) *mtag.File {
	t.Helper()

	file, err := mtag.Open(path, mtag.WithReadOnly())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
	})
	return file
}

func assertM4ATags(t *testing.T, path string, tags m4aTagInput) {
	t.Helper()

	file := openTaggedM4A(t, path)
	assert.Equal(t, mtag.ContainerMP4, file.Container())
	assert.Equal(t, tags.Title, file.Title())
	assert.Equal(t, tags.Artist, file.Artist())
	assert.Equal(t, tags.Album, file.Album())
	assert.Equal(t, tags.AlbumArtist, file.AlbumArtist())
	assert.Equal(t, tags.Genre, file.Genre())
	assert.Equal(t, tags.Year, file.Year())
	assert.Equal(t, tags.Track, file.Track())
	assert.Equal(t, tags.TrackTotal, file.TrackTotal())
	assert.Equal(t, tags.Disc, file.Disc())
	assert.Equal(t, tags.DiscTotal, file.DiscTotal())
	assert.Equal(t, tags.SourceURL, file.CustomValue(m4aSourceURLField))

	if len(tags.CoverData) == 0 {
		return
	}
	images := file.Images()
	require.Len(t, images, 1)
	assert.Equal(t, tags.CoverMIME, images[0].MIME)
	assert.Equal(t, tags.CoverData, images[0].Data)
}

func assertM4AMetadataTree(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	moov, ok, err := findDirectMP4Atom(data, "moov")
	require.NoError(t, err)
	require.True(t, ok)
	udta, ok, err := findDirectMP4Atom(data[moov.start+moov.headerSize:moov.end], "udta")
	require.NoError(t, err)
	require.True(t, ok)
	meta, ok, err := findDirectMP4Atom(data[moov.start+moov.headerSize+udta.start+udta.headerSize:moov.start+moov.headerSize+udta.end], "meta")
	require.NoError(t, err)
	require.True(t, ok)
	metaStart := moov.start + moov.headerSize + udta.start + udta.headerSize + meta.start + meta.headerSize
	metaEnd := moov.start + moov.headerSize + udta.start + udta.headerSize + meta.end
	require.GreaterOrEqual(t, metaEnd-metaStart, 4)
	hdlr, ok, err := findDirectMP4Atom(data[metaStart+4:metaEnd], "hdlr")
	require.NoError(t, err)
	require.True(t, ok)
	hdlrStart := metaStart + 4 + hdlr.start + hdlr.headerSize
	require.GreaterOrEqual(t, metaStart+4+hdlr.end-hdlrStart, 16)
	assert.Equal(t, "mdir", string(data[hdlrStart+8:hdlrStart+12]))
	assert.Equal(t, "appl", string(data[hdlrStart+12:hdlrStart+16]))
	_, ok, err = findDirectMP4Atom(data[metaStart+4:metaEnd], "ilst")
	require.NoError(t, err)
	require.True(t, ok)
}

func mdatPayload(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	mdat, ok, err := findDirectMP4Atom(data, "mdat")
	require.NoError(t, err)
	require.True(t, ok)
	return append([]byte(nil), data[mdat.start+mdat.headerSize:mdat.end]...)
}

func findMP4AtomFromFile(path string, target string) (mp4Atom, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mp4Atom{}, false, err
	}
	return findDirectMP4Atom(data, target)
}

func findDirectMP4Atom(data []byte, target string) (mp4Atom, bool, error) {
	for offset := 0; offset < len(data); {
		atom, err := parseMP4Atom(data, offset)
		if err != nil {
			return mp4Atom{}, false, err
		}
		if atom.typ == target {
			return atom, true, nil
		}
		offset = atom.end
	}
	return mp4Atom{}, false, nil
}

func firstMP4ChunkOffset(t *testing.T, path string) uint64 {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	stco, ok, err := findMP4Atom(data, "", "stco")
	require.NoError(t, err)
	if ok {
		payload := data[stco.start+stco.headerSize : stco.end]
		require.GreaterOrEqual(t, len(payload), 12)
		return uint64(binary.BigEndian.Uint32(payload[8:12]))
	}
	co64, ok, err := findMP4Atom(data, "", "co64")
	require.NoError(t, err)
	require.True(t, ok)
	payload := data[co64.start+co64.headerSize : co64.end]
	require.GreaterOrEqual(t, len(payload), 16)
	return binary.BigEndian.Uint64(payload[8:16])
}

type mp4ChunkOffset struct {
	atomType string
	value    uint64
}

type mp4ChunkOffsetLayout struct {
	mdatPayloadStart uint64
	mdatEnd          uint64
	offsets          []mp4ChunkOffset
}

func readMP4ChunkOffsetLayout(t *testing.T, path string) mp4ChunkOffsetLayout {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	mdat, ok, err := findDirectMP4Atom(data, "mdat")
	require.NoError(t, err)
	require.True(t, ok)
	offsets, err := collectMP4ChunkOffsets(data, "")
	require.NoError(t, err)
	require.NotEmpty(t, offsets)
	return mp4ChunkOffsetLayout{
		mdatPayloadStart: uint64(mdat.start + mdat.headerSize),
		mdatEnd:          uint64(mdat.end),
		offsets:          offsets,
	}
}

func assertMP4ChunkOffsetsRemainRelative(t *testing.T, before, after mp4ChunkOffsetLayout) {
	t.Helper()

	require.Len(t, after.offsets, len(before.offsets))
	for i, beforeOffset := range before.offsets {
		afterOffset := after.offsets[i]
		assert.Equal(t, beforeOffset.atomType, afterOffset.atomType)
		assert.Equal(t, beforeOffset.value-before.mdatPayloadStart, afterOffset.value-after.mdatPayloadStart)
		assert.GreaterOrEqual(t, afterOffset.value, after.mdatPayloadStart)
		assert.Less(t, afterOffset.value, after.mdatEnd)
	}
}

func collectMP4ChunkOffsets(data []byte, parentType string) ([]mp4ChunkOffset, error) {
	var offsets []mp4ChunkOffset
	for offset := 0; offset < len(data); {
		atom, err := parseMP4Atom(data, offset)
		if err != nil {
			return nil, err
		}
		payload := data[offset+atom.headerSize : atom.end]
		switch atom.typ {
		case "stco":
			entries, err := parseMP4ChunkOffsetEntries(payload, 4)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				offsets = append(offsets, mp4ChunkOffset{atomType: "stco", value: entry})
			}
		case "co64":
			entries, err := parseMP4ChunkOffsetEntries(payload, 8)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				offsets = append(offsets, mp4ChunkOffset{atomType: "co64", value: entry})
			}
		default:
			if isMP4ContainerAtom(parentType, atom.typ) {
				prefixLen := 0
				if atom.typ == "meta" {
					if len(payload) < 4 {
						return nil, errors.New("meta atom payload too short")
					}
					prefixLen = 4
				}
				nested, err := collectMP4ChunkOffsets(payload[prefixLen:], atom.typ)
				if err != nil {
					return nil, err
				}
				offsets = append(offsets, nested...)
			}
		}
		offset = atom.end
	}
	return offsets, nil
}

func parseMP4ChunkOffsetEntries(payload []byte, width int) ([]uint64, error) {
	if len(payload) < 8 {
		return nil, errors.New("chunk offset atom payload too short")
	}
	count := int(binary.BigEndian.Uint32(payload[4:8]))
	if count > (len(payload)-8)/width || len(payload) != 8+count*width {
		return nil, errors.New("chunk offset atom payload length mismatch")
	}
	entries := make([]uint64, count)
	for i := range entries {
		start := 8 + i*width
		if width == 4 {
			entries[i] = uint64(binary.BigEndian.Uint32(payload[start : start+4]))
		} else {
			entries[i] = binary.BigEndian.Uint64(payload[start : start+8])
		}
	}
	return entries, nil
}

func mustBuildM4AAtom(t *testing.T, typ string, payload []byte) []byte {
	t.Helper()

	atom, err := buildMP4Atom(typ, payload)
	require.NoError(t, err)
	return atom
}

func terminalM4AAtom(typ string, payload []byte) []byte {
	atom := make([]byte, 8+len(payload))
	copy(atom[4:8], typ)
	copy(atom[8:], payload)
	return atom
}

func directM4AAtomTypes(t *testing.T, body []byte) []string {
	t.Helper()

	var types []string
	for offset := 0; offset < len(body); {
		atom, err := parseM4AAtom(body, offset)
		require.NoError(t, err)
		types = append(types, atom.typ)
		offset = atom.end
	}
	return types
}

func directM4AChildBody(t *testing.T, body []byte, typ string) []byte {
	t.Helper()

	atom, ok, err := findMP4ChildAtom(body, typ)
	require.NoError(t, err)
	require.True(t, ok)
	return append([]byte(nil), body[atom.offset+atom.headerSize:atom.end]...)
}

func directM4AChildAtom(t *testing.T, body []byte, typ string) []byte {
	t.Helper()

	atom, ok, err := findMP4ChildAtom(body, typ)
	require.NoError(t, err)
	require.True(t, ok)
	return append([]byte(nil), body[atom.offset:atom.end]...)
}

func hasMP4Atom(t *testing.T, path string, target string) bool {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	_, ok, err := findMP4Atom(data, "", target)
	require.NoError(t, err)
	return ok
}

type mp4Atom struct {
	start      int
	end        int
	size       int
	headerSize int
	typ        string
}

func findMP4Atom(data []byte, parentType string, targetType string) (mp4Atom, bool, error) {
	return findMP4AtomFrom(data, 0, parentType, targetType)
}

func findMP4AtomFrom(data []byte, baseOffset int, parentType string, targetType string) (mp4Atom, bool, error) {
	for offset := 0; offset < len(data); {
		current, err := parseMP4Atom(data, offset)
		if err != nil {
			return mp4Atom{}, false, err
		}

		current.start += baseOffset
		current.end += baseOffset
		if current.typ == targetType {
			return current, true, nil
		}

		if isMP4ContainerAtom(parentType, current.typ) {
			payload := data[offset+current.headerSize : offset+current.size]
			prefixLen := 0
			if current.typ == "meta" {
				if len(payload) < 4 {
					return mp4Atom{}, false, errors.New("meta atom payload too short")
				}
				prefixLen = 4
			}
			found, ok, err := findMP4AtomFrom(payload[prefixLen:], current.start+current.headerSize+prefixLen, current.typ, targetType)
			if err != nil {
				return mp4Atom{}, false, err
			}
			if ok {
				return found, true, nil
			}
		}

		offset += current.size
	}

	return mp4Atom{}, false, nil
}

func parseMP4Atom(data []byte, offset int) (mp4Atom, error) {
	if len(data[offset:]) < 8 {
		return mp4Atom{}, errors.New("truncated atom header")
	}

	size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	typ := string(data[offset+4 : offset+8])
	headerSize := 8

	switch size {
	case 0:
		size = len(data) - offset
	case 1:
		if len(data[offset:]) < 16 {
			return mp4Atom{}, errors.New("truncated 64-bit atom header")
		}
		size64 := binary.BigEndian.Uint64(data[offset+8 : offset+16])
		if size64 > uint64(len(data)-offset) {
			return mp4Atom{}, errors.New("64-bit atom exceeds file bounds")
		}
		size = int(size64)
		headerSize = 16
	}

	if size < headerSize {
		return mp4Atom{}, errors.New("invalid atom size")
	}
	if offset+size > len(data) {
		return mp4Atom{}, errors.New("atom exceeds file bounds")
	}

	return mp4Atom{
		start:      offset,
		end:        offset + size,
		size:       size,
		headerSize: headerSize,
		typ:        typ,
	}, nil
}

func isMP4ContainerAtom(parentType string, typ string) bool {
	if parentType == "ilst" {
		return true
	}

	switch typ {
	case "moov", "trak", "mdia", "minf", "stbl", "udta", "meta", "ilst":
		return true
	default:
		return false
	}
}
