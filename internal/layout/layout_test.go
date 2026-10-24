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

func TestTrackOutputDirGroupsPlaylistFlatIgnoringArtistAndAlbum(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{
			ByArtist:     true,
			ByAlbum:      true,
			ByPlaylist:   true,
			AlbumDirMask: "{year} - {title}",
		},
		FolderArtist:   "ABBA",
		FolderPlaylist: "Favorites",
	})
	assert.Equal(t, filepath.Join("downloads", "Favorites"), got)
}

func TestTrackOutputDirFallsBackWithoutPlaylistName(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout: config.Layout{
			ByArtist:     true,
			ByAlbum:      true,
			ByPlaylist:   true,
			AlbumDirMask: "{year} - {title}",
		},
		FolderArtist: "ABBA",
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA", "1975 - Arrival"), got)
}

func TestTrackOutputDirIgnoresPlaylistNameWhenFlagDisabled(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout:         config.Layout{ByArtist: true},
		FolderArtist:   "ABBA",
		FolderPlaylist: "Favorites",
	})
	assert.Equal(t, filepath.Join("downloads", "ABBA"), got)
}

func TestTrackOutputDirSanitizesPlaylistFolder(t *testing.T) {
	track := sampleTrack()
	got := TrackOutputDir("downloads", track, Options{
		Layout:         config.Layout{ByPlaylist: true},
		FolderPlaylist: "My/Playlist: Mix",
	})
	assert.Equal(t, filepath.Join("downloads", "My_Playlist_ Mix"), got)
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

func TestApplyTrackFileMaskSubstitutesAllPlaceholders(t *testing.T) {
	track := sampleTrack()
	assert.Equal(t, "01 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
	assert.Equal(t, "1 - Mamma Mia", ApplyTrackFileMask("{n} - {title}", track))
	assert.Equal(t, "1975 - Arrival - Mamma Mia", ApplyTrackFileMask("{year} - {album} - {title}", track))
	assert.Equal(t, "ABBA - Mamma Mia", ApplyTrackFileMask("{artist} - {title}", track))
}

func TestApplyTrackFileMaskPadsTrackNumbers(t *testing.T) {
	track := sampleTrack()
	track.Albums[0].TrackPosition.Index = 9
	assert.Equal(t, "09 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
	track.Albums[0].TrackPosition.Index = 12
	assert.Equal(t, "12 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
	track.Albums[0].TrackPosition.Index = 100
	assert.Equal(t, "100 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
}

func TestApplyTrackFileMaskReplacesNNBeforeN(t *testing.T) {
	track := sampleTrack()
	track.Albums[0].TrackPosition.Index = 5
	assert.Equal(t, "05 / 5", ApplyTrackFileMask("{nn} / {n}", track))
}

func TestApplyTrackFileMaskDropsMissingValues(t *testing.T) {
	track := model.Track{Title: "Mamma Mia"}
	assert.Equal(t, "Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
	assert.Equal(t, "Mamma Mia", ApplyTrackFileMask("{n} - {year} - {album} - {title}", track))
	assert.Equal(t, "Mamma Mia", ApplyTrackFileMask("{artist} - {title}", track))
}

func TestApplyTrackFileMaskPrefersPlaylistIndexOverAlbumPosition(t *testing.T) {
	track := sampleTrack()
	track.PlaylistIndex = 3
	assert.Equal(t, "03 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
	assert.Equal(t, "3 - Mamma Mia", ApplyTrackFileMask("{n} - {title}", track))
}

func TestApplyTrackFileMaskUsesPlaylistIndexWithoutAlbum(t *testing.T) {
	track := model.Track{Title: "Mamma Mia", PlaylistIndex: 4}
	assert.Equal(t, "04 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
}

func TestApplyTrackFileMaskUsesAlbumPositionWithoutPlaylistIndex(t *testing.T) {
	track := sampleTrack()
	assert.Equal(t, "01 - Mamma Mia", ApplyTrackFileMask("{nn} - {title}", track))
}

func TestApplyTrackFileMaskUsesFullTitleAndDefaultMask(t *testing.T) {
	track := sampleTrack()
	track.Version = "Live"
	assert.Equal(t, "ABBA - Mamma Mia Live", ApplyTrackFileMask("", track))
}

func TestTrackFilenameBaseSanitizesAndFallsBackToID(t *testing.T) {
	track := sampleTrack()
	track.Title = "Mamma/Mia"
	assert.Equal(t, "01 - Mamma_Mia", TrackFilenameBase(track, "{nn} - {title}"))
	assert.Equal(t, "456", TrackFilenameBase(model.Track{ID: "456"}, "{nn} - {title}"))
}

func sampleTrack() model.Track {
	return model.Track{
		Title:   "Mamma Mia",
		Artists: []model.Artist{{Name: "ABBA"}},
		Albums: []model.Album{{
			Title:         "Arrival",
			Year:          1975,
			Artists:       []model.Artist{{Name: "ABBA"}},
			TrackPosition: model.TrackPosition{Index: 1},
		}},
	}
}
