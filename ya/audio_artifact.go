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

type artifactMetadata struct {
	Track     model.Track
	CoverPath string
}

type artifactTagger interface {
	Write(path string, metadata artifactMetadata) error
}

type artifactWriteFunc func(path string) error

type metadataFailurePolicy uint8

const (
	metadataRequired metadataFailurePolicy = iota
	metadataBestEffort
)

type artifactSpec struct {
	Format             string
	DownloadStage      string
	MetadataStage      string
	CompletionStage    string
	Tagger             artifactTagger
	FailurePolicy      metadataFailurePolicy
	MetadataSuccessMsg string
	MetadataSkipMsg    string
}

type artifactPublishResult struct {
	Filename      string
	CoverFilename string
}

func (c *Client) publishAudioArtifact(
	track model.Track,
	destination string,
	options DownloadOptions,
	spec artifactSpec,
	writeAudio artifactWriteFunc,
) (artifactPublishResult, error) {
	if spec.Tagger == nil {
		return artifactPublishResult{}, fmt.Errorf("artifact tagger is required")
	}
	if writeAudio == nil {
		return artifactPublishResult{}, fmt.Errorf("artifact writer is required")
	}
	if strings.TrimSpace(destination) == "" {
		return artifactPublishResult{}, fmt.Errorf("empty destination path")
	}
	destDir := filepath.Dir(destination)
	dirInfo, err := os.Stat(destDir)
	if err != nil {
		return artifactPublishResult{}, fmt.Errorf("destination directory unavailable: %w", err)
	}
	if !dirInfo.IsDir() {
		return artifactPublishResult{}, fmt.Errorf("destination parent is not a directory: %s", destDir)
	}

	trackCtx := utils.NewTrackLogContext(track)

	tempFile, err := os.CreateTemp(destDir, "."+filepath.Base(destination)+".artifact-*")
	if err != nil {
		return artifactPublishResult{}, fmt.Errorf("error creating artifact temp file: %w", err)
	}
	tempFilename := tempFile.Name()
	_ = tempFile.Close()

	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempFilename)
		}
	}()

	coverCh := c.startCoverDownload(track, destination, options)
	if err := writeAudio(tempFilename); err != nil {
		c.logTrackFailure(trackCtx, spec.DownloadStage, err,
			"filename", destination,
			"temp_filename", tempFilename,
			"format", spec.Format,
		)
		cover := c.waitCoverDownload(trackCtx, coverCh)
		if cover.filename != "" {
			c.removeCoverFile(trackCtx, cover.filename)
		}
		return artifactPublishResult{}, err
	}

	cover := c.waitCoverDownload(trackCtx, coverCh)
	if cover.filename != "" {
		defer c.removeCoverFile(trackCtx, cover.filename)
	}

	metadata := artifactMetadata{Track: track, CoverPath: cover.filename}
	if err := spec.Tagger.Write(tempFilename, metadata); err != nil {
		if spec.FailurePolicy == metadataRequired {
			c.logTrackFailure(trackCtx, spec.MetadataStage, err,
				"filename", destination,
				"temp_filename", tempFilename,
				"cover_filename", cover.filename,
			)
			return artifactPublishResult{Filename: destination, CoverFilename: cover.filename},
				fmt.Errorf("failed to write %s tags: %w", spec.Format, err)
		}

		c.logTrack(slog.LevelWarn, trackCtx, spec.MetadataSkipMsg,
			"stage", spec.MetadataStage,
			"filename", destination,
			"temp_filename", tempFilename,
			"error", err,
		)
	} else {
		c.logTrack(slog.LevelInfo, trackCtx, spec.MetadataSuccessMsg,
			"stage", spec.MetadataStage,
			"filename", destination,
			"cover_filename", cover.filename,
		)
	}

	if err := os.Rename(tempFilename, destination); err != nil {
		c.logTrackFailure(trackCtx, spec.DownloadStage, err,
			"filename", destination,
			"temp_filename", tempFilename,
			"format", spec.Format,
		)
		return artifactPublishResult{}, fmt.Errorf("failed to publish %s file: %w", spec.Format, err)
	}
	cleanupTemp = false

	c.logTrack(slog.LevelInfo, trackCtx, "success",
		"stage", spec.CompletionStage,
		"filename", destination,
		"cover_filename", cover.filename,
	)
	return artifactPublishResult{Filename: destination, CoverFilename: cover.filename}, nil
}
