package cli

import (
	"bytes"
	"strings"
	"testing"
	"ya-music/ya"
)

func TestParseTUIOptionsRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"download"}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, want 2, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unexpected command; use 'yamdl download --help' for batch downloads") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseTUIOptionsAcceptsTimeoutAndSkipCover(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"--timeout", "30", "--skip-cover"}, &stderr)
	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.timeoutSeconds != 30 || !parsed.options.skipCover {
		t.Fatalf("unexpected options: %#v", parsed.options)
	}
}

func TestParseTUIOptionsRejectsNegativeTimeout(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"--timeout", "-1"}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "timeout must be >= 0 seconds") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseDownloadOptionsRejectsNegativeTimeout(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc123",
		"--link", "https://music.yandex.ru/album/123",
		"--timeout", "-1",
	}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "timeout must be >= 0 seconds") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseDownloadOptionsAcceptsCurrentDownloadFlags(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "  abc123  ",
		"--link", "  https://music.yandex.ru/album/123  ",
		"--format", "FLAC",
		"--output", "./music",
		"--timeout", "180",
		"--skip-cover",
	}, &stderr)

	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	options := parsed.options
	if options.token != "abc123" || options.link != "https://music.yandex.ru/album/123" {
		t.Fatalf("unexpected required options: %#v", options)
	}
	if options.format != ya.AudioFormatFLAC || options.output != "./music" || options.timeoutSeconds != 180 || !options.skipCover {
		t.Fatalf("unexpected parsed options: %#v", options)
	}
}

func TestParseDownloadOptionsRejectsInvalidInput(t *testing.T) {
	tests := [][]string{
		{"--link", "https://music.yandex.ru/album/123"},
		{"--token", "abc123"},
		{"--token", "abc123", "--link", "https://music.yandex.ru/album/123", "--format", "aac"},
		{"--token", "abc123", "--link", "https://music.yandex.ru/album/123", "--timeout", "-1"},
	}

	for _, args := range tests {
		var stderr bytes.Buffer
		parsed := parseDownloadOptions(args, &stderr)
		if parsed.proceed || parsed.exitCode != 2 {
			t.Fatalf("args %q: proceed = %v, exit code = %d, stderr: %s", args, parsed.proceed, parsed.exitCode, stderr.String())
		}
	}
}

func TestParseDownloadOptionsRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc",
		"--link", "https://music.yandex.ru/album/1",
		"extra",
	}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
}
