package ui

import (
	"encoding/json"
	"testing"
	"ya-music/ya/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAddTracks(t *testing.T) {
	m := NewDownloadModel(nil)
	tracks := []model.Track{
		{
			ID:        json.Number(uuid.New().String()),
			Title:     "Track 1",
			Available: true,
		},
		{
			ID:        json.Number(uuid.New().String()),
			Title:     "Track 2",
			Available: false,
		},
	}

	m.AddTracks(tracks)

	assert.Equal(t, 2, len(m.tracksProgress))
	assert.Equal(t, TrackStatusReady, m.tracksProgress[0].status)
	assert.Equal(t, TrackStatusNotAvailable, m.tracksProgress[1].status)
	assert.Equal(t, 2, m.tracksTotalCount)
	assert.Equal(t, 1, m.downloadableCount)
}

func TestCycleFocus(t *testing.T) {
	m := NewDownloadModel(nil)

	assert.Equal(t, viewList, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewDownloadButton, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewQuitButton, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewList, m.focusedView)
}

func TestResetState(t *testing.T) {
	m := NewDownloadModel(nil)
	m.tracksProgress = []*TrackProgress{
		{status: TrackStatusDownloaded},
		{status: TrackStatusError},
		{status: TrackStatusDuplicate},
		{status: TrackStatusNotAvailable},
	}

	m.resetState()

	assert.Equal(t, TrackStatusReady, m.tracksProgress[0].status)
	assert.Equal(t, TrackStatusReady, m.tracksProgress[1].status)
	assert.Equal(t, TrackStatusDuplicate, m.tracksProgress[2].status)
	assert.Equal(t, TrackStatusNotAvailable, m.tracksProgress[3].status)
	assert.Equal(t, 4, m.tracksTotalCount)
	assert.Equal(t, 2, m.downloadableCount)
}

func TestUpdateTrackList(t *testing.T) {
	m := NewDownloadModel(nil)
	m.tracksProgress = []*TrackProgress{
		{status: TrackStatusReady},
		{status: TrackStatusDuplicate},
	}

	m.updateTrackList()
	assert.Equal(t, 2, len(m.trackList.Items()))

	m.hideDuplicates = true
	m.updateTrackList()
	assert.Equal(t, 1, len(m.trackList.Items()))
}

func TestGetTrackInfo(t *testing.T) {
	m := NewDownloadModel(nil)
	track := &model.Track{
		Title: "Test Track",
	}

	uid := uuid.New().String()
	m.tracksProgress = []*TrackProgress{
		{
			uid:      uid,
			track:    track,
			filename: "test.mp3",
		},
	}

	info := m.getTrackInfo(uid)
	assert.Equal(t, "Downloaded: test.mp3", info)

	// Test error message
	m.tracksProgress[0].errMsg = "Download failed"
	info = m.getTrackInfo(uid)
	assert.Equal(t, "Download failed", info)
}

func TestCountStatus(t *testing.T) {
	tracks := []*TrackProgress{
		{status: TrackStatusReady},
		{status: TrackStatusReady},
		{status: TrackStatusError},
		{status: TrackStatusDownloaded},
	}

	assert.Equal(t, 2, countStatus(tracks, TrackStatusReady))
	assert.Equal(t, 1, countStatus(tracks, TrackStatusError))
	assert.Equal(t, 1, countStatus(tracks, TrackStatusDownloaded))
	assert.Equal(t, 0, countStatus(tracks, TrackStatusDuplicate))
}

func TestRenderHeader(t *testing.T) {
	header := renderHeader(5, 10, 8, 2)
	expected := "Total tracks: 10\nTo download: 8\nCompleted: 5\nErrors: 2\n\n"
	assert.Equal(t, expected, header)
}

func TestSortTracksByTitle(t *testing.T) {
	tracks := []*TrackProgress{
		{track: &model.Track{Title: "C"}},
		{track: &model.Track{Title: "A"}},
		{track: &model.Track{Title: "B"}},
	}

	sortTracksByTitle(tracks)

	assert.Equal(t, "A", tracks[0].track.Title)
	assert.Equal(t, "B", tracks[1].track.Title)
	assert.Equal(t, "C", tracks[2].track.Title)
}

func TestFindDuplicates(t *testing.T) {
	id1 := uuid.New()
	tracks := []*TrackProgress{
		{track: &model.Track{ID: json.Number(id1.String()), Title: "Same"}},
		{track: &model.Track{ID: json.Number(id1.String()), Title: "Same"}},
		{track: &model.Track{ID: json.Number(uuid.New().String()), Title: "Same"}},
		{track: &model.Track{ID: json.Number(uuid.New().String()), Title: "Unique"}},
	}

	findDuplicates(tracks)

	assert.Equal(t, TrackStatusReady, tracks[0].status)
	assert.Equal(t, TrackStatusDuplicate, tracks[1].status)
	assert.Equal(t, TrackStatusDuplicate, tracks[2].status)
	assert.Equal(t, TrackStatusReady, tracks[3].status)
}
