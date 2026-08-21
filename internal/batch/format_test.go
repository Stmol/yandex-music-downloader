package batch

import "testing"

func TestFormatFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     Container
	}{
		{filename: "track.mp3", want: ContainerMP3},
		{filename: "track.flac", want: ContainerFLAC},
		{filename: "track.m4a", want: ContainerM4A},
		{filename: "track.mp4", want: ContainerM4A},
		{filename: "track", want: ContainerMP3},
		{filename: "", want: ContainerMP3},
	}
	for _, tt := range tests {
		if got := formatFromFilename(tt.filename); got != tt.want {
			t.Fatalf("formatFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}
