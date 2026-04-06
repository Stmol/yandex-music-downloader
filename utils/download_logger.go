package utils

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"ya-music/ya/model"
)

const (
	DefaultDownloadLogPath = "dl_logs.txt"
	publicTrackBaseURL     = "https://music.yandex.ru/track"
)

type DownloadLogger struct {
	path   string
	file   *os.File
	logger *slog.Logger
}

type TrackLogContext struct {
	ID    string
	Title string
	URL   string
}

type RequestLogContext struct {
	Track     TrackLogContext
	Stage     string
	Operation string
}

type synchronizedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

var discardDownloadLogger = newDownloadLoggerWithWriter("", io.Discard, nil)

func NewDownloadLogger(path string) (*DownloadLogger, error) {
	if path == "" {
		path = DefaultDownloadLogPath
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return newDownloadLoggerWithWriter(path, file, file), nil
}

func NewDiscardDownloadLogger() *DownloadLogger {
	return discardDownloadLogger
}

func NewDownloadLoggerForWriter(writer io.Writer) *DownloadLogger {
	if writer == nil {
		writer = io.Discard
	}

	return newDownloadLoggerWithWriter("", writer, nil)
}

func newDownloadLoggerWithWriter(path string, writer io.Writer, file *os.File) *DownloadLogger {
	handler := slog.NewTextHandler(&synchronizedWriter{writer: writer}, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return &DownloadLogger{
		path:   path,
		file:   file,
		logger: slog.New(handler),
	}
}

func NewTrackLogContext(track model.Track) TrackLogContext {
	title := strings.TrimSpace(track.FullTitle())
	artists := strings.TrimSpace(track.ArtistsString())
	if artists != "" {
		title = strings.TrimSpace(title + " - " + artists)
	}

	return TrackLogContext{
		ID:    track.ID.String(),
		Title: title,
		URL:   BuildTrackURL(track.ID.String()),
	}
}

func BuildTrackURL(trackID string) string {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return ""
	}

	return publicTrackBaseURL + "/" + trackID
}

func (l *DownloadLogger) Path() string {
	if l == nil {
		return ""
	}

	return l.path
}

func (l *DownloadLogger) Reset() error {
	if l == nil || l.file == nil {
		return nil
	}

	if err := l.file.Truncate(0); err != nil {
		return err
	}

	_, err := l.file.Seek(0, 0)
	return err
}

func (l *DownloadLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	return l.file.Close()
}

func (l *DownloadLogger) Info(msg string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, args...)
}

func (l *DownloadLogger) Error(msg string, args ...any) {
	l.log(context.Background(), slog.LevelError, msg, args...)
}

func (l *DownloadLogger) LogTrack(level slog.Level, track TrackLogContext, msg string, args ...any) {
	attrs := append(track.Attrs(), args...)
	l.log(context.Background(), level, msg, attrs...)
}

func (l *DownloadLogger) LogRequest(level slog.Level, reqCtx RequestLogContext, msg string, args ...any) {
	attrs := append(reqCtx.Attrs(), args...)
	l.log(context.Background(), level, msg, attrs...)
}

func (l *DownloadLogger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}

	l.logger.Log(ctx, level, msg, args...)
}

func (t TrackLogContext) Attrs() []any {
	attrs := make([]any, 0, 6)
	if t.ID != "" {
		attrs = append(attrs, "track_id", t.ID)
	}
	if t.Title != "" {
		attrs = append(attrs, "track_title", t.Title)
	}
	if t.URL != "" {
		attrs = append(attrs, "track_url", t.URL)
	}

	return attrs
}

func (r RequestLogContext) Attrs() []any {
	attrs := make([]any, 0, 10)
	if r.Stage != "" {
		attrs = append(attrs, "stage", r.Stage)
	}
	if r.Operation != "" {
		attrs = append(attrs, "operation", r.Operation)
	}

	attrs = append(attrs, r.Track.Attrs()...)
	return attrs
}

func SanitizeHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}

	sanitized := make(map[string]string, len(headers))
	for key, values := range headers {
		if strings.EqualFold(key, "Authorization") {
			sanitized[key] = "***"
			continue
		}

		sanitized[key] = strings.Join(values, ", ")
	}

	return sanitized
}

func SanitizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}

	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "" {
		return parsed.Scheme + "://" + parsed.Host
	}

	segments := strings.Split(path, "/")
	redactedPath := "/" + segments[0]
	if len(segments) > 1 {
		redactedPath += "/<redacted>"
	}

	return parsed.Scheme + "://" + parsed.Host + redactedPath
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.writer.Write(p)
}
