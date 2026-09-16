package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"ya-music/ya/model"

	"charm.land/bubbles/v2/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestTrackListItemRenderKeepsStatusColumnAlignedForTripleDigitIndexes(t *testing.T) {
	items := make([]list.Item, 120)
	for i := range items {
		items[i] = TrackListItem{
			uid: "item",
			track: &model.Track{
				Title:   "Mystic Passage",
				Artists: []model.Artist{{Name: "Margot Reisinger"}},
			},
			status: TrackStatusReady,
		}
	}

	modelList := list.New(items, TrackListItem{}, 80, 20)
	renderer := TrackListItem{}

	var twoDigit bytes.Buffer
	renderer.Render(&twoDigit, modelList, 90, items[90])

	var threeDigit bytes.Buffer
	renderer.Render(&threeDigit, modelList, 100, items[100])

	twoDigitRow := ansi.Strip(twoDigit.String())
	threeDigitRow := ansi.Strip(threeDigit.String())

	assert.Equal(t, strings.Index(twoDigitRow, "Ready"), strings.Index(threeDigitRow, "Ready"))
}

func TestTrackListItemRenderFillsListWidth(t *testing.T) {
	items := []list.Item{
		TrackListItem{
			uid: "item",
			track: &model.Track{
				Title:   "Intazrin",
				Artists: []model.Artist{{Name: "D"}},
			},
			status: TrackStatusReady,
		},
	}
	modelList := list.New(items, TrackListItem{}, 160, 20)
	renderer := TrackListItem{}

	var row bytes.Buffer
	renderer.Render(&row, modelList, 0, items[0])

	assert.Equal(t, 160, ansi.StringWidth(ansi.Strip(row.String())))
}

func TestTrackListItemRenderDownloadedStatusFitsListWidth(t *testing.T) {
	formats := []string{"MP3", "FLAC"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			items := []list.Item{
				TrackListItem{
					uid:    "item",
					track:  &model.Track{Title: "Song"},
					status: TrackStatusDownloaded,
					format: format,
				},
			}
			modelList := list.New(items, TrackListItem{}, 80, 20)
			renderer := TrackListItem{}

			var row bytes.Buffer
			renderer.Render(&row, modelList, 0, items[0])

			assert.Equal(t, 80, ansi.StringWidth(ansi.Strip(row.String())))
		})
	}
}

func TestTrackListItemRenderLongStatusFitsListWidth(t *testing.T) {
	items := []list.Item{
		TrackListItem{
			uid:    "item",
			track:  &model.Track{Title: "Song"},
			status: TrackStatusDownloaded,
			format: "SUPERLONGFORMATNAME",
		},
	}
	modelList := list.New(items, TrackListItem{}, 80, 20)
	renderer := TrackListItem{}

	var row bytes.Buffer
	renderer.Render(&row, modelList, 0, items[0])

	assert.Equal(t, 80, ansi.StringWidth(ansi.Strip(row.String())))
}

func TestTrackListItemDownloadedStatusIncludesFormat(t *testing.T) {
	items := []list.Item{
		TrackListItem{
			uid:    "item",
			track:  &model.Track{Title: "Song"},
			status: TrackStatusDownloaded,
			format: "FLAC",
		},
	}
	modelList := list.New(items, TrackListItem{}, 80, 20)
	renderer := TrackListItem{}

	var row bytes.Buffer
	renderer.Render(&row, modelList, 0, items[0])

	assert.Contains(t, ansi.Strip(row.String()), "✓ FLAC")
}

func TestTrackListItemDownloadedStatusDefaultsToMP3(t *testing.T) {
	item := TrackListItem{status: TrackStatusDownloaded}

	assert.Equal(t, "✓ MP3", item.statusLabel())
}

func TestTrackListItemDownloadingStatusUsesRunningMarker(t *testing.T) {
	items := []list.Item{
		TrackListItem{
			uid:    "item",
			track:  &model.Track{Title: "Song"},
			status: TrackStatusDownloading,
		},
	}
	modelList := list.New(items, TrackListItem{}, 80, 20)
	renderer := TrackListItem{}

	var row bytes.Buffer
	renderer.Render(&row, modelList, 0, items[0])

	plainRow := ansi.Strip(row.String())
	assert.Contains(t, plainRow, "● Downloading...")
	assert.Equal(t, 80, ansi.StringWidth(plainRow))
}

func TestTrackListItemKeepsNumberAndTitleColumnsStableWhenHidingDuplicates(t *testing.T) {
	m := NewDownloadModel(nil)
	m.tracksProgress = make([]*TrackProgress, 108)
	for i := range m.tracksProgress {
		status := TrackStatusReady
		if (i+1)%4 == 0 {
			status = TrackStatusDuplicate
		}
		m.tracksProgress[i] = &TrackProgress{
			uid:    fmt.Sprintf("track-%d", i),
			track:  &model.Track{Title: "Stable Track"},
			status: status,
		}
	}

	m.updateTrackList()
	fullItems := m.trackList.Items()
	var fullRow bytes.Buffer
	TrackListItem{}.Render(&fullRow, m.trackList, 0, fullItems[0])

	_, _ = m.Update(keyText("t"))
	hiddenItems := m.trackList.Items()
	assert.Len(t, hiddenItems, 81)
	var hiddenRow bytes.Buffer
	TrackListItem{}.Render(&hiddenRow, m.trackList, 0, hiddenItems[0])

	fullPlain := ansi.Strip(fullRow.String())
	hiddenPlain := ansi.Strip(hiddenRow.String())

	assert.Contains(t, fullPlain, "01. Stable Track")
	assert.Contains(t, hiddenPlain, "01. Stable Track")
	assert.Equal(t, strings.Index(fullPlain, "Stable Track"), strings.Index(hiddenPlain, "Stable Track"))
	assert.Equal(t, strings.Index(fullPlain, "Ready"), strings.Index(hiddenPlain, "Ready"))
}

func TestFormatTrackNumberAddsLeadingZeroOnlyToSingleDigits(t *testing.T) {
	assert.Equal(t, "01. ", formatTrackNumber(0, 2))
	assert.Equal(t, "10. ", formatTrackNumber(9, 2))
	assert.Equal(t, " 01. ", formatTrackNumber(0, 3))
	assert.Equal(t, " 10. ", formatTrackNumber(9, 3))
	assert.Equal(t, "100. ", formatTrackNumber(99, 3))
}

func TestTrackListItemRenderHandlesWideTitleCharacters(t *testing.T) {
	items := []list.Item{
		TrackListItem{
			uid: "item",
			track: &model.Track{
				Title:   "広いタイトル🙂",
				Artists: []model.Artist{{Name: "演奏者"}},
			},
			status: TrackStatusReady,
		},
	}
	modelList := list.New(items, TrackListItem{}, 80, 20)
	renderer := TrackListItem{}

	var row bytes.Buffer
	renderer.Render(&row, modelList, 0, items[0])

	assert.LessOrEqual(t, ansi.StringWidth(ansi.Strip(row.String())), 80)
}
