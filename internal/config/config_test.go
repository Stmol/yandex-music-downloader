package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingDefaultFileReturnsDefaults(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, Default(), cfg)
	assert.False(t, cfg.Layout.ByArtist)
	assert.False(t, cfg.Layout.ByAlbum)
	assert.Equal(t, DefaultAlbumMask, cfg.Layout.AlbumDirMask)
}

func TestLoadMissingExplicitFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestLoadParsesLayoutFlagsAndMask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yamdl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
layout:
  by_artist: true
  by_album: true
  album_dir_mask: "{year} — {title}"
`), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.Layout.ByArtist)
	assert.True(t, cfg.Layout.ByAlbum)
	assert.Equal(t, "{year} — {title}", cfg.Layout.AlbumDirMask)
}

func TestLoadEmptyMaskWithByAlbumDefaultsToTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yamdl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
layout:
  by_album: true
  album_dir_mask: ""
`), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "{title}", cfg.Layout.AlbumDirMask)
}

func TestLoadRejectsUnknownPlaceholder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yamdl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
layout:
  album_dir_mask: "{year} - {name}"
`), 0644))

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown album_dir_mask placeholder {name}")
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yamdl.yaml")
	require.NoError(t, os.WriteFile(path, []byte("layout: [broken"), 0644))

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}
