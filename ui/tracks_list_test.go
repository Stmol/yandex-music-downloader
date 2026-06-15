package ui

import (
	"bytes"
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

	assert.Contains(t, ansi.Strip(row.String()), "✅ FLAC")
}

func TestTrackListItemDownloadedStatusDefaultsToMP3(t *testing.T) {
	item := TrackListItem{status: TrackStatusDownloaded}

	assert.Equal(t, "✅ MP3", item.statusLabel())
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
