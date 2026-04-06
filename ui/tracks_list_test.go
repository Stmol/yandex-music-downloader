package ui

import (
	"bytes"
	"strings"
	"testing"
	"ya-music/ya/model"

	"github.com/charmbracelet/bubbles/list"
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
