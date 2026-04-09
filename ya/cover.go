package ya

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"ya-music/utils"
	"ya-music/ya/model"
)

const defaultCoverSize = "400x400"

type coverDownloadResult struct {
	filename string
	err      error
}

func (c *Client) startCoverDownload(track model.Track, audioFilename string, options DownloadOptions) <-chan coverDownloadResult {
	ch := make(chan coverDownloadResult, 1)
	trackCtx := utils.NewTrackLogContext(track)

	if options.SkipCover {
		c.logTrack(slog.LevelInfo, trackCtx, "cover download skipped",
			"stage", "download_cover",
			"reason", "skip_cover",
		)
		ch <- coverDownloadResult{}
		close(ch)
		return ch
	}

	coverURL := buildCoverURL(track)
	if coverURL == "" {
		c.logTrack(slog.LevelInfo, trackCtx, "cover download skipped",
			"stage", "download_cover",
			"reason", "missing_cover_uri",
		)
		ch <- coverDownloadResult{}
		close(ch)
		return ch
	}

	coverPath := buildCoverFilename(audioFilename)
	go func() {
		defer close(ch)
		c.logTrack(slog.LevelInfo, trackCtx, "cover download started",
			"stage", "download_cover",
			"cover_url", utils.SanitizeURL(coverURL),
			"cover_filename", coverPath,
		)

		if err := os.Remove(coverPath); err != nil && !os.IsNotExist(err) {
			c.logTrack(slog.LevelWarn, trackCtx, "stale cover cleanup ignored",
				"stage", "download_cover",
				"cover_filename", coverPath,
				"error", err,
			)
		}

		err := c.httpClient.DownloadFileWithContext(
			c.requestContext(trackCtx, "download_cover", "download_cover"),
			coverURL,
			coverPath,
		)
		if err != nil {
			ch <- coverDownloadResult{err: err}
			return
		}

		c.logTrack(slog.LevelInfo, trackCtx, "cover download finished",
			"stage", "download_cover",
			"cover_filename", coverPath,
		)
		ch <- coverDownloadResult{filename: coverPath}
	}()

	return ch
}

func buildCoverURL(track model.Track) string {
	uri := strings.TrimSpace(track.CoverURI)
	if uri == "" && len(track.Albums) > 0 {
		uri = strings.TrimSpace(track.Albums[0].CoverURI)
	}
	if uri == "" {
		return ""
	}

	uri = strings.ReplaceAll(uri, "%%", defaultCoverSize)
	switch {
	case strings.HasPrefix(uri, "http://"), strings.HasPrefix(uri, "https://"):
		return uri
	case strings.HasPrefix(uri, "//"):
		return "https:" + uri
	default:
		return "https://" + uri
	}
}

func buildCoverFilename(audioFilename string) string {
	dir := filepath.Dir(audioFilename)
	base := filepath.Base(audioFilename)
	return filepath.Join(dir, fmt.Sprintf("%s.cover", base))
}

func (c *Client) removeCoverFile(trackCtx utils.TrackLogContext, coverPath string) {
	if err := os.Remove(coverPath); err != nil && !os.IsNotExist(err) {
		c.logTrack(slog.LevelWarn, trackCtx, "cover cleanup ignored",
			"stage", "cleanup_cover",
			"cover_filename", coverPath,
			"error", err,
		)
		return
	}

	c.logTrack(slog.LevelInfo, trackCtx, "cover cleanup finished",
		"stage", "cleanup_cover",
		"cover_filename", coverPath,
	)
}
