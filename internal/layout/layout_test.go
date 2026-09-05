package layout

import (
	"path/filepath"
	"testing"
	"ya-music/internal/config"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
)

func TestTrackOutputDirKeepsRootWhenLayoutDisabled(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{})
	assert.Equal(t, "downloads", got)
}

func TestTrackOutputDirGroupsByArtistOnly(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{ByArtist: true},
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA"), got)
}

func TestTrackOutputDirGroupsByAlbumOnly(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{
			ByAlbum:      true,
			AlbumDirMask: "{year} - {title}",
		},
	})
	assert.Equal(t, filepath.Join("downloads", "1975 - Arrival"), got)
}

func TestTrackOutputDirGroupsByArtistAndAlbum(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{
			ByArtist:     true,
			ByAlbum:      true,
			AlbumDirMask: "{year} - {title}",
		},
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA", "1975 - Arrival"), got)
}

func TestTrackOutputDirPrefersFolderArtistOverFeaturedArtists(t *testing.T) {
	track := sampleTrack()
	track.Artists = []model.Artist{{Name: "ABBA"}, {Name: "Cher"}}
	got := TrackOutputDir("downloads", track, Options{
		Layout:       config.Layout{ByArtist: true},
		FolderArtist: "ABBA",
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA"), got)
}

func TestTrackOutputDirUsesAlbumArtistWhenFolderArtistEmpty(t *testing.T) {
	track := sampleTrack()
	track.Artists = []model.Artist{{Name: "Guest"}}
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{ByArtist: true},
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA"), got)
}

func TestTrackOutputDirOmitsAlbumSegmentWithoutMetadata(t *testing.T) {
	track := model.Track{
		Title:   "Song",
		Artists: []model.Artist{{Name: "ABBA"}},
	}
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{ByArtist: true, ByAlbum: true, AlbumDirMask: "{year} - {title}"},
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA"), got)
}

func TestTrackOutputDirSanitizesPathSegments(t *testing.T) {
	track := model.Track{
		Artists: []model.Artist{{Name: "AC/DC"}},
		Albums: []model.Album{{
			Title: "Back:In/Black",
			Year:  1980,
		}},
	}
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{
			ByArtist:     true,
			ByAlbum:      true,
			AlbumDirMask: "{year} - {title}",
		},
	})
	assert.Equal(t, filepath.Join("downloads", "AC_DC", "1980 - Back_In_Black"), got)
}

func TestApplyAlbumDirMaskDropsMissingYear(t *testing.T) {
	assert.Equal(t, "Arrival", ApplyAlbumDirMask("{year} - {title}", 0, "Arrival"))
	assert.Equal(t, "1975 - Arrival", ApplyAlbumDirMask("{year} - {title}", 1975, "Arrival"))
	assert.Equal(t, "Arrival", ApplyAlbumDirMask("{title}", 1975, "Arrival"))
}

func sampleTrack() model.Track {
	return model.Track{
		Title:   "Mamma Mia",
		Artists: []model.Artist{{Name: "ABBA"}},
		Albums: []model.Album{{
			Title:   "Arrival",
			Year:    1975,
			Artists: []model.Artist{{Name: "ABBA"}},
		}},
	}
}
