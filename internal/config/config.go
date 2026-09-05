package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPath      = "yamdl.yaml"
	DefaultAlbumMask = "{year} - {title}"
)

var placeholderPattern = regexp.MustCompile(`\{[^{}]+\}`)

var allowedPlaceholders = map[string]struct{}{
	"{year}":  {},
	"{title}": {},
}

// Config holds the application settings read from YAML.
type Config struct {
	Layout Layout `yaml:"layout"`
}

// Layout defines how downloaded files are arranged by artist and release.
type Layout struct {
	ByArtist     bool   `yaml:"by_artist"`
	ByAlbum      bool   `yaml:"by_album"`
	AlbumDirMask string `yaml:"album_dir_mask"`
}

func Default() Config {
	return Config{
		Layout: Layout{
			AlbumDirMask: DefaultAlbumMask,
		},
	}
}

// Load reads the YAML config. An empty path means DefaultPath; a missing default file
// is not an error and returns the defaults.
func Load(path string) (Config, error) {
	explicit := strings.TrimSpace(path) != ""
	if !explicit {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	if err := cfg.Layout.normalize(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

func (l *Layout) normalize() error {
	l.AlbumDirMask = strings.TrimSpace(l.AlbumDirMask)
	if l.AlbumDirMask == "" {
		if l.ByAlbum {
			l.AlbumDirMask = "{title}"
		} else {
			l.AlbumDirMask = DefaultAlbumMask
		}
	}
	return l.validateMask()
}

func (l Layout) validateMask() error {
	for _, match := range placeholderPattern.FindAllString(l.AlbumDirMask, -1) {
		if _, ok := allowedPlaceholders[match]; !ok {
			return fmt.Errorf("unknown album_dir_mask placeholder %s", match)
		}
	}
	return nil
}
