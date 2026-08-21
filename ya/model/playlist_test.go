package model

import "testing"

func TestPlaylistTracksList(t *testing.T) {
	t.Run("nil tracks", func(t *testing.T) {
		var p Playlist
		if got := p.TracksList(); got != nil {
			t.Fatalf("TracksList() = %#v, want nil", got)
		}
	})

	t.Run("empty tracks", func(t *testing.T) {
		p := Playlist{Tracks: []TrackShort{}}
		if got := p.TracksList(); len(got) != 0 {
			t.Fatalf("TracksList() = %#v, want empty slice", got)
		}
	})

	t.Run("multiple tracks", func(t *testing.T) {
		p := Playlist{
			Tracks: []TrackShort{
				{Track: Track{ID: "1", Title: "A"}},
				{Track: Track{ID: "2", Title: "B"}},
			},
		}
		got := p.TracksList()
		if len(got) != 2 || got[0].ID.String() != "1" || got[1].ID.String() != "2" {
			t.Fatalf("TracksList() = %#v", got)
		}
	})
}
