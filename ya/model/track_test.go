package model

import "testing"

func TestTrackDisplayLabel(t *testing.T) {
	tests := []struct {
		name  string
		track Track
		want  string
	}{
		{
			name:  "artists and title",
			track: Track{Title: "Song", Artists: []Artist{{Name: "Artist"}}},
			want:  "Artist — Song",
		},
		{
			name:  "title only",
			track: Track{Title: "Song"},
			want:  "Song",
		},
		{
			name:  "artists only",
			track: Track{Artists: []Artist{{Name: "Artist"}}},
			want:  "Artist",
		},
		{
			name:  "version in title",
			track: Track{Title: "Song", Version: "Remix", Artists: []Artist{{Name: "Artist"}}},
			want:  "Artist — Song Remix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.track.DisplayLabel(); got != tt.want {
				t.Fatalf("DisplayLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
